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

func ReverseVertical(values []float64, width int) []float64 {
	height := len(values) / width
	maxY := height - 1
	bottomCopy := make([]float64, width)
	for i := 0; i < height/2; i++ {
		bottomStart := i * width
		bottomEnd := bottomStart + width
		topStart := (maxY - i) * width
		topEnd := topStart + width
		copy(bottomCopy, values[bottomStart:bottomEnd])
		copy(values[bottomStart:bottomEnd], values[topStart:topEnd])
		copy(values[topStart:topEnd], bottomCopy)
	}
	return values
}
