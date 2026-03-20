package grid_to_isobands_test

import (
	"reflect"
	"testing"

	"github.com/skysparq/grid-to-isobands"
)

func TestSwapEvenSizedRows(t *testing.T) {
	values := &grid_to_isobands.GridValues{
		SizeX: 10,
		SizeY: 2,
		Values: []float64{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		},
		Lats: []float64{
			1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
			2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		},
		Lons: []float64{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		},
	}
	transform := grid_to_isobands.SwapRightAndLeftTransformer()
	transform(values)
	expected := &grid_to_isobands.GridValues{
		SizeX: 10,
		SizeY: 2,
		Values: []float64{
			6, 7, 8, 9, 10, 1, 2, 3, 4, 5,
			16, 17, 18, 19, 20, 11, 12, 13, 14, 15,
		},
		Lats: []float64{
			1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
			2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		},
		Lons: []float64{
			6, 7, 8, 9, 10, 1, 2, 3, 4, 5,
			6, 7, 8, 9, 10, 1, 2, 3, 4, 5,
		},
	}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf(`expected %v, got %v`, expected, values)
	}
}

func TestSwapOddSizedRows(t *testing.T) {
	values := &grid_to_isobands.GridValues{
		SizeX: 5,
		SizeY: 2,
		Values: []float64{
			1, 2, 3, 4, 5,
			6, 7, 8, 9, 10,
		},
		Lats: []float64{
			1, 1, 1, 1, 1,
			2, 2, 2, 2, 2,
		},
		Lons: []float64{
			1, 2, 3, 4, 5,
			1, 2, 3, 4, 5,
		},
	}
	transform := grid_to_isobands.SwapRightAndLeftTransformer()
	transform(values)
	expected := &grid_to_isobands.GridValues{
		SizeX: 5,
		SizeY: 2,
		Values: []float64{
			4, 5, 3, 1, 2,
			9, 10, 8, 6, 7,
		},
		Lats: []float64{
			1, 1, 1, 1, 1,
			2, 2, 2, 2, 2,
		},
		Lons: []float64{
			4, 5, 3, 1, 2,
			4, 5, 3, 1, 2,
		},
	}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf(`expected %v, got %v`, expected, values)
	}
}

func TestReverseVerticalOdd(t *testing.T) {
	values := &grid_to_isobands.GridValues{
		SizeX: 5,
		SizeY: 3,
		Values: []float64{
			1, 2, 3, 4, 5,
			6, 7, 8, 9, 10,
			11, 12, 13, 14, 15,
		},
		Lats: []float64{
			1, 1, 1, 1, 1,
			2, 2, 2, 2, 2,
			3, 3, 3, 3, 3,
		},
		Lons: []float64{
			1, 2, 3, 4, 5,
			1, 2, 3, 4, 5,
			1, 2, 3, 4, 5,
		},
	}
	transform := grid_to_isobands.ReverseVerticalTransformer()
	transform(values)
	expected := &grid_to_isobands.GridValues{
		SizeX: 5,
		SizeY: 3,
		Values: []float64{
			11, 12, 13, 14, 15,
			6, 7, 8, 9, 10,
			1, 2, 3, 4, 5,
		},
		Lats: []float64{
			3, 3, 3, 3, 3,
			2, 2, 2, 2, 2,
			1, 1, 1, 1, 1,
		},
		Lons: []float64{
			1, 2, 3, 4, 5,
			1, 2, 3, 4, 5,
			1, 2, 3, 4, 5,
		},
	}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf(`expected %v, got %v`, expected, values)
	}
}

func TestReverseVerticalEven(t *testing.T) {
	values := &grid_to_isobands.GridValues{
		SizeX: 3,
		SizeY: 4,
		Values: []float64{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
			10, 11, 12,
		},
		Lats: []float64{
			1, 1, 1,
			2, 2, 2,
			3, 3, 3,
			4, 4, 4,
		},
		Lons: []float64{
			1, 2, 3,
			1, 2, 3,
			1, 2, 3,
			1, 2, 3,
		},
	}
	transform := grid_to_isobands.ReverseVerticalTransformer()
	transform(values)
	expected := &grid_to_isobands.GridValues{
		SizeX: 3,
		SizeY: 4,
		Values: []float64{
			10, 11, 12,
			7, 8, 9,
			4, 5, 6,
			1, 2, 3,
		},
		Lats: []float64{
			4, 4, 4,
			3, 3, 3,
			2, 2, 2,
			1, 1, 1,
		},
		Lons: []float64{
			1, 2, 3,
			1, 2, 3,
			1, 2, 3,
			1, 2, 3,
		},
	}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf(`expected %v, got %v`, expected, values)
	}
}
