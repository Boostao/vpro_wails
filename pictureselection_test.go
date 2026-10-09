package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var pictureSelectionFixtureSequence atomic.Uint64

func pictureSelectionDirectory(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("literal Windows selected-file contract")
	}
	directory, err := filepath.Abs(fmt.Sprintf(".picture-selection-fixture-%d-%d", os.Getpid(), pictureSelectionFixtureSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	return directory
}

func pictureSelectionBytes(t *testing.T, format string) []byte {
	t.Helper()
	pixels := image.NewRGBA(image.Rect(0, 0, 3, 2))
	pixels.Set(1, 1, color.RGBA{R: 171, G: 41, B: 83, A: 255})
	var encoded bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&encoded, pixels, &jpeg.Options{Quality: 91})
	case "png":
		err = png.Encode(&encoded, pixels)
	case "gif":
		err = gif.Encode(&encoded, pixels, nil)
	default:
		t.Fatal("unknown fixture format")
	}
	if err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func pictureSelectionWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func pictureSelectionOwn(t *testing.T, path string) *ownedPictureSelection {
	t.Helper()
	selection, err := newOwnedPictureSelection(path)
	if err != nil {
		t.Fatal(err)
	}
	return selection
}

func pictureSelectionRefused(t *testing.T, selection *ownedPictureSelection, ctx context.Context) error {
	t.Helper()
	got, err := selection.readImage(ctx)
	if err == nil || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatalf("failed preview returned an image: %#v, %v", got, err)
	}
	return err
}

func pictureSelectionAlias(t *testing.T, directory, target string) (string, func(string)) {
	t.Helper()
	link := filepath.Join(directory, "Alias # 😀 %;'")
	junction := false
	if err := os.Symlink(target, link); err != nil {
		t.Logf("file symlink unavailable; using owned directory junction: %v", err)
		junction = true
	}
	create := func(target string) error {
		if !junction {
			return os.Symlink(target, link)
		}
		command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
			`$ErrorActionPreference='Stop'; New-Item -ItemType Junction -Path $env:VPRO_SELECTION_ALIAS -Target $env:VPRO_SELECTION_TARGET | Out-Null`)
		command.Env = append(os.Environ(), "VPRO_SELECTION_ALIAS="+link, "VPRO_SELECTION_TARGET="+filepath.Dir(target))
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("owned no-privilege junction creation failed: %w: %s", err, output)
		}
		return nil
	}
	if junction {
		if err := create(target); err != nil {
			t.Fatal(err)
		}
	}
	// Remove the alias itself before the fixture tree; never traverse its target.
	t.Cleanup(func() {
		if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Error(err)
		}
	})
	requested := link
	if junction {
		requested = filepath.Join(link, filepath.Base(target))
	}
	return requested, func(replacement string) {
		t.Helper()
		if junction && filepath.Base(replacement) != filepath.Base(target) {
			t.Fatal("junction retarget fixture must preserve the literal filename")
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := create(replacement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPictureSelectionLiteralPreviewAndNoSiblingGrant(t *testing.T) {
	for _, format := range []string{"jpeg", "png"} {
		t.Run(format, func(t *testing.T) {
			directory := pictureSelectionDirectory(t)
			path := filepath.Join(directory, "  Literal é 😀 #100%;' .JpG")
			data := pictureSelectionBytes(t, format)
			pictureSelectionWrite(t, path, data)
			requested := strings.ToLower(path)
			selection := pictureSelectionOwn(t, requested)
			if selection.requestedPath != requested || selection.info == nil || !filepath.IsAbs(selection.path) {
				t.Fatal("selection repaired the stored literal or lost physical identity")
			}
			if err := selection.checkFile(); err != nil {
				t.Fatal(err)
			}
			got, err := selection.readImage(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(data)
			expected := pictureImage{
				MIME: "image/" + format, Width: 3, Height: 2, SHA256: hex.EncodeToString(hash[:]),
				DataURL: "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(data),
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("independent byte/hash/dimension expectation failed: %#v", got)
			}
			sibling := filepath.Join(directory, "sibling.png")
			pictureSelectionWrite(t, sibling, data)
			// Even substituting a sibling path cannot extend the captured identity.
			forged := *selection
			forged.requestedPath, forged.path = sibling, sibling
			if err := forged.checkFile(); err == nil {
				t.Fatal("sibling inherited selected-file authority")
			}
			pictureSelectionRefused(t, &forged, context.Background())
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, data) {
				t.Fatal("preview changed selected bytes", err)
			}
		})
	}
}

func TestPictureSelectionRejectsUnrepairedPaths(t *testing.T) {
	directory := pictureSelectionDirectory(t)
	prefix := directory + `\`
	for _, path := range []string{
		"", "relative.png", `C:relative.png`, `\rooted.png`, `C:/image.png`,
		`\\?\C:\image.png`, `\\.\C:\image.png`, `\\server\share`, `\\server\share\`,
		prefix + `.\p.png`, prefix + `..\p.png`, prefix + `sub\\p.png`,
		prefix + `p.png:ads`, prefix + `p.png.`, prefix + `p.png `,
		prefix + `sub \p.png`, prefix + `sub.\p.png`, prefix + `CON.png`,
		prefix + `con .png`, prefix + `NUL`, prefix + `COM1.png`, prefix + `LPT³.png`,
		prefix + `p?.png`, prefix + `p*.png`, prefix + `p|.png`, prefix + `p<.png`,
		prefix + "p\x00.png", prefix + "p\n.png", prefix + "p\xff.png",
		prefix + strings.Repeat("😀", 128),
		`C:\` + strings.Repeat("d", 252) + `\p`,
		filepath.Join(directory, "missing.png"), directory,
	} {
		if selection, err := newOwnedPictureSelection(path); err == nil || selection != nil {
			t.Fatalf("invalid/missing/nonregular path authorized: %q, %#v, %v", path, selection, err)
		}
	}
	subdirectory := filepath.Join(directory, "folder.png")
	if err := os.Mkdir(subdirectory, 0700); err != nil {
		t.Fatal(err)
	}
	if selection, err := newOwnedPictureSelection(subdirectory); err == nil || selection != nil {
		t.Fatal("directory acquired file authority", selection, err)
	}
	var unavailable *ownedPictureSelection
	if err := unavailable.checkFile(); err == nil {
		t.Fatal("nil selection accepted")
	}
	pictureSelectionRefused(t, unavailable, context.Background())
	pictureSelectionRefused(t, &ownedPictureSelection{}, context.Background())
}

func TestPictureSelectionFrozenOpenedHandleRefusesActualReplacement(t *testing.T) {
	directory := pictureSelectionDirectory(t)
	path := filepath.Join(directory, "selected.png")
	data := pictureSelectionBytes(t, "png")
	pictureSelectionWrite(t, path, data)
	selection := pictureSelectionOwn(t, path)
	old := filepath.Join(directory, "original.png")
	if err := os.Rename(path, old); err != nil {
		t.Fatal(err)
	}
	pictureSelectionWrite(t, path, data)
	if err := os.Chtimes(path, selection.info.ModTime(), selection.info.ModTime()); err != nil {
		t.Fatal(err)
	}
	// Identical bytes, size and time leave physical file identity as the witness.
	if err := selection.checkFile(); err == nil {
		t.Fatal("replacement before staging passed frozen handle identity")
	}
	pictureSelectionRefused(t, selection, context.Background())
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(old, path); err != nil {
		t.Fatal(err)
	}
	if err := selection.checkFile(); err != nil {
		t.Fatal("exact physical restoration refused", err)
	}
	if _, err := selection.readImage(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A completed preview cannot authorize a later replacement at the same path.
	if err := os.Rename(path, old); err != nil {
		t.Fatal(err)
	}
	pictureSelectionWrite(t, path, data)
	if err := os.Chtimes(path, selection.info.ModTime(), selection.info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := selection.checkFile(); err == nil {
		t.Fatal("replacement after completed staging passed frozen handle identity")
	}
	pictureSelectionRefused(t, selection, context.Background())
}

func TestPictureSelectionSameFileContentDrift(t *testing.T) {
	for _, drift := range []string{"size", "time"} {
		t.Run(drift, func(t *testing.T) {
			directory := pictureSelectionDirectory(t)
			path := filepath.Join(directory, "selected.png")
			data := pictureSelectionBytes(t, "png")
			pictureSelectionWrite(t, path, data)
			selection := pictureSelectionOwn(t, path)
			if drift == "size" {
				pictureSelectionWrite(t, path, append(data, 0))
			} else {
				data[len(data)-1] ^= 1
				pictureSelectionWrite(t, path, data)
				changed := selection.info.ModTime().Add(2 * time.Second)
				if err := os.Chtimes(path, changed, changed); err != nil {
					t.Fatal(err)
				}
			}
			if err := selection.checkFile(); err == nil {
				t.Fatal("same physical file drift accepted")
			}
			pictureSelectionRefused(t, selection, context.Background())
		})
	}
}

func TestPictureSelectionAliasKeepsLiteralAndBothIdentities(t *testing.T) {
	directory := pictureSelectionDirectory(t)
	first := filepath.Join(directory, "first")
	second := filepath.Join(directory, "second")
	for _, dir := range []string{first, second} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(first, "target.png")
	data := pictureSelectionBytes(t, "png")
	pictureSelectionWrite(t, path, data)
	alias, retarget := pictureSelectionAlias(t, directory, path)
	selection := pictureSelectionOwn(t, alias)
	if selection.requestedPath != alias || !strings.EqualFold(selection.path, path) {
		t.Fatal("alias literal or physical resolution lost", selection)
	}
	if _, err := selection.readImage(context.Background()); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(second, "target.png")
	pictureSelectionWrite(t, other, data)
	retarget(other)
	if err := selection.checkFile(); err == nil {
		t.Fatal("retargeted literal alias accepted")
	}
	pictureSelectionRefused(t, selection, context.Background())
	retarget(path)
	if err := selection.checkFile(); err != nil {
		t.Fatal("exact alias target restoration refused", err)
	}
	if err := os.Rename(path, filepath.Join(directory, "old.png")); err != nil {
		t.Fatal(err)
	}
	pictureSelectionWrite(t, path, data)
	if err := selection.checkFile(); err == nil {
		t.Fatal("resolved target replacement accepted")
	}
	pictureSelectionRefused(t, selection, context.Background())
}

type pictureSelectionObservedContext struct {
	context.Context
	checks int
	at     int
	action func()
}

func (ctx *pictureSelectionObservedContext) Err() error {
	ctx.checks++
	if ctx.checks == ctx.at {
		ctx.action()
	}
	return ctx.Context.Err()
}

func TestPictureSelectionCancellationAndRetry(t *testing.T) {
	directory := pictureSelectionDirectory(t)
	source := filepath.Join(directory, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "selected.png")
	pictureSelectionWrite(t, path, pictureSelectionBytes(t, "png"))
	selection := pictureSelectionOwn(t, path)
	pictureSelectionRefused(t, selection, nil)
	for _, inFlight := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		var request context.Context = ctx
		if inFlight {
			// The reader's first check occurs after the checked handle is open.
			request = &pictureSelectionObservedContext{Context: ctx, at: 2, action: cancel}
		} else {
			cancel()
		}
		err := pictureSelectionRefused(t, selection, request)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation lost", err)
		}
		if _, err := selection.readImage(context.Background()); err != nil {
			t.Fatal("cancelled read poisoned retry", err)
		}
	}
}

func TestPictureSelectionRechecksAfterRead(t *testing.T) {
	for _, action := range []string{"handle-time", "literal-replacement"} {
		t.Run(action, func(t *testing.T) {
			directory := pictureSelectionDirectory(t)
			source := filepath.Join(directory, "source")
			if err := os.Mkdir(source, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(source, "selected.png")
			data := pictureSelectionBytes(t, "png")
			pictureSelectionWrite(t, path, data)
			requested := path
			var retarget func(string)
			var replacement string
			if action == "literal-replacement" {
				other := filepath.Join(directory, "replacement")
				if err := os.Mkdir(other, 0700); err != nil {
					t.Fatal(err)
				}
				replacement = filepath.Join(other, "selected.png")
				pictureSelectionWrite(t, replacement, data)
				requested, retarget = pictureSelectionAlias(t, directory, path)
			}
			selection := pictureSelectionOwn(t, requested)
			triggered := false
			at := 2
			if action == "literal-replacement" {
				// A short PNG requires one data read and one EOF read. The
				// decoder's first check follows the completed file read.
				at = 4
			}
			ctx := &pictureSelectionObservedContext{Context: context.Background(), at: at, action: func() {
				triggered = true
				if action == "handle-time" {
					changed := selection.info.ModTime().Add(2 * time.Second)
					if err := os.Chtimes(path, changed, changed); err != nil {
						t.Fatal(err)
					}
				} else {
					// Windows ordinary file handles deny rename of the open
					// target. Replace its alias without changing that handle.
					if err := os.Chtimes(replacement, selection.info.ModTime(), selection.info.ModTime()); err != nil {
						t.Fatal(err)
					}
					retarget(replacement)
				}
			}}
			pictureSelectionRefused(t, selection, ctx)
			if !triggered {
				t.Fatal("planned in-flight mutation did not run")
			}
		})
	}
}

func TestPictureSelectionPreviewLimitsDoNotPreventMetadataSelection(t *testing.T) {
	directory := pictureSelectionDirectory(t)
	pngData := pictureSelectionBytes(t, "png")
	largeDimensions := append([]byte(nil), pngData...)
	binary.BigEndian.PutUint32(largeDimensions[16:20], 4097)
	binary.BigEndian.PutUint32(largeDimensions[20:24], 4096)
	binary.BigEndian.PutUint32(largeDimensions[29:33], crc32.ChecksumIEEE(largeDimensions[12:29]))
	for _, test := range []struct {
		name string
		data []byte
		want string
	}{
		{"unsupported.txt", []byte("not an image"), "header"},
		{"unsupported.gif", pictureSelectionBytes(t, "gif"), "JPEG and PNG"},
		{"truncated.png", pngData[:40], "invalid"},
		{"dimensions.png", largeDimensions, "16-megapixel"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(directory, test.name)
			pictureSelectionWrite(t, path, test.data)
			selection := pictureSelectionOwn(t, path)
			err := pictureSelectionRefused(t, selection, context.Background())
			if !strings.Contains(err.Error(), test.want) {
				t.Fatal("unexpected preview rejection", err)
			}
			if err := selection.checkFile(); err != nil {
				t.Fatal("preview error revoked metadata selection", err)
			}
		})
	}
	path := filepath.Join(directory, "oversize.png")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = errors.Join(file.Truncate(pictureImageMaxBytes+1), file.Close())
	if err != nil {
		t.Fatal(err)
	}
	selection := pictureSelectionOwn(t, path)
	if err := pictureSelectionRefused(t, selection, context.Background()); !strings.Contains(err.Error(), "32 MiB") {
		t.Fatal(err)
	}
	if err := selection.checkFile(); err != nil {
		t.Fatal("oversize file lost metadata selection", err)
	}
}
