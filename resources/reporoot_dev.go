//go:build !production

package resources

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var resolveRepoRoot = sync.OnceValue(func() string {
	if testing.Testing() {
		if _, file, _, ok := runtime.Caller(0); ok {
			candidate := filepath.Dir(filepath.Dir(file))
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	abs, err := filepath.Abs(".")
	if err != nil {
		return "."
	}
	return abs
})

// RepoRoot returns the absolute path to the repository root directory during tests,
// or the current working directory outside test binaries.
func RepoRoot() string {
	return resolveRepoRoot()
}
