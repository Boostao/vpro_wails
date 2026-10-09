package main

import (
	"context"
	"errors"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestPictureNativeChooserRefusesUnownedRequestsBeforeDialog(t *testing.T) {
	var missing *application.WebviewWindow
	for _, test := range []struct {
		name string
		ctx  context.Context
	}{
		{"nil-context", nil},
		{"no-window", t.Context()},
		{"wrong-key-type", context.WithValue(t.Context(), "Window", &application.WebviewWindow{})},
		{"typed-nil-window", context.WithValue(t.Context(), application.WindowKey, missing)},
		{"nonnative-window", context.WithValue(t.Context(), application.WindowKey, "browser")},
		{"not-live-or-registered", context.WithValue(t.Context(), application.WindowKey, &application.WebviewWindow{})},
	} {
		t.Run(test.name, func(t *testing.T) {
			if selected, err := chooseNativePictureFile(test.ctx); err == nil || selected != "" {
				t.Fatal("unowned request opened a dialog or returned a selection", selected, err)
			}
		})
	}
}

func TestPictureNativeChooserCancelledRequestNeverOpensDialog(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if selected, err := chooseNativePictureFile(ctx); !errors.Is(err, context.Canceled) || selected != "" {
		t.Fatal("cancelled request acquired a selection", selected, err)
	}
}
