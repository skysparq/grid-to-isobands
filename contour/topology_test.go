package contour

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/paulmach/orb"
	"github.com/skysparq/grid-to-isobands/s2check"
)

// TestTopologyInvariants checks random grids, including integer grids where
// many values sit exactly on levels, for the properties the shared-edge design
// relies on.
func TestTopologyInvariants(t *testing.T) {
	for seed := range 300 {
		rng := rand.New(rand.NewPCG(uint64(seed), 0))
		kind := valueKind(seed % 3)
		nanFraction := []float64{0, 0.05, 0.2}[seed/3%3]
		grid := randomGrid(rng, 4+rng.IntN(20), 4+rng.IntN(20), kind, nanFraction)
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
			checkArcReferences(t, topo)
			checkRingsValid(t, topo)
			checkPartition(t, topo, grid)
		})
	}
}

type valueKind int

const (
	continuous valueKind = iota
	// integers sit exactly on levels.
	integers
	// decimals are multiples of 0.1, like MRMS QPE. Cell centres average them
	// to values within rounding error of a level, e.g. (1.1+0.9+1.2+0.8)/4 is
	// 0.9999999999999999.
	decimals
)

// randomGrid makes a slightly curvilinear grid of values in [0, 4] with a
// fraction of them missing.
func randomGrid(rng *rand.Rand, nx, ny int, kind valueKind, nanFraction float64) *Grid {
	g := &Grid{NX: nx, NY: ny}
	for j := range ny {
		for i := range nx {
			g.X = append(g.X, float64(i)+0.1*float64(j))
			g.Y = append(g.Y, float64(j)+0.05*float64(i*i)/float64(nx))
			var z float64
			switch kind {
			case continuous:
				z = rng.Float64() * 4
			case integers:
				z = float64(rng.IntN(5))
			case decimals:
				z = float64(rng.IntN(41)) * 0.1
			}
			if rng.Float64() < nanFraction {
				z = math.NaN()
			}
			g.Z = append(g.Z, z)
		}
	}
	return g
}

// checkArcReferences checks that each contour-line arc is used by exactly the
// bands on either side of its level (forward by the band above, reversed by
// the band below) and each edge-of-data arc by exactly its own band.
func checkArcReferences(t *testing.T, topo *Topology) {
	t.Helper()
	type use struct{ forward, reversed []int }
	uses := make([]use, len(topo.Arcs))
	for k, band := range topo.Bands {
		for _, ring := range band.Rings {
			for _, ref := range ring {
				if ref.Reversed {
					uses[ref.Arc].reversed = append(uses[ref.Arc].reversed, k)
				} else {
					uses[ref.Arc].forward = append(uses[ref.Arc].forward, k)
				}
			}
		}
	}
	for a, arc := range topo.Arcs {
		var wantForward, wantReversed []int
		switch {
		case arc.Level < 0:
			wantForward = []int{arc.Band}
		default:
			if arc.Level < len(topo.Bands) {
				wantForward = []int{arc.Level}
			}
			if arc.Level > 0 {
				wantReversed = []int{arc.Level - 1}
			}
		}
		if fmt.Sprint(uses[a].forward) != fmt.Sprint(wantForward) || fmt.Sprint(uses[a].reversed) != fmt.Sprint(wantReversed) {
			t.Errorf(`arc %d (level %d, band %d): used forward by bands %v and reversed by %v, want %v and %v`,
				a, arc.Level, arc.Band, uses[a].forward, uses[a].reversed, wantForward, wantReversed)
		}
	}
}

// checkRingsValid checks every ring on its own with s2check, oriented as a
// shell. (Validating whole polygons needs holes assigned to shells.)
func checkRingsValid(t *testing.T, topo *Topology) {
	t.Helper()
	for k, band := range topo.Bands {
		for r, ring := range band.Rings {
			coords := topo.RingCoords(ring)
			if topo.IsHole(ring) {
				coords.Reverse()
			}
			if err := s2check.ValidatePolygon([]orb.Ring{coords}); err != nil {
				t.Errorf(`band %d ring %d: %v`, k, r, err)
			}
		}
	}
}

// checkPartition checks that the bands exactly cover the contoured area when
// the levels span every value: their areas (shells minus holes) must add up to
// the area of the triangles that aren't masked by missing data.
func checkPartition(t *testing.T, topo *Topology, grid *Grid) {
	t.Helper()
	var bands float64
	for _, band := range topo.Bands {
		for _, ring := range band.Rings {
			bands += shoelace(topo.RingCoords(ring))
		}
	}
	var domain float64
	g := topo.geom
	for j := range grid.NY - 1 {
		for i := range grid.NX - 1 {
			var kept orb.Ring
			for _, c := range g.corners(i, j) {
				if !math.IsNaN(grid.Z[c]) {
					kept = append(kept, orb.Point{grid.X[c], grid.Y[c]})
				}
			}
			if len(kept) >= 3 {
				domain += shoelace(append(kept, kept[0]))
			}
		}
	}
	if math.Abs(bands-domain) > 1e-9*math.Max(1, domain) {
		t.Errorf(`bands cover area %v, but the contoured area is %v`, bands, domain)
	}
}
