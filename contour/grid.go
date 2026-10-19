// Package contour computes filled contours (isobands) of a curvilinear grid as
// a topology: shared edges ("arcs") and, for each band, rings made of arc
// references. Adjacent bands reference the same arc for their common boundary,
// so the boundary can later be simplified once and stay consistent in both.
//
// The contouring matches contourpy's serial algorithm with quad_as_tri=True and
// corner_mask=True: each cell is split into four triangles around its centre
// (the average of its corners), a cell with one missing (NaN) corner keeps only
// the triangle of its other three corners, and a cell with two or more is
// skipped. A band (lower, upper] holds the values lower < z <= upper.
package contour

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Grid is a curvilinear grid in row-major order: point (i, j) is at j*NX+i.
type Grid struct {
	NX, NY int
	X, Y   []float64 // coordinates of each point, e.g. longitude and latitude
	Z      []float64 // values; NaN marks missing data
}

func (g *Grid) validate() error {
	if g.NX < 2 || g.NY < 2 {
		return fmt.Errorf(`grid must be at least 2x2, got %dx%d`, g.NX, g.NY)
	}
	n := g.NX * g.NY
	if len(g.X) != n || len(g.Y) != n || len(g.Z) != n {
		return fmt.Errorf(`grid is %dx%d but has %d x, %d y and %d z values`, g.NX, g.NY, len(g.X), len(g.Y), len(g.Z))
	}
	return nil
}

func validateLevels(levels []float64) error {
	if len(levels) < 2 {
		return errors.New(`need at least 2 levels`)
	}
	for i, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf(`level %d is %v`, i, level)
		}
		if i > 0 && level <= levels[i-1] {
			return fmt.Errorf(`levels must be strictly increasing, got %v after %v`, level, levels[i-1])
		}
	}
	return nil
}

// A node is a triangle corner: a grid point (0 to NX*NY-1) or, after those,
// the centre of a cell (NX*NY + j*(NX-1) + i).
type node int

// pointID identifies a contour vertex by where it comes from rather than by its
// coordinates, so every triangle that produces a vertex produces the same ID
// and segments join exactly. A vertex is either a node (b == -1) or the point
// where level index `level` crosses the triangle edge between nodes a < b.
type pointID struct {
	a, b  node
	level int32
}

func nodePoint(n node) pointID {
	return pointID{a: n, b: -1, level: -1}
}

// snapTolerance, as a fraction of the smallest gap between levels, is how
// close a value must be to a level to be treated as exactly on it.
const snapTolerance = 1e-9

// geometry holds the grid and levels being contoured and computes the values
// and positions of nodes and points.
type geometry struct {
	grid   *Grid
	levels []float64
	snap   float64 // values within this of a level are treated as on it
	// flipped reports whether the grid's X/Y coordinates run clockwise as the
	// index space runs counterclockwise, e.g. when rows go from north to south.
	flipped bool
}

func newGeometry(grid *Grid, levels []float64) *geometry {
	gap := math.Inf(1)
	for i := 1; i < len(levels); i++ {
		gap = min(gap, levels[i]-levels[i-1])
	}
	g := &geometry{grid: grid, levels: levels, snap: snapTolerance * gap}
	// The grid is assumed not to fold over itself, so one cell's orientation
	// holds for all of them. Take one from the middle, away from any polar
	// rows that degenerate in some projections.
	var area float64
	cs := g.corners((grid.NX-1)/2, (grid.NY-1)/2)
	for k, a := range cs {
		b := cs[(k+1)%4]
		area += grid.X[a]*grid.Y[b] - grid.X[b]*grid.Y[a]
	}
	g.flipped = area < 0
	return g
}

// value returns z, or the level it is within rounding error of. A centre is
// an average of its corners and often lands a few ULPs off a level the data
// sits on, e.g. (1.1+0.9+1.2+0.8)/4 is 0.9999999999999999. Unsnapped, the
// level would cross the edges around it so close to it that the crossings
// round to its coordinates but get IDs of their own, leaving zero-length
// edges and pinches the topology can't see. Snapped, the crossings are the
// node itself.
func (g *geometry) value(z float64) float64 {
	m := sort.SearchFloat64s(g.levels, z)
	if m < len(g.levels) && g.levels[m]-z <= g.snap {
		return g.levels[m]
	}
	if m > 0 && z-g.levels[m-1] <= g.snap {
		return g.levels[m-1]
	}
	return z
}

func (g *geometry) node(i, j int) node {
	return node(j*g.grid.NX + i)
}

func (g *geometry) centre(i, j int) node {
	return node(g.grid.NX*g.grid.NY + j*(g.grid.NX-1) + i)
}

// corners returns the grid points of cell (i, j) counterclockwise in index
// space (i to the right, j up).
func (g *geometry) corners(i, j int) [4]node {
	n00 := g.node(i, j)
	nx := node(g.grid.NX)
	return [4]node{n00, n00 + 1, n00 + nx + 1, n00 + nx}
}

// nodeValues returns a node's value, coordinates, and position in index space.
// A centre averages its cell's corners, as contourpy does.
func (g *geometry) nodeValues(n node) (z, x, y, fi, fj float64) {
	grid := g.grid
	points := grid.NX * grid.NY
	if int(n) < points {
		return g.value(grid.Z[n]), grid.X[n], grid.Y[n], float64(int(n) % grid.NX), float64(int(n) / grid.NX)
	}
	q := int(n) - points
	i, j := q%(grid.NX-1), q/(grid.NX-1)
	for _, c := range g.corners(i, j) {
		z += grid.Z[c] // the corners' raw values, as contourpy averages them
		x += grid.X[c]
		y += grid.Y[c]
	}
	return g.value(z / 4), x / 4, y / 4, float64(i) + 0.5, float64(j) + 0.5
}

// crossing returns the point where level m crosses the edge between nodes a
// and b. A crossing exactly at a node is that node, so the triangles around it
// agree on one point instead of emitting coincident duplicates.
func (g *geometry) crossing(a, b node, za, zb float64, m int) pointID {
	level := g.levels[m]
	if za == level {
		return nodePoint(a)
	}
	if zb == level {
		return nodePoint(b)
	}
	if a > b {
		a, b = b, a
	}
	return pointID{a: a, b: b, level: int32(m)}
}

// position returns a point's coordinates and its position in index space. A
// crossing is interpolated linearly from the lower-numbered node, so the
// result doesn't depend on which triangle asks.
func (g *geometry) position(p pointID) (x, y, fi, fj float64) {
	za, xa, ya, ia, ja := g.nodeValues(p.a)
	if p.b < 0 {
		return xa, ya, ia, ja
	}
	zb, xb, yb, ib, jb := g.nodeValues(p.b)
	t := (g.levels[p.level] - za) / (zb - za)
	return xa + t*(xb-xa), ya + t*(yb-ya), ia + t*(ib-ia), ja + t*(jb-ja)
}

// bin is the number of levels below z, so a band (levels[k], levels[k+1]]
// holds the values with bin k+1.
func (g *geometry) bin(z float64) int {
	return sort.SearchFloat64s(g.levels, z)
}
