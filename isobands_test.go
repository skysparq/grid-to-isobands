package grid_to_isobands_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	grid_to_isobands "github.com/skysparq/grid-to-isobands"
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
			grid_to_isobands.BilateralTransformer(3, 5),
			grid_to_isobands.CloseOpenTransformer(7),
			grid_to_isobands.GaussianTransformer(9, 2.0),
			grid_to_isobands.RemoveInfTransformer(),
		},
		Grid:  gridValues,
		Floor: 5,
		Step:  2.5,
		AddlProps: map[string]any{
			`measure`: `base-reflectivity`,
			`at`:      time.Date(2025, 12, 11, 23, 59, 17, 0, time.UTC),
		},
	}
	isobands, err := grid_to_isobands.IsobandsFromGrid(args)
	if err != nil {
		t.Fatal(err)
	}
	isobandJson, err := json.Marshal(isobands)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(`./test-output/mrms-base-reflectivity.geojson`, isobandJson, 0644)
	if err != nil {
		t.Fatal(err)
	}
}

/*
	func TestMrmsBaseReflectivity2(t *testing.T) {
		testData, err := getTestData(`mrms-base-reflectivity-2.json`)
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
			Grid:  gridValues,
			Floor: 5,
			Step:  2.5,
			AddlProps: map[string]any{
				`measure`: `base-reflectivity`,
				`at`:      time.Date(2026, 1, 6, 17, 2, 0, 0, time.UTC),
			},
			Tolerance: 1000,
			Clip:      grid_to_isobands.Clip{Top: 1, Bottom: 1, Left: 1, Right: 1},
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `base-reflectivity-2.json`)
		if err != nil {
			t.Fatal(err)
		}
	}

	func TestMrmsCompositeReflectivity(t *testing.T) {
		testData, err := getTestData(`mrms-composite-reflectivity.json`)
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
			Grid:  gridValues,
			Floor: 5,
			Step:  2.5,
			AddlProps: map[string]any{
				`measure`: `composite-reflectivity`,
				`at`:      time.Date(2026, 1, 7, 19, 2, 36, 0, time.UTC),
			},
			Tolerance: 1000,
			Clip:      grid_to_isobands.Clip{Top: 1, Bottom: 1, Left: 1, Right: 1},
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `composite-reflectivity.json`)
		if err != nil {
			t.Fatal(err)
		}
	}

	func TestMrmsCompositeReflectivity2(t *testing.T) {
		testData, err := getTestData(`mrms-composite-reflectivity-2.json`)
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
			Grid:  gridValues,
			Floor: 5,
			Step:  2.5,
			AddlProps: map[string]any{
				`measure`: `composite-reflectivity`,
				`at`:      time.Date(2026, 1, 7, 20, 30, 37, 0, time.UTC),
			},
			Tolerance: 1000,
			Clip:      grid_to_isobands.Clip{Top: 1, Bottom: 1, Left: 1, Right: 1},
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `composite-reflectivity-2.json`)
		if err != nil {
			t.Fatal(err)
		}
	}

	func TestGfsBaroPressure(t *testing.T) {
		testData, err := getTestData(`gfs-baro-pressure-msl.json`)
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
				`at`:      time.Date(2025, 10, 2, 12, 0, 0, 0, time.UTC),
			},
			WorkDir:   "./tmp",
			Tolerance: 5000,
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `baro-pressure-msl.json`)
		if err != nil {
			t.Fatal(err)
		}
	}

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

	func TestSurfaceTemp(t *testing.T) {
		testData, err := getTestData(`temperature-surface.json`)
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
			Step:             2,
			AddlProps: map[string]any{
				`measure`: `temperature-surface`,
				`at`:      time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			},
			WorkDir:   "./tmp",
			Tolerance: 5000,
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `temperature-surface.json`)
		if err != nil {
			t.Fatal(err)
		}
	}

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

	func TestVisibilitySurface(t *testing.T) {
		testData, err := getTestData(`visibility-surface.json`)
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
			Step:             2500,
			AddlProps: map[string]any{
				`measure`: `visibility-surface`,
				`at`:      time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			},
			WorkDir:   "./tmp",
			Tolerance: 5000,
		}
		isogons, err := grid_to_isobands.IsobandsFromGrid(args)
		if err != nil {
			t.Fatal(err)
		}

		err = saveTestOutput(isogons, `visibility-surface.json`)
		if err != nil {
			t.Fatal(err)
		}
	}

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

func saveTestOutput(isogons *grid_to_isobands.FeatureCollection, name string) error {
	out, err := os.Create(filepath.Join(`./test-output`, name))
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
	if err != nil {
		return err
	}
	return nil
}
