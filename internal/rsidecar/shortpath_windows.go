//go:build windows

package rsidecar

import "golang.org/x/sys/windows"

func tryPlatformSpaceFreePath(path string) string {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	n, err := windows.GetShortPathName(ptr, nil, 0)
	if err != nil || n == 0 || n > 32768 {
		return ""
	}
	buf := make([]uint16, n)
	written, err := windows.GetShortPathName(ptr, &buf[0], n)
	if err != nil || written == 0 || written >= n {
		return ""
	}
	return windows.UTF16ToString(buf[:written])
}
