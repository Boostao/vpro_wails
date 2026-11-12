//go:build linux

package main

import (
	"debug/elf"
	"os"
	"strings"
	"testing"
)

func TestAccessReaderDoesNotExportGLibSymbols(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := elf.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer binary.Close()

	symbols, err := binary.DynamicSymbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range symbols {
		if strings.HasPrefix(symbol.Name, "g_") && symbol.Section != elf.SHN_UNDEF &&
			elf.ST_VISIBILITY(symbol.Other) == elf.STV_DEFAULT {
			t.Errorf("executable exports GLib symbol %s, which can interpose on GTK", symbol.Name)
		}
	}
}
