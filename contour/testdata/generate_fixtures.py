# Generates fixtures.json: each band's region for small synthetic grids, from
# contourpy's filled contours with the settings isobands.py uses, so the Go
# contourer can be checked against contourpy after the Python dependency is
# gone.
#
# Run from the repo root: python3 contour/testdata/generate_fixtures.py
import json
import math
import os

import sys

import contourpy
import numpy as np
import shapely

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
import isobands  # noqa: E402

NAN = float('nan')


def uniform(z):
    z = np.array(z, dtype=float)
    ny, nx = z.shape
    x, y = np.meshgrid(np.arange(nx, dtype=float), np.arange(ny, dtype=float))
    return x, y, z


def curvilinear(z):
    # Non-uniform, non-rectangular cells, to pin down how the centre point and
    # crossings are interpolated in coordinate space.
    z = np.array(z, dtype=float)
    ny, nx = z.shape
    i, j = np.meshgrid(np.arange(nx, dtype=float), np.arange(ny, dtype=float))
    return i + 0.3 * j + 0.05 * j * j, 1.5 * j + 0.1 * i * i, z


def smooth(nx, ny, seed):
    rng = np.random.default_rng(seed)
    i, j = np.meshgrid(np.arange(nx, dtype=float), np.arange(ny, dtype=float))
    z = np.zeros((ny, nx))
    for _ in range(6):
        cx, cy = rng.uniform(0, nx), rng.uniform(0, ny)
        r, h = rng.uniform(2, nx / 2), rng.uniform(-5, 10)
        z += h * np.exp(-((i - cx) ** 2 + (j - cy) ** 2) / (2 * r * r))
    return z


def with_nans(z, fraction, seed):
    z = np.array(z, dtype=float)
    rng = np.random.default_rng(seed)
    z[rng.uniform(size=z.shape) < fraction] = NAN
    return z


def filled(x, y, z, levels):
    # contourpy's filled contours with the settings isobands.py uses, repaired
    # into each band's true region:
    # - zero-area rings are dropped, as isobands.py does;
    # - every polygon is repaired with isobands.repair (shell and holes
    #   separately, holes subtracted), even valid ones, since the result is
    #   the same and it keeps this independent of the is_valid shortcut;
    # - each band's polygons are unioned. contourpy splits a region along a
    #   line of points exactly at the band's (exclusive) lower level, which has
    #   no area, while the Go contourer keeps the region whole. Union joins
    #   polygons sharing an edge but keeps ones touching at a point separate,
    #   as the Go contourer does.
    gen = contourpy.contour_generator(x=x, y=y, z=z, name='serial',
                                      fill_type=contourpy.FillType.ChunkCombinedOffsetOffset,
                                      quad_as_tri=True)
    bands = []
    for lower, upper in zip(levels, levels[1:]):
        regions = []
        for points, offsets, outer in zip(*gen.filled(lower, upper)):
            if points is None:
                continue
            for k in range(len(outer) - 1):
                rings = [points[offsets[r]:offsets[r + 1]].tolist() for r in range(outer[k], outer[k + 1])]
                rings = [r if len(r) >= 4 and isobands.ring_area(r) > 1e-9 else [] for r in rings]
                if not rings[0]:
                    continue
                regions.append(isobands.repair(rings[0], [r for r in rings[1:] if r]))
        shells = holes = 0
        area = 0.0
        vertices = set()
        polygons = []
        for polygon in isobands.polygon_geoms(shapely.unary_union(regions)):
            shells += 1
            holes += len(polygon.interiors)
            area += polygon.area
            polygons.append([polygon.area, len(polygon.interiors)])  # sorted by holes, then area
            for ring in [polygon.exterior, *polygon.interiors]:
                vertices.update((round(px, 9), round(py, 9)) for px, py in ring.coords)
        bands.append({'lower': lower, 'upper': upper, 'shells': shells, 'holes': holes,
                      'area': area, 'vertices': sorted(vertices), 'polygons': sorted(polygons, key=lambda p: (p[1], p[0]))})
    return bands


def case(name, xyz, levels):
    x, y, z = xyz
    ny, nx = z.shape
    def flat(a):
        return [None if math.isnan(v) else v for v in a.ravel().tolist()]
    return {'name': name, 'nx': nx, 'ny': ny, 'x': flat(x), 'y': flat(y), 'z': flat(z),
            'levels': levels, 'bands': filled(x, y, z, levels)}


rng = np.random.default_rng(7)
cases = [
    case('flat at a level', uniform([[1, 1], [1, 1]]), [0, 1, 2]),
    case('peak corner', uniform([[0, 0], [0, 4]]), [0.5, 2, 10]),
    case('ring band', uniform([[0, 0, 0, 0], [0, 5, 5, 0], [0, 5, 5, 0], [0, 0, 0, 0]]), [1, 10]),
    case('band with hole', uniform([[0, 0, 0, 0, 0], [0, 2, 2, 2, 0], [0, 2, 9, 2, 0], [0, 2, 2, 2, 0], [0, 0, 0, 0, 0]]), [1, 5, 10]),
    # Concentric squares alternating 0 and 5: a shell with a hole containing
    # an island with a hole, so the inner hole is inside two shells.
    case('nested islands and holes', uniform(
        [[5 * (1 - max(abs(i - 4), abs(j - 4)) % 2) for i in range(9)] for j in range(9)]), [1, 10]),
    case('saddle', uniform([[0, 5], [5, 0]]), [1, 2, 4]),
    case('saddle centre at a level', uniform([[0, 5], [5, 0]]), [1, 2.5, 4]),
    case('one NaN corner', uniform([[1, 1], [1, NAN]]), [0, 2]),
    case('one NaN corner in 3x3', uniform([[1, 1, 1], [1, 1, 1], [1, 1, NAN]]), [0, 2]),
    case('NaN centre', uniform([[1, 1, 1], [1, NAN, 1], [1, 1, 1]]), [0, 2]),
    case('NaN crossing a band', uniform([[0, 1, 2, 3], [1, 2, NAN, 4], [2, 3, 4, 5]]), [0.5, 1.5, 2.5, 3.5, 4.5]),
    case('integer plateaus', uniform(rng.integers(0, 4, (8, 8))), [0, 1, 2, 3]),
    case('integer plateaus with NaN', uniform(with_nans(rng.integers(0, 4, (8, 8)), 0.15, 1)), [-1, 0, 1, 2, 3]),
    case('curvilinear', curvilinear(smooth(9, 7, 3)), [-4, -2, 0, 2, 4, 6, 8, 10]),
    case('smooth', uniform(smooth(30, 20, 4)), [-6, -4, -2, 0, 1, 2, 4, 6, 8, 10, 12]),
    case('smooth with NaN', uniform(with_nans(smooth(30, 20, 5), 0.08, 2)), [-6, -4, -2, 0, 1, 2, 4, 6, 8, 10, 12]),
    # From the MRMS multi-sensor QPE test data at the grid's top edge (row 0),
    # where a plateau sits exactly on a band's upper level.
    case('QPE plateau at a level', uniform([
        [1.4, 1.3, 1.2, 1.1, 1.0, 0.9, 0.9, 0.9, 0.8, 0.7, 0.7, 0.8, 0.8, 0.7, 0.7],
        [1.2, 1.1, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.9, 0.8, 0.7, 0.7, 0.7, 0.7, 0.7],
        [1.1, 1.1, 1.0, 1.0, 1.1, 1.0, 1.0, 1.0, 0.9, 0.8, 0.8, 0.7, 0.7, 0.8, 0.7],
        [1.0, 1.0, 1.0, 1.2, 1.1, 1.0, 1.0, 1.1, 1.1, 0.9, 0.9, 0.8, 0.8, 0.7, 0.7],
        [1.1, 1.3, 1.2, 1.1, 1.0, 1.0, 1.1, 1.1, 1.1, 1.0, 0.9, 0.9, 0.8, 0.8, 0.8],
        [1.1, 1.1, 1.1, 1.0, 0.9, 0.8, 0.9, 1.0, 1.0, 1.0, 0.9, 0.9, 0.8, 0.9, 0.9],
        [0.9, 0.9, 0.8, 0.8, 0.7, 0.7, 0.7, 0.8, 1.0, 1.0, 1.0, 1.1, 1.0, 1.0, 1.0],
        [0.8, 0.8, 0.8, 0.8, 0.8, 0.8, 0.8, 0.9, 1.0, 1.1, 1.1, 1.1, 1.2, 1.1, 1.1],
        [0.8, 0.7, 0.6, 0.7, 0.8, 0.9, 0.9, 1.0, 1.1, 1.1, 1.1, 1.1, 1.1, 1.1, 1.2],
    ]), [0.25, 1, 2.5]),
    case('noise with NaN', uniform(with_nans(rng.uniform(0, 10, (25, 35)), 0.1, 3)), [1, 2.5, 4, 5, 6, 7.5, 9]),
]

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'fixtures.json')
with open(out, 'w') as f:
    json.dump(cases, f)
print('wrote', len(cases), 'cases to', out)
