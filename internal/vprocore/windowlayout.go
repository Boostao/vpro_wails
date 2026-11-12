package vprocore

import "fmt"

type windowDimensions struct {
	Width, Height, MinWidth, MinHeight int `json:"-" yaml:"-"`
}

func InitialWindowDimensions(workWidth, workHeight int) (windowDimensions, error) {
	if workWidth <= 64 || workHeight <= 64 {
		return windowDimensions{}, fmt.Errorf("invalid display work area: %dx%d", workWidth, workHeight)
	}
	width, height := min(1400, workWidth-32), min(900, workHeight-32)
	return windowDimensions{
		Width: width, Height: height,
		MinWidth: min(560, width), MinHeight: min(520, height),
	}, nil
}
