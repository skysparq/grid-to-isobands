package grid_to_isobands

import "math"

type Clip struct {
	Top    int
	Bottom int
	Left   int
	Right  int
}

func ClipGrid(vals *GridValues, clip Clip) {
	// clip grid with NaN
	for y := 0; y < vals.SizeY; y++ {
		for x := 0; x < vals.SizeX; x++ {
			if x < clip.Left || x > vals.SizeX-clip.Right-1 || y < clip.Bottom || y > vals.SizeY-clip.Top-1 {
				i := y*vals.SizeX + x
				vals.Values[i] = math.NaN()
			}
		}
	}
}
