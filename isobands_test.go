package grid_to_isobands_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/paulmach/orb"
	grid_to_isobands "github.com/skysparq/grid-to-isobands"
	"github.com/skysparq/grid-to-isobands/s2check"
	"github.com/skysparq/grid-to-isobands/transformers"
)

func TestGenerateLevels(t *testing.T) {
	levels := grid_to_isobands.GenerateLevels(5, 41.2, 2.5)
	expected := []float64{
		5, 7.5, 10, 12.5, 15, 17.5, 20, 22.5, 25, 27.5, 30, 32.5, 35, 37.5, 40, 42.5,
	}
	if !slices.Equal(levels, expected) {
		t.Errorf("got %v, want %v", levels, expected)
	}
}

func TestMrmsBaseReflectivity(t *testing.T) {
	testData, err := getTestData(`mrms-base-reflectivity.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.BilateralTransformer(1, 2.5),
			grid_to_isobands.CloseOpenTransformer(3),
			grid_to_isobands.GaussianTransformer(5, 1.0),
		},
		Grid:  gridValues,
		Floor: 3.75,
		Step:  2.5,
		AddlProps: map[string]any{
			`measure`: `base-reflectivity`,
			`at`:      time.Date(2025, 12, 11, 23, 59, 17, 0, time.UTC),
		},
	}
	isobands, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	err = saveTestOutput(isobands, `mrms-base-reflectivity.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMrmsBaseReflectivity2(t *testing.T) {
	testData, err := getTestData(`mrms-base-reflectivity-2.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.BilateralTransformer(2, 5),
			grid_to_isobands.CloseOpenTransformer(7),
			grid_to_isobands.GaussianTransformer(7, 1.5),
		},
		Grid:  gridValues,
		Floor: 5,
		Step:  2.5,
		AddlProps: map[string]any{
			`measure`: `base-reflectivity`,
			`at`:      time.Date(2026, 1, 6, 17, 2, 0, 0, time.UTC),
		},
	}
	isobands, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	err = saveTestOutput(isobands, `mrms-base-reflectivity-2.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMrmsKnoxTornadoBaseReflectivity(t *testing.T) {
	testData, err := getTestData(`mrms-knox-tornado-base-reflectivity.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.BilateralTransformer(1, 2.5),
			grid_to_isobands.CloseOpenTransformer(3),
			grid_to_isobands.GaussianTransformer(5, 1.0),
		},
		Grid:  gridValues,
		Floor: 5,
		Step:  2.5,
		AddlProps: map[string]any{
			`measure`: `base-reflectivity`,
			`at`:      time.Date(2026, 3, 11, 1, 10, 14, 0, time.UTC),
		},
	}
	isobands, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	err = saveTestOutput(isobands, `mrms-knox-tornado-base-reflectivity.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMrmsCompositeReflectivity(t *testing.T) {
	testData, err := getTestData(`mrms-composite-reflectivity.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.BilateralTransformer(2, 5),
			grid_to_isobands.CloseOpenTransformer(7),
			grid_to_isobands.GaussianTransformer(7, 1.5),
		},
		Grid:  gridValues,
		Floor: 5,
		Step:  2.5,
		AddlProps: map[string]any{
			`measure`: `composite-reflectivity`,
			`at`:      time.Date(2026, 1, 7, 19, 2, 36, 0, time.UTC),
		},
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `mrms-composite-reflectivity.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMrmsMultiSensorQpe(t *testing.T) {
	testData, err := getTestData(`multi-sensor-qpe.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{},
		Grid:         gridValues,
		Levels:       []float64{0.25, 1, 2.5, 5, 10, 15, 25, 40, 50, 75, 100, 150, 1000},
		AddlProps: map[string]any{
			`measure`: `qpe`,
			`at`:      time.Date(2026, 8, 5, 0, 2, 0, 0, time.UTC),
		},
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `qpe.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMrmsPrecipRate(t *testing.T) {
	testData, err := getTestData(`precip-rate.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.ThresholdMaskTransformer(transformers.LessThan(0), 0),
			grid_to_isobands.MedianTransformer(3),
			grid_to_isobands.GaussianTransformer(5, 0.7),
		},
		Grid:   gridValues,
		Levels: []float64{0.1, 0.5, 1, 2, 5, 10, 15, 20, 30, 50, 75, 100, 150, 200, 10000},
		AddlProps: map[string]any{
			`measure`: `precip-rate`,
			`at`:      time.Date(2026, 8, 5, 0, 2, 0, 0, time.UTC),
		},
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `precip-rate.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMrmsPrecipFlag(t *testing.T) {
	testData, err := getTestData(`precip-flag.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.ThresholdMaskTransformer(transformers.LessThan(0), 0),
		},
		Grid:   gridValues,
		Levels: []float64{0.5, 1.5, 2.5, 3.5, 5.5, 6.5, 7.5, 9.5, 10.5, 90.5, 91.5, 95.5, 96.5},
		AddlProps: map[string]any{
			`measure`: `precip-flag`,
			`at`:      time.Date(2026, 8, 5, 0, 2, 0, 0, time.UTC),
		},
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `precip-flag.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMrmsCompositeReflectivity2(t *testing.T) {
	testData, err := getTestData(`mrms-composite-reflectivity-2.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.BilateralTransformer(2, 5),
			grid_to_isobands.CloseOpenTransformer(7),
			grid_to_isobands.GaussianTransformer(7, 1.5),
		},
		Grid:  gridValues,
		Floor: 5,
		Step:  2.5,
		AddlProps: map[string]any{
			`measure`: `composite-reflectivity`,
			`at`:      time.Date(2026, 1, 7, 20, 30, 37, 0, time.UTC),
		},
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `mrms-composite-reflectivity-2.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGfsBaroPressure(t *testing.T) {
	testData, err := getTestData(`gfs-baro-pressure-msl.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.SwapRightAndLeftTransformer(),
			grid_to_isobands.ClipTransformer(transformers.Clip{
				Top:    20,
				Bottom: 20,
				Left:   1,
				Right:  1,
			}),
			grid_to_isobands.GaussianTransformer(7, 1),
		},
		Grid:  gridValues,
		Floor: 0,
		Step:  250,
		AddlProps: map[string]any{
			`measure`: `baro-pressure-msl`,
			`at`:      time.Date(2025, 10, 2, 12, 0, 0, 0, time.UTC),
		},
		WorkDir: "./tmp",
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `baro-pressure-msl.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

/*
	func TestGfsBaroPressure2(t *testing.T) {
		testData, err := getTestData(`gfs-baro-pressure-msl-2.json`)
		if err != nil {
			t.Fatal(err)
		}
		gridValues := grid_to_isobands.GridValues{
			SizeX:  testData.SizeX,
			SizeY:  testData.SizeY,
			Lats:   testData.Lats,
			Lons:   testData.Lngs,
			Values: testData.Values,
		}
		args := grid_to_isobands.IsobandArgs{
			Grid:             gridValues,
			InitialTransform: grid_to_isobands.SwapRightAndLeft,
			Clip:             grid_to_isobands.Clip{Left: 1, Right: 1, Top: 40, Bottom: 40},
			Floor:            0,
			Step:             250,
			AddlProps: map[string]any{
				`measure`: `baro-pressure-msl`,
				`at`:      time.Date(2026, 1, 9, 6, 0, 0, 0, time.UTC).Add(366 * time.Hour),
			},
			WorkDir:   "./tmp",
			Tolerance: 5000,
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `baro-ressure-msl-2.json`)
		if err != nil {
			t.Fatal(err)
		}
	}
*/
func TestSurfaceTemp(t *testing.T) {
	testData, err := getTestData(`temperature-surface.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.SwapRightAndLeftTransformer(),
			grid_to_isobands.ClipTransformer(transformers.Clip{
				Top:    20,
				Bottom: 20,
				Left:   1,
				Right:  1,
			}),
			grid_to_isobands.GaussianTransformer(7, 1),
		},
		Grid:  gridValues,
		Floor: 0,
		Step:  2,
		AddlProps: map[string]any{
			`measure`: `temperature-surface`,
			`at`:      time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
		},
		WorkDir: "./tmp",
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `temperature-surface.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

/*
	func TestWindU(t *testing.T) {
		testData, err := getTestData(`wind-u-100hpa.json`)
		if err != nil {
			t.Fatal(err)
		}
		gridValues := grid_to_isobands.GridValues{
			SizeX:  testData.SizeX,
			SizeY:  testData.SizeY,
			Lats:   testData.Lats,
			Lons:   testData.Lngs,
			Values: testData.Values,
		}
		args := grid_to_isobands.IsobandArgs{
			Grid:             gridValues,
			InitialTransform: grid_to_isobands.SwapRightAndLeft,
			Clip:             grid_to_isobands.Clip{Left: 1, Right: 1, Top: 40, Bottom: 40},
			Floor:            -1000,
			Step:             10,
			AddlProps: map[string]any{
				`measure`: `wind-u-100hpa`,
				`at`:      time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			},
			WorkDir:   "./tmp",
			Tolerance: 5000,
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `wind-u-100hpa.json`)
		if err != nil {
			t.Fatal(err)
		}
	}
*/
func TestVisibilitySurface(t *testing.T) {
	testData, err := getTestData(`visibility-surface.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.SwapRightAndLeftTransformer(),
			grid_to_isobands.ClipTransformer(transformers.Clip{
				Top:    20,
				Bottom: 20,
				Left:   1,
				Right:  1,
			}),
			grid_to_isobands.BilateralTransformer(1, 2500),
			grid_to_isobands.GaussianTransformer(3, 1.0),
		},
		Grid:  gridValues,
		Floor: 0,
		Step:  2500,
		AddlProps: map[string]any{
			`measure`: `visibility-surface`,
			`at`:      time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
		},
		WorkDir: "./tmp",
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `visibility-surface.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFailedGfsHumidity100hpa(t *testing.T) {
	testData, err := getTestData(`failed-gfs-humidity-100hpa.json`)
	if err != nil {
		t.Fatal(err)
	}
	gridValues := &grid_to_isobands.GridValues{
		SizeX:  testData.SizeX,
		SizeY:  testData.SizeY,
		Lats:   testData.Lats,
		Lons:   testData.Lngs,
		Values: testData.Values,
	}
	args := &grid_to_isobands.IsobandArgs{
		Preprocesses: []grid_to_isobands.GridTransformer{
			grid_to_isobands.SwapRightAndLeftTransformer(),
			grid_to_isobands.GaussianTransformer(7, 1),
			grid_to_isobands.ClipTransformer(transformers.Clip{
				Top:    20,
				Bottom: 20,
				Left:   1,
				Right:  1,
			}),
		},
		Grid:  gridValues,
		Floor: 0,
		Step:  10,
		AddlProps: map[string]any{
			`measure`: `humidity-100hpa`,
			`at`:      time.Date(2025, 10, 2, 12, 0, 0, 0, time.UTC),
		},
		WorkDir: "./tmp",
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	err = saveTestOutput(isogons, `failed-gfs-humidity-100hpa.geojson`)
	if err != nil {
		t.Fatal(err)
	}
}

/*
	func TestReflectivityAtmosphere(t *testing.T) {
		testData, err := getTestData(`reflectivity-atmosphere.json`)
		if err != nil {
			t.Fatal(err)
		}
		gridValues := grid_to_isobands.GridValues{
			SizeX:  testData.SizeX,
			SizeY:  testData.SizeY,
			Lats:   testData.Lats,
			Lons:   testData.Lngs,
			Values: testData.Values,
		}
		args := grid_to_isobands.IsobandArgs{
			Grid:             gridValues,
			InitialTransform: grid_to_isobands.SwapRightAndLeft,
			Clip:             grid_to_isobands.Clip{Left: 1, Right: 1, Top: 40, Bottom: 40},
			Floor:            5,
			Step:             2.5,
			AddlProps: map[string]any{
				`measure`: `reflectivity-atmosphere`,
				`at`:      time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			},
			WorkDir:   "./tmp",
			Tolerance: 5000,
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `reflectivity-atmosphere.json`)
		if err != nil {
			t.Fatal(err)
		}
	}
*/
func getTestData(filename string) (TestData, error) {
	var testData TestData
	f, err := os.Open(filepath.Join(`./.test_files`, filename))
	if err != nil {
		return testData, fmt.Errorf(`error opening file: %w`, err)
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&testData)
	_ = f.Close()
	if err != nil {
		return testData, fmt.Errorf(`error decoding test data: %w`, err)
	}
	return testData, nil
}

// saveTestOutput writes the isobands for visual inspection, then fails if any
// feature would be rejected by an S2-based geography engine (e.g. Snowflake).
func saveTestOutput(isogons *grid_to_isobands.ReturnValues, name string) error {
	out, err := os.Create(filepath.Join(`./test-output`, name))
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons.Isobands)
	if err != nil {
		return err
	}
	return validateS2(isogons.Isobands)
}

// validateS2 reports the S2 validation failures in a FeatureCollection, up to a
// limit, along with how many features failed in total.
func validateS2(fc *grid_to_isobands.FeatureCollection) error {
	const maxReported = 20
	var errs []error
	failed := 0
	for i, feature := range fc.Features {
		err := s2check.ValidatePolygon(feature.Geometry.Coordinates)
		if err == nil {
			continue
		}
		failed++
		if len(errs) < maxReported {
			errs = append(errs, fmt.Errorf(`feature %d (levelIndex %v): %w`, i, feature.Properties[`levelIndex`], err))
		}
	}
	if failed == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d features fail S2 validation:\n%w", failed, len(fc.Features), errors.Join(errs...))
}

func TestSelectionsOnlyAffectIsobands(t *testing.T) {
	const size = 20
	values := make([]float64, 0, size*size)
	lats := make([]float64, 0, size*size)
	lons := make([]float64, 0, size*size)
	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			values = append(values, float64(row*size+col)*25)
			lats = append(lats, 40-float64(row)*0.1)
			lons = append(lons, -100+float64(col)*0.1)
		}
	}
	original := slices.Clone(values)

	args := &grid_to_isobands.IsobandArgs{
		Grid: &grid_to_isobands.GridValues{SizeX: size, SizeY: size, Values: values, Lats: lats, Lons: lons},
		Selections: []grid_to_isobands.GridTransformer{
			grid_to_isobands.ThresholdMaskTransformer(transformers.GreaterThan(3000), 3150),
		},
		Floor: 0,
		Step:  150,
	}
	result, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(result.Grid.Values, original) {
		t.Fatal(`expected selections to leave the returned grid unchanged`)
	}
	if len(result.Isobands.Features) == 0 {
		t.Fatal(`expected isobands`)
	}
	for _, feature := range result.Isobands.Features {
		if ceiling := feature.Properties[`ceiling`].(float64); ceiling > 3300 {
			t.Fatalf(`expected no isoband above the selected maximum, got ceiling %v`, ceiling)
		}
	}
}

func TestLargeBandsSplitAtPrimeMeridian(t *testing.T) {
	// Everything between 80S and 80N in one band, which S2 would read as its
	// complement if it were emitted as one polygon.
	result := globalBands(t, func(lat float64) float64 { return 1 })

	if len(result.Isobands.Features) != 2 {
		t.Fatalf(`expected the band to be split into 2 features, got %d`, len(result.Isobands.Features))
	}
	for i, feature := range result.Isobands.Features {
		bound := orb.Polygon(feature.Geometry.Coordinates).Bound()
		if bound.Min.Lon() < 0 && bound.Max.Lon() > 0 {
			t.Errorf(`feature %d spans the prime meridian: lon %v to %v`, i, bound.Min.Lon(), bound.Max.Lon())
		}
	}
	if err := validateS2(result.Isobands); err != nil {
		t.Fatal(err)
	}
}

func TestThinGlobalBandsStayWhole(t *testing.T) {
	// A band from 40N to 50N circles the globe but covers only a few percent of
	// it, so it is already read correctly and shouldn't be split.
	result := globalBands(t, func(lat float64) float64 {
		if lat >= 40 && lat <= 50 {
			return 1
		}
		return 5
	})

	if len(result.Isobands.Features) != 1 {
		t.Fatalf(`expected the band to stay a single feature, got %d`, len(result.Isobands.Features))
	}
	if err := validateS2(result.Isobands); err != nil {
		t.Fatal(err)
	}
}

// globalBands contours a 1-degree grid from 80S to 80N with values set by
// latitude, as a single band between levels 0 and 2.
func globalBands(t *testing.T, valueAt func(lat float64) float64) *grid_to_isobands.ReturnValues {
	t.Helper()
	const sizeX, sizeY = 360, 161
	values := make([]float64, 0, sizeX*sizeY)
	lats := make([]float64, 0, sizeX*sizeY)
	lons := make([]float64, 0, sizeX*sizeY)
	for row := 0; row < sizeY; row++ {
		lat := 80 - float64(row)
		for col := 0; col < sizeX; col++ {
			values = append(values, valueAt(lat))
			lats = append(lats, lat)
			lons = append(lons, -179.5+float64(col))
		}
	}

	args := &grid_to_isobands.IsobandArgs{
		Grid:   &grid_to_isobands.GridValues{SizeX: sizeX, SizeY: sizeY, Values: values, Lats: lats, Lons: lons},
		Levels: []float64{0, 2},
	}
	result, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestBandsPartitionTheGrid(t *testing.T) {
	// A patch of the MRMS QPE test data where a band-0 polygon's shell
	// touches itself around a hole of band-1 values. Repairing the polygon as
	// a whole dropped the hole, so band 0 also covered it and overlapped band 1.
	patch := [][]float64{
		{1.4, 1.3, 1.2, 1.1, 1.0, 0.9, 0.9, 0.9, 0.8, 0.7, 0.7, 0.8, 0.8, 0.7, 0.7},
		{1.2, 1.1, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.9, 0.8, 0.7, 0.7, 0.7, 0.7, 0.7},
		{1.1, 1.1, 1.0, 1.0, 1.1, 1.0, 1.0, 1.0, 0.9, 0.8, 0.8, 0.7, 0.7, 0.8, 0.7},
		{1.0, 1.0, 1.0, 1.2, 1.1, 1.0, 1.0, 1.1, 1.1, 0.9, 0.9, 0.8, 0.8, 0.7, 0.7},
		{1.1, 1.3, 1.2, 1.1, 1.0, 1.0, 1.1, 1.1, 1.1, 1.0, 0.9, 0.9, 0.8, 0.8, 0.8},
		{1.1, 1.1, 1.1, 1.0, 0.9, 0.8, 0.9, 1.0, 1.0, 1.0, 0.9, 0.9, 0.8, 0.9, 0.9},
		{0.9, 0.9, 0.8, 0.8, 0.7, 0.7, 0.7, 0.8, 1.0, 1.0, 1.0, 1.1, 1.0, 1.0, 1.0},
		{0.8, 0.8, 0.8, 0.8, 0.8, 0.8, 0.8, 0.9, 1.0, 1.1, 1.1, 1.1, 1.2, 1.1, 1.1},
		{0.8, 0.7, 0.6, 0.7, 0.8, 0.9, 0.9, 1.0, 1.1, 1.1, 1.1, 1.1, 1.1, 1.1, 1.2},
	}
	sizeX, sizeY := len(patch[0]), len(patch)
	var values, lats, lons []float64
	for row := range sizeY {
		for col := range sizeX {
			values = append(values, patch[row][col])
			lats = append(lats, float64(row))
			lons = append(lons, float64(col))
		}
	}
	args := &grid_to_isobands.IsobandArgs{
		Grid:   &grid_to_isobands.GridValues{SizeX: sizeX, SizeY: sizeY, Values: values, Lats: lats, Lons: lons},
		Levels: []float64{0.25, 1, 2.5},
	}
	result, err := grid_to_isobands.IsobandsFromGrid(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}

	// Every value is inside the levels, so the bands must cover the grid
	// exactly once: their areas (shells CCW, holes CW) add up to its area.
	var area float64
	for _, feature := range result.Isobands.Features {
		for _, ring := range feature.Geometry.Coordinates {
			area += signedArea(ring)
		}
	}
	if want := float64((sizeX - 1) * (sizeY - 1)); math.Abs(area-want) > 1e-6 {
		t.Fatalf(`bands cover a total area of %v, but the grid's area is %v`, area, want)
	}
	if err = validateS2(result.Isobands); err != nil {
		t.Fatal(err)
	}
}

// signedArea is a ring's planar area, positive when counterclockwise.
func signedArea(ring orb.Ring) float64 {
	var area float64
	for i := 0; i+1 < len(ring); i++ {
		area += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	return area / 2
}
