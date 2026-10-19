package grid_to_isobands

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/fxamacker/cbor"
	"github.com/google/uuid"
	"github.com/skysparq/grid-to-isobands/contour"
	"github.com/skysparq/grid-to-isobands/meridian"
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
	Grid *GridValues
	// Preprocesses clean up the grid itself: they're applied to Grid in place, and the result
	// is returned as ReturnValues.Grid.
	Preprocesses []GridTransformer
	// Selections choose which isobands are output without changing the grid: they run after
	// Preprocesses on a copy of Grid's Values that only the contouring sees. Lats and Lons are
	// shared with Grid rather than copied, so a selection must only change values, never
	// reorder points.
	Selections  []GridTransformer
	Floor, Step float64
	// Levels, if provided, is used as-is instead of generating levels from
	// Floor and Step.
	Levels    []float64
	AddlProps map[string]any
	// WorkDir holds the Python backend's temporary files.
	WorkDir string
	// Backend chooses how isobands are contoured. The default is BackendPython.
	Backend Backend
}

// Backend is how isobands are contoured.
type Backend int

const (
	// BackendPython pipes the grid to isobands.py (contourpy and shapely).
	BackendPython Backend = iota
	// BackendGo contours in Go with the contour and meridian packages, with no
	// Python, temporary files or repair step.
	BackendGo
)

// pyArgs is the wire format sent to the Python isoband-generation subprocess.
// Values, Lats, and Lons are packed as raw little-endian float64 bytes (see
// packFloat64) instead of CBOR arrays, so the Python side reads them directly
// into numpy via np.frombuffer rather than decoding a Python list of boxed
// floats first. For large grids (tens of millions of points), that
// intermediate list roughly doubles peak memory in the subprocess before any
// contouring work even starts.
type pyArgs struct {
	SizeX  int
	SizeY  int
	Values []byte
	Lats   []byte
	Lons   []byte
	Levels []float64
}

// packFloat64 packs a []float64 into a little-endian byte buffer for the
// CBOR byte-string wire format used by pyArgs.
func packFloat64(values []float64) []byte {
	buf := make([]byte, len(values)*8)
	for i, v := range values {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
	}
	return buf
}

type ReturnValues struct {
	Isobands *FeatureCollection
	Grid     *GridValues
}

func IsobandsFromGrid(ctx context.Context, args *IsobandArgs) (*ReturnValues, error) {
	preprocessGrid(args)
	isobands, err := toIsobands(ctx, args, selectGrid(args))
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: %w", err)
	}
	return &ReturnValues{
		Grid:     args.Grid,
		Isobands: isobands,
	}, nil
}

func preprocessArgs(args *IsobandArgs, grid *GridValues) {
	if args.WorkDir == "" {
		args.WorkDir = `./tmp`
	}
	if len(args.Levels) > 0 {
		return
	}
	floor, step := args.Floor, args.Step
	steps := math.Floor((sliceMinFromFloor(grid.Values, floor) - floor) / step)
	args.Floor = floor + steps*step
}

func preprocessGrid(args *IsobandArgs) {
	for _, preprocess := range args.Preprocesses {
		preprocess(args.Grid)
	}
}

// selectGrid returns the grid to contour: Grid itself when there are no Selections, otherwise
// a copy of its values with Selections applied, so Grid is left as Preprocesses produced it.
func selectGrid(args *IsobandArgs) *GridValues {
	if len(args.Selections) == 0 {
		return args.Grid
	}
	selected := &GridValues{
		SizeX:  args.Grid.SizeX,
		SizeY:  args.Grid.SizeY,
		Values: slices.Clone(args.Grid.Values),
		Lats:   args.Grid.Lats,
		Lons:   args.Grid.Lons,
	}
	for _, selection := range args.Selections {
		selection(selected)
	}
	return selected
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

func toIsobands(ctx context.Context, args *IsobandArgs, grid *GridValues) (*FeatureCollection, error) {
	preprocessArgs(args, grid)
	maxVal := slicesMaxNotNaN(grid.Values)
	if math.IsNaN(maxVal) {
		return &FeatureCollection{Features: []Feature{}}, nil
	}
	levels := args.Levels
	if len(levels) == 0 {
		levels = GenerateLevels(args.Floor, maxVal, args.Step)
	}

	var isobands *FeatureCollection
	var err error
	switch args.Backend {
	case BackendPython:
		isobands, err = pythonIsobands(ctx, args, grid, levels)
	case BackendGo:
		isobands, err = goIsobands(ctx, grid, levels)
	default:
		err = fmt.Errorf("unknown backend %d", args.Backend)
	}
	if err != nil {
		return nil, fmt.Errorf("error generating isobands: %w", err)
	}
	isobands.Features = slices.DeleteFunc(isobands.Features, func(f Feature) bool { return f.Geometry.Coordinates == nil })
	isobands.Properties = args.AddlProps
	return isobands, nil
}

// goIsobands contours the grid in Go, splitting any polygon too large for S2
// engines at the prime meridian, as isobands.py does.
func goIsobands(ctx context.Context, grid *GridValues, levels []float64) (*FeatureCollection, error) {
	topo, err := contour.Contour(&contour.Grid{NX: grid.SizeX, NY: grid.SizeY, X: grid.Lons, Y: grid.Lats, Z: grid.Values}, levels)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	isobands := &FeatureCollection{Type: "FeatureCollection", Features: []Feature{}}
	for k, band := range topo.Bands {
		for _, polygon := range band.Polygons {
			pieces, err := meridian.SplitIfOversized(topo.PolygonCoords(polygon))
			if err != nil {
				return nil, fmt.Errorf("level %d: %w", k, err)
			}
			for _, piece := range pieces {
				isobands.AddPolygon(piece, map[string]any{
					// levelIndex is a float64 as it is when decoded from the
					// Python backend's JSON, so either backend's properties
					// assert the same way.
					"levelIndex": float64(k),
					"floor":      band.Lower,
					"ceiling":    band.Upper,
					"nominal":    band.Lower + (band.Upper-band.Lower)/2,
				})
			}
		}
	}
	return isobands, nil
}

// pythonIsobands contours the grid with isobands.py.
func pythonIsobands(ctx context.Context, args *IsobandArgs, grid *GridValues, levels []float64) (*FeatureCollection, error) {
	jobId := uuid.NewString()
	pyData := &pyArgs{
		SizeX:  grid.SizeX,
		SizeY:  grid.SizeY,
		Values: packFloat64(grid.Values),
		Lats:   packFloat64(grid.Lats),
		Lons:   packFloat64(grid.Lons),
		Levels: levels,
	}

	inPath := gridPath(jobId, args.WorkDir)
	in, err := os.Create(inPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create input file: %w", err)
	}
	encoder := cbor.NewEncoder(in, cbor.EncOptions{})
	err = encoder.Encode(pyData)
	_ = in.Close()
	defer func() { _ = os.Remove(inPath) }()
	if err != nil {
		return nil, fmt.Errorf("failed to encode input file: %w", err)
	}

	outPath := isobandPath(jobId, args.WorkDir)

	err = execCmd(ctx, `python3`, inPath, outPath)
	defer func() { _ = os.Remove(outPath) }()
	if err != nil {
		return nil, err
	}

	banded, err := os.Open(outPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open isobands: %w", err)
	}
	defer func() { _ = banded.Close() }()

	decoder := json.NewDecoder(banded)
	isobands := &FeatureCollection{}
	err = decoder.Decode(isobands)
	if err != nil {
		return nil, fmt.Errorf("failed to decode isobands: %w", err)
	}
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

func execCmd(ctx context.Context, name string, args ...string) error {
	stdErr := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	stdOut := bytes.NewBuffer(make([]byte, 0, 1024*1024))

	fullArgs := append([]string{"-"}, args...)
	cmd := exec.CommandContext(ctx, name, fullArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = stdErr
	cmd.Stdout = stdOut

	if err := cmd.Start(); err != nil {
		return err
	}

	writeErr := func() error {
		if _, err := stdin.Write(pyScript); err != nil {
			return err
		}
		return stdin.Close() // sends EOF to Python
	}()

	if writeErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // reap regardless -- prevents a zombie + a leaked goroutine
		return fmt.Errorf("error writing script to stdin: %w", writeErr)
	}

	if err = cmd.Wait(); err != nil {
		return fmt.Errorf("error executing %v: %w\nstderr: %s\nstdout: %s",
			name, err, stdErr.String(), stdOut.String())
	}

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
