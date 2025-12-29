package grid_to_isobands_test

import (
	"encoding/json"
	"fmt"
	"grid-to-isobands"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	isogons, err := grid_to_isobands.IsobandsFromGrid(gridValues, 5, 2.5, map[string]any{
		`measure`: `base-reflectivity`,
		`at`:      time.Date(2025, 12, 11, 23, 59, 17, 0, time.UTC),
	})
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
