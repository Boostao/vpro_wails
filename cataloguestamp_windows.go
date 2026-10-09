package main

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func catalogueFileStamp(path string) (_ catalogueStamp, resultErr error) {
	info, err := os.Stat(path)
	if err != nil {
		return catalogueStamp{}, fmt.Errorf("stat catalogue: %w", err)
	}
	if !info.Mode().IsRegular() {
		return catalogueStamp{}, fmt.Errorf("catalogue is not a regular file: %s", path)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return catalogueStamp{}, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return catalogueStamp{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, windows.CloseHandle(handle)) }()
	var identity windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &identity); err != nil {
		return catalogueStamp{}, fmt.Errorf("query catalogue identity: %w", err)
	}
	// FILE_BASIC_INFO requires eight-byte alignment, including its output buffer.
	var basic [5]uint64
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileBasicInfo,
		(*byte)(unsafe.Pointer(&basic[0])), uint32(unsafe.Sizeof(basic))); err != nil {
		return catalogueStamp{}, fmt.Errorf("query catalogue change time: %w", err)
	}
	return catalogueStamp{info: info,
		identity: fmt.Sprintf("%x:%x:%x", identity.VolumeSerialNumber, identity.FileIndexHigh, identity.FileIndexLow),
		change:   basic[3]}, nil
}
