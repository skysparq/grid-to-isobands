import json
import math
import sys

import cbor2
import numpy as np
import contourpy


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
                transf_points = []
                for j in range(len(points)):
                    point = points[j]
                    if not (math.isfinite(point[0]) and math.isfinite(point[1])):
                        print(f'got inf or nan at level {i} point {j}')
                        continue

                    if point[0] < -179.99999:
                        point = -179.99999, point[1]
                    if point[0] > 179.99999:
                        point = 179.99999, point[1]
                    if point[1] > 89.99999:
                        point = point[0], 89.99999
                    if point[1] < -89.99999:
                        point = point[0], -89.99999

                    transf_points.append(list(point))

                boundaries = []
                for j in range(len(boundary_offsets) - 1):
                    start = boundary_offsets[j]
                    end = boundary_offsets[j + 1]
                    if end - start < 4:
                        continue
                    boundaries.append(transf_points[start:end])
                for j in range(len(poly_offsets) - 1):
                    start = poly_offsets[j]
                    end = poly_offsets[j + 1]
                    poly = boundaries[start:end]
                    features.append({
                        "type": "Feature",
                        "properties": {
                            "levelIndex": i,
                            "floor": lower,
                            "ceiling": upper,
                        },
                        "geometry": {
                            "type": "Polygon",
                            "coordinates": poly,
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
