package grid_to_isobands

import (
	"math"

	"github.com/skysparq/grid-to-isobands/transformers"
)

type GridTransformer func(values *GridValues)

func SwapRightAndLeftTransformer() GridTransformer {
	return func(values *GridValues) {
		width := values.SizeX
		height := len(values.Values) / width
		halfWidth := width / 2
		for i := 0; i < height; i++ {
			leftStart := i * width
			leftEnd := leftStart + halfWidth
			rightStart := leftStart + width - halfWidth
			rightEnd := rightStart + halfWidth
			left := make([]float64, width)
			copy(left, values.Values[leftStart:leftEnd])
			copy(values.Values[leftStart:leftEnd], values.Values[rightStart:rightEnd])
			copy(values.Values[rightStart:rightEnd], left)

			copy(left, values.Lats[leftStart:leftEnd])
			copy(values.Lats[leftStart:leftEnd], values.Lats[rightStart:rightEnd])
			copy(values.Lats[rightStart:rightEnd], left)

			copy(left, values.Lons[leftStart:leftEnd])
			copy(values.Lons[leftStart:leftEnd], values.Lons[rightStart:rightEnd])
			copy(values.Lons[rightStart:rightEnd], left)
		}
	}
}

func ReverseVerticalTransformer() GridTransformer {
	return func(values *GridValues) {
		width := values.SizeX
		height := len(values.Values) / width
		maxY := height - 1
		bottomCopy := make([]float64, width)
		for i := 0; i < height/2; i++ {
			bottomStart := i * width
			bottomEnd := bottomStart + width
			topStart := (maxY - i) * width
			topEnd := topStart + width
			copy(bottomCopy, values.Values[bottomStart:bottomEnd])
			copy(values.Values[bottomStart:bottomEnd], values.Values[topStart:topEnd])
			copy(values.Values[topStart:topEnd], bottomCopy)

			copy(bottomCopy, values.Lats[bottomStart:bottomEnd])
			copy(values.Lats[bottomStart:bottomEnd], values.Lats[topStart:topEnd])
			copy(values.Lats[topStart:topEnd], bottomCopy)

			copy(bottomCopy, values.Lons[bottomStart:bottomEnd])
			copy(values.Lons[bottomStart:bottomEnd], values.Lons[topStart:topEnd])
			copy(values.Lons[topStart:topEnd], bottomCopy)
		}
	}
}

func OpenCloseTransformer(kernel int) GridTransformer {
	return func(values *GridValues) {
		morphOps := transformers.NewMorphologicalOps(values.SizeX, values.SizeY)
		values.Values = morphOps.OpenClose(values.Values, kernel)
	}
}

func CloseOpenTransformer(kernel int) GridTransformer {
	return func(values *GridValues) {
		morphOps := transformers.NewMorphologicalOps(values.SizeX, values.SizeY)
		values.Values = morphOps.CloseOpen(values.Values, kernel)
	}
}

func GaussianTransformer(kernel int, sigma float64) GridTransformer {
	return func(values *GridValues) {
		values.Values = transformers.FastGaussian(values.Values, values.SizeX, values.SizeY, kernel, sigma)
	}
}

func MedianTransformer(kernel int) GridTransformer {
	return func(values *GridValues) {
		newValues, _ := transformers.MedianFilter(values.Values, values.SizeX, values.SizeY, kernel)
		values.Values = newValues
	}
}

func ClipTransformer(clip transformers.Clip) GridTransformer {
	return func(values *GridValues) {
		transformers.ClipGrid(values.Values, values.SizeX, values.SizeY, clip)
	}
}

func ThresholdMaskTransformer(f transformers.ThresholdFunc, replacement float64) GridTransformer {
	return func(values *GridValues) {
		transformers.ThresholdMask(values.Values, f, replacement)
	}
}

func RemoveInfTransformer() GridTransformer {
	return func(values *GridValues) {
		for i := 0; i < len(values.Values); i++ {
			if math.IsInf(values.Values[i], 0) {
				values.Values[i] = math.NaN()
			}
		}
	}
}

func BilateralTransformer(sigma, color float64) GridTransformer {
	return func(values *GridValues) {
		values.Values = transformers.BilateralFilter(values.Values, values.SizeX, values.SizeY, sigma, color)
	}
}
