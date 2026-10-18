package contour

import "math"

// A segment is a directed piece of a band boundary with the region it bounds
// on its left (in index space).
type segment struct {
	from, to pointID
}

// segments holds every boundary piece found in the grid.
type segments struct {
	// isolines[m] are the pieces of the contour line at levels[m], with the
	// higher values on their left. The band above uses them as they are and
	// the band below uses them reversed.
	isolines [][]segment
	// outer[k] are the pieces of band k's boundary that run along the grid
	// edge or the edge of missing data, with band k on their left.
	outer [][]segment
}

// trace walks every triangle of the grid and collects its boundary segments.
func (g *geometry) trace() *segments {
	grid := g.grid
	s := &segments{
		isolines: make([][]segment, len(g.levels)),
		outer:    make([][]segment, len(g.levels)-1),
	}
	values := make([]float64, len(grid.Z))
	bins := make([]int, len(grid.Z))
	for n, z := range grid.Z {
		values[n] = g.value(z)
		bins[n] = g.bin(values[n])
	}
	for j := 0; j < grid.NY-1; j++ {
		for i := 0; i < grid.NX-1; i++ {
			cs := g.corners(i, j)
			missing, nanCorner := 0, 0
			for k, c := range cs {
				if math.IsNaN(grid.Z[c]) {
					missing++
					nanCorner = k
				}
			}
			switch missing {
			case 0:
				c := g.centre(i, j)
				zc, _, _, _, _ := g.nodeValues(c)
				bc := g.bin(zc)
				for k := range 4 {
					a, b := cs[k], cs[(k+1)%4]
					s.triangle(g,
						[3]node{a, b, c},
						[3]float64{values[a], values[b], zc},
						[3]int{bins[a], bins[b], bc},
						[3]bool{g.outerSide(i, j, k), false, false})
				}
			case 1:
				// Keep the triangle of the three valid corners; its diagonal
				// borders the missing corner, so it is always outer.
				var t [3]node
				var z [3]float64
				var b [3]int
				for k := range 3 {
					t[k] = cs[(nanCorner+1+k)%4]
					z[k], b[k] = values[t[k]], bins[t[k]]
				}
				s.triangle(g, t, z, b, [3]bool{
					g.outerSide(i, j, (nanCorner+1)%4),
					g.outerSide(i, j, (nanCorner+2)%4),
					true,
				})
			}
		}
	}
	for m := range s.isolines {
		s.isolines[m] = cancelOpposites(s.isolines[m])
	}
	return s
}

// outerSide reports whether side k of cell (i, j) (0 bottom, 1 right, 2 top,
// 3 left, matching corners) is on the edge of the contoured area: the cell
// across it is off the grid or skipped for having two or more missing corners.
func (g *geometry) outerSide(i, j, k int) bool {
	switch k {
	case 0:
		j--
	case 1:
		i++
	case 2:
		j++
	case 3:
		i--
	}
	if i < 0 || j < 0 || i >= g.grid.NX-1 || j >= g.grid.NY-1 {
		return true
	}
	missing := 0
	for _, c := range g.corners(i, j) {
		if math.IsNaN(g.grid.Z[c]) {
			missing++
		}
	}
	return missing >= 2
}

// triangle adds the boundary segments of one counterclockwise triangle. outer[k]
// reports whether its edge from p[k] to p[k+1] is on the edge of the contoured
// area.
func (s *segments) triangle(g *geometry, p [3]node, z [3]float64, bins [3]int, outer [3]bool) {
	lo, hi := min(bins[0], bins[1], bins[2]), max(bins[0], bins[1], bins[2])

	// Each level crossing the triangle cuts it with one straight segment. The
	// segment runs from where the boundary leaves the higher side to where it
	// enters it, which keeps the higher side on its left.
	for m := lo; m < hi; m++ {
		var enter, leave pointID
		for k := range 3 {
			a, b := k, (k+1)%3
			belowA, belowB := bins[a] <= m, bins[b] <= m
			switch {
			case belowA && !belowB:
				enter = g.crossing(p[a], p[b], z[a], z[b], m)
			case !belowA && belowB:
				leave = g.crossing(p[a], p[b], z[a], z[b], m)
			}
		}
		if leave == enter {
			continue
		}
		// When the level runs exactly along an outer edge (both ends exactly
		// at the level), the outer pass emits that edge instead.
		if onOuterEdge(p, outer, leave, enter) {
			continue
		}
		s.isolines[m] = append(s.isolines[m], segment{leave, enter})
	}

	for k := range 3 {
		if !outer[k] {
			continue
		}
		s.outerEdge(g, p, z, bins, k)
	}
}

// outerEdge adds the pieces of the triangle's outer edge from p[k] to p[k+1],
// split where levels cross it, each to the band it bounds.
func (s *segments) outerEdge(g *geometry, p [3]node, z [3]float64, bins [3]int, k int) {
	a, b, c := k, (k+1)%3, (k+2)%3
	points := []pointID{nodePoint(p[a])}
	pieceBins := []int{bins[a]}
	switch {
	case bins[a] < bins[b]:
		for m := bins[a]; m < bins[b]; m++ {
			points = append(points, g.crossing(p[a], p[b], z[a], z[b], m))
			pieceBins = append(pieceBins, m+1)
		}
	case bins[a] > bins[b]:
		for m := bins[a] - 1; m >= bins[b]; m-- {
			points = append(points, g.crossing(p[a], p[b], z[a], z[b], m))
			pieceBins = append(pieceBins, m)
		}
	default:
		// An edge lying exactly on a level borders the band on the
		// triangle's side of it, which the third corner decides.
		if m := bins[a]; z[a] == z[b] && m < len(g.levels) && z[a] == g.levels[m] && bins[c] > m {
			pieceBins[0] = m + 1
		}
	}
	points = append(points, nodePoint(p[b]))

	for t, bin := range pieceBins {
		from, to := points[t], points[t+1]
		band := bin - 1
		if from == to || band < 0 || band >= len(s.outer) {
			continue
		}
		s.outer[band] = append(s.outer[band], segment{from, to})
	}
}

func onOuterEdge(p [3]node, outer [3]bool, u, v pointID) bool {
	if u.b >= 0 || v.b >= 0 {
		return false
	}
	for k := range 3 {
		a, b := p[k], p[(k+1)%3]
		if outer[k] && (u.a == a && v.a == b || u.a == b && v.a == a) {
			return true
		}
	}
	return false
}

// cancelOpposites removes pairs of segments that run along the same triangle
// edge in opposite directions. These come from a level lying exactly along an
// interior edge with higher values on both sides, which is not a boundary.
func cancelOpposites(segs []segment) []segment {
	index := make(map[segment]int, len(segs))
	removed := make([]bool, len(segs))
	for i, s := range segs {
		if j, ok := index[segment{s.to, s.from}]; ok {
			removed[i], removed[j] = true, true
			delete(index, segment{s.to, s.from})
			continue
		}
		index[s] = i
	}
	kept := segs[:0]
	for i, s := range segs {
		if !removed[i] {
			kept = append(kept, s)
		}
	}
	return kept
}
