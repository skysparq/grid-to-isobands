package grid_to_isobands

import "math"

// BilateralFilter applies a bilateral filter to a flat 2D grid represented as a slice.
// Input: data (flat slice, row-major), width, height, sigmaSpatial (spatial Gaussian stddev in pixels),
// sigmaRange (intensity difference Gaussian stddev, e.g. in dBZ units).
// Returns: new flat slice of same length with filtered values.
func BilateralFilter(data []float64, width, height int, sigmaSpatial, sigmaRange float64) []float64 {
	if len(data) != width*height {
		panic("data length does not match width*height")
	}

	result := make([]float64, len(data))

	// Kernel radius: usually 2–3 * sigmaSpatial covers >99% of Gaussian mass
	radius := int(math.Ceil(3 * sigmaSpatial))
	if radius < 1 {
		radius = 1
	}

	// Precompute spatial Gaussian weights (depends only on distance)
	// We could use a lookup table, but for simplicity we compute on the fly

	twoSigmaSpatialSq := 2 * sigmaSpatial * sigmaSpatial
	twoSigmaRangeSq := 2 * sigmaRange * sigmaRange

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			centerIdx := y*width + x
			centerVal := data[centerIdx]

			var sum float64
			var weightSum float64

			// Iterate over local window
			for dy := -radius; dy <= radius; dy++ {
				ny := y + dy
				if ny < 0 || ny >= height {
					continue
				}

				for dx := -radius; dx <= radius; dx++ {
					nx := x + dx
					if nx < 0 || nx >= width {
						continue
					}

					nIdx := ny*width + nx
					nVal := data[nIdx]

					// Spatial distance (Euclidean)
					spatialDistSq := float64(dx*dx + dy*dy)
					spatialWeight := math.Exp(-spatialDistSq / twoSigmaSpatialSq)

					// Range (intensity) difference
					rangeDist := centerVal - nVal
					rangeWeight := math.Exp(-(rangeDist * rangeDist) / twoSigmaRangeSq)

					w := spatialWeight * rangeWeight

					sum += w * nVal
					weightSum += w
				}
			}

			if weightSum > 0 {
				result[centerIdx] = sum / weightSum
			} else {
				// Very rare fallback (numerical issues)
				result[centerIdx] = centerVal
			}
		}
	}

	return result
}
