package sqlutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"syscall"
)

// IsHardlinkUnsupportedError reports whether an os.Link error indicates solely
// that the underlying filesystem or mount does not support hard links (such as
// FAT32, exFAT, SMB/CIFS, NFS, or FUSE mounts), as opposed to a collision,
// permission denial on a hardlink-capable filesystem, or injected I/O fault.
func IsHardlinkUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errors.ErrUnsupported) ||
		errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.EOPNOTSUPP) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.ENOSYS) ||
		errors.Is(err, syscall.EXDEV) {
		return true
	}
	if runtime.GOOS == "windows" {
		// ERROR_INVALID_FUNCTION (1), ERROR_NOT_SAME_DEVICE (17), and ERROR_NOT_SUPPORTED (50) on FAT32/exFAT/SMB
		var errno syscall.Errno
		if errors.As(err, &errno) && (errno == 1 || errno == 17 || errno == 50) {
			return true
		}
	}
	return false
}

// LinkOrCreateExclusive atomically publishes from -> to without ever overwriting
// an existing target. It prefers os.Link and falls back to O_CREATE|O_EXCL copy
// exclusively when the target filesystem does not support hard links.
// In both paths, `from` remains intact for caller-owned stage cleanup.
func LinkOrCreateExclusive(from, to string) error {
	_, err := LinkOrCreateExclusiveWith(from, to, nil)
	return err
}

// LinkOrCreateExclusiveWith is LinkOrCreateExclusive with an injectable linkFn
// for deterministic unit testing of hardlink-unsupported fallbacks.
func LinkOrCreateExclusiveWith(from, to string, linkFn func(string, string) error) (bool, error) {
	if linkFn == nil {
		linkFn = os.Link
	}
	linkErr := linkFn(from, to)
	if linkErr == nil {
		return false, nil
	}
	if !IsHardlinkUnsupportedError(linkErr) {
		return false, linkErr
	}

	src, err := os.Open(from)
	if err != nil {
		return false, err
	}
	defer src.Close()

	srcInfo, err := src.Stat()
	if err != nil {
		return false, err
	}
	if !srcInfo.Mode().IsRegular() {
		return false, fmt.Errorf("staged source %q is not a regular file", from)
	}

	perm := srcInfo.Mode().Perm()
	if perm == 0 {
		perm = 0o600
	}
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return false, err
	}
	dstInfo, statErr := dst.Stat()
	cleanupFailedTarget := func() {
		if dstInfo == nil {
			return
		}
		if cur, lErr := os.Lstat(to); lErr == nil && os.SameFile(dstInfo, cur) {
			_ = os.Remove(to)
		}
	}
	if statErr != nil {
		_ = dst.Close()
		cleanupFailedTarget()
		return false, statErr
	}
	written, copyErr := io.Copy(dst, src)
	if copyErr == nil && written != srcInfo.Size() {
		copyErr = io.ErrShortWrite
	}
	var syncErr error
	if copyErr == nil {
		syncErr = dst.Sync()
	}
	closeErr := dst.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		cleanupFailedTarget()
		return false, err
	}
	return true, nil
}
