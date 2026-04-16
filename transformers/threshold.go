package transformers

type ThresholdFunc func(float64) bool

// ThresholdMask sets values to maskVal where the predicate returns true.
func ThresholdMask(data []float64, predicate ThresholdFunc, replacement float64) {
	for i, v := range data {
		if predicate(v) {
			data[i] = replacement
		}
	}
}

func GreaterThan(threshold float64) ThresholdFunc {
	return func(v float64) bool { return v > threshold }
}

func LessThan(threshold float64) ThresholdFunc {
	return func(v float64) bool { return v < threshold }
}
