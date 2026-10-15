//go:build !sqliteonly && (!windows || !arm64)

package main

import _ "github.com/duckdb/duckdb-go/v2"

const duckDBDriverAvailable = true
const duckDBDriverUnavailableReason = ""
