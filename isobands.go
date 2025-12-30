package grid_to_isobands

import (
	"bytes"
	_ "embed"
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
	"github.com/paulmach/orb/clip"
)

const levelEpsilon = 1e-5

//go:embed isobands.R
var rScript []byte // Embed the R script as bytes

type GridValues struct {
	SizeX  int
	SizeY  int
	Values []float64
	Lats   []float64
	Lons   []float64
}

type IsobandArgs struct {
	Grid        GridValues
	Transform   Transformer
	Floor, Step float64
	AddlProps   map[string]any
	WorkDir     string
}

func IsobandsFromGrid(args IsobandArgs) (*FeatureCollection, error) {
	grid, transform, floor, step := args.Grid, args.Transform, args.Floor, args.Step
	addlProps := args.AddlProps
	workDir := args.WorkDir
	if workDir == "" {
		workDir = `./tmp`
	}

	jobId := uuid.NewString()
	grid = preprocessGrid(grid, transform, floor, step)
	isogons := createIsogons(grid, floor, step)
	isobands, err := toIsobands(isogons, jobId, workDir)
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
		levelFloor := i
		levelTop := i + step
		currentFills := make([]orb.Polygon, 0, 10)
		currentHoles := make([]orb.Polygon, 0, 10)

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
				currentFills = append(currentFills, orb.Polygon{ring})
			} else {
				ring = ring.Clone()
				ring.Reverse()
				currentHoles = append(currentHoles, orb.Polygon{ring})
			}
		}

		//polys := buildPolygonHierarchy(currentFills, currentHoles)
		fillFeatures := splitToQuadrants(currentFills)
		for featurei, feature := range fillFeatures {
			props := feature.Properties
			props[`levelIndex`] = levelIndex
			props[`floor`] = levelFloor
			props[`ceiling`] = levelTop
			props[`isHole`] = false
			fillFeatures[featurei].Properties = props
		}
		collection.Features = append(collection.Features, fillFeatures...)

		holeFeatures := splitToQuadrants(currentHoles)
		for featurei, feature := range holeFeatures {
			props := feature.Properties
			props[`levelIndex`] = levelIndex
			props[`floor`] = levelFloor
			props[`ceiling`] = levelTop
			props[`isHole`] = true
			holeFeatures[featurei].Properties = props
		}
		collection.Features = append(collection.Features, holeFeatures...)

		levelIndex++
	}
	return collection
}

func toIsobands(isogons *FeatureCollection, jobId string, workDir string) (*FeatureCollection, error) {
	inPath := densePath(jobId, workDir)
	outPath := simplePath(jobId, workDir)
	in, err := os.Create(inPath)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to create input file: %w", err)
	}
	defer func() {
		_ = in.Close()
		_ = os.Remove(inPath)
	}()
	encoder := json.NewEncoder(in)
	err = encoder.Encode(isogons)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to encode input file: %w", err)
	}
	err = execCmd(`Rscript`, inPath, outPath, `1000`)
	defer func() { _ = os.Remove(outPath) }()
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
	fullArgs := append([]string{`-`}, args...)
	cmd := exec.Command(name, fullArgs...)
	cmd.Stdin = bytes.NewReader(rScript)
	cmd.Stderr = stdErr
	cmd.Stdout = stdOut
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error executing %v: %w: %v\nstdout: %v", name, err, stdErr.String(), stdOut.String())
	}
	return nil
}

func densePath(jobId, workDir string) string {
	return tmpFilePath(jobId+`-dense.geojson`, workDir)
}

func simplePath(jobId, workDir string) string {
	return tmpFilePath(jobId+`-simple.geojson`, workDir)
}

func tmpFilePath(filename, workDir string) string {
	path := filepath.Join(workDir, filename)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}

func splitToQuadrants(polys []orb.Polygon) []Feature {
	features := make([]Feature, 0, len(polys))
	for _, poly := range polys {
		for quadi, quad := range quadrants {
			clipped := clip.Polygon(quad, poly.Clone())
			for ringi, ring := range clipped {
				ring = ring.Clone()
				for i := 1; i < len(ring); i++ {
					currentPoint := ring[i]
					priorPoint := ring[i-1]
					if currentPoint.Equal(priorPoint) {
						ring = slices.Delete(ring, i-1, i)
						i--
					}
				}
				clipped[ringi] = ring
			}
			if clipped != nil {
				features = append(features, Feature{
					Type:       "Feature",
					Geometry:   Polygon{Type: "Polygon", Coordinates: clipped},
					Properties: map[string]any{`quadrant`: quadi},
				})
			}
		}
	}
	return features
}

var quadrants = []orb.Bound{
	{ // top left
		Min: orb.Point{-180, 0},
		Max: orb.Point{0, 90},
	},
	{ // top right
		Min: orb.Point{0, 0},
		Max: orb.Point{180, 90},
	},
	{ // bottom left
		Min: orb.Point{-180, -90},
		Max: orb.Point{0, 0},
	},
	{ // bottom right
		Min: orb.Point{0, -90},
		Max: orb.Point{180, 0},
	},
}
