package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// A private preparation kernel, not a project lease or desktop export API.
type tableCSVPublication struct {
	Path      string
	SHA256    string
	Published bool
}

// Per-call fault seams keep tests independent; production uses the nil defaults.
type tableCSVPublicationHooks struct {
	observe func(phase, temporary string) error
	write   func(*os.File, []byte) (int, error)
	sync    func(*os.File) error
	close   func(*os.File) error
	link    func(string, string) error
	remove  func(string) error
}

func publishTableCSVBundle(ctx context.Context, requested string, document tableCSVDocument) (tableCSVPublication, error) {
	return publishTableCSVBundleWithHooks(ctx, requested, document, tableCSVPublicationHooks{})
}

func publishTableCSVBundleWithHooks(ctx context.Context, requested string, document tableCSVDocument, hooks tableCSVPublicationHooks) (result tableCSVPublication, resultErr error) {
	return publishTableCSVBundleChecked(ctx, requested, document, nil, hooks)
}

// Source authority is independent of the publisher's test-only fault seams.
// Validation precedes the final artifact/parent rechecks and irreversible link.
func publishTableCSVBundleChecked(ctx context.Context, requested string, document tableCSVDocument, validateSource func() error, hooks tableCSVPublicationHooks) (result tableCSVPublication, resultErr error) {
	document = snapshotTableCSVBundleDocument(document)
	observe := func(phase, temporary string) error {
		if hooks.observe != nil {
			if err := hooks.observe(phase, temporary); err != nil {
				return fmt.Errorf("table CSV publication %s: %w", phase, err)
			}
		}
		return nil
	}
	if err := observe("snapshot", ""); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("table CSV publication canceled: %w", err)
	}
	path, parent, err := tableCSVPublicationDestination(requested)
	if err != nil {
		return result, err
	}
	encoded, err := encodeTableCSVBundle(ctx, document)
	if err != nil {
		return result, fmt.Errorf("table CSV publication encode: %w", err)
	}
	if err := tableCSVPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("table CSV publication prestage parent: %w", err)
	}
	digest := sha256.Sum256(encoded)
	result.Path, result.SHA256 = path, hex.EncodeToString(digest[:])
	file, err := os.CreateTemp(filepath.Dir(path), ".vpro-table-csv-*")
	if err != nil {
		return result, fmt.Errorf("table CSV publication create stage: %w", err)
	}
	temporary := file.Name()
	owned, statErr := file.Stat()
	closed := false
	defer func() {
		if !closed {
			resultErr = errors.Join(resultErr, tableCSVPublicationClose(file, hooks.close))
		}
		// Never follow or remove a replacement, even during failed publication.
		if err := tableCSVPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("table CSV publication cleanup parent: %w", err))
		} else if owned == nil {
			resultErr = errors.Join(resultErr, errors.New("table CSV publication cleanup lacks stage identity; stage retained"))
		} else if err := observe("cleanup", temporary); err != nil {
			resultErr = errors.Join(resultErr, err)
		} else if err := tableCSVPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("table CSV publication cleanup parent changed: %w", err))
		} else if err := tableCSVPublicationIdentity(temporary, owned, false); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("table CSV publication cleanup stage retained: %w", err))
		} else {
			remove := hooks.remove
			if remove == nil {
				remove = os.Remove
			}
			if err := remove(temporary); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("table CSV publication remove owned stage: %w", err))
			}
		}
		if result.Published && resultErr != nil {
			resultErr = fmt.Errorf("table CSV bundle published; do not replay publication: %w", resultErr)
		}
	}()
	if statErr != nil {
		return result, fmt.Errorf("table CSV publication stage identity: %w", statErr)
	}
	if err := file.Chmod(0600); err != nil {
		return result, fmt.Errorf("table CSV publication stage permissions: %w", err)
	}
	write := hooks.write
	if write == nil {
		write = func(file *os.File, data []byte) (int, error) { return file.Write(data) }
	}
	for offset := 0; offset < len(encoded); {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("table CSV publication staging: %w", err)
		}
		end := min(offset+32*1024, len(encoded))
		n, err := write(file, encoded[offset:end])
		if err != nil {
			return result, fmt.Errorf("table CSV publication stage write: %w", err)
		}
		if n != end-offset {
			return result, fmt.Errorf("table CSV publication stage write: %w", io.ErrShortWrite)
		}
		offset = end
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("table CSV publication staging canceled: %w", err)
	}
	sync := hooks.sync
	if sync == nil {
		sync = (*os.File).Sync
	}
	if err := sync(file); err != nil {
		return result, fmt.Errorf("table CSV publication stage sync: %w", err)
	}
	err = tableCSVPublicationClose(file, hooks.close)
	closed = true
	if err != nil {
		return result, err
	}
	if err := observe("staged", temporary); err != nil {
		return result, err
	}
	if err := observe("prelink", temporary); err != nil {
		return result, err
	}
	if err := tableCSVPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("table CSV publication parent: %w", err)
	}
	if err := tableCSVPublicationIdentity(temporary, owned, false); err != nil {
		return result, fmt.Errorf("table CSV publication stage: %w", err)
	}
	verification, err := os.Open(temporary)
	if err != nil {
		return result, fmt.Errorf("table CSV publication verify open: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, tableCSVPublicationClose(verification, hooks.close))
	}()
	current, statErr := verification.Stat()
	data, readErr := io.ReadAll(io.LimitReader(tableCSVBundleReader{ctx, verification}, int64(len(encoded))+1))
	if statErr != nil || current == nil || !os.SameFile(owned, current) {
		return result, errors.Join(statErr, readErr, errors.New("table CSV publication verification file identity changed"))
	}
	if readErr != nil {
		return result, fmt.Errorf("table CSV publication verify read: %w", readErr)
	}
	if !bytes.Equal(data, encoded) {
		return result, errors.New("table CSV publication staged artifact differs from encoded bundle")
	}
	if _, err := decodeTableCSVBundle(ctx, data, int64(len(data))); err != nil {
		return result, fmt.Errorf("table CSV publication staged bundle: %w", err)
	}
	if validateSource != nil {
		if err := validateSource(); err != nil {
			return result, fmt.Errorf("table CSV publication source validation: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("table CSV publication prelink canceled: %w", err)
	}
	// Reobserve after the final context callback, not just before it. Keep the
	// verification handle open through commit; no callback intervenes here.
	if _, err := verification.Seek(0, io.SeekStart); err != nil {
		return result, fmt.Errorf("table CSV publication final verify seek: %w", err)
	}
	data, err = io.ReadAll(io.LimitReader(verification, int64(len(encoded))+1))
	if err != nil {
		return result, fmt.Errorf("table CSV publication final verify read: %w", err)
	}
	if !bytes.Equal(data, encoded) {
		return result, errors.New("table CSV publication final staged artifact changed")
	}
	if err := tableCSVPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("table CSV publication prelink parent: %w", err)
	}
	if err := tableCSVPublicationIdentity(temporary, owned, false); err != nil {
		return result, fmt.Errorf("table CSV publication prelink stage: %w", err)
	}
	link := hooks.link
	if link == nil {
		link = os.Link
	}
	if err := link(temporary, path); err != nil {
		return result, fmt.Errorf("table CSV publication requires atomic no-replace linking: %w", err)
	}
	result.Published = true // The irreversible commit boundary precedes every fallible callback.
	if err := observe("published", temporary); err != nil {
		return result, err
	}
	if err := tableCSVPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("table CSV publication committed parent: %w", err)
	}
	if err := tableCSVPublicationIdentity(path, owned, false); err != nil {
		return result, fmt.Errorf("table CSV publication committed destination: %w", err)
	}
	return result, nil
}

func tableCSVPublicationClose(file *os.File, close func(*os.File) error) error {
	if close == nil {
		close = (*os.File).Close
	}
	if err := close(file); err != nil {
		return fmt.Errorf("table CSV publication stage close: %w", err)
	}
	return nil
}

func tableCSVPublicationIdentity(path string, owned os.FileInfo, directory bool) error {
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if owned == nil || current.Mode()&os.ModeSymlink != 0 || current.IsDir() != directory ||
		(!directory && !current.Mode().IsRegular()) || !os.SameFile(owned, current) {
		return errors.New("physical identity changed")
	}
	return nil
}

func tableCSVPublicationDestination(requested string) (string, os.FileInfo, error) {
	windowsPath := strings.ReplaceAll(requested, "/", `\`)
	if !utf8.ValidString(requested) || strings.ContainsRune(requested, 0) || !filepath.IsAbs(requested) ||
		strings.HasPrefix(windowsPath, `\\?\`) || strings.HasPrefix(windowsPath, `\\.\`) {
		return "", nil, errors.New("table CSV publication requires an explicit literal absolute destination")
	}
	// Unlike freshSQLiteDestination, an archive has no .db extension contract.
	// Reject Win32 aliases rather than silently normalizing their meaning.
	tail := strings.TrimPrefix(requested, filepath.VolumeName(requested))
	for _, component := range strings.FieldsFunc(tail, func(r rune) bool { return r == '\\' || r == '/' }) {
		if component == "." || component == ".." || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") ||
			strings.ContainsAny(component, `<>:"|?*`) || strings.ContainsFunc(component, func(r rune) bool { return r < 32 }) {
			return "", nil, errors.New("table CSV publication destination contains an ambiguous or invalid component")
		}
		stem := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		stem = strings.TrimRight(stem, " ")
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || stem == "CONIN$" || stem == "CONOUT$" ||
			(len([]rune(stem)) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) &&
				strings.ContainsRune("123456789¹²³", []rune(stem)[3])) {
			return "", nil, errors.New("table CSV publication destination contains a reserved Windows name")
		}
	}
	if strings.HasSuffix(requested, `\`) || strings.HasSuffix(requested, "/") {
		return "", nil, errors.New("table CSV publication destination must name a file")
	}
	parentPath, err := filepath.EvalSymlinks(filepath.Dir(requested))
	if err != nil {
		return "", nil, fmt.Errorf("table CSV publication resolve parent: %w", err)
	}
	directory, err := os.Open(parentPath)
	if err != nil {
		return "", nil, fmt.Errorf("table CSV publication open parent: %w", err)
	}
	// Windows path Stat may defer file-ID loading until SameFile. Handle Stat
	// freezes identity now, before encoding can invoke fallible callbacks.
	parent, statErr := directory.Stat()
	closeErr := directory.Close()
	if statErr != nil || closeErr != nil {
		return "", nil, fmt.Errorf("table CSV publication parent stat/close: %w", errors.Join(statErr, closeErr))
	}
	if !parent.IsDir() {
		return "", nil, errors.New("table CSV publication parent must be an existing physical directory")
	}
	path := filepath.Join(parentPath, filepath.Base(requested))
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return "", nil, errors.Join(err, errors.New("table CSV publication destination exists or cannot be checked; never replace"))
	}
	return path, parent, nil
}
