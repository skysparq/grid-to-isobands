package grid_to_isobands

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/fogleman/contourmap"
	"github.com/google/uuid"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/planar"
)

const levelEpsilon = 1e-5

type GridValues struct {
	SizeX  int
	SizeY  int
	Values []float64
	Lats   []float64
	Lons   []float64
}

func IsobandsFromGrid(grid GridValues, transform Transformer, floor, step float64, addlProps map[string]any) (*FeatureCollection, error) {
	jobId := uuid.NewString()
	grid = preprocessGrid(grid, transform, floor, step)
	isogons := createIsogons(grid, floor, step)
	isobands, err := toIsobands(isogons, jobId)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: %w", err)
	}
	isobands.Properties = addlProps
	return isobands, nil
}

func preprocessGrid(vals GridValues, transform Transformer, floor, step float64) GridValues {
	vals.Values = transform(vals.Values, vals.SizeX)
	vals.Lons = transform(vals.Lons, vals.SizeX)
	vals.Lats = transform(vals.Lats, vals.SizeX)
	sentinel := floor - (step * 10)
	// replace invalid values with a value much lower than floor
	for i, value := range vals.Values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < floor {
			vals.Values[i] = sentinel
		}
	}

	morph := NewMorphologicalOps(vals.SizeX, vals.SizeY)
	morphed := morph.OpenClose(vals.Values, 3)
	smoothed := FastGaussian(morphed, vals.SizeX, vals.SizeY, 3, 0.5)

	// replace edge of grid with sentinel values
	for y := 0; y < vals.SizeY; y++ {
		if y == 0 || y == vals.SizeY-1 {
			for x := 0; x < vals.SizeX; x++ {
				smoothed[y*vals.SizeX+x] = sentinel
			}
			continue
		}
		left := y * vals.SizeX
		right := (y+1)*vals.SizeX - 1
		smoothed[left] = sentinel
		smoothed[right] = sentinel
	}

	vals.Values = smoothed
	return vals
}

func createIsogons(grid GridValues, floor, step float64) *FeatureCollection {
	collection := NewFeatureCollection(nil)
	minVal, maxVal := minMax(grid.Values, floor)
	m := contourmap.FromFloat64s(grid.SizeX, grid.SizeY, grid.Values)

	start := math.Floor(minVal/step) * step
	numSteps := int((maxVal - start) / step)
	levelIndex := 0

	for stepIndex := 0; stepIndex < numSteps; stepIndex++ {
		i := start + float64(stepIndex)*step
		contours := m.Contours(i - levelEpsilon)
		currentFills := make([]orb.Ring, 0, 10)
		currentHoles := make([]orb.Ring, 0, 10)
		levelFloor := i
		levelTop := i + step

		for _, contour := range contours {
			ring := contourToRing(contour, grid)

			if !ring[0].Equal(ring[len(ring)-1]) {
				ring = append(ring, ring[0]) // close any open loops
			}

			if len(ring) < 4 {
				continue
			}

			if ring.Orientation() == orb.CW {
				ring = ring.Clone()
				ring.Reverse()
				currentFills = append(currentFills, ring)
			} else {
				ring = ring.Clone()
				ring.Reverse()
				currentHoles = append(currentHoles, ring)
			}
		}

		polys := buildPolygonHierarchy(currentFills, currentHoles)
		for _, poly := range polys {
			collection.AddPolygon(poly, map[string]any{
				`levelIndex`: levelIndex,
				`floor`:      levelFloor,
				`ceiling`:    levelTop,
			})
		}
		levelIndex++
	}
	return collection
}

func toIsobands(isogons *FeatureCollection, jobId string) (*FeatureCollection, error) {
	inPath := densePath(jobId)
	outPath := simplePath(jobId)
	in, err := os.Create(inPath)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to create input file: %w", err)
	}
	defer func() {
		_ = in.Close()
		//_ = os.Remove(inPath)
	}()
	encoder := json.NewEncoder(in)
	err = encoder.Encode(isogons)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to encode input file: %w", err)
	}
	err = execCmd(`Rscript`, `isobands.R`, inPath, outPath, `1000`)
	//defer func() { _ = os.Remove(outPath) }()
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: %w", err)
	}
	banded, err := os.Open(outPath)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to open isobands: %w", err)
	}
	decoder := json.NewDecoder(banded)
	isobands := &FeatureCollection{}
	err = decoder.Decode(isobands)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to decode isobands: %w", err)
	}
	for i, feature := range isobands.Features {
		if feature.Geometry.Coordinates == nil {
			slices.Delete(isogons.Features, i, i+1)
			i--
		}
	}
	return isobands, nil
}

func indexToSpatial(x, y float64, width, height int, lngs, lats []float64) (float64, float64) {
	y1 := int(y)
	x1 := int(x)
	x2 := x1 + 1
	if x2 >= width-1 {
		x2 = width - 1
	}
	y2 := y1 + 1
	if y2 >= height-1 {
		y2 = height - 1
	}
	i1 := y1*width + x1
	i2 := y1*width + x2
	i3 := y2*width + x1
	i4 := y2*width + x2
	fx := x - math.Floor(x)
	fy := y - math.Floor(y)

	lat := (1 - fx) * (1 - fy) * lats[i1]
	lat += (1 - fx) * fy * lats[i3]
	lat += fx * (1 - fy) * lats[i2]
	lat += fx * fy * lats[i4]

	lng := (1 - fx) * (1 - fy) * lngs[i1]
	lng += (1 - fx) * fy * lngs[i3]
	lng += fx * (1 - fy) * lngs[i2]
	lng += fx * fy * lngs[i4]

	return lng, lat
}

func minMax(values []float64, floor float64) (float64, float64) {
	minVal, maxVal := math.NaN(), math.NaN()
	for _, val := range values {
		minVal = minFloor(floor, minVal, val)
		maxVal = maxFloor(floor, maxVal, val)
	}
	return minVal, maxVal

}

func minFloor(floor, val1, val2 float64) float64 {
	if math.IsNaN(val1) || math.IsInf(val1, 0) {
		val1 = floor - 1
	}
	if math.IsNaN(val2) || math.IsInf(val2, 0) {
		val2 = floor - 1
	}
	if val1 < floor && val2 < floor {
		return floor - 1
	}
	if val1 < floor {
		return val2
	}
	if val2 < floor {
		return val1
	}
	if val1 < val2 {
		return val1
	}
	return val2
}

func maxFloor(floor, val1, val2 float64) float64 {
	if math.IsNaN(val1) || math.IsInf(val1, 0) {
		val1 = floor - 1
	}
	if math.IsNaN(val2) || math.IsInf(val2, 0) {
		val2 = floor - 1
	}

	if val1 < floor && val2 < floor {
		return floor - 1
	}
	if val1 < floor {
		return val2
	}
	if val2 < floor {
		return val1
	}
	if val1 > val2 {
		return val1
	}
	return val2
}

func contourToRing(contour contourmap.Contour, grid GridValues) orb.Ring {
	ring := make(orb.Ring, 0, 10)
	width := grid.SizeX
	height := grid.SizeY
	lngs := grid.Lons
	lats := grid.Lats
	for _, point := range contour {
		lng, lat := indexToSpatial(point.X, point.Y, width, height, lngs, lats)
		orbPoint := orb.Point{lng, lat}
		ring = append(ring, orbPoint)
	}

	return ring
}

func execCmd(name string, args ...string) error {
	stdErr := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	stdOut := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	cmd := exec.Command(name, args...)
	cmd.Stderr = stdErr
	cmd.Stdout = stdOut
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error executing %v: %w: %v", name, err, stdErr.String())
	}
	print(stdOut.String())
	return nil
}

func densePath(jobId string) string {
	return tmpFilePath(jobId + `-dense.geojson`)
}

func simplePath(jobId string) string {
	return tmpFilePath(jobId + `-simple.geojson`)
}

func tmpFilePath(filename string) string {
	path := filepath.Join(`./tmp`, filename)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}

func saveCollection(isogons *FeatureCollection, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error saving collection: failed to create isobands file: %w", err)
	}
	defer func() { _ = f.Close() }()
	encoder := json.NewEncoder(f)
	err = encoder.Encode(isogons)
	if err != nil {
		return fmt.Errorf("error saving collection: failed to encode isobands file: %w", err)
	}
	return nil
}

func loadCollection(path string) (*FeatureCollection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error loading collection: failed to open isobands file: %w", err)
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	collection := &FeatureCollection{}
	err = decoder.Decode(collection)
	if err != nil {
		return nil, fmt.Errorf("error loading collection: failed to decode isobands file: %w", err)
	}
	return collection, nil
}

func cmpCcwRingArea(a, b orb.Ring) int {
	return cmp.Compare(planar.Area(a), planar.Area(b)) * -1
}
func cmpCwRingArea(a, b orb.Ring) int {
	return cmp.Compare(planar.Area(a), planar.Area(b))
}

// buildPolygonHierarchy builds polygons with proper hole handling
// by checking containment relationships at each level
func buildPolygonHierarchy(fills, holes []orb.Ring) []orb.Polygon {
	if len(fills) == 0 {
		return nil
	}

	slices.SortFunc(fills, cmpCcwRingArea)
	slices.SortFunc(holes, cmpCwRingArea)

	polys := make([]orb.Polygon, 0, len(fills))
	for _, fill := range fills {
		poly := orb.Polygon{fill}
		for holei := 0; holei < len(holes); holei++ {
			hole := holes[holei]
			if planar.PolygonContains(poly, hole[0]) || planar.PolygonContains(poly, hole[len(hole)/2]) {
				copyHole := make([]orb.Point, len(hole))
				copy(copyHole, hole)
				poly = append(poly, copyHole)
				holes = slices.Delete(holes, holei, holei+1)
				holei--
			}
		}
		polys = append(polys, poly)
	}
	return polys
}
