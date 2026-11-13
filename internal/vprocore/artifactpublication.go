package vprocore

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

	"github.com/boostao/vpro-wails/internal/sqlutil"
)

// A private preparation kernel, not a project lease or desktop export API.
type ArtifactPublication struct {
	Path      string
	SHA256    string
	Published bool
}

// Per-call fault seams keep tests independent; production uses the nil defaults.
type ArtifactPublicationHooks struct {
	Observe func(phase, temporary string) error `json:"-" yaml:"-"`
	Write   func(*os.File, []byte) (int, error) `json:"-" yaml:"-"`
	Sync    func(*os.File) error                `json:"-" yaml:"-"`
	Close   func(*os.File) error                `json:"-" yaml:"-"`
	Link    func(string, string) error          `json:"-" yaml:"-"`
	Remove  func(string) error                  `json:"-" yaml:"-"`
}

type ArtifactPublicationFormat struct {
	Label        string                                `json:"-" yaml:"-"`
	Artifact     string                                `json:"-" yaml:"-"`
	CommitLabel  string                                `json:"-" yaml:"-"`
	StagePattern string                                `json:"-" yaml:"-"`
	Encode       func(context.Context) ([]byte, error) `json:"-" yaml:"-"`
	Validate     func(context.Context, []byte) error   `json:"-" yaml:"-"`
}

// Source authority is independent of the publisher's test-only fault seams.
// Validation precedes the final artifact/parent rechecks and irreversible link.
func PublishArtifactChecked(ctx context.Context, requested string, format ArtifactPublicationFormat, validateSource func() error, hooks ArtifactPublicationHooks) (result ArtifactPublication, resultErr error) {
	if ctx == nil {
		return result, errors.New("artifact publication requires a context")
	}
	if format.Label == "" || format.Artifact == "" || format.CommitLabel == "" ||
		format.StagePattern == "" || format.Encode == nil || format.Validate == nil {
		return result, errors.New("artifact publication requires an explicit format contract")
	}
	observe := func(phase, temporary string) error {
		if hooks.Observe != nil {
			if err := hooks.Observe(phase, temporary); err != nil {
				return fmt.Errorf("%s %s: %w", format.Label, phase, err)
			}
		}
		return nil
	}
	if err := observe("snapshot", ""); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("%s canceled: %w", format.Label, err)
	}
	path, parent, err := artifactPublicationDestination(requested, format.Label)
	if err != nil {
		return result, err
	}
	encoded, err := format.Encode(ctx)
	if err != nil {
		return result, fmt.Errorf("%s encode: %w", format.Label, err)
	}
	encoded = bytes.Clone(encoded)
	if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("%s prestage parent: %w", format.Label, err)
	}
	digest := sha256.Sum256(encoded)
	result.Path, result.SHA256 = path, hex.EncodeToString(digest[:])
	file, err := os.CreateTemp(filepath.Dir(path), format.StagePattern)
	if err != nil {
		return result, fmt.Errorf("%s create stage: %w", format.Label, err)
	}
	temporary := file.Name()
	owned, statErr := file.Stat()
	closed := false
	defer func() {
		if !closed {
			resultErr = errors.Join(resultErr, artifactPublicationClose(file, hooks.Close, format.Label))
		}
		// Never follow or remove a replacement, even during failed publication.
		if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup parent: %w", format.Label, err))
		} else if owned == nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup lacks stage identity; stage retained", format.Label))
		} else if err := observe("cleanup", temporary); err != nil {
			resultErr = errors.Join(resultErr, err)
		} else if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup parent changed: %w", format.Label, err))
		} else if err := artifactPublicationIdentity(temporary, owned, false); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%s cleanup stage retained: %w", format.Label, err))
		} else {
			remove := hooks.Remove
			if remove == nil {
				remove = os.Remove
			}
			if err := remove(temporary); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("%s remove owned stage: %w", format.Label, err))
			}
		}
		if result.Published && resultErr != nil {
			resultErr = fmt.Errorf("%s published; do not replay publication: %w", format.CommitLabel, resultErr)
		}
	}()
	if statErr != nil {
		return result, fmt.Errorf("%s stage identity: %w", format.Label, statErr)
	}
	if err := file.Chmod(0600); err != nil {
		return result, fmt.Errorf("%s stage permissions: %w", format.Label, err)
	}
	write := hooks.Write
	if write == nil {
		write = func(file *os.File, data []byte) (int, error) { return file.Write(data) }
	}
	for offset := 0; offset < len(encoded); {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("%s staging: %w", format.Label, err)
		}
		end := min(offset+32*1024, len(encoded))
		n, err := write(file, encoded[offset:end])
		if err != nil {
			return result, fmt.Errorf("%s stage write: %w", format.Label, err)
		}
		if n != end-offset {
			return result, fmt.Errorf("%s stage write: %w", format.Label, io.ErrShortWrite)
		}
		offset = end
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("%s staging canceled: %w", format.Label, err)
	}
	sync := hooks.Sync
	if sync == nil {
		sync = (*os.File).Sync
	}
	if err := sync(file); err != nil {
		return result, fmt.Errorf("%s stage sync: %w", format.Label, err)
	}
	err = artifactPublicationClose(file, hooks.Close, format.Label)
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
		return result, fmt.Errorf("%s parent: %w", format.Label, err)
	}
	if err := artifactPublicationIdentity(temporary, owned, false); err != nil {
		return result, fmt.Errorf("%s stage: %w", format.Label, err)
	}
	verification, err := os.Open(temporary)
	if err != nil {
		return result, fmt.Errorf("%s verify open: %w", format.Label, err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, artifactPublicationClose(verification, hooks.Close, format.Label))
	}()
	current, statErr := verification.Stat()
	data, readErr := io.ReadAll(io.LimitReader(ArtifactPublicationReader{ctx, verification}, int64(len(encoded))+1))
	if statErr != nil || current == nil || !os.SameFile(owned, current) {
		return result, errors.Join(statErr, readErr, fmt.Errorf("%s verification file identity changed", format.Label))
	}
	if readErr != nil {
		return result, fmt.Errorf("%s verify read: %w", format.Label, readErr)
	}
	if !bytes.Equal(data, encoded) {
		return result, fmt.Errorf("%s staged artifact differs from encoded %s", format.Label, format.Artifact)
	}
	if err := format.Validate(ctx, data); err != nil {
		return result, fmt.Errorf("%s staged %s: %w", format.Label, format.Artifact, err)
	}
	if validateSource != nil {
		if err := validateSource(); err != nil {
			return result, fmt.Errorf("%s source validation: %w", format.Label, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("%s prelink canceled: %w", format.Label, err)
	}
	// Reobserve after the final context callback, not just before it. Keep the
	// verification handle open through commit; no callback intervenes here.
	if _, err := verification.Seek(0, io.SeekStart); err != nil {
		return result, fmt.Errorf("%s final verify seek: %w", format.Label, err)
	}
	data, err = io.ReadAll(io.LimitReader(verification, int64(len(encoded))+1))
	if err != nil {
		return result, fmt.Errorf("%s final verify read: %w", format.Label, err)
	}
	if !bytes.Equal(data, encoded) {
		return result, fmt.Errorf("%s final staged artifact changed", format.Label)
	}
	if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("%s prelink parent: %w", format.Label, err)
	}
	if err := artifactPublicationIdentity(temporary, owned, false); err != nil {
		return result, fmt.Errorf("%s prelink stage: %w", format.Label, err)
	}
	link := hooks.Link
	if link == nil {
		link = os.Link
	}
	usedExclusiveCopy, err := sqlutil.LinkOrCreateExclusiveWith(temporary, path, link)
	if err != nil {
		return result, fmt.Errorf("%s requires atomic no-replace linking: %w", format.Label, err)
	}
	committedExpected := owned
	if usedExclusiveCopy {
		copiedInfo, statErr := os.Lstat(path)
		if statErr != nil {
			return result, fmt.Errorf("%s committed destination stat: %w", format.Label, statErr)
		}
		committedExpected = copiedInfo
	}
	result.Published = true // The irreversible commit boundary precedes every fallible callback.
	if err := observe("published", temporary); err != nil {
		return result, err
	}
	if err := artifactPublicationIdentity(filepath.Dir(path), parent, true); err != nil {
		return result, fmt.Errorf("%s committed parent: %w", format.Label, err)
	}
	if err := artifactPublicationIdentity(path, committedExpected, false); err != nil {
		return result, fmt.Errorf("%s committed destination: %w", format.Label, err)
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

type ArtifactPublicationReader struct {
	Ctx    context.Context `json:"-" yaml:"-"`
	Source io.Reader       `json:"-" yaml:"-"`
}

func (reader ArtifactPublicationReader) Read(data []byte) (int, error) {
	if err := reader.Ctx.Err(); err != nil {
		return 0, err
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	return reader.Source.Read(data)
}
