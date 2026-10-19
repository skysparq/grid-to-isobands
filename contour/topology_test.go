package contour

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/golang/geo/s2"
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
			checkPolygonsValid(t, topo)
			checkNoCrossings(t, topo)
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
		for _, polygon := range band.Polygons {
			for _, ref := range slices.Concat(polygon.Rings()...) {
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
		case arc.Dropped:
			// dropped with a tiny ring: used by nothing
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

// checkPolygonsValid checks every polygon, holes and all, with s2check.
func checkPolygonsValid(t *testing.T, topo *Topology) {
	t.Helper()
	for k, band := range topo.Bands {
		for p, polygon := range band.Polygons {
			if err := s2check.ValidatePolygon(topo.PolygonCoords(polygon)); err != nil {
				t.Errorf(`band %d polygon %d: %v`, k, p, err)
			}
		}
	}
}

// checkNoCrossings checks the whole topology at once on the sphere: no edge of
// any arc crosses another, and no two arcs share an edge. s2check validates
// one polygon at a time, so it can't see one band's boundary pushed across
// another's.
func checkNoCrossings(t *testing.T, topo *Topology) {
	t.Helper()
	index := s2.NewShapeIndex()
	type edgeKey struct{ a, b s2.Point }
	edges := make(map[edgeKey]int)
	for a, arc := range topo.Arcs {
		if arc.Dropped {
			continue
		}
		points := make([]s2.Point, len(arc.coords))
		for i, c := range arc.coords {
			points[i] = s2.PointFromLatLng(s2.LatLngFromDegrees(c[1], c[0]))
		}
		for i := 0; i+1 < len(points); i++ {
			k := edgeKey{points[i], points[i+1]}
			if points[i+1].Cmp(points[i].Vector) < 0 {
				k = edgeKey{points[i+1], points[i]}
			}
			if other, ok := edges[k]; ok {
				t.Errorf(`arcs %d and %d share the edge %v - %v`, other, a, arc.coords[i], arc.coords[i+1])
			}
			edges[k] = a
		}
		polyline := s2.Polyline(points)
		index.Add(&polyline)
	}
	query := s2.NewCrossingEdgeQuery(index)
	crossings := 0
	for a, arc := range topo.Arcs {
		if arc.Dropped {
			continue
		}
		for i := 0; i+1 < len(arc.coords); i++ {
			u := s2.PointFromLatLng(s2.LatLngFromDegrees(arc.coords[i][1], arc.coords[i][0]))
			v := s2.PointFromLatLng(s2.LatLngFromDegrees(arc.coords[i+1][1], arc.coords[i+1][0]))
			for _, hits := range query.CrossingsEdgeMap(u, v, s2.CrossingTypeInterior) {
				crossings += len(hits)
			}
		}
		if crossings > 0 {
			t.Errorf(`edges of arc %d cross other edges`, a)
			return
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
		for _, polygon := range band.Polygons {
			for _, ring := range topo.PolygonCoords(polygon) {
				bands += shoelace(ring)
			}
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
