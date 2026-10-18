import json
import math
import sys

import cbor2
import numpy as np
import contourpy
import shapely
from shapely.geometry import Polygon, box
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


# The western and eastern halves of the globe, split along the prime meridian.
HALVES = (box(-180, -90, 0, 90), box(0, -90, 180, 90))

# Polygons covering more than this fraction of the sphere are split. Snowflake
# flips at one half; the margin covers the area approximation and rounding.
MAX_SPHERE_FRACTION = 0.45


def ring_sphere_fraction(coords):
    # Area enclosed by a lon/lat ring as a fraction of the sphere (the formula
    # used by d3-geo and turf). Accurate for short edges like grid-cell steps.
    total = 0.0
    for (lon1, lat1), (lon2, lat2) in zip(coords, coords[1:]):
        total += math.radians(lon2 - lon1) * (2 + math.sin(math.radians(lat1)) + math.sin(math.radians(lat2)))
    return abs(total) / (8 * math.pi)


def sphere_fraction(polygon):
    return ring_sphere_fraction(polygon.exterior.coords) - sum(
        ring_sphere_fraction(hole.coords) for hole in polygon.interiors)


def split_at_prime_meridian(geom):
    # S2 engines read a polygon larger than a hemisphere as its complement:
    # Snowflake loads it without error but stores the smaller side, so a
    # near-global band silently becomes everything outside it. Polygons near
    # or over half the sphere are split at 0 degrees into halves that each
    # span at most 180 degrees of longitude, so each lies within a hemisphere.
    # (The antimeridian is already a boundary, since grids crossing it are
    # clipped there.) Thin bands that circle the globe are well under half the
    # sphere and stay whole. The cut edge runs along a meridian, a great
    # circle, so S2 draws it exactly as GEOS does and it needs no densifying
    # while it stays under 180 degrees, i.e. while the polar rows are clipped.
    # intersection, unlike clip_by_rect, always returns valid geometry.
    if sum(sphere_fraction(p) for p in polygon_geoms(geom)) <= MAX_SPHERE_FRACTION:
        return [geom]
    return [shapely.intersection(geom, half) for half in HALVES]


def polygonal(geom):
    # The polygon parts of a make_valid result as one geometry, so overlay
    # operations never see the stray points and lines a GeometryCollection
    # can carry.
    return shapely.unary_union(list(polygon_geoms(geom)))


def repair(shell, holes):
    # Repairs the shell and each hole on its own, then subtracts the holes.
    # make_valid on the whole polygon can drop a real hole when the shell
    # touches itself (seen in MRMS QPE data, where a plateau sits exactly on a
    # level), leaving that area in this band as well as the band it belongs
    # to, so the bands overlap.
    region = polygonal(make_valid(Polygon(shell)))
    if holes:
        region = region.difference(polygonal(shapely.unary_union([make_valid(Polygon(h)) for h in holes])))
    return region


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
        nominal = lower + ((upper - lower) / 2)
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
                    # Repair is relatively expensive, so only pay for it when
                    # contourpy has actually produced a self-intersecting (or
                    # otherwise invalid) ring -- the common case is valid.
                    # set_precision's docs warn results are unexpected on
                    # invalid input, so validity must be repaired first.
                    repaired = candidate if candidate.is_valid else repair(shell, holes)
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
                    for part in split_at_prime_meridian(repaired):
                        cleaned = shapely.set_precision(part, grid_size=1e-9)
                        for polygon in polygon_geoms(cleaned):
                            # Splitting along a polygon edge that lies exactly
                            # on the meridian can leave zero-area slivers.
                            if polygon.area <= 1e-9:
                                continue
                            features.append({
                                "type": "Feature",
                                "properties": {
                                    "levelIndex": i,
                                    "floor": lower,
                                    "ceiling": upper,
                                    "nominal": nominal
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
