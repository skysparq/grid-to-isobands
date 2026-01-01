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
		Grid:                gridValues,
		InitialTransform:    grid_to_isobands.SwapRightAndLeft,
		PostSmoothTransform: grid_to_isobands.RemoveTop10DegreesFromGlobal0p25Grid,
		Floor:               0,
		Step:                250,
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
		Grid:                gridValues,
		InitialTransform:    grid_to_isobands.SwapRightAndLeft,
		PostSmoothTransform: grid_to_isobands.RemoveTop10DegreesFromGlobal0p25Grid,
		Floor:               0,
		Step:                2,
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
