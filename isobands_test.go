package grid_to_isobands_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/skysparq/grid-to-isobands"
)

func TestMrmsBaseReflectivity(t *testing.T) {
	testData, err := getTestData(`mrms-base-reflectivity.json`)
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
			`at`:      time.Date(2025, 12, 11, 23, 59, 17, 0, time.UTC),
		},
		Tolerance: 1000,
		Clip:      grid_to_isobands.Clip{Top: 1, Bottom: 1, Left: 1, Right: 1},
	}
	isogons, err := grid_to_isobands.IsobandsFromGrid(args)
	if err != nil {
		t.Fatal(err)
	}

	out, err := os.Create(`./base-reflectivity.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
	if err != nil {
		t.Fatal(err)
	}
}

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

	out, err := os.Create(`./base-reflectivity-2.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
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

	out, err := os.Create(`./composite-reflectivity.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
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

	out, err := os.Create(`./baro-pressure-msl.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
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

	out, err := os.Create(`./temperature-surface.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
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

	out, err := os.Create(`./wind-u-100hpa.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
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

	out, err := os.Create(`./visibility-surface.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
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

	out, err := os.Create(`./reflectivity-atmosphere.json`)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	err = encoder.Encode(isogons)
	if err != nil {
		t.Fatal(err)
	}
}

func getTestData(filename string) (TestData, error) {
	var testData TestData
	f, err := os.Open(filepath.Join(`./.test_files`, filename))
	if err != nil {
		return testData, fmt.Errorf(`error opening file: %w`, err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&testData)
	_ = f.Close()
	if err != nil {
		return testData, fmt.Errorf(`error decoding test data: %w`, err)
	}
	return testData, nil
}
