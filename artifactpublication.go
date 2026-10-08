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
type artifactPublication struct {
	Path      string
	SHA256    string
	Published bool
}

// Per-call fault seams keep tests independent; production uses the nil defaults.
type artifactPublicationHooks struct {
	observe func(phase, temporary string) error
	write   func(*os.File, []byte) (int, error)
	sync    func(*os.File) error
	close   func(*os.File) error
	link    func(string, string) error
	remove  func(string) error
}

type artifactPublicationFormat struct {
	label        string
	artifact     string
	commitLabel  string
	stagePattern string
	encode       func(context.Context) ([]byte, error)
	validate     func(context.Context, []byte) error
}

// Source authority is independent of the publisher's test-only fault seams.
// Validation precedes the final artifact/parent rechecks and irreversible link.
func publishArtifactChecked(ctx context.Context, requested string, format artifactPublicationFormat, validateSource func() error, hooks artifactPublicationHooks) (result artifactPublication, resultErr error) {
	if ctx == nil {
		return result, errors.New("artifact publication requires a context")
	}
	if format.label == "" || format.artifact == "" || format.commitLabel == "" ||
		format.stagePattern == "" || format.encode == nil || format.validate == nil {
		return result, errors.New("artifact publication requires an explicit format contract")
	}
	observe := func(phase, temporary string) error {
		if hooks.observe != nil {
			if err := hooks.observe(phase, temporary); err != nil {
				return fmt.Errorf("%s %s: %w", format.label, phase, err)
			}
		}
		return nil
	}
	if err := observe("snapshot", ""); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("%s canceled: %w", format.label, err)
	}
	path, parent, err := artifactPublicationDestination(requested, format.label)
	if err != nil {
		return result, err
	}
	encoded, err := format.encode(ctx)
	if err != nil {
		return result, fmt.Errorf("%s encode: %w", format.label, err)
	}
	encoded = bytes.Clone(encoded)
	if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("%s prestage parent: %w", format.label, err)
	}
	digest := sha256.Sum256(encoded)
	result.Path, result.SHA256 = path, hex.EncodeToString(digest[:])
	file, err := os.CreateTemp(filepath.Dir(path), format.stagePattern)
	if err != nil {
		return result, fmt.Errorf("%s create stage: %w", format.label, err)
	}
	temporary := file.Name()
	owned, statErr := file.Stat()
	closed := false
	defer func() {
		if !closed {
			resultErr = errors.Join(resultErr, artifactPublicationClose(file, hooks.close, format.label))
		}
		// Never follow or remove a replacement, even during failed publication.
		if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup parent: %w", format.label, err))
		} else if owned == nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup lacks stage identity; stage retained", format.label))
		} else if err := observe("cleanup", temporary); err != nil {
			resultErr = errors.Join(resultErr, err)
		} else if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup parent changed: %w", format.label, err))
		} else if err := artifactPublicationIdentity(temporary, owned, false); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup stage retained: %w", format.label, err))
		} else {
			remove := hooks.remove
			if remove == nil {
				remove = os.Remove
			}
			if err := remove(temporary); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("%s remove owned stage: %w", format.label, err))
			}
		}
		if result.Published && resultErr != nil {
			resultErr = fmt.Errorf("%s published; do not replay publication: %w", format.commitLabel, resultErr)
		}
	}()
	if statErr != nil {
		return result, fmt.Errorf("%s stage identity: %w", format.label, statErr)
	}
	if err := file.Chmod(0600); err != nil {
		return result, fmt.Errorf("%s stage permissions: %w", format.label, err)
	}
	write := hooks.write
	if write == nil {
		write = func(file *os.File, data []byte) (int, error) { return file.Write(data) }
	}
	for offset := 0; offset < len(encoded); {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("%s staging: %w", format.label, err)
		}
		end := min(offset+32*1024, len(encoded))
		n, err := write(file, encoded[offset:end])
		if err != nil {
			return result, fmt.Errorf("%s stage write: %w", format.label, err)
		}
		if n != end-offset {
			return result, fmt.Errorf("%s stage write: %w", format.label, io.ErrShortWrite)
		}
		offset = end
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("%s staging canceled: %w", format.label, err)
	}
	sync := hooks.sync
	if sync == nil {
		sync = (*os.File).Sync
	}
	if err := sync(file); err != nil {
		return result, fmt.Errorf("%s stage sync: %w", format.label, err)
	}
	err = artifactPublicationClose(file, hooks.close, format.label)
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
	if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("%s parent: %w", format.label, err)
	}
	if err := artifactPublicationIdentity(temporary, owned, false); err != nil {
		return result, fmt.Errorf("%s stage: %w", format.label, err)
	}
	verification, err := os.Open(temporary)
	if err != nil {
		return result, fmt.Errorf("%s verify open: %w", format.label, err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, artifactPublicationClose(verification, hooks.close, format.label))
	}()
	current, statErr := verification.Stat()
	data, readErr := io.ReadAll(io.LimitReader(artifactPublicationReader{ctx, verification}, int64(len(encoded))+1))
	if statErr != nil || current == nil || !os.SameFile(owned, current) {
		return result, errors.Join(statErr, readErr, fmt.Errorf("%s verification file identity changed", format.label))
	}
	if readErr != nil {
		return result, fmt.Errorf("%s verify read: %w", format.label, readErr)
	}
	if !bytes.Equal(data, encoded) {
		return result, fmt.Errorf("%s staged artifact differs from encoded %s", format.label, format.artifact)
	}
	if err := format.validate(ctx, data); err != nil {
		return result, fmt.Errorf("%s staged %s: %w", format.label, format.artifact, err)
	}
	if validateSource != nil {
		if err := validateSource(); err != nil {
			return result, fmt.Errorf("%s source validation: %w", format.label, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("%s prelink canceled: %w", format.label, err)
	}
	// Reobserve after the final context callback, not just before it. Keep the
	// verification handle open through commit; no callback intervenes here.
	if _, err := verification.Seek(0, io.SeekStart); err != nil {
		return result, fmt.Errorf("%s final verify seek: %w", format.label, err)
	}
	data, err = io.ReadAll(io.LimitReader(verification, int64(len(encoded))+1))
	if err != nil {
		return result, fmt.Errorf("%s final verify read: %w", format.label, err)
	}
	if !bytes.Equal(data, encoded) {
		return result, fmt.Errorf("%s final staged artifact changed", format.label)
	}
	if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("%s prelink parent: %w", format.label, err)
	}
	if err := artifactPublicationIdentity(temporary, owned, false); err != nil {
		return result, fmt.Errorf("%s prelink stage: %w", format.label, err)
	}
	link := hooks.link
	if link == nil {
		link = os.Link
	}
	if err := link(temporary, path); err != nil {
		return result, fmt.Errorf("%s requires atomic no-replace linking: %w", format.label, err)
	}
	result.Published = true // The irreversible commit boundary precedes every fallible callback.
	if err := observe("published", temporary); err != nil {
		return result, err
	}
	if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("%s committed parent: %w", format.label, err)
	}
	if err := artifactPublicationIdentity(path, owned, false); err != nil {
		return result, fmt.Errorf("%s committed destination: %w", format.label, err)
	}
	return result, nil
}

func artifactPublicationClose(file *os.File, close func(*os.File) error, label string) error {
	if close == nil {
		close = (*os.File).Close
	}
	if err := close(file); err != nil {
		return fmt.Errorf("%s stage close: %w", label, err)
	}
	return nil
}

func artifactPublicationIdentity(path string, owned os.FileInfo, directory bool) error {
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

func artifactPublicationDestination(requested, label string) (string, os.FileInfo, error) {
	windowsPath := strings.ReplaceAll(requested, "/", `\`)
	if !utf8.ValidString(requested) || strings.ContainsRune(requested, 0) || !filepath.IsAbs(requested) ||
		strings.HasPrefix(windowsPath, `\\?\`) || strings.HasPrefix(windowsPath, `\\.\`) {
		return "", nil, fmt.Errorf("%s requires an explicit literal absolute destination", label)
	}
	// Unlike freshSQLiteDestination, an archive has no .db extension contract.
	// Reject Win32 aliases rather than silently normalizing their meaning.
	tail := strings.TrimPrefix(requested, filepath.VolumeName(requested))
	for _, component := range strings.FieldsFunc(tail, func(r rune) bool { return r == '\\' || r == '/' }) {
		if component == "." || component == ".." || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") ||
			strings.ContainsAny(component, `<>:"|?*`) || strings.ContainsFunc(component, func(r rune) bool { return r < 32 }) {
			return "", nil, fmt.Errorf("%s destination contains an ambiguous or invalid component", label)
		}
		stem := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		stem = strings.TrimRight(stem, " ")
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || stem == "CONIN$" || stem == "CONOUT$" ||
			(len([]rune(stem)) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) &&
				strings.ContainsRune("123456789¹²³", []rune(stem)[3])) {
			return "", nil, fmt.Errorf("%s destination contains a reserved Windows name", label)
		}
	}
	if strings.HasSuffix(requested, `\`) || strings.HasSuffix(requested, "/") {
		return "", nil, fmt.Errorf("%s destination must name a file", label)
	}
	parentPath, err := filepath.EvalSymlinks(filepath.Dir(requested))
	if err != nil {
		return "", nil, fmt.Errorf("%s resolve parent: %w", label, err)
	}
	directory, err := os.Open(parentPath)
	if err != nil {
		return "", nil, fmt.Errorf("%s open parent: %w", label, err)
	}
	// Windows path Stat may defer file-ID loading until SameFile. Handle Stat
	// freezes identity now, before encoding can invoke fallible callbacks.
	parent, statErr := directory.Stat()
	closeErr := directory.Close()
	if statErr != nil || closeErr != nil {
		return "", nil, fmt.Errorf("%s parent stat/close: %w", label, errors.Join(statErr, closeErr))
	}
	if !parent.IsDir() {
		return "", nil, fmt.Errorf("%s parent must be an existing physical directory", label)
	}
	path := filepath.Join(parentPath, filepath.Base(requested))
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return "", nil, errors.Join(err, fmt.Errorf("%s destination exists or cannot be checked; never replace", label))
	}
	return path, parent, nil
}

type artifactPublicationReader struct {
	ctx    context.Context
	source io.Reader
}

func (reader artifactPublicationReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	return reader.source.Read(data)
}
