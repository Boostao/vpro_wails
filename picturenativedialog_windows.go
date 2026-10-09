package main

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var errPictureNativeFileCancelled = errors.New("native picture selection was cancelled")

type pictureWindowsFileDialog struct {
	object *ole.IUnknown
	owner  uintptr
}

// COM slots are defined by IFileDialog and IShellItem in shobjidl_core.h.
const (
	pictureDialogShow       = 3
	pictureDialogSetOptions = 9
	pictureDialogSetTitle   = 17
	pictureDialogGetResult  = 20
	pictureShellDisplayName = 5
	pictureCancelledHR      = 0x800704c7
)

func newPictureWindowsFileDialog() (*pictureWindowsFileDialog, error) {
	object, err := ole.CreateInstance(
		ole.NewGUID("{DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7}"),
		ole.NewGUID("{D57C7288-D4AD-4768-BE02-9D969532D960}"))
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, errors.New("native file dialog creation returned no interface")
	}
	return &pictureWindowsFileDialog{object: object}, nil
}

func (dialog *pictureWindowsFileDialog) SetParentWindowHandle(owner uintptr) {
	dialog.owner = owner
}

func (dialog *pictureWindowsFileDialog) SetTitle(title string) error {
	value, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}
	table := (*[21]uintptr)(unsafe.Pointer(dialog.object.RawVTable))
	result, _, _ := syscall.SyscallN(table[pictureDialogSetTitle], uintptr(unsafe.Pointer(dialog.object)), uintptr(unsafe.Pointer(value)))
	runtime.KeepAlive(value)
	return pictureNativeHRESULT(result)
}

func (dialog *pictureWindowsFileDialog) ShowAndGetResult() (string, error) {
	table := (*[21]uintptr)(unsafe.Pointer(dialog.object.RawVTable))
	// Existing filesystem file, single selection, and no process-directory change.
	result, _, _ := syscall.SyscallN(table[pictureDialogSetOptions], uintptr(unsafe.Pointer(dialog.object)), 0x40|0x800|0x1000|0x8)
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("set native picture selection options: %w", err)
	}
	result, _, _ = syscall.SyscallN(table[pictureDialogShow], uintptr(unsafe.Pointer(dialog.object)), dialog.owner)
	if err := pictureNativeShowResult(result); err != nil {
		return "", err
	}
	var item *ole.IUnknown
	result, _, _ = syscall.SyscallN(table[pictureDialogGetResult], uintptr(unsafe.Pointer(dialog.object)), uintptr(unsafe.Pointer(&item)))
	if item != nil {
		defer item.Release()
	}
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("get native picture selection: %w", err)
	}
	if item == nil {
		return "", errors.New("native picture selection returned no filesystem item")
	}
	shellTable := (*[6]uintptr)(unsafe.Pointer(item.RawVTable))
	var path *uint16
	result, _, _ = syscall.SyscallN(shellTable[pictureShellDisplayName], uintptr(unsafe.Pointer(item)), 0x80058000, uintptr(unsafe.Pointer(&path)))
	if path != nil {
		defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(path)))
	}
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("get native picture filesystem path: %w", err)
	}
	if path == nil {
		return "", errors.New("native picture selection returned no filesystem path")
	}
	units := unsafe.Slice(path, 32768)
	for end, value := range units {
		if value == 0 {
			return pictureNativePathUTF16(units[:end])
		}
	}
	return "", errors.New("native picture filesystem path exceeds the Windows path bound")
}

func (dialog *pictureWindowsFileDialog) Release() error {
	dialog.object.Release()
	dialog.object = nil
	return nil
}

func pictureNativeHRESULT(result uintptr) error {
	if int32(uint32(result)) < 0 {
		return ole.NewError(uintptr(uint32(result)))
	}
	return nil
}

func pictureNativeShowResult(result uintptr) error {
	if uint32(result) == pictureCancelledHR {
		return errPictureNativeFileCancelled
	}
	if err := pictureNativeHRESULT(result); err != nil {
		return fmt.Errorf("show native picture selection: %w", err)
	}
	return nil
}

func pictureNativePathUTF16(units []uint16) (string, error) {
	for index := 0; index < len(units); index++ {
		value := units[index]
		if value >= 0xd800 && value <= 0xdbff {
			if index+1 >= len(units) || units[index+1] < 0xdc00 || units[index+1] > 0xdfff {
				return "", errors.New("native picture filesystem path contains invalid UTF-16")
			}
			index++
		} else if value >= 0xdc00 && value <= 0xdfff || value == 0 {
			return "", errors.New("native picture filesystem path contains invalid UTF-16")
		}
	}
	return string(utf16.Decode(units)), nil
}
