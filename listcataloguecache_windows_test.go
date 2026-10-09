package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestCatalogueCacheWindowsChangeTimeDetectsSameSizePreservedModTime(t *testing.T) {
	service, err := NewParentCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	info, err := os.Stat(service.path)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), parentCodeDatabase...)
	data[0] ^= 1
	if err := os.WriteFile(service.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	restoreCatalogueModTime(t, service.path, info.ModTime())
	if rows, err := service.ListChoices(context.Background(), "HumusForm"); err == nil || rows != nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal("preserved size/modtime concealed corruption", err)
	}
}
