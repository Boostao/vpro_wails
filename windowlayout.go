package main

import "fmt"

type windowDimensions struct {
	width, height, minWidth, minHeight int
}

func initialWindowDimensions(workWidth, workHeight int) (windowDimensions, error) {
	if workWidth <= 64 || workHeight <= 64 {
		return windowDimensions{}, fmt.Errorf("invalid display work area: %dx%d", workWidth, workHeight)
	}
	width, height := min(1400, workWidth-32), min(900, workHeight-32)
	return windowDimensions{
		width: width, height: height,
		minWidth: min(560, width), minHeight: min(520, height),
	}, nil
}
