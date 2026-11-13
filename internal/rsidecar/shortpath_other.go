//go:build !windows

package rsidecar

func tryPlatformSpaceFreePath(string) string {
	return ""
}
