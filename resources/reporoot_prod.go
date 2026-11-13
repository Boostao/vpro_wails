//go:build production

package resources

import (
	"path/filepath"
	"sync"
)

var resolveRepoRoot = sync.OnceValue(func() string {
	abs, err := filepath.Abs(".")
	if err != nil {
		return "."
	}
	return abs
})

// RepoRoot avoids runtime.Caller in production binaries so build-host filesystem paths never leak.
func RepoRoot() string {
	return resolveRepoRoot()
}
