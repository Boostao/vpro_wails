package vprocore

import (
	"github.com/boostao/vpro-wails/internal/references"
)

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestCatalogueCacheWindowsChangeTimeDetectsSameSizePreservedModTime(t *testing.T) {
	service, err := references.NewParentCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	info, err := os.Stat(service.Path)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), references.ParentCodeDatabase...)
	data[0] ^= 1
	if err := os.WriteFile(service.Path, data, 0600); err != nil {
		t.Fatal(err)
	}
	restoreCatalogueModTime(t, service.Path, info.ModTime())
	if rows, err := service.ListChoices(context.Background(), "HumusForm"); err == nil || rows != nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal("preserved size/modtime concealed corruption", err)
	}
}
