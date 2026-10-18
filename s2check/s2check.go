// Package s2check validates GeoJSON polygons against the loop and polygon rules
// that S2-based geography engines (e.g. Snowflake GEOGRAPHY) enforce on load.
//
// golang/geo's Polygon.Validate isn't used: its self-intersection check is still
// an unported TODO, and it builds the nesting hierarchy from loops normalized to
// their smaller side, which misreads shells larger than a hemisphere. Every rule
// is checked here directly, using the GeoJSON ring roles and orientation.
package s2check

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang/geo/s2"
	"github.com/paulmach/orb"
)

// maxErrors caps how many problems of one kind are reported for a single polygon.
const maxErrors = 10

type vertexRef struct {
	ring, index int
}

// ValidatePolygon reports why a GeoJSON polygon (shell first, then holes, each
// ring closed and wound by the right-hand rule) would be rejected by S2, or nil
// if it would load. Ring and vertex numbers in errors refer to the GeoJSON rings.
func ValidatePolygon(rings []orb.Ring) error {
	if len(rings) == 0 {
		return errors.New(`polygon has no rings`)
	}
	loops, err := toLoopPoints(rings)
	if err != nil {
		return err
	}
	if err = checkVertices(loops); err != nil {
		return err
	}
	if err = checkCrossings(loops); err != nil {
		return err
	}
	regions := regionLoops(loops)
	if err = checkHoles(regions); err != nil {
		return err
	}
	return checkArea(regions)
}

// toLoopPoints converts closed GeoJSON rings to open S2 vertex lists.
func toLoopPoints(rings []orb.Ring) ([][]s2.Point, error) {
	loops := make([][]s2.Point, len(rings))
	for i, ring := range rings {
		if len(ring) < 4 {
			return nil, fmt.Errorf(`ring %d: has %d points, need at least 4`, i, len(ring))
		}
		if ring[0] != ring[len(ring)-1] {
			return nil, fmt.Errorf(`ring %d: is not closed`, i)
		}
		points := make([]s2.Point, len(ring)-1)
		for j, p := range ring[:len(ring)-1] {
			lon, lat := p[0], p[1]
			if math.IsNaN(lon) || math.IsNaN(lat) || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
				return nil, fmt.Errorf(`ring %d: vertex %d (%v, %v) is not a valid lon/lat`, i, j, lon, lat)
			}
			points[j] = s2.PointFromLatLng(s2.LatLngFromDegrees(lat, lon))
		}
		loops[i] = points
	}
	return loops, nil
}

// checkVertices finds zero-length edges, vertices repeated within a ring, and
// rings that share an edge or cross each other at a shared vertex. S2 allows
// different rings to touch at a vertex as long as they don't cross there.
func checkVertices(loops [][]s2.Point) error {
	var errs []error
	seen := make(map[s2.Point]vertexRef)
	for i, loop := range loops {
		n := len(loop)
		for j, v := range loop {
			if len(errs) >= maxErrors {
				return errors.Join(errs...)
			}
			next := loop[(j+1)%n]
			if v == next {
				errs = append(errs, fmt.Errorf(`ring %d: edge %d is degenerate (duplicate vertex)`, i, j))
				continue
			}
			if v == (s2.Point{Vector: next.Mul(-1)}) {
				errs = append(errs, fmt.Errorf(`ring %d: edge %d joins antipodal vertices`, i, j))
				continue
			}
			prev, found := seen[v]
			if !found {
				seen[v] = vertexRef{i, j}
				continue
			}
			if prev.ring == i {
				errs = append(errs, fmt.Errorf(`ring %d: vertex %d repeats vertex %d`, i, j, prev.index))
				continue
			}
			if err := checkSharedVertex(loops, prev, vertexRef{i, j}); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// checkSharedVertex checks two rings that meet at a vertex. They share an edge
// if they have a neighbor in common there, and they cross if their edges
// alternate (a, b, a, b) going around the vertex.
func checkSharedVertex(loops [][]s2.Point, a, b vertexRef) error {
	v := loops[a.ring][a.index]
	aPrev, aNext := neighbors(loops[a.ring], a.index)
	bPrev, bNext := neighbors(loops[b.ring], b.index)
	for _, x := range []s2.Point{bPrev, bNext} {
		if x == aPrev || x == aNext {
			return fmt.Errorf(`ring %d vertex %d and ring %d vertex %d: rings share an edge`, a.ring, a.index, b.ring, b.index)
		}
	}
	if s2.OrderedCCW(aPrev, bPrev, aNext, v) != s2.OrderedCCW(aPrev, bNext, aNext, v) {
		return fmt.Errorf(`ring %d vertex %d and ring %d vertex %d: rings cross at a shared vertex`, a.ring, a.index, b.ring, b.index)
	}
	return nil
}

func neighbors(loop []s2.Point, i int) (prev, next s2.Point) {
	n := len(loop)
	return loop[(i+n-1)%n], loop[(i+1)%n]
}

// checkCrossings finds pairs of edges that cross at a point interior to both,
// within one ring or between rings.
func checkCrossings(loops [][]s2.Point) error {
	shape := s2.LaxPolygonFromPoints(loops)
	index := s2.NewShapeIndex()
	index.Add(shape)
	query := s2.NewCrossingEdgeQuery(index)

	var errs []error
	for e := range shape.NumEdges() {
		edge := shape.Edge(e)
		for _, f := range query.Crossings(edge.V0, edge.V1, shape, s2.CrossingTypeInterior) {
			// Each crossing is found from both edges; report it once.
			if f <= e {
				continue
			}
			pe, pf := shape.ChainPosition(e), shape.ChainPosition(f)
			errs = append(errs, fmt.Errorf(`ring %d edge %d crosses ring %d edge %d`, pe.ChainID, pe.Offset, pf.ChainID, pf.Offset))
			if len(errs) >= maxErrors {
				return errors.Join(errs...)
			}
		}
	}
	return errors.Join(errs...)
}

// regionLoops builds an S2 loop enclosing the region each ring bounds: the
// shell's interior, or the area a hole removes. An S2 loop's interior is on its
// left, so a CCW shell is used as is and each CW hole is inverted. Loops are not
// normalized to their smaller side, which would flip a shell larger than a
// hemisphere, such as a near-global GFS band, to its complement.
func regionLoops(loops [][]s2.Point) []*s2.Loop {
	regions := make([]*s2.Loop, len(loops))
	for i, points := range loops {
		regions[i] = s2.LoopFromPoints(points)
		if i > 0 {
			regions[i].Invert()
		}
	}
	return regions
}

// checkHoles enforces the GeoJSON ring roles: every hole must be inside the
// shell and not inside another hole. A hole wound CCW encloses everything but
// the hole, so it fails the first check. Assumes no crossings.
func checkHoles(regions []*s2.Loop) error {
	var errs []error
	for i := 1; i < len(regions) && len(errs) < maxErrors; i++ {
		if !regions[0].Contains(regions[i]) {
			errs = append(errs, fmt.Errorf(`ring %d: hole is not inside the shell`, i))
		}
		for j := i + 1; j < len(regions) && len(errs) < maxErrors; j++ {
			if regions[i].Contains(regions[j]) || regions[j].Contains(regions[i]) {
				errs = append(errs, fmt.Errorf(`ring %d: hole is nested with hole ring %d`, i, j))
			}
		}
	}
	return errors.Join(errs...)
}

// checkArea rejects polygons larger than a hemisphere. A shell wound clockwise
// reads as everything outside it, but a genuinely huge band does too, and S2
// engines may reject such polygons or silently take the smaller side instead.
func checkArea(regions []*s2.Loop) error {
	area := regions[0].Area()
	for _, hole := range regions[1:] {
		area -= hole.Area()
	}
	if area > 2*math.Pi {
		return fmt.Errorf(`polygon covers %.1f%% of the sphere, more than a hemisphere; check ring orientation, or split polygons this large`, area/(4*math.Pi)*100)
	}
	return nil
}
