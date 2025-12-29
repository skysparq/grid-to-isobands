package grid_to_isobands_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/skysparq/grib2-go/file"
	"github.com/skysparq/grib2-go/templates"
)

type TestData struct {
	SizeX, SizeY       int
	Lats, Lngs, Values []float64
}

func TestGenerateTestData(t *testing.T) {
	t.Skip()
	err := generateTestData(
		"https://noaa-mrms-pds.s3.amazonaws.com/CONUS/MergedBaseReflectivityQC_00.50/20251211/MRMS_MergedBaseReflectivityQC_00.50_20251211-235917.grib2.gz",
		"mrms-base-reflectivity.json",
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = generateTestData(
		"https://noaa-gfs-bdp-pds.s3.amazonaws.com/gfs.20251002/12/atmos/gfs.t12z.pgrb2.0p25.f022",
		"gfs-baro-pressure-msl.json",
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func generateTestData(url string, dest string, index int) error {
	err := os.MkdirAll(`./.test_files`, 0755)
	if err != nil {
		return fmt.Errorf("failed to create test files directory: %w", err)
	}
	var gribR io.Reader
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download test file: %w", err)
	}
	gribR = resp.Body
	defer func() { _ = resp.Body.Close() }()
	if filepath.Ext(url) == ".gz" {
		gribR, err = gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to decompress test file: %w", err)
		}
	}

	grib := file.NewGribFile(gribR, templates.Version33())

	for indexed, err := range grib.Records {
		if indexed.MessageNumber != index {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to read grib record: %w", err)
		}
		dataRep, err := indexed.Record.DataRepresentation.Definition()
		if err != nil {
			return fmt.Errorf("failed to read data representation: %w", err)
		}
		gridDef, err := indexed.Record.Grid.Definition()
		if err != nil {
			return fmt.Errorf("failed to read grid definition: %w", err)
		}
		points, err := gridDef.Points()
		if err != nil {
			return fmt.Errorf("failed to read grid points: %w", err)
		}

		values, err := dataRep.GetValues(indexed.Record)
		data := TestData{
			SizeX:  gridDef.XVals(),
			SizeY:  gridDef.YVals(),
			Lats:   points.Lats,
			Lngs:   points.Lngs,
			Values: values,
		}
		f, err := os.Create(filepath.Join(`./.test_files`, dest))
		if err != nil {
			return fmt.Errorf("failed to create test file: %w", err)
		}
		encoder := json.NewEncoder(f)
		err = encoder.Encode(data)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("failed to write test file: %w", err)
		}
		return nil
	}
	return nil
}
