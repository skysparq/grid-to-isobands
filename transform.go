package grid_to_isobands

type InitialTransformer func(values []float64, width int) []float64
type PostSmoothTransformer func(vals []float64, width int, sentinel, floor, step float64) []float64

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

func RemoveTop10DegreesFromGlobal0p25Grid(vals []float64, width int, sentinel, floor, step float64) []float64 {
	height := len(vals) / width
	rowsToRemove := 4 * 10
	for y := 0; y < rowsToRemove; y++ {
		for x := 0; x < width; x++ {
			top := (y * width) + x
			bottom := ((height - 1 - y) * width) + x
			vals[top] = sentinel
			vals[bottom] = sentinel
		}
	}
	return vals
}
