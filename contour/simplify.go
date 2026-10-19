package contour

import (
	"container/heap"
	"math"

	"github.com/golang/geo/s1"
	"github.com/golang/geo/s2"
	"github.com/paulmach/orb"
)

// earthRadius is the mean Earth radius in metres.
const earthRadius = 6371008.8

// maxEdge is the longest edge simplification may create. S2 draws edges as
// great-circle arcs, which bow away from the straight lon/lat line between
// their ends as they lengthen; capped at a degree, simplified boundaries stay
// close to how they look in lon/lat.
const maxEdge = s1.Angle(math.Pi / 180)

// clearance is the closest a new edge may come to a vertex it doesn't end at.
// Grid rows and columns line many vertices up exactly (on a column, along a
// meridian), so a simplified edge can run straight through another ring's
// vertex. Which side such a vertex falls on is down to rounding when lon/lat
// is converted to points, and engines round differently: golang/geo may call
// it clear while Snowflake reports a crossing. About 6 mm, far above rounding
// and far below any tolerance.
const clearance = s1.Angle(1e-9)

// hashCell is the size, in degrees, of the cells of the vertex index used to
// find vertices near a removal.
const hashCell = 0.05

// SimplifyOptions configures Topology.Simplify. Coordinates must be longitude
// and latitude in degrees.
type SimplifyOptions struct {
	// Tolerance is the farthest, in metres, a simplified boundary may be from
	// any point of the original boundary. Zero skips point removal.
	Tolerance float64
	// MinRingArea, in square metres, drops rings enclosing less: an island
	// together with the matching hole in the band around it (which then
	// covers the island's area), or a ring entirely along the edge of the
	// data, such as a hole left by a few missing values. Kept: rings that run
	// partly along the edge of the data and partly along contour lines, and
	// islands that touch the contour line around them at a point (where a
	// value sits exactly on a level), which border more than one neighbouring
	// polygon rather than filling a single hole. Zero drops nothing.
	MinRingArea float64
}

// Simplify simplifies the topology in place: it drops tiny rings, then
// removes points from arcs within the tolerance. Shared arcs are simplified
// once, so the bands on either side of a boundary stay consistent, and no
// removal may change the topology: arc endpoints are kept, no edge may come to
// cross another or sweep across a vertex, and no ring may collapse or flip.
// The checks are made on the sphere, as S2 engines will judge the result.
func (t *Topology) Simplify(opts SimplifyOptions) {
	if opts.MinRingArea > 0 {
		t.dropTinyRings(opts.MinRingArea)
	}
	if opts.Tolerance > 0 {
		newSimplifier(t, s1.Angle(opts.Tolerance/earthRadius)).run()
	}
}

func s2Point(c orb.Point) s2.Point {
	return s2.PointFromLatLng(s2.LatLngFromDegrees(c[1], c[0]))
}

// ringArea is the area a ring encloses on the sphere, in square metres.
func (t *Topology) ringArea(r Ring) float64 {
	coords := t.RingCoords(r)
	points := make([]s2.Point, len(coords)-1)
	for i, c := range coords[:len(coords)-1] {
		points[i] = s2Point(c)
	}
	loop := s2.LoopFromPoints(points)
	loop.Normalize() // tiny rings: take the smaller side, whichever way they run
	return loop.Area() * earthRadius * earthRadius
}

// ringArcs returns the set of arcs a ring uses.
func ringArcs(r Ring) map[int]bool {
	arcs := make(map[int]bool, len(r))
	for _, ref := range r {
		arcs[ref.Arc] = true
	}
	return arcs
}

// dropTinyRings drops rings smaller than minArea, repeating until none are
// left, since dropping an island can leave the polygon around it hole-free
// and small enough to drop in turn.
func (t *Topology) dropTinyRings(minArea float64) {
	for t.dropTinyRingsOnce(minArea) {
	}
}

// dropTinyRingsOnce decides every drop against the topology as it is, then
// applies them all, so lookups never see a half-edited band.
func (t *Topology) dropTinyRingsOnce(minArea float64) bool {
	type polygonAt struct{ band, polygon int }
	type holeAt struct{ band, polygon, hole int }
	holesByArc := make(map[int]holeAt)
	for k, band := range t.Bands {
		for p, polygon := range band.Polygons {
			for h, hole := range polygon.Holes {
				for _, ref := range hole {
					holesByArc[ref.Arc] = holeAt{k, p, h}
				}
			}
		}
	}
	allEdge := func(r Ring) bool {
		for _, ref := range r {
			if t.Arcs[ref.Arc].Level >= 0 {
				return false
			}
		}
		return true
	}

	dropPolygons := make(map[polygonAt]bool)
	dropHoles := make(map[holeAt]bool)
	for k, band := range t.Bands {
		for p, polygon := range band.Polygons {
			// Holes entirely along the edge of the data: missing values
			// inside the band, which it then covers.
			for h, hole := range polygon.Holes {
				if allEdge(hole) && t.ringArea(hole) < minArea {
					dropHoles[holeAt{k, p, h}] = true
				}
			}
			if len(polygon.Holes) > 0 || t.ringArea(polygon.Shell) >= minArea {
				continue
			}
			// Hole-free polygons: islands of data, dropped alone, or islands
			// bounded by contour lines, matched by a hole of the same arcs
			// in the band around them; dropping both leaves that band
			// covering the island.
			if allEdge(polygon.Shell) {
				dropPolygons[polygonAt{k, p}] = true
				continue
			}
			at, ok := holesByArc[polygon.Shell[0].Arc]
			if ok && at.band != k && sameArcs(polygon.Shell, t.Bands[at.band].Polygons[at.polygon].Holes[at.hole]) {
				dropPolygons[polygonAt{k, p}] = true
				dropHoles[at] = true
			}
		}
	}
	if len(dropPolygons) == 0 && len(dropHoles) == 0 {
		return false
	}

	for k := range t.Bands {
		var polygons []Polygon
		for p, polygon := range t.Bands[k].Polygons {
			if dropPolygons[polygonAt{k, p}] {
				t.dropArcs(polygon.Shell)
				continue
			}
			var holes []Ring
			for h, hole := range polygon.Holes {
				if dropHoles[holeAt{k, p, h}] {
					t.dropArcs(hole)
					continue
				}
				holes = append(holes, hole)
			}
			polygon.Holes = holes
			polygons = append(polygons, polygon)
		}
		t.Bands[k].Polygons = polygons
	}
	return true
}

func (t *Topology) dropArcs(r Ring) {
	for _, ref := range r {
		t.Arcs[ref.Arc].Dropped = true
	}
}

func sameArcs(a, b Ring) bool {
	arcs := ringArcs(a)
	if len(arcs) != len(ringArcs(b)) {
		return false
	}
	for _, ref := range b {
		if !arcs[ref.Arc] {
			return false
		}
	}
	return true
}

// simplifier removes points from every live arc at once, least important
// first, so the checks see the current state of all boundaries.
type simplifier struct {
	t         *Topology
	tolerance s1.Angle
	arcs      []*arcState
	cells     map[[2]int][]vertexRef // index of live vertices
	rings     []ringState
	single    map[[2]s2.Point]int // arcs reduced to one edge, by their ends
	queue     removalQueue
}

type arcState struct {
	arc        int
	original   []s2.Point
	prev, next []int
	alive      []bool
	live       int
	closed     bool
	version    []int     // bumped whenever a point's neighbours change
	rings      []ringUse // the rings using this arc
}

type ringUse struct {
	ring int
	sign float64 // +1 if the ring runs along the arc, -1 if against it
}

type ringState struct {
	area float64 // signed area on the unit sphere, positive for counterclockwise
}

type vertexRef struct{ arc, index int }

type removal struct {
	cost    s1.Angle
	arc     int
	index   int
	version int
}

type removalQueue []removal

func (q removalQueue) Len() int           { return len(q) }
func (q removalQueue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q removalQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *removalQueue) Push(x any)        { *q = append(*q, x.(removal)) }
func (q *removalQueue) Pop() any {
	old := *q
	r := old[len(old)-1]
	*q = old[:len(old)-1]
	return r
}

func newSimplifier(t *Topology, tolerance s1.Angle) *simplifier {
	s := &simplifier{
		t:         t,
		tolerance: tolerance,
		cells:     make(map[[2]int][]vertexRef),
		single:    make(map[[2]s2.Point]int),
	}
	byArc := make(map[int]*arcState)
	for a, arc := range t.Arcs {
		if arc.Dropped {
			continue
		}
		n := len(arc.coords)
		st := &arcState{
			arc:      a,
			original: make([]s2.Point, n),
			prev:     make([]int, n),
			next:     make([]int, n),
			alive:    make([]bool, n),
			live:     n,
			closed:   arc.points[0] == arc.points[n-1],
			version:  make([]int, n),
		}
		for i, c := range arc.coords {
			st.original[i] = s2Point(c)
			st.prev[i], st.next[i], st.alive[i] = i-1, i+1, true
			s.addVertex(len(s.arcs), i, c)
		}
		byArc[a] = st
		s.arcs = append(s.arcs, st)
		if !st.closed && n == 2 {
			s.single[edgeKey(st.original[0], st.original[1])] = len(s.arcs) - 1
		}
	}
	for _, band := range t.Bands {
		for _, polygon := range band.Polygons {
			for _, ring := range polygon.Rings() {
				r := len(s.rings)
				s.rings = append(s.rings, ringState{area: s.signedArea(ring)})
				for _, ref := range ring {
					sign := 1.0
					if ref.Reversed {
						sign = -1
					}
					byArc[ref.Arc].rings = append(byArc[ref.Arc].rings, ringUse{r, sign})
				}
			}
		}
	}
	for a, st := range s.arcs {
		for i := 1; i < len(st.original)-1; i++ {
			s.consider(a, i)
		}
	}
	return s
}

func edgeKey(a, b s2.Point) [2]s2.Point {
	if b.Cmp(a.Vector) < 0 {
		a, b = b, a
	}
	return [2]s2.Point{a, b}
}

// signedArea is a ring's area on the unit sphere as a fan of triangles from
// its first vertex, an arc end that is never removed, so removing a vertex
// changes it by exactly the swept triangle.
func (s *simplifier) signedArea(r Ring) float64 {
	coords := s.t.RingCoords(r)
	origin := s2Point(coords[0])
	var area float64
	for i := 1; i+1 < len(coords)-1; i++ {
		area += s2.SignedArea(origin, s2Point(coords[i]), s2Point(coords[i+1]))
	}
	return area
}

func cellOf(c orb.Point) [2]int {
	return [2]int{int(math.Floor(c[0] / hashCell)), int(math.Floor(c[1] / hashCell))}
}

func (s *simplifier) addVertex(arc, index int, c orb.Point) {
	cell := cellOf(c)
	s.cells[cell] = append(s.cells[cell], vertexRef{arc, index})
}

func (s *simplifier) removeVertex(arc, index int) {
	cell := cellOf(s.t.Arcs[s.arcs[arc].arc].coords[index])
	refs := s.cells[cell]
	for i, r := range refs {
		if r == (vertexRef{arc, index}) {
			refs[i] = refs[len(refs)-1]
			s.cells[cell] = refs[:len(refs)-1]
			return
		}
	}
}

// consider queues vertex i of arc a for removal if it can ever be removed.
func (s *simplifier) consider(a, i int) {
	st := s.arcs[a]
	p, n := st.prev[i], st.next[i]
	if p < 0 || n >= len(st.original) {
		return // an arc end
	}
	if st.closed && st.live <= 4 {
		return // a closed arc keeps three distinct points
	}
	cost := s1.Angle(0)
	for k := p + 1; k < n; k++ {
		cost = max(cost, s2.DistanceFromSegment(st.original[k], st.original[p], st.original[n]))
	}
	if cost > s.tolerance || st.original[p].Distance(st.original[n]) > maxEdge {
		return
	}
	heap.Push(&s.queue, removal{cost: cost, arc: a, index: i, version: st.version[i]})
}

func (s *simplifier) run() {
	// A removal refused because of a nearby vertex may become possible once
	// that vertex is removed, so refused removals are retried after every
	// round that made progress.
	for {
		var refused []removal
		removed := false
		for s.queue.Len() > 0 {
			r := heap.Pop(&s.queue).(removal)
			st := s.arcs[r.arc]
			if !st.alive[r.index] || st.version[r.index] != r.version {
				continue // stale: the vertex is gone or its neighbours changed
			}
			if s.allowed(r.arc, r.index) {
				s.remove(r.arc, r.index)
				removed = true
			} else {
				refused = append(refused, r)
			}
		}
		if !removed || len(refused) == 0 {
			break
		}
		for _, r := range refused {
			if st := s.arcs[r.arc]; st.alive[r.index] && st.version[r.index] == r.version {
				heap.Push(&s.queue, r)
			}
		}
	}
	for _, st := range s.arcs {
		arc := &s.t.Arcs[st.arc]
		var points []pointID
		var coords []orb.Point
		for i := 0; i < len(st.original); i = st.next[i] {
			points = append(points, arc.points[i])
			coords = append(coords, arc.coords[i])
		}
		arc.points, arc.coords = points, coords
	}
}

// allowed reports whether removing vertex i of arc a keeps the topology.
func (s *simplifier) allowed(a, i int) bool {
	st := s.arcs[a]
	p, n := st.prev[i], st.next[i]
	P, V, N := st.original[p], st.original[i], st.original[n]

	// A closed arc keeps three distinct points. (consider checked this when
	// queueing, but other points of the arc may have gone since.)
	if st.closed && st.live <= 4 {
		return false
	}

	// Two arcs between the same ends must not both become that one edge.
	if !st.closed && st.live == 3 {
		if other, ok := s.single[edgeKey(P, N)]; ok && other != a {
			return false
		}
	}

	// No ring may flip or collapse.
	swept := s2.SignedArea(P, V, N)
	for _, use := range st.rings {
		before := s.rings[use.ring].area
		if after := before - use.sign*swept; after == 0 || (after > 0) != (before > 0) {
			return false
		}
	}

	// No other vertex may lie in the triangle the removal sweeps across, or
	// within the clearance of the new edge. If none does, no edge can cross
	// the new one either: an edge entering the triangle can't leave through
	// the two existing edges (that would be a crossing already), so it would
	// have to end inside. Nor can one come within the clearance without a
	// vertex doing so, as the closest points of two edges that don't cross
	// include an end of one of them.
	bounder := s2.NewRectBounder()
	for _, x := range []s2.Point{P, V, N, P} {
		bounder.AddPoint(x)
	}
	bound := bounder.RectBound()
	// Pad the bound by the clearance (wider in longitude away from the equator).
	maxLat := math.Max(math.Abs(bound.Lo().Lat.Radians()), math.Abs(bound.Hi().Lat.Radians()))
	padLat := clearance.Degrees()
	padLng := padLat / math.Cos(maxLat+float64(clearance))
	lo := cellOf(orb.Point{bound.Lo().Lng.Degrees() - padLng, bound.Lo().Lat.Degrees() - padLat})
	hi := cellOf(orb.Point{bound.Hi().Lng.Degrees() + padLng, bound.Hi().Lat.Degrees() + padLat})
	orientation := s2.RobustSign(P, V, N)
	for cx := lo[0]; cx <= hi[0]; cx++ {
		for cy := lo[1]; cy <= hi[1]; cy++ {
			for _, ref := range s.cells[[2]int{cx, cy}] {
				w := s.arcs[ref.arc].original[ref.index]
				if w == P || w == V || w == N {
					continue // these vertices, or another arc's copy of an end
				}
				if s2.DistanceFromSegment(w, P, N) < clearance {
					return false
				}
				if s2.RobustSign(P, V, w) == orientation && s2.RobustSign(V, N, w) == orientation && s2.RobustSign(N, P, w) == orientation {
					return false
				}
			}
		}
	}
	return true
}

func (s *simplifier) remove(a, i int) {
	st := s.arcs[a]
	p, n := st.prev[i], st.next[i]
	swept := s2.SignedArea(st.original[p], st.original[i], st.original[n])
	for _, use := range st.rings {
		s.rings[use.ring].area -= use.sign * swept
	}
	st.alive[i] = false
	st.live--
	st.next[p], st.prev[n] = n, p
	st.version[p]++
	st.version[n]++
	s.removeVertex(a, i)
	if !st.closed && st.live == 2 {
		s.single[edgeKey(st.original[p], st.original[n])] = a
	}
	s.consider(a, p)
	s.consider(a, n)
}
