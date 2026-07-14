package transformers

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

// gaussianKernel1D creates a 1D Gaussian kernel
func gaussianKernel1D(size int, sigma float64) []float64 {
	if size%2 == 0 {
		panic("kernel size must be odd")
	}

	kernel := make([]float64, size)
	center := size / 2
	sum := 0.0

	for i := 0; i < size; i++ {
		x := float64(i - center)
		value := math.Exp(-(x * x) / (2 * sigma * sigma))
		kernel[i] = value
		sum += value
	}

	// Normalize
	for i := range kernel {
		kernel[i] /= sum
	}

	return kernel
}

// separableConvolve2D performs separable 2D convolution, skipping NaN neighbors.
// Each output pixel is computed from the weighted sum of non-NaN neighbors only,
// with the kernel renormalized over those neighbors. Pixels with no valid neighbors
// remain NaN.
func separableConvolve2D(data *mat.Dense, kernel []float64) *mat.Dense {
	rows, cols := data.Dims()
	kSize := len(kernel)
	kHalf := kSize / 2

	// First pass: convolve rows
	temp := mat.NewDense(rows, cols, nil)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			sum, weight := 0.0, 0.0
			for k := 0; k < kSize; k++ {
				srcJ := clamp(j+k-kHalf, 0, cols-1)
				v := data.At(i, srcJ)
				if !math.IsNaN(v) {
					sum += kernel[k] * v
					weight += kernel[k]
				}
			}
			if weight == 0 {
				temp.Set(i, j, math.NaN())
			} else {
				temp.Set(i, j, sum/weight)
			}
		}
	}

	// Second pass: convolve columns
	result := mat.NewDense(rows, cols, nil)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			sum, weight := 0.0, 0.0
			for k := 0; k < kSize; k++ {
				srcI := clamp(i+k-kHalf, 0, rows-1)
				v := temp.At(srcI, j)
				if !math.IsNaN(v) {
					sum += kernel[k] * v
					weight += kernel[k]
				}
			}
			if weight == 0 {
				result.Set(i, j, math.NaN())
			} else {
				result.Set(i, j, sum/weight)
			}
		}
	}

	return result
}

// FastGaussian applies Gaussian smoothing to raw gridded data
// data: raw reflectivity values as a flat slice (row-major order)
// width: number of columns in the grid
// height: number of rows in the grid
// kernelSize: size of the Gaussian kernel (must be odd, e.g., 3, 5, 7)
// sigma: standard deviation of the Gaussian (e.g., 1.0)
// Returns: smoothed data as a flat slice in the same format as input
func FastGaussian(data []float64, width, height, kernelSize int, sigma float64) []float64 {
	if len(data) != width*height {
		panic("data length must equal width * height")
	}

	// Create gonum matrix from raw data
	matrix := mat.NewDense(height, width, data)

	// Apply separable Gaussian convolution
	kernel := gaussianKernel1D(kernelSize, sigma)
	smoothed := separableConvolve2D(matrix, kernel)

	// Extract and return raw data
	return smoothed.RawMatrix().Data
}
