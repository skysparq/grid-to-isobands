package transformers

import (
	"math"
)

// BilateralFilter applies a bilateral filter to a flat row-major grid.
//
// Optimizations:
//  1. Spatial kernel fully precomputed (eliminates radius^2 * W * H Exp calls)
//  2. Range kernel approximated via LUT keyed on discretized dBZ difference
//  3. Interior pixels processed without bounds checks
func BilateralFilter(data []float64, width, height int, sigma, color float64) []float64 {
	if len(data) != width*height {
		panic("bilateral: data length does not match width*height")
	}

	radius := int(math.Ceil(3 * sigma))
	if radius < 1 {
		radius = 1
	}

	windowSize := 2*radius + 1
	spatialKernel := make([]float64, windowSize*windowSize)
	twoSigmaSpatialSq := 2 * sigma * sigma
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			d2 := float64(dx*dx + dy*dy)
			spatialKernel[(dy+radius)*windowSize+(dx+radius)] = math.Exp(-d2 / twoSigmaSpatialSq)
		}
	}

	const lutSize = 1100
	const rangeStep = 0.1
	twoSigmaRangeSq := 2 * color * color
	rangeLUT := make([]float64, lutSize)
	for i := range rangeLUT {
		diff := float64(i) * rangeStep
		rangeLUT[i] = math.Exp(-(diff * diff) / twoSigmaRangeSq)
	}
	rangeLUTLookup := func(diff float64) float64 {
		idx := int(math.Abs(diff) / rangeStep)
		if idx >= lutSize {
			return 0
		}
		return rangeLUT[idx]
	}

	result := make([]float64, len(data))

	for y := 0; y < height; y++ {
		interiorRow := y >= radius && y < height-radius
		for x := 0; x < width; x++ {
			centerVal := data[y*width+x]
			var sum, weightSum float64

			if interiorRow && x >= radius && x < width-radius {
				for dy := -radius; dy <= radius; dy++ {
					rowBase := (y+dy)*width + x
					kRowBase := (dy + radius) * windowSize
					for dx := -radius; dx <= radius; dx++ {
						nVal := data[rowBase+dx]
						w := spatialKernel[kRowBase+(dx+radius)] * rangeLUTLookup(centerVal-nVal)
						sum += w * nVal
						weightSum += w
					}
				}
			} else {
				for dy := -radius; dy <= radius; dy++ {
					ny := y + dy
					if ny < 0 || ny >= height {
						continue
					}
					kRowBase := (dy + radius) * windowSize
					for dx := -radius; dx <= radius; dx++ {
						nx := x + dx
						if nx < 0 || nx >= width {
							continue
						}
						nVal := data[ny*width+nx]
						w := spatialKernel[kRowBase+(dx+radius)] * rangeLUTLookup(centerVal-nVal)
						sum += w * nVal
						weightSum += w
					}
				}
			}

			if weightSum > 0 {
				result[y*width+x] = sum / weightSum
			} else {
				result[y*width+x] = centerVal
			}
		}
	}

	return result
}
