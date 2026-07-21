import json
import sys

import cbor2
import numpy as np
import contourpy
import shapely
from shapely.geometry import Polygon
from shapely.geometry.polygon import orient
from shapely.validation import make_valid


def polygon_geoms(geom):
    # Flatten a (possibly repaired) geometry down to its Polygon components.
    # make_valid on a self-intersecting Polygon commonly returns a
    # MultiPolygon (e.g. a bowtie splits into two disjoint pieces), and can
    # rarely return a GeometryCollection mixing polygons with lower-dimension
    # leftovers (stray points/lines) that aren't meaningful isobands.
    if geom.is_empty:
        return
    if geom.geom_type == "Polygon":
        yield geom
    elif geom.geom_type in ("MultiPolygon", "GeometryCollection"):
        for part in geom.geoms:
            yield from polygon_geoms(part)


def polygon_to_rings(polygon):
    # orient() enforces the GeoJSON right-hand-rule convention (CCW shell,
    # CW holes), which repaired/reassembled polygons don't otherwise guarantee.
    polygon = orient(polygon, sign=1.0)
    rings = [polygon.exterior] + list(polygon.interiors)
    return [[list(point) for point in ring.coords] for ring in rings]


def ring_area(ring):
    # Shoelace formula. Used to catch rings that have enough points to pass the
    # length check but are geometrically degenerate (e.g. a pinch point where
    # marching squares revisits the same spot), which S2/Snowflake reject as
    # "empty loops" even though they're valid, non-empty GeoJSON coordinates.
    area = 0.0
    n = len(ring)
    for i in range(n):
        x1, y1 = ring[i]
        x2, y2 = ring[(i + 1) % n]
        area += x1 * y2 - x2 * y1
    return abs(area) / 2.0


def grid_to_isobands(values, lats, lons, nx, ny, levels):
    xi_grid = np.reshape(lons, (ny, nx))
    yi_grid = np.reshape(lats, (ny, nx))
    zi_grid = np.reshape(values, (ny, nx))
    cont_gen = contourpy.contour_generator(x=xi_grid, y=yi_grid, z=zi_grid, name='serial',
                                           fill_type=contourpy.FillType.ChunkCombinedOffsetOffset,
                                           quad_as_tri=True)

    features = []
    for i in range(len(levels) - 1):
        lower = levels[i]
        upper = levels[i + 1]
        filled = cont_gen.filled(lower, upper)

        if filled:
            # Parse the filled output: list of chunks, each chunk has points and offsets
            for points, boundary_offsets, poly_offsets in zip(*filled):
                if points is None:
                    continue
                transf_points = [list(point) for point in points]

                # boundaries stays index-aligned with boundary_offsets: rings with too
                # few points become an empty placeholder rather than being skipped,
                # since poly_offsets indexes into this list positionally.
                boundaries = []
                for j in range(len(boundary_offsets) - 1):
                    start = boundary_offsets[j]
                    end = boundary_offsets[j + 1]
                    ring = transf_points[start:end]
                    boundaries.append(ring if len(ring) >= 4 and ring_area(ring) > 1e-9 else [])

                for j in range(len(poly_offsets) - 1):
                    start = poly_offsets[j]
                    end = poly_offsets[j + 1]
                    rings = boundaries[start:end]
                    if not rings or not rings[0]:
                        continue
                    shell, *holes = [rings[0]] + [ring for ring in rings[1:] if ring]
                    candidate = Polygon(shell, holes)
                    # make_valid is relatively expensive, so only pay for it
                    # when contourpy has actually produced a self-intersecting
                    # (or otherwise invalid) ring -- the common case is valid.
                    # set_precision's docs warn results are unexpected on
                    # invalid input, so validity must be repaired first.
                    repaired = candidate if candidate.is_valid else make_valid(candidate)
                    # quad_as_tri routinely emits the same point twice where a
                    # filled boundary runs along a grid edge shared by two
                    # adjacent triangles -- a zero-length edge that GEOS
                    # doesn't consider invalid (is_valid is True either way)
                    # but that S2 rejects as crossing its real neighboring
                    # edge. set_precision's own topology rebuild collapses
                    # these (and any other near-duplicate vertices) into a
                    # guaranteed-valid result, unlike a hand-rolled per-point
                    # distance check, which can silently merge two distinct,
                    # legitimately close vertices and introduce a *new*
                    # self-intersection instead of removing one.
                    cleaned = shapely.set_precision(repaired, grid_size=1e-9)
                    for polygon in polygon_geoms(cleaned):
                        features.append({
                            "type": "Feature",
                            "properties": {
                                "levelIndex": i,
                                "floor": lower,
                                "ceiling": upper,
                            },
                            "geometry": {
                                "type": "Polygon",
                                "coordinates": polygon_to_rings(polygon),
                            }
                        })
    return {
        "type": "FeatureCollection",
        "features": features
    }


if __name__ == '__main__':
    in_path = sys.argv[1]
    out_path = sys.argv[2]

    with open(in_path, 'rb') as inf:
        data = cbor2.load(inf)

    rows = data['SizeY']
    cols = data['SizeX']
    # Values/Lats/Lons now arrive as packed little-endian float64 byte strings rather than CBOR
    # arrays, so np.frombuffer wraps them directly with no intermediate Python list of boxed floats.
    vals = np.frombuffer(data['Values'], dtype='<f8')
    lats = np.frombuffer(data['Lats'], dtype='<f8')
    lons = np.frombuffer(data['Lons'], dtype='<f8')
    levels = np.array(data['Levels'])
    del data

    isobands = grid_to_isobands(vals, lats, lons, cols, rows, levels)

    with open(out_path, 'w') as outf:
        json.dump(isobands, outf)
