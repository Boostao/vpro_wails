package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// A selection owns one physical file, never its containing directory. Construction
// is private preparation, not evidence that a native chooser has run.
type ownedPictureSelection struct {
	requestedPath string
	path          string
	info          os.FileInfo
}

func newOwnedPictureSelection(path string) (*ownedPictureSelection, error) {
	if _, _, err := splitPictureCreationPath(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("selected picture unavailable: %w", err)
	}
	// Windows path-based Stat can load file IDs lazily. Freeze identity while
	// the original file is still open, before any rename or replacement.
	info, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		return nil, fmt.Errorf("selected picture identity capture failed: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("selected picture must be a regular file")
	}
	resolved, err := resolvePictureSelectionPath(path, 0)
	if err != nil {
		return nil, fmt.Errorf("selected picture resolution failed: %w", err)
	}
	selection := &ownedPictureSelection{requestedPath: path, path: resolved, info: info}
	if err := selection.checkFile(); err != nil {
		return nil, err
	}
	return selection, nil
}

func resolvePictureSelectionPath(path string, links int) (string, error) {
	if links > 255 {
		return "", errors.New("selected picture resolution contains too many links")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("selected picture physical resolution requires an absolute path")
	}
	volume := filepath.VolumeName(path)
	current := volume + string(os.PathSeparator)
	components := strings.Split(strings.TrimPrefix(path[len(volume):], string(os.PathSeparator)), string(os.PathSeparator))
	for index, component := range components {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		// Go 1.25 exposes Windows junctions as irregular reparse points, not
		// symlinks; EvalSymlinks alone can leave this physical alias unresolved.
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			target, err := os.Readlink(current)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				if info.Mode()&os.ModeSymlink == 0 {
					return "", errors.New("selected picture junction target is not absolute")
				}
				target = filepath.Join(filepath.Dir(current), target)
			}
			target = filepath.Join(append([]string{target}, components[index+1:]...)...)
			return resolvePictureSelectionPath(target, links+1)
		}
	}
	return filepath.EvalSymlinks(path)
}

func (selection *ownedPictureSelection) checkInfo(info os.FileInfo) error {
	if !info.Mode().IsRegular() || !os.SameFile(selection.info, info) ||
		selection.info.Size() != info.Size() || !selection.info.ModTime().Equal(info.ModTime()) {
		return errors.New("selected picture identity or contents changed; select the file again")
	}
	return nil
}

func (selection *ownedPictureSelection) checkFile() error {
	if selection == nil || selection.info == nil || selection.requestedPath == "" || selection.path == "" {
		return errors.New("selected picture is unavailable; explicit single-file selection is required")
	}
	for _, path := range []string{selection.requestedPath, selection.path} {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("selected picture unavailable: %w", err)
		}
		info, statErr := file.Stat()
		var identityErr error
		if statErr == nil {
			identityErr = selection.checkInfo(info)
		}
		if err := errors.Join(statErr, identityErr, file.Close()); err != nil {
			return fmt.Errorf("selected picture verification failed: %w", err)
		}
	}
	return nil
}

func (selection *ownedPictureSelection) readImage(ctx context.Context) (result pictureImage, resultErr error) {
	if ctx == nil {
		return result, errors.New("selected picture preview requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := selection.checkFile(); err != nil {
		return result, err
	}
	file, err := os.Open(selection.path)
	if err != nil {
		return result, fmt.Errorf("selected picture preview unavailable: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, file.Close())
		if resultErr != nil {
			result = pictureImage{}
		}
	}()
	before, err := file.Stat()
	if err != nil {
		return result, err
	}
	if err := selection.checkInfo(before); err != nil {
		return result, err
	}
	if before.Size() > pictureImageMaxBytes {
		return result, errors.New("selected picture preview exceeds 32 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(artifactPublicationReader{ctx, file}, pictureImageMaxBytes+1))
	if err != nil {
		return result, err
	}
	after, err := file.Stat()
	if err != nil {
		return result, err
	}
	if err := selection.checkInfo(after); err != nil {
		return result, err
	}
	if int64(len(data)) != before.Size() {
		return result, errors.New("selected picture size changed during preview read")
	}
	preview, err := decodePictureImageBytes(ctx, data)
	if err != nil {
		return result, err
	}
	if err := selection.checkFile(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return preview, nil
}
