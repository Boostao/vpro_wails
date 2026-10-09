package main

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unicode/utf16"

	"github.com/go-ole/go-ole"
)

type pictureNativeDialogStub struct {
	parent     uintptr
	title      string
	calls      []string
	path       string
	titleErr   error
	showErr    error
	releaseErr error
}

func TestPictureNativeHRESULTAndShowCancellationAreDistinct(t *testing.T) {
	for _, result := range []uintptr{0, 1} {
		if err := pictureNativeHRESULT(result); err != nil {
			t.Fatal("successful HRESULT refused", result, err)
		}
	}
	for _, result := range []uintptr{pictureCancelledHR, 0x80070005, 0x80004005, ^uintptr(0)} {
		err := pictureNativeHRESULT(result)
		var native *ole.OleError
		if !errors.As(err, &native) || native.Code() != uintptr(uint32(result)) || errors.Is(err, errPictureNativeFileCancelled) {
			t.Fatal("failure HRESULT was lost or treated as Show cancellation", result, err)
		}
		showErr := pictureNativeShowResult(result)
		if showErr == nil || errors.Is(showErr, errPictureNativeFileCancelled) != (uint32(result) == pictureCancelledHR) {
			t.Fatal("only Show ERROR_CANCELLED may cancel", result, showErr)
		}
	}
}

func TestPictureNativeUTF16PathIsLiteralAndRejectsRepair(t *testing.T) {
	literal := `Q:\  Literal #100% 😀\Case.JpG`
	got, err := pictureNativePathUTF16(utf16.Encode([]rune(literal)))
	if got != literal || err != nil {
		t.Fatal("native path was changed", got, err)
	}
	for _, units := range [][]uint16{{0}, {0xd800}, {0xdc00}, {0xd800, 'x'}, {'x', 0xdc00}, {0xd800, 0xd800}} {
		if got, err := pictureNativePathUTF16(units); err == nil || got != "" {
			t.Fatal("malformed native path was repaired", units, got, err)
		}
	}
}

func TestPictureNativeCOMInitializationPreservesFailure(t *testing.T) {
	for _, err := range []error{nil, ole.NewError(1)} {
		if result := pictureNativeCOMInitialization(err); result != nil {
			t.Fatal("successful COM initialization was rejected", result)
		}
	}
	for _, err := range []error{ole.NewError(0x80010106), errors.New("operation completed successfully")} {
		if result := pictureNativeCOMInitialization(err); result == nil || !errors.Is(result, err) {
			t.Fatal("COM initialization failure was lost", result)
		}
	}
}

func (d *pictureNativeDialogStub) SetParentWindowHandle(parent uintptr) {
	d.parent = parent
	d.calls = append(d.calls, "parent")
}
func (d *pictureNativeDialogStub) SetTitle(title string) error {
	d.title = title
	d.calls = append(d.calls, "title")
	return d.titleErr
}
func (d *pictureNativeDialogStub) ShowAndGetResult() (string, error) {
	d.calls = append(d.calls, "show")
	return d.path, d.showErr
}
func (d *pictureNativeDialogStub) Release() error {
	d.calls = append(d.calls, "release")
	return d.releaseErr
}

func TestPictureNativeFileDialogTypedCancelAndLiteralSelection(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		dialog := &pictureNativeDialogStub{path: `Q:\  Literal #100% 😀\Case.JpG`}
		if cancelled {
			dialog.path = ""
			dialog.showErr = fmt.Errorf("wrapped native result: %w", errPictureNativeFileCancelled)
		}
		selected, err := readPictureNativeFileDialog(dialog, 123)
		if err != nil || selected != dialog.path || dialog.parent != 123 || dialog.title != "Add a Picture" ||
			!reflect.DeepEqual(dialog.calls, []string{"parent", "title", "show", "release"}) {
			t.Fatal("native typed result or lifetime differs", selected, err, dialog)
		}
	}
}

func TestPictureNativeFileDialogNeverRepairsFailureIntoCancel(t *testing.T) {
	failure := errors.New("injected native failure")
	for _, stage := range []string{"nil", "owner", "title", "show", "same-text", "empty", "cancel-path", "cancel-release", "selected-release", "show-release"} {
		t.Run(stage, func(t *testing.T) {
			dialog := &pictureNativeDialogStub{}
			var input pictureNativeFileDialog = dialog
			owner := uintptr(123)
			switch stage {
			case "nil":
				input = nil
			case "owner":
				owner = 0
			case "title":
				dialog.titleErr = failure
			case "show":
				dialog.showErr = failure
			case "same-text":
				dialog.showErr = errors.New(errPictureNativeFileCancelled.Error())
			case "empty":
			case "cancel-path":
				dialog.path = `Q:\unexpected.jpg`
				dialog.showErr = errPictureNativeFileCancelled
			case "cancel-release":
				dialog.showErr = errPictureNativeFileCancelled
				dialog.releaseErr = failure
			case "selected-release":
				dialog.path = `Q:\selected.jpg`
				dialog.releaseErr = failure
			case "show-release":
				dialog.showErr = errors.New("independent Show failure")
				dialog.releaseErr = failure
			}
			selected, err := readPictureNativeFileDialog(input, owner)
			if selected != "" || err == nil {
				t.Fatal("native failure appeared as selection or Cancel", selected, err)
			}
			if stage != "nil" && (len(dialog.calls) == 0 || dialog.calls[len(dialog.calls)-1] != "release") {
				t.Fatal("acquired native dialog was not released", dialog.calls)
			}
			if stage == "owner" && !reflect.DeepEqual(dialog.calls, []string{"release"}) ||
				stage == "title" && !reflect.DeepEqual(dialog.calls, []string{"parent", "title", "release"}) {
				t.Fatal("failed setup entered native Show", dialog.calls)
			}
			if stage == "show-release" && (!errors.Is(err, failure) || !errors.Is(err, dialog.showErr)) {
				t.Fatal("cleanup erased independent Show error", err)
			}
			if stage == "same-text" && !errors.Is(err, dialog.showErr) {
				t.Fatal("ordinary same-text error was misclassified as typed Cancel", err)
			}
		})
	}
}
