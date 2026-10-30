//go:build !windows

package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type nativeFileFilter struct {
	DisplayName string
	Pattern     string
}

func promptNativeOpenFileWithTitle(app *application.App, window *application.WebviewWindow, handle unsafe.Pointer, title string, filters []nativeFileFilter) (string, error) {
	if app == nil || app.Dialog == nil || window == nil || handle == nil || window.NativeWindow() != handle {
		return "", errors.New("native file selection window changed before its dialog opened")
	}
	dialog := app.Dialog.OpenFile().
		AttachToWindow(window).
		CanChooseFiles(true).
		CanChooseDirectories(false).
		SetTitle(title)
	for _, f := range filters {
		dialog = dialog.AddFilter(f.DisplayName, f.Pattern)
	}
	return dialog.PromptForSingleSelection()
}

func promptNativeSaveFileWithTitle(app *application.App, window *application.WebviewWindow, handle unsafe.Pointer, title, defaultFileName, _ string, filters []nativeFileFilter) (string, error) {
	if app == nil || app.Dialog == nil || window == nil || handle == nil || window.NativeWindow() != handle {
		return "", errors.New("native file save window changed before its dialog opened")
	}
	dialog := app.Dialog.SaveFile().
		AttachToWindow(window).
		CanCreateDirectories(true).
		SetFilename(defaultFileName).
		SetMessage(title)
	for _, f := range filters {
		dialog = dialog.AddFilter(f.DisplayName, f.Pattern)
	}
	return dialog.PromptForSingleSelection()
}

func openPathInSystemFileExplorer(target string, isDir bool) error {
	clean := filepath.Clean(target)
	if runtime.GOOS == "darwin" {
		if isDir {
			return exec.Command("open", clean).Start()
		}
		return exec.Command("open", "-R", clean).Start()
	}
	dir := clean
	if !isDir {
		dir = filepath.Dir(clean)
	}
	return exec.Command("xdg-open", dir).Start()
}
