package main

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func nativeWindowsOptions(debugPort, configDir string) (application.WindowsOptions, error) {
	options := application.WindowsOptions{}
	if debugPort == "" {
		return options, nil
	}
	port, err := strconv.Atoi(debugPort)
	if err != nil || port < 1024 || port > 65535 {
		return options, fmt.Errorf("VPRO_WEBVIEW_DEBUG_PORT must be an integer between 1024 and 65535")
	}
	options.AdditionalBrowserArgs = []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=127.0.0.1",
	}
	// A separate WebView environment prevents an existing browser process from
	// ignoring new flags or sharing the normal application's browser storage.
	options.WebviewUserDataPath = filepath.Join(configDir, "webview-debug")
	return options, nil
}
