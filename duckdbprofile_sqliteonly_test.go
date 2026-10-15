//go:build sqliteonly

package main

import "testing"

func TestDuckDBSQLiteOnlyBuildProfile(t *testing.T) {
	if duckDBDriverAvailable || duckDBDriverUnavailableReason != "in the sqliteonly build profile" {
		t.Fatal("sqliteonly profile linked or misidentified the optional driver")
	}
}
