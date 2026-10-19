package s2check

import (
	"math"
	"strings"
	"testing"

	"github.com/paulmach/orb"
)

// shell and hole are wound by the GeoJSON right-hand rule (CCW shell, CW hole).
var (
	shell = orb.Ring{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {0, 0}}
	hole  = orb.Ring{{1, 1}, {1, 3}, {3, 3}, {3, 1}, {1, 1}}
)

func TestValidatePolygon(t *testing.T) {
	tests := []struct {
		name  string
		rings []orb.Ring
		want  string // substring of the expected error; empty means valid
	}{
		{`square`, []orb.Ring{shell}, ``},
		{`square with hole`, []orb.Ring{shell, hole}, ``},
		{`hole touching shell at a vertex`, []orb.Ring{shell, {{0, 0}, {1, 2}, {2, 2}, {2, 1}, {0, 0}}}, ``},
		{`no rings`, nil, `no rings`},
		{`too few points`, []orb.Ring{{{0, 0}, {1, 0}, {0, 0}}}, `need at least 4`},
		{`not closed`, []orb.Ring{{{0, 0}, {4, 0}, {4, 4}, {0, 4}}}, `not closed`},
		{`NaN coordinate`, []orb.Ring{{{0, 0}, {4, 0}, {math.NaN(), 4}, {0, 4}, {0, 0}}}, `not a valid lon/lat`},
		{`latitude out of range`, []orb.Ring{{{0, 0}, {4, 0}, {4, 91}, {0, 4}, {0, 0}}}, `not a valid lon/lat`},
		{`zero-length edge`, []orb.Ring{{{0, 0}, {4, 0}, {4, 0}, {4, 4}, {0, 4}, {0, 0}}}, `degenerate`},
		{`bowtie`, []orb.Ring{{{0, 0}, {4, 4}, {4, 0}, {0, 4}, {0, 0}}}, `crosses`},
		{`pinched ring`, []orb.Ring{{{0, 0}, {2, 0}, {2, 2}, {4, 2}, {4, 4}, {2, 4}, {2, 2}, {0, 2}, {0, 0}}}, `repeats vertex`},
		{`hole crossing shell`, []orb.Ring{shell, {{3, 1}, {3, 3}, {5, 3}, {5, 1}, {3, 1}}}, `ring 0 edge 1 crosses ring 1 edge 1`},
		{`hole sharing an edge with shell`, []orb.Ring{shell, {{0, 0}, {0, 2}, {2, 2}, {2, 0}, {4, 0}, {0, 0}}}, `share an edge`},
		{`hole crossing shell at a shared vertex`, []orb.Ring{shell, {{0, 4}, {-1, 2}, {1, 2}, {0, 4}}}, `cross at a shared vertex`},
		{`hole vertex a hair inside the shell's edge`, []orb.Ring{shell, {{1e-13, 2}, {1, 3}, {2, 2}, {1, 1}, {1e-13, 2}}}, `ring 1 vertex 0 is too close to ring 0 edge 3`},
		{`notch along a meridian`, []orb.Ring{{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {0, 2.5}, {2, 2}, {0, 1.5}, {0, 0}}}, ``},
		{`hole outside shell`, []orb.Ring{shell, {{6, 1}, {6, 3}, {8, 3}, {8, 1}, {6, 1}}}, `not inside the shell`},
		{`hole inside hole`, []orb.Ring{shell, hole, {{1.5, 1.5}, {1.5, 2.5}, {2.5, 2.5}, {2.5, 1.5}, {1.5, 1.5}}}, `nested with hole`},
		{`clockwise shell`, []orb.Ring{{{0, 0}, {0, 4}, {4, 4}, {4, 0}, {0, 0}}}, `more than a hemisphere`},
		{`counterclockwise hole`, []orb.Ring{shell, {{1, 1}, {3, 1}, {3, 3}, {1, 3}, {1, 1}}}, `not inside the shell`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePolygon(test.rings)
			switch {
			case test.want == `` && err != nil:
				t.Fatalf(`expected valid, got %v`, err)
			case test.want != `` && err == nil:
				t.Fatalf(`expected error containing %q, got nil`, test.want)
			case test.want != `` && !strings.Contains(err.Error(), test.want):
				t.Fatalf(`expected error containing %q, got %v`, test.want, err)
			}
		})
	}
}
