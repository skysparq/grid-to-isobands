package contour

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/golang/geo/s2"
	"github.com/paulmach/orb"
)

// TestSimplify simplifies random grids laid out like real ones (0.05 degree
// cells, rows north to south) and checks that the topology survives.
func TestSimplify(t *testing.T) {
	const tolerance = 3000.0  // metres; cells are about 5.5 by 4.3 km here
	const minRingArea = 2.5e7 // square metres; about one cell
	totalBefore, totalAfter := 0, 0
	for seed := range 150 {
		rng := rand.New(rand.NewPCG(uint64(seed), 2))
		kind := valueKind(seed % 3)
		grid := lonLatGrid(rng, 10+rng.IntN(40), 10+rng.IntN(40), kind, []float64{0, 0.03, 0.15}[seed/3%3])
		levels := map[valueKind][]float64{
			continuous: {-0.5, 0.7, 1.3, 2.2, 2.9, 3.6, 4},
			integers:   {-1, 0, 1, 2, 3, 4},
			decimals:   {-0.1, 0.5, 1, 1.5, 2, 3, 4.5},
		}[kind]
		t.Run(fmt.Sprintf(`seed %d`, seed), func(t *testing.T) {
			topo, err := Contour(grid, levels)
			if err != nil {
				t.Fatal(err)
			}

			topo.Simplify(SimplifyOptions{MinRingArea: minRingArea})
			checkTinyRingsDropped(t, topo, minRingArea)
			checkArcReferences(t, topo)
			checkPolygonsValid(t, topo)

			original := make([][]orb.Point, len(topo.Arcs))
			for a := range topo.Arcs {
				original[a] = topo.ArcCoords(a)
				if !topo.Arcs[a].Dropped {
					totalBefore += len(original[a])
				}
			}
			topo.Simplify(SimplifyOptions{Tolerance: tolerance})
			for _, arc := range topo.Arcs {
				if !arc.Dropped {
					totalAfter += len(arc.coords)
				}
			}
			checkArcReferences(t, topo)
			checkPolygonsValid(t, topo)
			checkNoCrossings(t, topo)
			checkWithinTolerance(t, topo, original, tolerance)
		})
	}
	t.Logf(`kept %d of %d arc points (%.0f%%)`, totalAfter, totalBefore, 100*float64(totalAfter)/float64(totalBefore))
	if totalAfter >= totalBefore {
		t.Errorf(`simplification removed nothing`)
	}
}

// lonLatGrid is a 0.05 degree grid near 40N, 100W with rows north to south,
// smoothed so contour lines run for many cells, with a fraction of values
// missing.
func lonLatGrid(rng *rand.Rand, nx, ny int, kind valueKind, nanFraction float64) *Grid {
	raw := make([]float64, nx*ny)
	for i := range raw {
		raw[i] = rng.Float64() * 4
	}
	g := &Grid{NX: nx, NY: ny}
	for j := range ny {
		for i := range nx {
			// Average a 5x5 neighbourhood for smooth, winding contours.
			var sum float64
			var count int
			for dj := -2; dj <= 2; dj++ {
				for di := -2; di <= 2; di++ {
					if ii, jj := i+di, j+dj; ii >= 0 && jj >= 0 && ii < nx && jj < ny {
						sum += raw[jj*nx+ii]
						count++
					}
				}
			}
			z := (sum/float64(count)-2)*3 + 2
			switch kind {
			case integers:
				z = math.Round(z)
			case decimals:
				z = math.Round(z*10) / 10
			}
			if rng.Float64() < nanFraction {
				z = math.NaN()
			}
			g.X = append(g.X, -100+0.05*float64(i))
			g.Y = append(g.Y, 40-0.05*float64(j))
			g.Z = append(g.Z, z)
		}
	}
	return g
}

// checkTinyRingsDropped checks that no droppable ring under minArea is left:
// no hole-free polygon bounded only by the edge of the data, or only by
// contour lines and filling a hole of the same arcs in another band; and no
// hole along the edge of the data.
func checkTinyRingsDropped(t *testing.T, topo *Topology, minArea float64) {
	t.Helper()
	fillsHole := func(k int, shell Ring) bool {
		for k2, band := range topo.Bands {
			for _, polygon := range band.Polygons {
				for _, hole := range polygon.Holes {
					if k2 != k && sameArcs(shell, hole) {
						return true
					}
				}
			}
		}
		return false
	}
	kinds := func(r Ring) (contour, edge bool) {
		for _, ref := range r {
			if topo.Arcs[ref.Arc].Level >= 0 {
				contour = true
			} else {
				edge = true
			}
		}
		return contour, edge
	}
	for k, band := range topo.Bands {
		for p, polygon := range band.Polygons {
			for _, hole := range polygon.Holes {
				if c, _ := kinds(hole); !c && topo.ringArea(hole) < minArea {
					t.Errorf(`band %d polygon %d keeps an edge-of-data hole of %v m²`, k, p, topo.ringArea(hole))
				}
			}
			if len(polygon.Holes) > 0 {
				continue
			}
			if c, e := kinds(polygon.Shell); e && !c || c && !e && fillsHole(k, polygon.Shell) {
				if area := topo.ringArea(polygon.Shell); area < minArea {
					t.Errorf(`band %d polygon %d is a droppable island of %v m²`, k, p, area)
				}
			}
		}
	}
}

// checkWithinTolerance checks that every original point of every arc is within
// the tolerance of the simplified segment that replaced it, and that arc ends
// are kept. Simplified points are a subsequence of the original ones.
func checkWithinTolerance(t *testing.T, topo *Topology, original [][]orb.Point, tolerance float64) {
	t.Helper()
	limit := tolerance / earthRadius * (1 + 1e-9)
	for a, arc := range topo.Arcs {
		if arc.Dropped {
			continue
		}
		kept, orig := arc.coords, original[a]
		if kept[0] != orig[0] || kept[len(kept)-1] != orig[len(orig)-1] {
			t.Errorf(`arc %d: ends moved`, a)
			continue
		}
		at := []int{0} // where each kept point is in the original
		for _, p := range kept[1:] {
			i := slices.Index(orig[at[len(at)-1]+1:], p)
			if i < 0 {
				t.Fatalf(`arc %d: kept point %v is not an original point`, a, p)
			}
			at = append(at, at[len(at)-1]+1+i)
		}
		for m := 0; m+1 < len(kept); m++ {
			for _, p := range orig[at[m]+1 : at[m+1]] {
				d := s2.DistanceFromSegment(s2Point(p), s2Point(kept[m]), s2Point(kept[m+1]))
				if float64(d) > limit {
					t.Errorf(`arc %d: original point %v is %.0f m from the simplified arc`, a, p, float64(d)*earthRadius)
					return
				}
			}
		}
	}
}
