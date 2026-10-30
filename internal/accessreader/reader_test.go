//go:build linux || windows

package accessreader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenInvalidAndMissing(t *testing.T) {
	if _, err := Open("missing-does-not-exist.accdb"); err == nil {
		t.Fatal("expected error for missing file")
	}
	if _, err := Open("invalid\x00path.accdb"); err == nil || !strings.Contains(err.Error(), "invalid Access path") {
		t.Fatalf("expected invalid path error, got %v", err)
	}
	dir := t.TempDir()
	shortFile := filepath.Join(dir, "short.mdb")
	if err := os.WriteFile(shortFile, []byte("not an access db"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(shortFile); err == nil {
		t.Fatal("expected error for invalid short file")
	}

	for _, candidate := range []struct {
		path    string
		project string
		version string
	}{
		{`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\VPro64.accdb`, "Sample", "VP08"},
		{`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro15.accdb`, "VPro15", "VP07"},
		{`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro13.accdb`, "VPro13", "VP06"},
		{`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro03.mdb`, "VPro03", "VP05"},
		{`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVProXP.mdb`, "VProXP", "VP05"},
	} {
		if _, err := os.Stat(candidate.path); err != nil {
			continue
		}
		db, err := Open(candidate.path)
		if err != nil {
			t.Fatalf("Open(%s): %v", candidate.path, err)
		}
		tables, err := db.Tables()
		if err != nil {
			t.Fatalf("Tables(%s): %v", candidate.path, err)
		}
		envName := candidate.project + "_Env"
		found := false
		for _, tbl := range tables {
			if strings.EqualFold(tbl.Name, envName) {
				found = true
				if tbl.Description != candidate.version {
					t.Fatalf("%s.%s description = %q, want %q", candidate.path, tbl.Name, tbl.Description, candidate.version)
				}
				cols, rows, err := db.ReadAll(tbl.Name)
				if err != nil || len(cols) == 0 {
					t.Fatalf("ReadAll(%s.%s): cols=%d rows=%d err=%v", candidate.path, tbl.Name, len(cols), len(rows), err)
				}
			}
		}
		if !found {
			t.Fatalf("%s missing expected table %s", candidate.path, envName)
		}
	}
}
