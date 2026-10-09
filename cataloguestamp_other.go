//go:build !windows

package main

import (
	"fmt"
	"os"
)

func catalogueFileStamp(path string) (catalogueStamp, error) {
	info, err := os.Stat(path)
	if err != nil {
		return catalogueStamp{}, err
	}
	if !info.Mode().IsRegular() {
		return catalogueStamp{}, fmt.Errorf("catalogue is not a regular file: %s", path)
	}
	return catalogueStamp{info: info}, nil
}
