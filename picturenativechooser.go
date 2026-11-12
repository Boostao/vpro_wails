package main

import (
	"context"
	"errors"
	"fmt"
)

func chooseNativePictureFile(ctx context.Context) (string, error) {
	app, window, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	handle := window.NativeWindow()
	if handle == nil {
		return "", errors.New("native picture selection window closed before opening its dialog")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selected, err := promptNativePictureFile(app, window, handle)
	if err != nil {
		return "", fmt.Errorf("native picture selection failed: %w", err)
	}
	currentApp, current, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	if currentApp != app || current != window || current.NativeWindow() != handle {
		return "", errors.New("native picture selection owner changed while the dialog was open")
	}
	if selected == "" {
		return "", nil
	}
	if _, _, err := splitPictureCreationPath(selected); err != nil {
		return "", fmt.Errorf("native picture selection cannot preserve the returned literal path: %w", err)
	}
	return selected, nil
}
