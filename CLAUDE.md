# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go library that converts gridded weather data (typically decoded from GRIB2) into isobands—filled contour polygons between successive levels—output as a GeoJSON FeatureCollection. It's used for weather visualization by transforming meteorological grid data (radar reflectivity, pressure, temperature, precipitation, visibility, etc.) into layered polygon features. Output is consumed downstream by S2-based geography engines (e.g. Snowflake GEOGRAPHY), so geometry validity matters.

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

The entry point is `IsobandsFromGrid(ctx context.Context, args *IsobandArgs) (*ReturnValues, error)`:

1. **Preprocesses** (`preprocessGrid`): Each `GridTransformer` in `args.Preprocesses` is applied to `args.Grid` **in place**, in order. The modified grid is returned as `ReturnValues.Grid`.
2. **Selections** (`selectGrid`): If `args.Selections` is non-empty, a copy of the grid's `Values` is made and each selection is applied to it. Only contouring sees this copy; `ReturnValues.Grid` is unaffected. `Lats`/`Lons` are shared, not copied, so selections must only change values, never reorder points. Use this for things like capping a maximum value for isobands only.
3. **Level calculation** (`preprocessArgs` / `GenerateLevels`): If `args.Levels` is set, it is used as-is. Otherwise the floor is raised to the lowest grid value at or above `Floor`, snapped down so it stays a whole number of `Step`s from the user-provided `Floor`, and levels run from there up past the grid's max (non-NaN) value. An all-NaN grid returns an empty FeatureCollection without calling Python.
4. **Contouring in Python** (`toIsobands` → `isobands.py`): The grid and levels are CBOR-encoded to `<WorkDir>/<uuid>.cbor` (`Values`/`Lats`/`Lons` are packed as raw little-endian float64 byte strings so Python can `np.frombuffer` them without a boxed-float list). The embedded script is piped to `python3 -` on stdin with the input and output paths as args; it writes `<WorkDir>/<uuid>.geojson`, which Go decodes. Both temp files are removed afterwards.
5. **Post-processing**: Features with nil coordinates are dropped, and `args.AddlProps` is set as the FeatureCollection's `properties`.

### Python Isoband Generation (`isobands.py`, embedded via `//go:embed`)

- Uses `contourpy.contour_generator` (serial, `FillType.ChunkCombinedOffsetOffset`, `quad_as_tri=True`) on the lon/lat/value grids and calls `filled(lower, upper)` for each adjacent pair of levels.
- Rings with fewer than 4 points or near-zero area (shoelace < 1e-9) are dropped (placeholders keep offsets aligned); a polygon whose shell is dropped is skipped.
- Invalid polygons (only when `is_valid` is false) are repaired by `repair()`: `make_valid` on the shell and on each hole separately, then the holes are subtracted. `make_valid` on the whole polygon can drop a real hole when the shell touches itself, making bands overlap (seen in the MRMS QPE data; `TestBandsPartitionTheGrid` covers it).
- Polygons covering more than 45% of the sphere (`MAX_SPHERE_FRACTION`, estimated with the d3/turf spherical-area formula) are split at the prime meridian with `shapely.intersection` (`split_at_prime_meridian`). Snowflake silently stores the complement of any polygon larger than a hemisphere (confirmed via `ST_AREA` on a near-global GFS visibility band). Each half spans at most 180° of longitude, so it fits in a hemisphere; thin bands that circle the globe stay whole. The cut follows a meridian (a great circle), so it needs no densifying while the polar rows are clipped (cut edges under 180°).
- `shapely.set_precision(grid_size=1e-9)` then collapses duplicate/near-duplicate vertices that S2 rejects. Results are flattened to Polygon parts, parts with area ≤ 1e-9 are dropped, and the rest are oriented to the GeoJSON right-hand rule (CCW shell, CW holes).
- Each feature is a single `Polygon` with properties `levelIndex`, `floor`, `ceiling`, and `nominal` (midpoint of floor and ceiling).

There is no geometry simplification or hole-punching pass.

### Grid Transformers

`GridTransformer` is `func(*GridValues)`. Constructors in `transform.go` (usable in either `Preprocesses` or `Selections`, subject to the Selections constraint above):

- Coordinate reordering (move values **and** Lats/Lons, so Preprocesses only): `SwapRightAndLeftTransformer` (e.g. GFS 0–360° longitudes → −180–180°), `ReverseVerticalTransformer`
- Smoothing/noise reduction: `GaussianTransformer(kernel, sigma)`, `BilateralTransformer(sigma, color)`, `MedianTransformer(kernel)`, `OpenCloseTransformer(kernel)`, `CloseOpenTransformer(kernel)`
- Masking: `ClipTransformer(transformers.Clip{Top, Bottom, Left, Right})` sets edge pixels to NaN; `ThresholdMaskTransformer(transformers.GreaterThan(x) | transformers.LessThan(x), replacement)`; `RemoveInfTransformer` converts ±Inf to NaN

The underlying algorithms live in the `transformers/` subpackage (`bilateral.go`, `gaussian.go` (separable, gonum), `median.go`, `morphology.go`, `clip.go`, `threshold.go`). NaN is the "no data" value throughout.

Missing coordinate-fixing Preprocesses (e.g. `SwapRightAndLeftTransformer` on global GFS grids) produce geometries that span the antimeridian incorrectly and fail to load downstream.

### Go Contourer (`contour/`, in progress)

A pure-Go replacement for the contourpy step, not yet wired into the pipeline. `contour.Contour(grid, levels)` returns a `Topology`: shared `Arcs` and, per band, `Polygons` (a `Shell` ring and the `Holes` directly inside it; islands in holes are polygons of their own). Rings are made of `ArcRef`s (arc index plus direction), so adjacent bands reference the same arc for their common boundary (groundwork for topology-preserving simplification). `PolygonCoords` expands a polygon to an `orb.Polygon` wound by the GeoJSON right-hand rule in X/Y (it flips rings when the grid's rows run north to south, detected from a middle cell); `RingCoords` expands one ring in traversal order (band on the left in index space); `IsHole` reports clockwise rings in index space.

- Matches contourpy's serial algorithm with `quad_as_tri=True`, `corner_mask=True`: 4 triangles per cell around the corner average; a cell with one NaN corner keeps the triangle of the other three; two or more NaN corners skip the cell. Bands hold `lower < z <= upper`.
- Vertices are identified by origin (`pointID`: a node, i.e. grid point or cell centre, or a level crossing a triangle edge between two nodes), never by float coordinates, so segments join exactly. A crossing exactly at a node is the node; values within `1e-9 × smallest level gap` of a level are snapped onto it, since centre averages land ULPs off levels (e.g. 0.9999999999999999) and would otherwise produce coincident points with different IDs.
- Opposite segments along an interior edge (a level lying exactly along it with the band on both sides) cancel. Where a band touches itself at a point, the tightest turn is taken and rings that still repeat a point are split, giving separate rings that share a vertex (S2-valid).
- Each hole goes to the smallest shell containing it (the containing shells are nested), tested by ray casting in index space from the midpoint of one of the hole's edges. Rings share vertices but never edges, so that point is never on a shell's boundary.
- Differs from the Python pipeline in two deliberate ways: a region contourpy splits along a zero-width line exactly at a band's exclusive lower level stays one polygon; and shared boundaries are built once, so adjacent bands can't overlap. (The pipeline's per-polygon `set_precision` leaves overlaps of up to ~1e-10 sq° between bands.)

### Meridian Split (`meridian/`)

The pure-Go counterpart of `split_at_prime_meridian` in `isobands.py`, for the Go pipeline. `SplitIfOversized(p)` returns `p` unchanged unless `SphereFraction(p)` (the d3/turf spherical-area formula, the same as the Python trigger) exceeds `MaxSphereFraction` (0.45); otherwise `Split(p)` cuts it into the pieces west and east of the prime meridian, wound by the right-hand rule. It works on output coordinates (not the topology), so it can run after future simplification.

- A polygon entirely on one side of 0° is returned unchanged. Otherwise the cut is at 0° unless a vertex lies exactly there (GFS has a grid column at 0°), in which case it moves east to halfway to the nearest vertex, so no vertex is ever on the cut and every crossing is clean.
- Each ring is split into runs on the side being traced. From each point where a run leaves the side, the boundary follows the cut (north for the west side, south for the east, keeping the interior on the left) to the next point where a run re-enters. Rings that pass through a point twice (a hole that touched its shell at a vertex, joined to the outer boundary by the cut) are split into simple loops. Holes off the cut go to the smallest containing shell.
- No GEOS: `orb/clip` is unsuitable because bounding-box clipping can leave zero-width edges along the cut, which S2 rejects.

### Other Files

- **geojson.go**: `FeatureCollection`, `Feature`, `Polygon` (coordinates as `[]orb.Ring`) and helpers `NewFeatureCollection`, `AddRing`, `AddPolygon`.

## External Dependencies

**Python requirement**: `python3` must be on `PATH` with `cbor2`, `numpy`, `contourpy`, and `shapely` (2.x, for `make_valid`/`set_precision`) installed.

**Go**: `fxamacker/cbor` (wire format), `google/uuid` (temp file names), `paulmach/orb` (ring types), `gonum` (Gaussian convolution), `skysparq/grib2-go` (tests only, for generating test data), `golang/geo` (S2 validation in `s2check/`). The Go contouring path (`contour/`, `meridian/`) is pure Go with no cgo.

**Work directory**: `WorkDir` defaults to `./tmp`. It is not created automatically—it must exist before calling `IsobandsFromGrid`.

## Key Data Structures

```go
type IsobandArgs struct {
    Grid         *GridValues
    Preprocesses []GridTransformer // applied to Grid in place
    Selections   []GridTransformer // applied to a values-only copy used for contouring
    Floor, Step  float64           // used to generate levels when Levels is empty
    Levels       []float64         // explicit levels; overrides Floor/Step
    AddlProps    map[string]any    // set as the FeatureCollection's properties
    WorkDir      string            // temp file directory (default ./tmp)
}

type GridValues struct {
    SizeX, SizeY int       // Grid dimensions
    Values       []float64 // Flat array, row-major order
    Lats, Lons   []float64 // Coordinates for each cell
}

type ReturnValues struct {
    Isobands *FeatureCollection
    Grid     *GridValues // grid after Preprocesses (not Selections)
}
```

## Testing

- `isobands_test.go`: End-to-end tests on real NOAA data (MRMS reflectivity/QPE/precip rate/precip flag, GFS pressure/temperature/visibility/humidity) loaded from `.test_files/*.json` (gitignored). Outputs are written to `./test-output/*.geojson` for visual inspection, then `saveTestOutput` fails the test if any feature fails `s2check.ValidatePolygon`; both `./test-output` and `./tmp` must exist. `TestSelectionsOnlyAffectIsobands`, `TestGenerateLevels`, `TestLargeBandsSplitAtPrimeMeridian`, `TestThinGlobalBandsStayWhole` and `TestBandsPartitionTheGrid` (a real QPE patch; bands must cover the grid exactly once) use synthetic or embedded data.
- `generate_test_data_test.go`: `TestGenerate*` tests download GRIB2 files from NOAA's public S3 buckets and write `.test_files/*.json`. Most are `t.Skip()`ped; enable selectively when new data is needed. Note that `TestGenerateFailedGfsHumidity100hpa`, `TestGenerateMultiSensorQpe`, `TestGeneratePrecipFlag`, and `TestGeneratePrecipRate` are currently **not** skipped, so `go test ./...` performs those downloads.
- `transform_test.go`: Unit tests for `SwapRightAndLeftTransformer` and `ReverseVerticalTransformer`.
- `contour/`: `TestMatchesContourpy` compares shell/hole counts, area, distinct vertices, and each polygon's area and hole count per band against `testdata/fixtures.json`, generated by `testdata/generate_fixtures.py` (run from the repo root; needs the Python dependencies) from contourpy with the pipeline's settings, repaired with `isobands.repair` and unioned per band. `TestTopologyInvariants` runs 300 random grids (continuous, integer, and 0.1-step decimal values, with NaN) and checks arc references (each contour-line arc used forward by the band above and reversed by the band below), S2 validity of every whole polygon, and that the bands exactly partition the unmasked area.
- `meridian/`: `TestSplit` covers hand-built shapes (a C crossing the cut, holes across and beside it, vertices and edges on 0°); `TestSplitContours` splits every polygon contoured from 100 random grids straddling 0° (north-to-south rows, a grid column at exactly 0°) and checks each piece with `s2check`, that no piece spans the cut, and that the pieces' areas add up to the polygon's; `TestSphereFraction` checks the area formula exactly on lon/lat rectangles.
- `s2check/`: `ValidatePolygon(rings []orb.Ring) error` checks a GeoJSON polygon against the rules S2/Snowflake GEOGRAPHY enforce: closed rings with at least 4 points, valid lon/lat, no zero-length edges, no vertex repeated within a ring, no crossing edges, no shared edges or rings crossing at a shared vertex, holes inside the shell and not nested, valid S2 nesting, and area under a hemisphere (a clockwise shell reads as its complement). golang/geo's `Polygon.Validate` doesn't detect self-intersections (an unported TODO), so crossings are found with `CrossingEdgeQuery`. Table tests in `s2check_test.go` cover each failure class.
