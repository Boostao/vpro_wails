package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	lifecyclepkg "github.com/boostao/vpro-wails/internal/lifecycle"
	sqlutilpkg "github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func resolveHostEnvironment() (dataDir, configDir string, windows application.WindowsOptions, err error) {
	dataDir, err = sqlutilpkg.UserDataDir()
	if err != nil {
		return "", "", application.WindowsOptions{}, err
	}
	configDir, err = sqlutilpkg.UserConfigDir()
	if err != nil {
		return "", "", application.WindowsOptions{}, err
	}
	rawDebugPort := os.Getenv("VPRO_WEBVIEW_DEBUG_PORT")
	if rawDebugPort != "" && lifecyclepkg.CurrentBuildInfo().BuildMode != lifecyclepkg.BuildModeDev {
		log.Printf("Ignoring VPRO_WEBVIEW_DEBUG_PORT in non-dev build (%s)", lifecyclepkg.CurrentBuildInfo().BuildMode)
	}
	windows, err = nativeWindowsOptions(rawDebugPort, dataDir, configDir)
	return dataDir, configDir, windows, err
}

func nativeWindowsOptions(debugPort, dataDir, configDir string) (application.WindowsOptions, error) {
	return nativeWindowsOptionsForMode(lifecyclepkg.CurrentBuildInfo().BuildMode, debugPort, dataDir, configDir)
}

func nativeWindowsOptionsForMode(buildMode, debugPort, dataDir, configDir string) (application.WindowsOptions, error) {
	options := application.WindowsOptions{}
	// Remote WebView inspection is strictly restricted to developer builds.
	if debugPort == "" || buildMode != lifecyclepkg.BuildModeDev {
		if dataDir != "" {
			options.WebviewUserDataPath = filepath.Join(dataDir, "webview")
		}
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
