package grid_to_isobands

import (
	"math"
)

// MorphologicalOps provides morphological image processing operations
type MorphologicalOps struct {
	width  int
	height int
}

// NewMorphologicalOps creates a new morphological operations processor
func NewMorphologicalOps(width, height int) *MorphologicalOps {
	return &MorphologicalOps{
		width:  width,
		height: height,
	}
}

// Erode performs morphological erosion with a given kernel size
// Replaces each pixel with the minimum in its neighborhood
func (m *MorphologicalOps) Erode(data []float64, kernelSize int) []float64 {
	result := make([]float64, len(data))
	halfKernel := kernelSize / 2

	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			minVal := math.Inf(1)

			// Scan kernel neighborhood
			for ky := -halfKernel; ky <= halfKernel; ky++ {
				for kx := -halfKernel; kx <= halfKernel; kx++ {
					nx := x + kx
					ny := y + ky

					// Boundary handling: clamp to edges
					if nx < 0 {
						nx = 0
					}
					if nx >= m.width {
						nx = m.width - 1
					}
					if ny < 0 {
						ny = 0
					}
					if ny >= m.height {
						ny = m.height - 1
					}

					idx := ny*m.width + nx
					if data[idx] < minVal {
						minVal = data[idx]
					}
				}
			}

			result[y*m.width+x] = minVal
		}
	}

	return result
}

// Dilate performs morphological dilation with a given kernel size
// Replaces each pixel with the maximum in its neighborhood
func (m *MorphologicalOps) Dilate(data []float64, kernelSize int) []float64 {
	result := make([]float64, len(data))
	halfKernel := kernelSize / 2

	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			maxVal := math.Inf(-1)

			// Scan kernel neighborhood
			for ky := -halfKernel; ky <= halfKernel; ky++ {
				for kx := -halfKernel; kx <= halfKernel; kx++ {
					nx := x + kx
					ny := y + ky

					// Boundary handling: clamp to edges
					if nx < 0 {
						nx = 0
					}
					if nx >= m.width {
						nx = m.width - 1
					}
					if ny < 0 {
						ny = 0
					}
					if ny >= m.height {
						ny = m.height - 1
					}

					idx := ny*m.width + nx
					if data[idx] > maxVal {
						maxVal = data[idx]
					}
				}
			}

			result[y*m.width+x] = maxVal
		}
	}

	return result
}

// Open performs morphological opening (erosion followed by dilation)
// Removes small bright features while preserving larger structures
func (m *MorphologicalOps) Open(data []float64, kernelSize int) []float64 {
	eroded := m.Erode(data, kernelSize)
	return m.Dilate(eroded, kernelSize)
}

// Close performs morphological closing (dilation followed by erosion)
// Fills small holes and connects nearby features
func (m *MorphologicalOps) Close(data []float64, kernelSize int) []float64 {
	dilated := m.Dilate(data, kernelSize)
	return m.Erode(dilated, kernelSize)
}

// Gradient computes morphological gradient (dilation - erosion)
// Highlights edges and boundaries
func (m *MorphologicalOps) Gradient(data []float64, kernelSize int) []float64 {
	dilated := m.Dilate(data, kernelSize)
	eroded := m.Erode(data, kernelSize)

	result := make([]float64, len(data))
	for i := range result {
		result[i] = dilated[i] - eroded[i]
	}

	return result
}

// TopHat computes white top-hat (original - opening)
// Extracts small bright features
func (m *MorphologicalOps) TopHat(data []float64, kernelSize int) []float64 {
	opened := m.Open(data, kernelSize)

	result := make([]float64, len(data))
	for i := range result {
		result[i] = data[i] - opened[i]
	}

	return result
}

// BlackHat computes black top-hat (closing - original)
// Extracts small dark features
func (m *MorphologicalOps) BlackHat(data []float64, kernelSize int) []float64 {
	closed := m.Close(data, kernelSize)

	result := make([]float64, len(data))
	for i := range result {
		result[i] = closed[i] - data[i]
	}

	return result
}

// OpenClose performs opening followed by closing
// Good general noise reduction for weather data
func (m *MorphologicalOps) OpenClose(data []float64, kernelSize int) []float64 {
	opened := m.Open(data, kernelSize)
	return m.Close(opened, kernelSize)
}

// CloseOpen performs closing followed by opening
// Alternative noise reduction approach
func (m *MorphologicalOps) CloseOpen(data []float64, kernelSize int) []float64 {
	closed := m.Close(data, kernelSize)
	return m.Open(closed, kernelSize)
}
