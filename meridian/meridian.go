// Package meridian splits lon/lat polygons that cover too much of the sphere
// for S2-based geography engines. Those engines read a polygon larger than a
// hemisphere as its complement: Snowflake loads it without error but stores
// the smaller side, so a near-global band silently becomes everything outside
// it. Splitting at the prime meridian leaves pieces that each span at most
// about 180 degrees of longitude (the antimeridian is already a boundary,
// since grids crossing it are clipped there), so each lies within a
// hemisphere.
//
// The cut follows a meridian, a great circle, so S2 draws its edges exactly as
// they are planned here and they need no densifying while they stay under 180
// degrees, i.e. while polar rows are clipped from the grid.
package meridian

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/paulmach/orb"
)

// MaxSphereFraction is the largest fraction of the sphere a polygon may cover
// before it is split. Snowflake flips at one half; the margin covers the area
// approximation and rounding.
const MaxSphereFraction = 0.45

// SphereFraction estimates the area a polygon covers (shell minus holes) as a
// fraction of the sphere, with the formula used by d3-geo and turf. It is
// accurate for short edges like grid-cell steps.
func SphereFraction(p orb.Polygon) float64 {
	if len(p) == 0 {
		return 0
	}
	fraction := ringFraction(p[0])
	for _, hole := range p[1:] {
		fraction -= ringFraction(hole)
	}
	return fraction
}

func ringFraction(ring orb.Ring) float64 {
	var total float64
	for i := 0; i+1 < len(ring); i++ {
		lon1, lat1 := ring[i][0]*math.Pi/180, ring[i][1]*math.Pi/180
		lon2, lat2 := ring[i+1][0]*math.Pi/180, ring[i+1][1]*math.Pi/180
		total += (lon2 - lon1) * (2 + math.Sin(lat1) + math.Sin(lat2))
	}
	return math.Abs(total) / (8 * math.Pi)
}

// SplitIfOversized returns the polygon unchanged if it covers at most
// MaxSphereFraction of the sphere, and otherwise its pieces from Split.
func SplitIfOversized(p orb.Polygon) ([]orb.Polygon, error) {
	if SphereFraction(p) <= MaxSphereFraction {
		return []orb.Polygon{p}, nil
	}
	return Split(p)
}

// Split cuts a valid polygon, wound by the GeoJSON right-hand rule (CCW shell,
// CW holes; the interior on the left of every ring), into the pieces west and
// east of the prime meridian, wound the same way.
//
// The cut is at 0 degrees unless a vertex lies exactly there (as on a grid
// with a column at 0), in which case it moves east to halfway to the nearest
// vertex. With no vertex on the cut, every ring crosses it cleanly: no
// touching and no edges along it, which keeps the tracing simple.
func Split(p orb.Polygon) ([]orb.Polygon, error) {
	if len(p) == 0 {
		return nil, nil
	}
	if b := p.Bound(); b.Min.Lon() >= 0 || b.Max.Lon() <= 0 {
		return []orb.Polygon{p}, nil // already on one side
	}
	cut := cutLongitude(p)
	west, err := side(p, cut, -1)
	if err != nil {
		return nil, fmt.Errorf(`west of %v: %w`, cut, err)
	}
	east, err := side(p, cut, 1)
	if err != nil {
		return nil, fmt.Errorf(`east of %v: %w`, cut, err)
	}
	return append(west, east...), nil
}

func cutLongitude(p orb.Polygon) float64 {
	onZero, nearest := false, math.Inf(1)
	for _, ring := range p {
		for _, pt := range ring {
			switch {
			case pt[0] == 0:
				onZero = true
			case pt[0] > 0:
				nearest = min(nearest, pt[0])
			}
		}
	}
	if !onZero {
		return 0
	}
	return min(nearest, 1) / 2
}

// A crossing is where a ring crosses the cut, entering or leaving the side
// being traced.
type crossing struct {
	point    orb.Point
	entering bool
	run      int // the run that starts (entering) or ends (leaving) here
}

// side traces the part of p on one side of the cut: dir -1 for west, +1 for
// east.
func side(p orb.Polygon, cut float64, dir float64) ([]orb.Polygon, error) {
	onSide := func(pt orb.Point) bool { return (pt[0]-cut)*dir > 0 }

	var whole []orb.Ring // rings entirely on this side, kept as they are
	var runs [][]orb.Point
	var crossings []crossing
	for _, ring := range p {
		open := ring[:len(ring)-1]
		start := -1
		for i := range open {
			if !onSide(open[i]) && onSide(open[(i+1)%len(open)]) {
				start = (i + 1) % len(open) // just after the ring enters this side
				break
			}
		}
		if start < 0 {
			if onSide(open[0]) {
				whole = append(whole, ring)
			}
			continue
		}
		// Walk the ring from just after a crossing, splitting it into runs on
		// this side, each from an entering crossing to a leaving one.
		var run []orb.Point
		for k := range len(open) {
			a, b := open[(start-1+k+len(open))%len(open)], open[(start+k)%len(open)]
			if onSide(a) != onSide(b) {
				x := crossPoint(a, b, cut)
				if onSide(b) {
					crossings = append(crossings, crossing{point: x, entering: true, run: len(runs)})
					run = []orb.Point{x}
				} else {
					run = append(run, x)
					crossings = append(crossings, crossing{point: x, entering: false, run: len(runs)})
					runs = append(runs, run)
					run = nil
				}
			}
			if onSide(b) {
				run = append(run, b)
			}
		}
	}

	// Along the cut, the polygon's interior lies on the side of a leaving
	// crossing that keeps it on the run's left: north for the west side, south
	// for the east. So from each leaving crossing the boundary follows the cut
	// that way to the next crossing, which must be an entering one.
	slices.SortFunc(crossings, func(a, b crossing) int { return cmp.Compare(-dir*a.point[1], -dir*b.point[1]) })
	next := make(map[int]int) // run index -> the run that follows it
	for i, c := range crossings {
		if c.entering {
			continue
		}
		if i+1 >= len(crossings) || !crossings[i+1].entering {
			return nil, fmt.Errorf(`boundary leaving at %v has nowhere to re-enter along the cut`, c.point)
		}
		next[c.run] = crossings[i+1].run
	}

	var rings []orb.Ring
	used := make([]bool, len(runs))
	for first := range runs {
		if used[first] {
			continue
		}
		var ring orb.Ring
		for r := first; ; {
			if used[r] {
				return nil, errors.New(`runs along the cut don't close into rings`)
			}
			used[r] = true
			ring = append(ring, runs[r]...)
			if r = next[r]; r == first {
				break
			}
		}
		rings = append(rings, splitAtRepeats(ring)...)
	}
	return assemble(append(rings, whole...))
}

// splitAtRepeats closes an open ring traced through the cut and splits it
// into loops that each pass through a point once. A hole touching its shell at
// a vertex is valid, but if the cut joins the hole into the outer boundary,
// that vertex appears twice in one ring, which S2 rejects; split, the loops
// are a shell and a hole sharing the vertex again.
func splitAtRepeats(open []orb.Point) []orb.Ring {
	var loops []orb.Ring
	var stack []orb.Point
	at := make(map[orb.Point]int)
	for _, p := range open {
		if i, ok := at[p]; ok {
			loop := append(orb.Ring(nil), stack[i:]...)
			for _, q := range loop {
				delete(at, q)
			}
			loops = append(loops, append(loop, loop[0]))
			stack = stack[:i]
		}
		at[p] = len(stack)
		stack = append(stack, p)
	}
	return append(loops, append(orb.Ring(stack), stack[0]))
}

// crossPoint is where the edge a-b crosses the meridian at lon. It is always
// interpolated from a to b in ring order, so both sides compute the same point.
func crossPoint(a, b orb.Point, lon float64) orb.Point {
	t := (lon - a[0]) / (b[0] - a[0])
	return orb.Point{lon, a[1] + t*(b[1]-a[1])}
}

// assemble groups rings into polygons: counterclockwise rings are shells and
// each clockwise ring is a hole of the smallest shell containing it.
func assemble(rings []orb.Ring) ([]orb.Polygon, error) {
	var shells, holes []orb.Ring
	for _, ring := range rings {
		if area(ring) > 0 {
			shells = append(shells, ring)
		} else {
			holes = append(holes, ring)
		}
	}
	slices.SortStableFunc(shells, func(a, b orb.Ring) int { return cmp.Compare(area(a), area(b)) })
	polygons := make([]orb.Polygon, len(shells))
	for i, shell := range shells {
		polygons[i] = orb.Polygon{shell}
	}
	for _, hole := range holes {
		// Rings share vertices but never edges, so the middle of one of the
		// hole's edges is never on a shell's boundary.
		probe := orb.Point{(hole[0][0] + hole[1][0]) / 2, (hole[0][1] + hole[1][1]) / 2}
		i := slices.IndexFunc(shells, func(shell orb.Ring) bool { return contains(shell, probe) })
		if i < 0 {
			return nil, fmt.Errorf(`hole at %v is inside no shell`, probe)
		}
		polygons[i] = append(polygons[i], hole)
	}
	return polygons, nil
}

// area is a ring's planar signed area, positive for counterclockwise. It is
// computed relative to the first point, since with absolute coordinates the
// rounding error can exceed a tiny ring's area and flip its sign.
func area(ring orb.Ring) float64 {
	var a float64
	o := ring[0]
	for i := 0; i+1 < len(ring); i++ {
		p, q := ring[i], ring[i+1]
		a += (p[0]-o[0])*(q[1]-o[1]) - (q[0]-o[0])*(p[1]-o[1])
	}
	return a / 2
}

// contains reports whether p is inside a closed ring, by counting crossings
// of a ray from p toward +x. p must not be on the ring.
func contains(ring orb.Ring, p orb.Point) bool {
	inside := false
	for i := 0; i+1 < len(ring); i++ {
		a, b := ring[i], ring[i+1]
		if (a[1] > p[1]) != (b[1] > p[1]) {
			if x := a[0] + (p[1]-a[1])/(b[1]-a[1])*(b[0]-a[0]); x > p[0] {
				inside = !inside
			}
		}
	}
	return inside
}
