package vprocore

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSameDesktopPathResolvesSymlinksAndPreservesLexicalFallback(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "Sample.db")
	if err := os.WriteFile(realFile, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(dir, "Other.db")
	if err := os.WriteFile(otherFile, []byte("sqlite2"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !SameDesktopPath(realFile, filepath.Join(dir, ".", "Sample.db")) {
		t.Fatal("SameDesktopPath rejected cleaned alias of existing file")
	}
	if SameDesktopPath(realFile, otherFile) {
		t.Fatal("SameDesktopPath equated two distinct existing files")
	}

	linkDir := filepath.Join(dir, "linked")
	if err := os.Symlink(dir, linkDir); err == nil {
		if !SameDesktopPath(realFile, filepath.Join(linkDir, "Sample.db")) {
			t.Fatal("SameDesktopPath did not resolve symlinked directory via os.SameFile")
		}
	}

	missingA := filepath.Join(dir, "nonexistent", "A.db")
	missingB := filepath.Join(dir, "nonexistent", ".", "A.db")
	if !SameDesktopPath(missingA, missingB) {
		t.Fatal("SameDesktopPath lexical fallback rejected cleaned non-existent path")
	}
	if runtime.GOOS == "windows" {
		if !SameDesktopPath(`C:\NonExistent\Project.db`, `c:\nonexistent\project.db`) {
			t.Fatal("SameDesktopPath Windows lexical fallback rejected case-folded path")
		}
	}
}
