package grid_to_isobands_test

import (
	"slices"
	"testing"

	"github.com/skysparq/grid-to-isobands"
)

func TestSwapEvenSizedRows(t *testing.T) {
	values := []float64{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
	}
	values = grid_to_isobands.SwapRightAndLeft(values, 10)
	expected := []float64{
		6, 7, 8, 9, 10, 1, 2, 3, 4, 5,
		16, 17, 18, 19, 20, 11, 12, 13, 14, 15,
	}
	if !slices.Equal(values, expected) {
		t.Fatalf(`expected %v, got %v`, expected, values)
	}
}

func TestSwapOddSizedRows(t *testing.T) {
	values := []float64{
		1, 2, 3, 4, 5,
		6, 7, 8, 9, 10,
	}
	values = grid_to_isobands.SwapRightAndLeft(values, 5)
	expected := []float64{
		4, 5, 3, 1, 2,
		9, 10, 8, 6, 7,
	}
	if !slices.Equal(values, expected) {
		t.Fatalf(`expected %v, got %v`, expected, values)
	}
}
