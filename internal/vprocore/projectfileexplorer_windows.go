package vprocore

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	fileDialogShow          = 3
	fileDialogSetFileTypes  = 4
	fileDialogSetOptions    = 9
	fileDialogSetFileName   = 15
	fileDialogSetTitle      = 17
	fileDialogGetResult     = 20
	fileDialogSetDefaultExt = 22
	fileSaveDialogCLSID     = "{C0B4E2F3-BA21-4773-8DBA-335EC946EB8B}"
	fileSaveDialogIID       = "{84BCCD23-5FDE-4CDB-AEA4-AF64B83D78AB}"
	fileOpenDialogCLSID     = "{DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7}"
	fileOpenDialogIID       = "{D57C7288-D4AD-4768-BE02-9D969532D960}"
	pictureShellDisplayName = 5
	pictureCancelledHR      = 0x800704c7
)

type comDlgFilterSpec struct {
	name *uint16
	spec *uint16
}

type nativeFileFilter struct {
	DisplayName string
	Pattern     string
}

func promptNativeOpenFileWithTitle(_ *application.App, window *application.WebviewWindow, handle unsafe.Pointer, title string, filters []nativeFileFilter) (string, error) {
	return application.InvokeSyncWithResultAndError(func() (string, error) {
		if window == nil || handle == nil || window.NativeWindow() != handle {
			return "", errors.New("native file selection window changed before its dialog opened")
		}
		if err := pictureNativeCOMInitialization(ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_DISABLE_OLE1DDE)); err != nil {
			return "", err
		}
		defer ole.CoUninitialize()
		object, err := ole.CreateInstance(ole.NewGUID(fileOpenDialogCLSID), ole.NewGUID(fileOpenDialogIID))
		if err != nil || object == nil {
			return "", fmt.Errorf("create native file dialog: %w", err)
		}
		defer object.Release()
		return runNativeWindowsFileDialog(object, uintptr(handle), title, "", "", filters, false)
	})
}

func promptNativeSaveFileWithTitle(_ *application.App, window *application.WebviewWindow, handle unsafe.Pointer, title, defaultFileName, defaultExt string, filters []nativeFileFilter) (string, error) {
	return application.InvokeSyncWithResultAndError(func() (string, error) {
		if window == nil || handle == nil || window.NativeWindow() != handle {
			return "", errors.New("native file save window changed before its dialog opened")
		}
		if err := pictureNativeCOMInitialization(ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_DISABLE_OLE1DDE)); err != nil {
			return "", err
		}
		defer ole.CoUninitialize()
		object, err := ole.CreateInstance(ole.NewGUID(fileSaveDialogCLSID), ole.NewGUID(fileSaveDialogIID))
		if err != nil || object == nil {
			return "", fmt.Errorf("create native save file dialog: %w", err)
		}
		defer object.Release()
		return runNativeWindowsFileDialog(object, uintptr(handle), title, defaultFileName, defaultExt, filters, true)
	})
}

func runNativeWindowsFileDialog(object *ole.IUnknown, owner uintptr, title, defaultFileName, defaultExt string, filters []nativeFileFilter, saveMode bool) (string, error) {
	if owner == 0 {
		return "", errors.New("native file dialog requires an owned window")
	}
	table := (*[24]uintptr)(unsafe.Pointer(object.RawVTable))
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}
	result, _, _ := syscall.SyscallN(table[fileDialogSetTitle], uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(titlePtr)))
	runtime.KeepAlive(titlePtr)
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("set native file dialog title: %w", err)
	}
	if len(filters) > 0 {
		specs := make([]comDlgFilterSpec, len(filters))
		keepAlive := make([]*uint16, 0, len(filters)*2)
		for i, f := range filters {
			namePtr, err := syscall.UTF16PtrFromString(f.DisplayName)
			if err != nil {
				return "", err
			}
			specPtr, err := syscall.UTF16PtrFromString(f.Pattern)
			if err != nil {
				return "", err
			}
			specs[i] = comDlgFilterSpec{name: namePtr, spec: specPtr}
			keepAlive = append(keepAlive, namePtr, specPtr)
		}
		result, _, _ = syscall.SyscallN(table[fileDialogSetFileTypes], uintptr(unsafe.Pointer(object)), uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0])))
		runtime.KeepAlive(specs)
		runtime.KeepAlive(keepAlive)
		if err := pictureNativeHRESULT(result); err != nil {
			return "", fmt.Errorf("set native file dialog filters: %w", err)
		}
	}
	if defaultFileName != "" {
		namePtr, err := syscall.UTF16PtrFromString(defaultFileName)
		if err != nil {
			return "", err
		}
		result, _, _ = syscall.SyscallN(table[fileDialogSetFileName], uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(namePtr)))
		runtime.KeepAlive(namePtr)
		if err := pictureNativeHRESULT(result); err != nil {
			return "", fmt.Errorf("set native file dialog default filename: %w", err)
		}
	}
	if defaultExt != "" {
		extPtr, err := syscall.UTF16PtrFromString(defaultExt)
		if err != nil {
			return "", err
		}
		result, _, _ = syscall.SyscallN(table[fileDialogSetDefaultExt], uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(extPtr)))
		runtime.KeepAlive(extPtr)
		if err := pictureNativeHRESULT(result); err != nil {
			return "", fmt.Errorf("set native file dialog default extension: %w", err)
		}
	}
	// FOS_FORCEFILESYSTEM (0x40) | FOS_PATHMUSTEXIST (0x800) | FOS_NOCHANGEDIR (0x8)
	options := uintptr(0x40 | 0x800 | 0x8)
	if !saveMode {
		options |= 0x1000 // FOS_FILEMUSTEXIST
	}
	result, _, _ = syscall.SyscallN(table[fileDialogSetOptions], uintptr(unsafe.Pointer(object)), options)
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("set native file dialog options: %w", err)
	}
	result, _, _ = syscall.SyscallN(table[fileDialogShow], uintptr(unsafe.Pointer(object)), owner)
	if uint32(result) == pictureCancelledHR {
		return "", nil
	}
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("show native file dialog: %w", err)
	}
	var item *ole.IUnknown
	result, _, _ = syscall.SyscallN(table[fileDialogGetResult], uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(&item)))
	if item != nil {
		defer item.Release()
	}
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("get native file dialog selection: %w", err)
	}
	if item == nil {
		return "", errors.New("native file dialog returned no filesystem item")
	}
	shellTable := (*[6]uintptr)(unsafe.Pointer(item.RawVTable))
	var path *uint16
	result, _, _ = syscall.SyscallN(shellTable[pictureShellDisplayName], uintptr(unsafe.Pointer(item)), 0x80058000, uintptr(unsafe.Pointer(&path)))
	if path != nil {
		defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(path)))
	}
	if err := pictureNativeHRESULT(result); err != nil {
		return "", fmt.Errorf("get native file dialog path: %w", err)
	}
	if path == nil {
		return "", errors.New("native file dialog returned no filesystem path")
	}
	units := unsafe.Slice(path, 32768)
	for end, value := range units {
		if value == 0 {
			return pictureNativePathUTF16(units[:end])
		}
	}
	return "", errors.New("native file dialog path exceeds the Windows path bound")
}

func openPathInSystemFileExplorer(target string, isDir bool) error {
	clean := filepath.Clean(target)
	if isDir {
		return exec.Command("explorer.exe", clean).Start()
	}
	return exec.Command("explorer.exe", "/select,"+clean).Start()
}

func pictureNativeCOMInitialization(err error) error {
	var result *ole.OleError
	if err == nil || errors.As(err, &result) && result.Code() == 1 {
		return nil
	}
	return fmt.Errorf("initialize native file dialog apartment: %w", err)
}

func pictureNativeHRESULT(result uintptr) error {
	if int32(uint32(result)) < 0 {
		return ole.NewError(uintptr(uint32(result)))
	}
	return nil
}

func pictureNativePathUTF16(units []uint16) (string, error) {
	for index := 0; index < len(units); index++ {
		value := units[index]
		if value >= 0xd800 && value <= 0xdbff {
			if index+1 >= len(units) || units[index+1] < 0xdc00 || units[index+1] > 0xdfff {
				return "", errors.New("native file dialog path contains invalid UTF-16")
			}
			index++
		} else if value >= 0xdc00 && value <= 0xdfff || value == 0 {
			return "", errors.New("native file dialog path contains invalid UTF-16")
		}
	}
	return string(utf16.Decode(units)), nil
}
