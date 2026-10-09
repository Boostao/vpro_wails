package main

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type pictureNativeFileDialog interface {
	SetParentWindowHandle(uintptr)
	SetTitle(string) error
	ShowAndGetResult() (string, error)
	Release() error
}

func promptNativePictureFile(_ *application.App, window *application.WebviewWindow, handle unsafe.Pointer) (string, error) {
	return application.InvokeSyncWithResultAndError(func() (string, error) {
		if window == nil || handle == nil || window.NativeWindow() != handle {
			return "", errors.New("native picture selection window changed before its dialog opened")
		}
		if err := pictureNativeCOMInitialization(ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_DISABLE_OLE1DDE)); err != nil {
			return "", err
		}
		defer ole.CoUninitialize()
		dialog, err := newPictureWindowsFileDialog()
		if err != nil {
			return "", fmt.Errorf("create native picture file dialog: %w", err)
		}
		return readPictureNativeFileDialog(dialog, uintptr(handle))
	})
}

func pictureNativeCOMInitialization(err error) error {
	var result *ole.OleError
	if err == nil || errors.As(err, &result) && result.Code() == 1 {
		return nil
	}
	return fmt.Errorf("initialize native picture file dialog apartment: %w", err)
}

func readPictureNativeFileDialog(dialog pictureNativeFileDialog, owner uintptr) (selected string, resultErr error) {
	if dialog == nil {
		return "", errors.New("native picture file dialog is unavailable")
	}
	defer func() {
		if err := dialog.Release(); err != nil {
			selected = ""
			resultErr = errors.Join(resultErr, fmt.Errorf("release native picture file dialog: %w", err))
		}
	}()
	if owner == 0 {
		return "", errors.New("native picture file dialog requires an owned window")
	}
	dialog.SetParentWindowHandle(owner)
	if err := dialog.SetTitle("Add a Picture"); err != nil {
		return "", fmt.Errorf("set native picture file dialog title: %w", err)
	}
	selected, err := dialog.ShowAndGetResult()
	if errors.Is(err, errPictureNativeFileCancelled) {
		if selected != "" {
			return "", errors.New("cancelled native picture file dialog returned an unexpected selection")
		}
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if selected == "" {
		return "", errors.New("native picture file dialog returned no selection without cancellation")
	}
	return selected, nil
}
