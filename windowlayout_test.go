package main

import "testing"

func TestInitialWindowDimensions(t *testing.T) {
	for _, test := range []struct {
		name          string
		width, height int
		want          windowDimensions
	}{
		{"wide", 1920, 1040, windowDimensions{1400, 900, 560, 520}},
		{"laptop", 1366, 728, windowDimensions{1334, 696, 560, 520}},
		{"small", 800, 600, windowDimensions{768, 568, 560, 520}},
		{"compact", 520, 480, windowDimensions{488, 448, 488, 448}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := initialWindowDimensions(test.width, test.height)
			if err != nil || got != test.want {
				t.Fatalf("dimensions = %+v, %v; want %+v", got, err, test.want)
			}
		})
	}
	for _, bounds := range [][2]int{{0, 800}, {800, 0}, {-1, 800}, {64, 800}, {800, 64}} {
		if _, err := initialWindowDimensions(bounds[0], bounds[1]); err == nil {
			t.Fatalf("invalid work area accepted: %v", bounds)
		}
	}
}
