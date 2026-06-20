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

	"github.com/fxamacker/cbor"
	"github.com/google/uuid"
)

//go:embed isobands.py
var pyScript []byte

type GridValues struct {
	SizeX  int
	SizeY  int
	Values []float64
	Lats   []float64
	Lons   []float64
}

type IsobandArgs struct {
	Grid         *GridValues
	Preprocesses []GridTransformer
	Floor, Step  float64
	AddlProps    map[string]any
	WorkDir      string
}

type pyArgs struct {
	*GridValues
	Levels []float64
}

type ReturnValues struct {
	Isobands *FeatureCollection
	Grid     *GridValues
}

func IsobandsFromGrid(args *IsobandArgs) (*ReturnValues, error) {
	preprocessGrid(args)
	isobands, err := toIsobands(args)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: %w", err)
	}
	return &ReturnValues{
		Grid:     args.Grid,
		Isobands: isobands,
	}, nil
}

func preprocessArgs(args *IsobandArgs) {
	grid, floor, step := args.Grid, args.Floor, args.Step
	if args.WorkDir == "" {
		args.WorkDir = `./tmp`
	}
	floor = math.Floor(sliceMinFromFloor(grid.Values, floor)/step) * step
	args.Floor = floor
}

func preprocessGrid(args *IsobandArgs) {
	for _, preprocess := range args.Preprocesses {
		preprocess(args.Grid)
	}
}

func GenerateLevels(start, stop, step float64) []float64 {
	levels := make([]float64, 0, int(stop-start+1))
	i := 0.0
	for i = start; i <= stop; i += step {
		levels = append(levels, i)
	}
	levels = append(levels, i)
	return levels
}

func toIsobands(args *IsobandArgs) (*FeatureCollection, error) {
	preprocessArgs(args)
	jobId := uuid.NewString()
	pyData := &pyArgs{
		GridValues: args.Grid,
		Levels:     GenerateLevels(args.Floor, slicesMaxNotNaN(args.Grid.Values), args.Step),
	}

	inPath := gridPath(jobId, args.WorkDir)
	in, err := os.Create(inPath)
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to create input file: %w", err)
	}
	encoder := cbor.NewEncoder(in, cbor.EncOptions{})
	err = encoder.Encode(pyData)
	_ = in.Close()
	defer func() { _ = os.Remove(inPath) }()
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: failed to encode input file: %w", err)
	}

	outPath := isobandPath(jobId, args.WorkDir)

	err = execCmd(`python3`, inPath, outPath)
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
			slices.Delete(isobands.Features, i, i+1)
			i--
		}
	}
	isobands.Properties = args.AddlProps
	return isobands, nil
}

func slicesMaxNotNaN(s []float64) float64 {
	maxVal := math.NaN()
	for _, n := range s {
		if math.IsNaN(n) {
			continue
		}
		if math.IsNaN(maxVal) {
			maxVal = n
		}
		if n > maxVal {
			maxVal = n
		}
	}
	return maxVal
}

func minMax(values []float64, floor float64) (float64, float64) {
	minVal, maxVal := math.NaN(), math.NaN()
	for _, val := range values {
		minVal = minFloor(floor, minVal, val)
		maxVal = maxFloor(floor, maxVal, val)
	}
	return minVal, maxVal

}

func sliceMinFromFloor(values []float64, floor float64) float64 {
	minVal := math.NaN()
	for _, val := range values {
		minVal = minFloor(floor, minVal, val)
	}
	return minVal
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

func execCmd(name string, args ...string) error {
	stdErr := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	stdOut := bytes.NewBuffer(make([]byte, 0, 1024*1024))

	fullArgs := append([]string{"-"}, args...)
	cmd := exec.Command(name, fullArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = stdErr
	cmd.Stdout = stdOut

	if err := cmd.Start(); err != nil {
		return err
	}

	// Write the script
	if _, err := stdin.Write(pyScript); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill() // best-effort
		return err
	}

	_ = stdin.Close() // ← This sends EOF to Python → crucial!

	if err = cmd.Wait(); err != nil {
		return fmt.Errorf("error executing %v: %w\nstderr: %s\nstdout: %s",
			name, err, stdErr.String(), stdOut.String())
	}

	// Optionally: check stdOut / process output here if needed
	return nil
}

func gridPath(jobId, workDir string) string {
	return tmpFilePath(jobId+`.cbor`, workDir)
}

func isobandPath(jobId, workDir string) string {
	return tmpFilePath(jobId+`.geojson`, workDir)
}

func tmpFilePath(filename, workDir string) string {
	path := filepath.Join(workDir, filename)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}
