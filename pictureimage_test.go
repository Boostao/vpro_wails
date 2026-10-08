package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func pictureImageFixture(t *testing.T, format string, width, height int) (*ownedPictureDirectory, []byte) {
	t.Helper()
	directory := t.TempDir()
	pixels := image.NewRGBA(image.Rect(0, 0, width, height))
	pixels.Set(0, 0, color.RGBA{R: 128, G: 64, B: 32, A: 255})
	var encoded bytes.Buffer
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&encoded, pixels, nil)
	} else {
		err = png.Encode(&encoded, pixels)
	}
	if err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	if err := os.WriteFile(filepath.Join(directory, "Pic01.jpg"), data, 0600); err != nil {
		t.Fatal(err)
	}
	owner, err := newOwnedPictureDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	return owner, data
}

func TestPictureImageOwnedRowAndIndependentDefaultDirectories(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	source, path := pictureLibraryFixture(t, pictureFixtureSchema)
	child, childBytes := pictureImageFixture(t, "jpeg", 448, 300)
	manager, managerBytes := pictureImageFixture(t, "png", 4, 3)
	directories := pictureImageDirectories{child: child, manager: manager}
	review, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", source)
	if err != nil {
		t.Fatal(err)
	}
	original := review.Records.Rows[3]
	files := databaseBytes(t, service.projects.sqlite.attachments)
	sourceBytes := databaseBytes(t, map[string]string{"pictures": path})["pictures"]
	for _, test := range []struct {
		view          string
		bytes         []byte
		mime          string
		width, height int
	}{
		{pictureChildView, childBytes, "image/jpeg", 448, 300},
		{pictureManagerView, managerBytes, "image/png", 4, 3},
	} {
		got, err := service.readPictureImage(context.Background(), state.ContextID, "108050", source, directories, test.view, original)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(test.bytes)
		if got.ContextID != state.ContextID || got.Project != "Sample" || got.PlotNumber != "108050" ||
			got.RowID != original.RowID || got.MIME != test.mime || got.Width != test.width || got.Height != test.height ||
			got.SHA256 != hex.EncodeToString(hash[:]) ||
			got.DataURL != "data:"+test.mime+";base64,"+base64.StdEncoding.EncodeToString(test.bytes) {
			t.Fatal("preview changed scope, directory semantics, bytes or measured dimensions", got)
		}
	}
	assertProfileSUFiles(t, service, files)
	if !reflect.DeepEqual(databaseBytes(t, map[string]string{"pictures": path})["pictures"], sourceBytes) {
		t.Fatal("preview changed picture metadata")
	}
	for _, test := range []struct {
		directory *ownedPictureDirectory
		bytes     []byte
	}{{child, childBytes}, {manager, managerBytes}} {
		data, err := os.ReadFile(filepath.Join(test.directory.path, "Pic01.jpg"))
		if err != nil || !bytes.Equal(data, test.bytes) {
			t.Fatal("preview rewrote an original image", err)
		}
	}
	for _, test := range []struct {
		contextID, plot, view string
		original              ProjectMetadataRow
		directories           pictureImageDirectories
	}{
		{"stale", "108050", pictureChildView, original, directories},
		{state.ContextID, "missing", pictureChildView, original, directories},
		{state.ContextID, "108050", "unknown", original, directories},
		{state.ContextID, "108050", pictureChildView, ProjectMetadataRow{RowID: "changed", Cells: original.Cells}, directories},
		{state.ContextID, "108050", pictureChildView, review.Records.Rows[0], directories},
		{state.ContextID, "108050", pictureChildView, review.Records.Rows[1], directories},
		{state.ContextID, "108050", pictureChildView, original, pictureImageDirectories{manager: manager}},
		{state.ContextID, "108050", pictureManagerView, original, pictureImageDirectories{child: child}},
	} {
		got, err := service.readPictureImage(context.Background(), test.contextID, test.plot, source, test.directories, test.view, test.original)
		if err == nil || !reflect.DeepEqual(got, pictureImage{}) {
			t.Fatal("unowned/stale metadata or missing independent directory policy returned an image", got, err)
		}
	}
	changed := ProjectMetadataRow{RowID: original.RowID, Cells: append([]ProjectMetadataCell{}, original.Cells...)}
	text := "not the reviewed NULL"
	changed.Cells[4] = ProjectMetadataCell{Storage: "text", Text: &text}
	if got, err := service.readPictureImage(context.Background(), state.ContextID, "108050", source, directories, pictureChildView, changed); err == nil || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("altered reviewed metadata was accepted", got, err)
	}
}

func TestPictureImageRejectsInvalidNamesDataAndLimits(t *testing.T) {
	directory, valid := pictureImageFixture(t, "png", 2, 2)
	for _, name := range []string{"", ".", "..", "..\\Pic01.jpg", "../Pic01.jpg", "sub\\Pic01.jpg", "C:\\Pic01.jpg",
		"file.jpg:stream", "file?.jpg", "file*.jpg", "file\x00.jpg", "file\n.jpg", "file.jpg.", "file.jpg ",
		"CON", "con.jpg", "NUL.jpg", "COM1.jpg", "lpt9.png", string([]byte{0xff})} {
		if got, err := readAuthorizedPictureImage(context.Background(), directory, name); err == nil || !reflect.DeepEqual(got, pictureImage{}) {
			t.Fatal("invalid filename returned an image", name, got, err)
		}
	}
	for name, data := range map[string][]byte{
		"empty.jpg": {},
		"svg.jpg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		"bad.jpg":   []byte("not a photograph"),
		"cut.jpg":   valid[:len(valid)/2],
	} {
		if err := os.WriteFile(filepath.Join(directory.path, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := readAuthorizedPictureImage(context.Background(), directory, name); err == nil || !reflect.DeepEqual(got, pictureImage{}) {
			t.Fatal("invalid/truncated/unsupported image returned a preview", name, got, err)
		}
	}
	oversized := append([]byte{}, valid...)
	binary.BigEndian.PutUint32(oversized[16:20], 100000)
	binary.BigEndian.PutUint32(oversized[20:24], 100000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	if err := os.WriteFile(filepath.Join(directory.path, "dimensions.png"), oversized, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := readAuthorizedPictureImage(context.Background(), directory, "dimensions.png"); err == nil || !strings.Contains(err.Error(), "16-megapixel") || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("oversized dimensions were decoded or threshold was not enforced", got, err)
	}
	file, err := os.Create(filepath.Join(directory.path, "oversized.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	err = file.Truncate(pictureImageMaxBytes + 1)
	if err := errors.Join(err, file.Close()); err != nil {
		t.Fatal(err)
	}
	if got, err := readAuthorizedPictureImage(context.Background(), directory, "oversized.jpg"); err == nil || !strings.Contains(err.Error(), "32 MiB") || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("byte threshold was not enforced before reading", got, err)
	}
	if err := os.Mkdir(filepath.Join(directory.path, "directory.jpg"), 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := readAuthorizedPictureImage(context.Background(), directory, "directory.jpg"); err == nil || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("non-regular file was read", got, err)
	}
}

func TestPictureImageDirectoryIdentityAndCancellation(t *testing.T) {
	directory, _ := pictureImageFixture(t, "jpeg", 2, 2)
	if _, err := newOwnedPictureDirectory("relative"); err == nil {
		t.Fatal("relative directory was authorized")
	}
	if _, err := newOwnedPictureDirectory(filepath.Join(directory.path, "Pic01.jpg")); err == nil {
		t.Fatal("file was authorized as a directory")
	}
	if got, err := readAuthorizedPictureImage(context.Background(), nil, "Pic01.jpg"); err == nil || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("missing directory permission returned a preview", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := readAuthorizedPictureImage(ctx, directory, "Pic01.jpg"); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("cancelled image read returned a preview", got, err)
	}
	before, err := readAuthorizedPictureImage(context.Background(), directory, "Pic01.jpg")
	if err != nil {
		t.Fatal("cancelled read prevented retry", err)
	}
	if before.Width != 2 || before.Height != 2 {
		t.Fatal("retry lost the actual image", before)
	}
	if err := os.Rename(directory.path, directory.path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory.path, 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := readAuthorizedPictureImage(context.Background(), directory, "Pic01.jpg"); err == nil || !strings.Contains(err.Error(), "identity changed") || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("replacement directory retained old authorization", got, err)
	}
}

func TestPictureImageRootRefusesExternalSymlink(t *testing.T) {
	directory, _ := pictureImageFixture(t, "jpeg", 2, 2)
	external, _ := pictureImageFixture(t, "jpeg", 3, 3)
	if err := os.Symlink(filepath.Join(external.path, "Pic01.jpg"), filepath.Join(directory.path, "external.jpg")); err != nil {
		t.Skipf("host does not permit creating the disposable test symlink: %v", err)
	}
	if got, err := readAuthorizedPictureImage(context.Background(), directory, "external.jpg"); err == nil || !reflect.DeepEqual(got, pictureImage{}) {
		t.Fatal("directory authorization escaped through a symlink", got, err)
	}
}
