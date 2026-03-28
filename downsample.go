package grid_to_isobands

func Downsample2x2(args *GridValues) {
	data, lats, lons := args.Values, args.Lats, args.Lons
	width, height := args.SizeX, args.SizeY

	if len(data) != width*height || len(lats) != width*height || len(lons) != width*height {
		panic("slice lengths must all equal width*height")
	}
	if height%2 != 0 || width%2 != 0 || height == 0 || width == 0 {
		panic("width and height must be even and positive")
	}

	newWidth := width / 2
	newHeight := height / 2
	newSize := newWidth * newHeight

	newData := make([]float64, newSize)
	newLats := make([]float64, newSize)
	newLons := make([]float64, newSize)

	for i := 0; i < newHeight; i++ { // new row
		for j := 0; j < newWidth; j++ { // new column
			// four original indices (row-major)
			idxTL := (2*i)*width + (2 * j)
			idxTR := idxTL + 1
			idxBL := idxTL + width
			idxBR := idxBL + 1

			// average data
			avgData := (data[idxTL] + data[idxTR] + data[idxBL] + data[idxBR]) / 4.0

			// average coordinates (center of the 2x2 quad)
			avgLat := (lats[idxTL] + lats[idxTR] + lats[idxBL] + lats[idxBR]) / 4.0
			avgLon := (lons[idxTL] + lons[idxTR] + lons[idxBL] + lons[idxBR]) / 4.0

			idxNew := i*newWidth + j
			newData[idxNew] = avgData
			newLats[idxNew] = avgLat
			newLons[idxNew] = avgLon
		}
	}

	args.SizeX = newWidth
	args.SizeY = newHeight
	args.Values = newData
	args.Lats = newLats
	args.Lons = newLons
}
