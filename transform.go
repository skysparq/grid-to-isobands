package grid_to_isobands

type InitialTransformer func(values []float64, width int) []float64

func NoopTransform(values []float64, _ int) []float64 {
	return values
}

func SwapRightAndLeft(values []float64, width int) []float64 {
	height := len(values) / width
	halfWidth := width / 2
	for i := 0; i < height; i++ {
		leftStart := i * width
		leftEnd := leftStart + halfWidth
		rightStart := leftStart + width - halfWidth
		rightEnd := rightStart + halfWidth
		left := make([]float64, width)
		copy(left, values[leftStart:leftEnd])
		copy(values[leftStart:leftEnd], values[rightStart:rightEnd])
		copy(values[rightStart:rightEnd], left)
	}
	return values
}
