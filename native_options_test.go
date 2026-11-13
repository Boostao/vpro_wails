package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNativeWindowsOptions(t *testing.T) {
	data := t.TempDir()
	config := t.TempDir()
	options, err := nativeWindowsOptions("", data, config)
	if err != nil || len(options.AdditionalBrowserArgs) != 0 || options.WebviewUserDataPath != filepath.Join(data, "webview") {
		t.Fatalf("default must isolate WebView2 storage under dataDir/webview without enabling inspection: %#v %v", options, err)
	}
	emptyDataOpts, err := nativeWindowsOptions("", "", config)
	if err != nil || emptyDataOpts.WebviewUserDataPath != "" {
		t.Fatalf("empty dataDir fallback: %#v %v", emptyDataOpts, err)
	}
	options, err = nativeWindowsOptions("9337", data, config)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--remote-debugging-port=9337", "--remote-debugging-address=127.0.0.1"}
	if !reflect.DeepEqual(options.AdditionalBrowserArgs, want) || options.WebviewUserDataPath != filepath.Join(config, "webview-debug") {
		t.Fatalf("isolated loopback inspection: %#v", options)
	}
	for _, port := range []string{"bad", "0", "-1", "1023", "65536", "9337 --other-flag"} {
		if _, err := nativeWindowsOptions(port, data, config); err == nil {
			t.Errorf("invalid port %q accepted", port)
		}
	}
}
