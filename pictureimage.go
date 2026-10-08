package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"
)

const (
	pictureImageMaxBytes  = 32 * 1024 * 1024
	pictureImageMaxPixels = 16 * 1024 * 1024
	pictureChildView      = "child"
	pictureManagerView    = "manager"
)

type ownedPictureDirectory struct {
	requestedPath string
	path          string
	info          os.FileInfo
}

type pictureImageDirectories struct {
	child   *ownedPictureDirectory
	manager *ownedPictureDirectory
}

type pictureImage struct {
	ContextID  string `json:"contextId"`
	Project    string `json:"project"`
	PlotNumber string `json:"plotNumber"`
	RowID      string `json:"rowId"`
	MIME       string `json:"mime"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SHA256     string `json:"sha256"`
	DataURL    string `json:"dataUrl"`
}

func newOwnedPictureDirectory(path string) (*ownedPictureDirectory, error) {
	if !utf8.ValidString(path) || strings.ContainsRune(path, 0) {
		return nil, errors.New("picture directory path contains malformed Unicode or NUL")
	}
	if path == "" || !filepath.IsAbs(path) {
		return nil, errors.New("picture directory requires an explicit absolute existing directory")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("picture directory unavailable: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("picture directory is not a directory")
	}
	return &ownedPictureDirectory{requestedPath: filepath.Clean(path), path: resolved, info: info}, nil
}

func (directory *ownedPictureDirectory) checkFile() error {
	if directory == nil || directory.info == nil {
		return errors.New("picture directory is unavailable; an explicit directory authorization is required")
	}
	for _, path := range []string{directory.requestedPath, directory.path} {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("picture directory unavailable: %w", err)
		}
		if !info.IsDir() || !os.SameFile(directory.info, info) {
			return errors.New("picture directory identity changed; reload its authorization")
		}
	}
	return nil
}

func (s *ContextService) readPictureImage(ctx context.Context, contextID, plot string, source *ownedPictureLibrary,
	directories pictureImageDirectories, view string, original ProjectMetadataRow) (pictureImage, error) {
	return withOwnedPictureRead(ctx, s, contextID, plot, source, func(project string, records ProjectMetadataTable) (pictureImage, error) {
		var row *ProjectMetadataRow
		for index := range records.Rows {
			if records.Rows[index].RowID == original.RowID {
				row = &records.Rows[index]
			}
		}
		if row == nil || !reflect.DeepEqual(*row, original) {
			return pictureImage{}, errors.New("picture metadata changed or is outside the owned plot; reload before previewing")
		}
		var directory *ownedPictureDirectory
		switch view {
		case pictureChildView:
			directory = directories.child
		case pictureManagerView:
			directory = directories.manager
		default:
			return pictureImage{}, errors.New("picture preview requires an explicit child or manager directory policy")
		}
		if row.Cells[1].Storage != "text" || row.Cells[1].Text == nil || *row.Cells[1].Text != "Default" {
			return pictureImage{}, errors.New("literal external, empty or NULL picture directory is not authorized; no path was repaired")
		}
		if row.Cells[2].Storage != "text" || row.Cells[2].Text == nil {
			return pictureImage{}, errors.New("picture name is not literal text; no filename was inferred")
		}
		result, err := readAuthorizedPictureImage(ctx, directory, *row.Cells[2].Text)
		if err != nil {
			return pictureImage{}, err
		}
		result.ContextID, result.Project, result.PlotNumber, result.RowID = contextID, project, plot, row.RowID
		return result, nil
	})
}

func validatePictureFilename(name string) error {
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) ||
		strings.ContainsAny(name, `<>:"/\|?*`) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return errors.New("picture name must be one literal filename without traversal, streams or Windows path repair")
	}
	for _, char := range name {
		if char < 32 {
			return errors.New("picture name contains a control character")
		}
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
		len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return errors.New("picture name denotes a reserved Windows device")
	}
	return nil
}

func readAuthorizedPictureImage(ctx context.Context, directory *ownedPictureDirectory, name string) (result pictureImage, resultErr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := validatePictureFilename(name); err != nil {
		return result, err
	}
	if err := directory.checkFile(); err != nil {
		return result, err
	}
	root, err := os.OpenRoot(directory.path)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, root.Close())
		if resultErr != nil {
			result = pictureImage{}
		}
	}()
	rootInfo, err := root.Stat(".")
	if err != nil {
		return result, err
	}
	if !os.SameFile(directory.info, rootInfo) {
		return result, errors.New("picture directory changed during root acquisition")
	}
	file, err := root.Open(name)
	if err != nil {
		return result, fmt.Errorf("picture image unavailable within authorized directory: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, file.Close())
	}()
	before, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !before.Mode().IsRegular() || before.Size() > pictureImageMaxBytes {
		return result, errors.New("picture image must be a regular file no larger than 32 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(artifactPublicationReader{ctx, file}, pictureImageMaxBytes+1))
	if err != nil {
		return result, err
	}
	if len(data) > pictureImageMaxBytes {
		return result, errors.New("picture image exceeds 32 MiB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return result, fmt.Errorf("picture image header is invalid or unsupported: %w", err)
	}
	if format != "jpeg" && format != "png" {
		return result, errors.New("picture preview supports verified JPEG and PNG images only")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > pictureImageMaxPixels/config.Height {
		return result, errors.New("picture image dimensions exceed the 16-megapixel preview limit")
	}
	decoded, decodedFormat, err := image.Decode(artifactPublicationReader{ctx, bytes.NewReader(data)})
	if err != nil {
		return result, fmt.Errorf("picture image data is invalid: %w", err)
	}
	if decodedFormat != format || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return result, errors.New("picture image header and decoded dimensions disagree")
	}
	after, err := root.Stat(name)
	if err != nil {
		return result, err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return result, errors.New("picture image changed during read; retry the preview")
	}
	if err := directory.checkFile(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	hash := sha256.Sum256(data)
	return pictureImage{
		MIME: "image/" + format, Width: config.Width, Height: config.Height,
		SHA256: hex.EncodeToString(hash[:]), DataURL: "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(data),
	}, nil
}
