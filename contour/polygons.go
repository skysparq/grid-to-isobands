package contour

import (
	"fmt"
	"slices"

	"github.com/paulmach/orb"
)

// A Polygon is a shell and the holes directly inside it. An island inside a
// hole is a polygon of its own.
type Polygon struct {
	Shell Ring
	Holes []Ring
}

// Rings returns the shell followed by the holes.
func (p Polygon) Rings() []Ring {
	return append([]Ring{p.Shell}, p.Holes...)
}

// ringShape is a ring's outline in index space, where the topology was built
// and rings are guaranteed not to cross whatever the grid's projection.
type ringShape struct {
	points orb.Ring // closed
	area   float64  // signed, positive for counterclockwise
	bound  orb.Bound
}

func (t *Topology) ringShape(r Ring) ringShape {
	var points orb.Ring
	for _, ref := range r {
		ids := t.refPoints(ref)
		for _, p := range ids[:len(ids)-1] {
			_, _, i, j := t.geom.position(p)
			points = append(points, orb.Point{i, j})
		}
	}
	points = append(points, points[0])
	var area float64
	for k := 0; k+1 < len(points); k++ {
		area += points[k][0]*points[k+1][1] - points[k+1][0]*points[k][1]
	}
	return ringShape{points: points, area: area / 2, bound: points.Bound()}
}

// IsHole reports whether a ring is a hole, i.e. runs clockwise in index space.
func (t *Topology) IsHole(r Ring) bool {
	return t.ringShape(r).area < 0
}

// buildPolygons groups a band's rings into polygons, giving each hole to the
// smallest shell that contains it. The shells containing a hole are nested,
// so the smallest is the one directly around it.
func (t *Topology) buildPolygons(rings []Ring) ([]Polygon, error) {
	shapes := make([]ringShape, len(rings))
	var shells []int
	for r, ring := range rings {
		shapes[r] = t.ringShape(ring)
		if shapes[r].area > 0 {
			shells = append(shells, r)
		}
	}
	bySize := slices.Clone(shells)
	slices.SortStableFunc(bySize, func(a, b int) int {
		switch {
		case shapes[a].area < shapes[b].area:
			return -1
		case shapes[a].area > shapes[b].area:
			return 1
		}
		return 0
	})

	holes := make(map[int][]Ring)
	for r, ring := range rings {
		if shapes[r].area > 0 {
			continue
		}
		// Rings share vertices but never edges, so the middle of one of the
		// hole's edges is never on a shell's boundary.
		a, b := shapes[r].points[0], shapes[r].points[1]
		probe := orb.Point{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2}
		parent := -1
		for _, s := range bySize {
			if shapes[s].bound.Contains(probe) && contains(shapes[s].points, probe) {
				parent = s
				break
			}
		}
		if parent < 0 {
			return nil, fmt.Errorf(`hole at index position %v is inside no shell`, probe)
		}
		holes[parent] = append(holes[parent], ring)
	}

	polygons := make([]Polygon, 0, len(shells))
	for _, s := range shells {
		polygons = append(polygons, Polygon{Shell: rings[s], Holes: holes[s]})
	}
	return polygons, nil
}

// contains reports whether p is inside a closed ring, by counting crossings
// of a ray from p in the +i direction. p must not be on the ring.
func contains(ring orb.Ring, p orb.Point) bool {
	inside := false
	for k := 0; k+1 < len(ring); k++ {
		a, b := ring[k], ring[k+1]
		if (a[1] > p[1]) != (b[1] > p[1]) {
			if x := a[0] + (p[1]-a[1])/(b[1]-a[1])*(b[0]-a[0]); x > p[0] {
				inside = !inside
			}
		}
	}
	return inside
}

// PolygonCoords returns a polygon's coordinates wound by the GeoJSON
// right-hand rule in X/Y: the shell counterclockwise and holes clockwise.
func (t *Topology) PolygonCoords(p Polygon) orb.Polygon {
	coords := orb.Polygon{t.RingCoords(p.Shell)}
	for _, hole := range p.Holes {
		coords = append(coords, t.RingCoords(hole))
	}
	if t.geom.flipped {
		for _, ring := range coords {
			ring.Reverse()
		}
	}
	return coords
}
