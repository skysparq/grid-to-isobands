package transformers

import (
	"math"
)

type Clip struct {
	Top    int
	Bottom int
	Left   int
	Right  int
}

func ClipGrid(data []float64, width, height int, clip Clip) {
	// clip grid with NaN
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < clip.Left || x > width-clip.Right-1 || y < clip.Bottom || y > height-clip.Top-1 {
				i := y*width + x
				data[i] = math.NaN()
			}
		}
	}
}
