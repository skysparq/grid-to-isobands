package meridian

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/paulmach/orb"
	"github.com/skysparq/grid-to-isobands/contour"
	"github.com/skysparq/grid-to-isobands/s2check"
)

func TestSplit(t *testing.T) {
	square := orb.Ring{{-3, -3}, {3, -3}, {3, 3}, {-3, 3}, {-3, -3}}
	tests := []struct {
		name       string
		polygon    orb.Polygon
		west, east []int // hole counts of the pieces on each side
	}{
		{`square`, orb.Polygon{square}, []int{0}, []int{0}},
		{`entirely west`, orb.Polygon{{{-5, 0}, {-1, 0}, {-1, 2}, {-5, 2}, {-5, 0}}}, []int{0}, nil},
		{`C opening east`, orb.Polygon{{{-2, -2}, {2, -2}, {2, -1}, {-1, -1}, {-1, 1}, {2, 1}, {2, 2}, {-2, 2}, {-2, -2}}}, []int{0}, []int{0, 0}},
		{`hole crossing the cut`, orb.Polygon{square, {{-1, -1}, {-1, 1}, {1, 1}, {1, -1}, {-1, -1}}}, []int{0}, []int{0}},
		{`hole west of the cut`, orb.Polygon{square, {{-2.5, -1}, {-2.5, 1}, {-1.5, 1}, {-1.5, -1}, {-2.5, -1}}}, []int{1}, []int{0}},
		{`island arm in a hole across the cut`, orb.Polygon{square, {{-2, -2}, {-2, 2}, {2, 2}, {2, -2}, {-2, -2}}}, []int{0}, []int{0}},
		{`vertices on the meridian`, orb.Polygon{{{-2, -1}, {0, -1}, {2, -1}, {2, 1}, {0, 1}, {-2, 1}, {-2, -1}}}, []int{0}, []int{0}},
		{`edge along the meridian`, orb.Polygon{{{0, -1}, {2, -1}, {2, 1}, {0, 1}, {0, -1}}}, nil, []int{0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pieces := checkSplit(t, test.polygon)
			cut := cutLongitude(test.polygon)
			var west, east []int
			for _, piece := range pieces {
				if piece.Bound().Max.Lon() <= cut {
					west = append(west, len(piece)-1)
				} else {
					east = append(east, len(piece)-1)
				}
			}
			if fmt.Sprint(west) != fmt.Sprint(test.west) || fmt.Sprint(east) != fmt.Sprint(test.east) {
				t.Errorf(`got pieces with holes %v west and %v east, want %v and %v`, west, east, test.west, test.east)
			}
		})
	}
}

// TestSplitContours splits every polygon contoured from random grids that
// straddle the prime meridian, including integer grids whose boundaries run
// along grid lines, one of which is at exactly 0 degrees.
func TestSplitContours(t *testing.T) {
	for seed := range 100 {
		rng := rand.New(rand.NewPCG(uint64(seed), 1))
		nx, ny := 5+rng.IntN(20), 5+rng.IntN(20)
		grid := &contour.Grid{NX: nx, NY: ny}
		for j := range ny {
			for i := range nx {
				grid.X = append(grid.X, float64(i-nx/2)*0.5)
				grid.Y = append(grid.Y, float64(ny-1-j)*0.5) // north to south, as grib2-go delivers
				z := rng.Float64() * 4
				if seed%2 == 0 {
					z = float64(rng.IntN(5))
				}
				if rng.Float64() < 0.1 {
					z = math.NaN()
				}
				grid.Z = append(grid.Z, z)
			}
		}
		topo, err := contour.Contour(grid, []float64{-1, 0.5, 1.5, 2.5, 4})
		if err != nil {
			t.Fatal(err)
		}
		t.Run(fmt.Sprintf(`seed %d`, seed), func(t *testing.T) {
			for _, band := range topo.Bands {
				for _, polygon := range band.Polygons {
					checkSplit(t, topo.PolygonCoords(polygon))
				}
			}
		})
	}
}

// checkSplit splits p and checks that the pieces are valid, each on one side
// of the cut, and together cover exactly p's area.
func checkSplit(t *testing.T, p orb.Polygon) []orb.Polygon {
	t.Helper()
	pieces, err := Split(p)
	if err != nil {
		t.Fatal(err)
	}
	if b := p.Bound(); b.Min.Lon() >= 0 || b.Max.Lon() <= 0 {
		if len(pieces) != 1 || !orb.Equal(pieces[0], p) {
			t.Errorf(`a polygon on one side of the meridian should be returned unchanged, got %d pieces`, len(pieces))
		}
		return pieces
	}
	cut := cutLongitude(p)
	var total float64
	for i, piece := range pieces {
		if err := s2check.ValidatePolygon(piece); err != nil {
			t.Errorf(`piece %d: %v`, i, err)
		}
		if b := piece.Bound(); b.Min.Lon() < cut && b.Max.Lon() > cut {
			t.Errorf(`piece %d spans the cut at %v: lon %v to %v`, i, cut, b.Min.Lon(), b.Max.Lon())
		}
		for _, ring := range piece {
			total += area(ring)
		}
	}
	var want float64
	for _, ring := range p {
		want += area(ring)
	}
	if math.Abs(total-want) > 1e-9*math.Max(1, math.Abs(want)) {
		t.Errorf(`pieces cover area %v, the polygon %v`, total, want)
	}
	return pieces
}

func TestSphereFraction(t *testing.T) {
	// A lon/lat rectangle covers (dlon/360) * (sin(lat2) - sin(lat1))/2 of the
	// sphere; edges along parallels are traced in 1-degree steps, as a grid's
	// would be.
	rect := func(lon1, lat1, lon2, lat2 float64) orb.Polygon {
		var ring orb.Ring
		for lon := lon1; lon <= lon2; lon++ {
			ring = append(ring, orb.Point{lon, lat1})
		}
		for lon := lon2; lon >= lon1; lon-- {
			ring = append(ring, orb.Point{lon, lat2})
		}
		return orb.Polygon{append(ring, ring[0])}
	}
	sin := func(deg float64) float64 { return math.Sin(deg * math.Pi / 180) }
	for _, test := range []struct{ lon1, lat1, lon2, lat2 float64 }{
		{-179, -80, 179, 80},
		{0, 0, 90, 45},
		{-10, 60, 10, 70},
	} {
		got := SphereFraction(rect(test.lon1, test.lat1, test.lon2, test.lat2))
		want := (test.lon2 - test.lon1) / 360 * (sin(test.lat2) - sin(test.lat1)) / 2
		if math.Abs(got-want) > 1e-12 {
			t.Errorf(`%v: got %v, want %v`, test, got, want)
		}
	}
}
