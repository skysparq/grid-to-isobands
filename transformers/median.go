package transformers

import (
	"fmt"
	"sort"
)

// MedianFilter applies a median filter to a 1D slice representing a 2D grid.
// data: the input grid values (row-major order)
// width, height: dimensions of the grid
// kernelSize: size of the filter kernel (must be odd, e.g. 3, 5, 7)
func MedianFilter(data []float64, width, height, kernelSize int) ([]float64, error) {
	if len(data) != width*height {
		return nil, fmt.Errorf("data length %d does not match width*height (%d*%d=%d)", len(data), width, height, width*height)
	}
	if kernelSize%2 == 0 {
		return nil, fmt.Errorf("kernelSize must be odd, got %d", kernelSize)
	}
	if kernelSize < 1 {
		return nil, fmt.Errorf("kernelSize must be at least 1, got %d", kernelSize)
	}

	output := make([]float64, len(data))
	radius := kernelSize / 2
	window := make([]float64, 0, kernelSize*kernelSize)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			window = window[:0]

			// Collect neighbors within the kernel bounds, clamping at edges
			for ky := -radius; ky <= radius; ky++ {
				ny := clamp(y+ky, 0, height-1)
				for kx := -radius; kx <= radius; kx++ {
					nx := clamp(x+kx, 0, width-1)
					window = append(window, data[ny*width+nx])
				}
			}

			sort.Float64s(window)
			output[y*width+x] = window[len(window)/2]
		}
	}

	return output, nil
}
