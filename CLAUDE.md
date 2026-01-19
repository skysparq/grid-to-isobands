# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go library that converts gridded weather data (GRIB2 format) into isobands—contour-based geographic polygons output as GeoJSON FeatureCollections. It's used for weather visualization by transforming meteorological grid data (radar reflectivity, pressure, temperature, wind, etc.) into layered polygon features.

## Build and Test Commands

```bash
# Run all tests
go test ./...

# Run a specific test
go test -run TestMrmsBaseReflectivity

# Run tests with verbose output
go test -v ./...

# Build (library package)
go build ./...
```

## Architecture

### Processing Pipeline (`isobands.go`)

The main entry point is `IsobandsFromGrid(args IsobandArgs)` which orchestrates:

1. **Preprocessing** (`preprocessGrid`): Applies initial transforms, replaces sentinel values (NaN/Inf), runs morphological open-close operations, applies Gaussian smoothing
2. **Contour Generation** (`createIsogons`): Uses `fogleman/contourmap` to create contours at each elevation level
3. **R Script Processing** (`toIsobands`): Calls embedded R script for geometry validation, simplification, and hole-punching between levels
4. **Quadrant Splitting** (`splitToQuadrants`): Divides polygons into 4 geographic quadrants

### Key Components

- **transform.go**: Grid transformations (`SwapRightAndLeft`, `ReverseVertical`, `RemoveTopAndBottom`) for handling different GRIB2 coordinate conventions
- **morphology.go**: Image processing operations (erode, dilate, open, close) for noise reduction
- **gaussian.go**: Separable 2D Gaussian convolution for grid smoothing
- **geojson.go**: GeoJSON data structures for output
- **isobands.R**: Embedded R script that validates geometries, simplifies with topology preservation, and handles hole-punching logic

### External Dependencies

**R Requirement**: The pipeline requires R with `sf`, `dplyr`, and `s2` packages. The R script is embedded via `//go:embed` and executed via `Rscript`.

**Temporary Files**: Processing creates temporary GeoJSON files in the work directory (default `./tmp`) for R script communication.

## Key Data Structures

```go
type IsobandArgs struct {
    Grid             GridValues         // Input grid data
    InitialTransform InitialTransformer // Optional grid transformation
    Floor, Step      float64            // Contour floor value and interval
    Tolerance        float64            // Simplification tolerance in meters
    Clip             Clip               // Edge pixels to remove
    WorkDir          string             // Temp file directory
}

type GridValues struct {
    SizeX, SizeY int       // Grid dimensions
    Values       []float64 // Flat array, row-major order
    Lats, Lons   []float64 // Coordinates for each cell
}
```

## Testing

Tests use real NOAA weather data (MRMS, GFS) stored in `.test_files/` as JSON. Some tests are skipped by default to avoid AWS downloads—enable them selectively when needed.

Test data includes: radar reflectivity, barometric pressure, surface temperature, wind components, and visibility.