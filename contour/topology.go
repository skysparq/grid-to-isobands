package contour

import (
	"fmt"
	"math"

	"github.com/paulmach/orb"
)

// Topology is the filled contours of a grid as shared arcs and the polygons of
// each band, whose rings are built from references to those arcs.
type Topology struct {
	geom  *geometry
	Arcs  []Arc
	Bands []Band
}

// An Arc is a maximal run of boundary between junctions. An arc along a
// contour line is shared by the bands on either side of it; an arc along the
// grid edge or the edge of missing data belongs to one band. A closed arc
// starts and ends at the same point.
type Arc struct {
	points []pointID
	// Level is the index of the level a contour-line arc follows, with the
	// higher values on its left, or -1 for an arc along the edge of the data.
	Level int
	// Band is the band an edge-of-data arc bounds, on its left, or -1.
	Band int
}

// An ArcRef is one arc's use in a ring, reversed if the ring runs against it.
type ArcRef struct {
	Arc      int
	Reversed bool
}

// A Ring is a closed boundary made of consecutive arcs, with its band on its
// left in index space: counterclockwise for an outer boundary, clockwise for
// a hole.
type Ring []ArcRef

// A Band is the region where lower < z <= upper.
type Band struct {
	Lower, Upper float64
	Polygons     []Polygon
}

// Contour computes the filled contours of grid between each pair of adjacent
// levels, which must be strictly increasing.
func Contour(grid *Grid, levels []float64) (*Topology, error) {
	if err := grid.validate(); err != nil {
		return nil, err
	}
	if err := validateLevels(levels); err != nil {
		return nil, err
	}
	g := newGeometry(grid, levels)
	t := &Topology{geom: g}
	t.buildArcs(g.trace())
	for k := range len(levels) - 1 {
		rings, err := t.buildRings(k)
		if err != nil {
			return nil, fmt.Errorf(`band %d: %w`, k, err)
		}
		polygons, err := t.buildPolygons(rings)
		if err != nil {
			return nil, fmt.Errorf(`band %d: %w`, k, err)
		}
		t.Bands = append(t.Bands, Band{Lower: levels[k], Upper: levels[k+1], Polygons: polygons})
	}
	return t, nil
}

// A graph is one set of segments that chain together: the contour line at one
// level, or the edge-of-data boundary of one band.
type graph struct {
	segs        []segment
	level, band int
}

// pointUse counts how a point is used, to find the junctions where arcs end.
type pointUse struct {
	graph     int
	inDegree  int
	outDegree int
	shared    bool // used by more than one graph
}

// buildArcs splits every graph into arcs at junctions: points where a graph
// branches or ends, or where two graphs meet (a contour line reaching the edge
// of the data). Runs with no junction are closed loops and become closed arcs.
func (t *Topology) buildArcs(s *segments) {
	var graphs []graph
	for m, segs := range s.isolines {
		graphs = append(graphs, graph{segs: segs, level: m, band: -1})
	}
	for k, segs := range s.outer {
		graphs = append(graphs, graph{segs: segs, level: -1, band: k})
	}

	uses := make(map[pointID]*pointUse)
	use := func(p pointID, gi int) *pointUse {
		u, ok := uses[p]
		if !ok {
			u = &pointUse{graph: gi}
			uses[p] = u
		} else if u.graph != gi {
			u.shared = true
		}
		return u
	}
	for gi, gr := range graphs {
		for _, seg := range gr.segs {
			use(seg.from, gi).outDegree++
			use(seg.to, gi).inDegree++
		}
	}
	junction := func(p pointID) bool {
		u := uses[p]
		return u.shared || u.inDegree != 1 || u.outDegree != 1
	}

	for _, gr := range graphs {
		next := make(map[pointID]int, len(gr.segs))
		for i, seg := range gr.segs {
			next[seg.from] = i // only followed from non-junctions, which have one
		}
		visited := make([]bool, len(gr.segs))
		walk := func(start int) {
			points := []pointID{gr.segs[start].from}
			for i := start; ; {
				visited[i] = true
				to := gr.segs[i].to
				points = append(points, to)
				if junction(to) {
					break
				}
				i = next[to]
				if visited[i] {
					break // back at the start of a closed loop
				}
			}
			t.Arcs = append(t.Arcs, Arc{points: points, Level: gr.level, Band: gr.band})
		}
		for i, seg := range gr.segs {
			if !visited[i] && junction(seg.from) {
				walk(i)
			}
		}
		for i := range gr.segs {
			if !visited[i] {
				walk(i)
			}
		}
	}
}

// buildRings assembles band k's rings from the arcs that bound it: the contour
// line at its lower level, the one at its upper level (reversed, since its
// higher side is outside the band), and its edge-of-data arcs.
func (t *Topology) buildRings(k int) ([]Ring, error) {
	var refs []ArcRef
	for a, arc := range t.Arcs {
		switch {
		case arc.Level == k, arc.Band == k:
			refs = append(refs, ArcRef{Arc: a})
		case arc.Level == k+1:
			refs = append(refs, ArcRef{Arc: a, Reversed: true})
		}
	}
	starts := make(map[pointID][]int)
	for r, ref := range refs {
		p := t.refPoints(ref)
		starts[p[0]] = append(starts[p[0]], r)
	}

	used := make([]bool, len(refs))
	var rings []Ring
	for first := range refs {
		if used[first] {
			continue
		}
		var ring Ring
		for r := first; ; {
			used[r] = true
			ring = append(ring, refs[r])
			points := t.refPoints(refs[r])
			next, err := t.nextRef(refs, starts[points[len(points)-1]], points)
			if err != nil {
				return nil, err
			}
			if next == first {
				break
			}
			if used[next] {
				return nil, fmt.Errorf(`arc %d is reached twice while tracing a ring`, refs[next].Arc)
			}
			r = next
		}
		rings = append(rings, t.splitAtRepeats(ring)...)
	}
	return rings, nil
}

// splitAtRepeats splits a ring that passes through a point more than once into
// loops that each pass through it once. Taking the tightest turn at junctions
// separates regions that touch at a point, but a hole touching its own shell
// (or another hole) still comes out as one ring visiting the point twice,
// which S2 rejects; split, it becomes a shell and a hole sharing a vertex,
// which S2 accepts. Repeats only happen at junctions, so the cuts fall
// between arcs.
func (t *Topology) splitAtRepeats(ring Ring) []Ring {
	var loops []Ring
	var stack Ring
	at := make(map[pointID]int)
	for _, ref := range ring {
		p := t.refPoints(ref)[0]
		if i, ok := at[p]; ok {
			loop := append(Ring(nil), stack[i:]...)
			for _, r := range loop {
				delete(at, t.refPoints(r)[0])
			}
			loops = append(loops, loop)
			stack = stack[:i]
		}
		at[p] = len(stack)
		stack = append(stack, ref)
	}
	return append(loops, stack)
}

// nextRef picks which arc a ring continues on after arriving along points.
// Where a band touches itself at a point, several arcs leave it; taking the
// first one clockwise from the way back keeps each ring as tight as possible,
// so regions touching at a point become separate rings, as in contourpy.
func (t *Topology) nextRef(refs []ArcRef, candidates []int, arriving []pointID) (int, error) {
	switch len(candidates) {
	case 0:
		return 0, fmt.Errorf(`ring is open at arc end %v`, arriving[len(arriving)-1])
	case 1:
		return candidates[0], nil
	}
	n := len(arriving)
	back := t.direction(arriving[n-1], arriving[n-2])
	best, bestTurn := -1, math.Inf(1)
	for _, c := range candidates {
		p := t.refPoints(refs[c])
		turn := math.Mod(back-t.direction(p[0], p[1])+4*math.Pi, 2*math.Pi)
		if turn == 0 {
			turn = 2 * math.Pi
		}
		if turn < bestTurn {
			best, bestTurn = c, turn
		}
	}
	return best, nil
}

// direction is the angle of the line from u to v in index space.
func (t *Topology) direction(u, v pointID) float64 {
	_, _, ui, uj := t.geom.position(u)
	_, _, vi, vj := t.geom.position(v)
	return math.Atan2(vj-uj, vi-ui)
}

// refPoints returns an arc's points in the direction the reference uses it.
func (t *Topology) refPoints(ref ArcRef) []pointID {
	points := t.Arcs[ref.Arc].points
	if !ref.Reversed {
		return points
	}
	reversed := make([]pointID, len(points))
	for i, p := range points {
		reversed[len(points)-1-i] = p
	}
	return reversed
}

// ArcCoords returns the coordinates of an arc's points.
func (t *Topology) ArcCoords(arc int) []orb.Point {
	points := t.Arcs[arc].points
	coords := make([]orb.Point, len(points))
	for i, p := range points {
		x, y, _, _ := t.geom.position(p)
		coords[i] = orb.Point{x, y}
	}
	return coords
}

// RingCoords returns a ring's coordinates as a closed ring.
func (t *Topology) RingCoords(r Ring) orb.Ring {
	var coords orb.Ring
	for _, ref := range r {
		points := t.refPoints(ref)
		for _, p := range points[:len(points)-1] {
			x, y, _, _ := t.geom.position(p)
			coords = append(coords, orb.Point{x, y})
		}
	}
	return append(coords, coords[0])
}
