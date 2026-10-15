//go:build !sqliteonly

package main

import (
	"runtime"
	"testing"
)

func TestDuckDBDefaultBuildProfile(t *testing.T) {
	want := runtime.GOOS != "windows" || runtime.GOARCH != "arm64"
	if duckDBDriverAvailable != want {
		t.Fatal("default profile changed optional driver platform availability")
	}
	if want && duckDBDriverUnavailableReason != "" {
		t.Fatal("available optional driver has an unavailable reason")
	}
	if !want && duckDBDriverUnavailableReason != "on Windows ARM64" {
		t.Fatal("unsupported platform lost its explicit refusal reason")
	}
}
