package interchange

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func tableCSVPublicationWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestTableCSVPublicationArchiveRoundtripHashSnapshotAndNeverReplay(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, " Literal_Bundle.no-inferred-extension")
	document := tableCSVBundleFixture(t)
	original := tableCSVClone(t, document)
	want, err := encodeTableCSVBundle(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	result, err := publishTableCSVBundleWithHooks(context.Background(), path, document, tableCSVPublicationHooks{
		Observe: func(phase, _ string) error {
			if phase == "snapshot" {
				document.Data[0] = 'X'
				document.Manifest.Columns[0].Name = "mutated"
				*document.Manifest.Descriptions[4].Value.Text = "mutated"
			}
			return nil
		},
	})
	digest := sha256.Sum256(want)
	physicalParent, resolveErr := filepath.EvalSymlinks(directory)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	if err != nil || !result.Published || result.Path != filepath.Join(physicalParent, filepath.Base(path)) || result.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("result %#v: %v", result, err)
	}
	data := vprotest.TableCSVPublicationRead(t, path)
	if !bytes.Equal(data, want) {
		t.Fatal("published bytes differ from independently encoded archive")
	}
	decoded, err := decodeTableCSVBundle(context.Background(), data, int64(len(data)))
	if err != nil || !reflect.DeepEqual(decoded, original) {
		t.Fatal("NULL, signed64, literal, duplicate or typed storage roundtrip differs:", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("private permissions:", info, err)
		}
	}
	for _, destination := range []string{path, filepath.Join(directory, ".", filepath.Base(path))} {
		retry, err := publishTableCSVBundle(context.Background(), destination, original)
		if err == nil || retry.Published || !bytes.Equal(vprotest.TableCSVPublicationRead(t, path), want) {
			t.Fatalf("replay replaced committed artifact: %#v %v", retry, err)
		}
	}
	vprotest.TableCSVPublicationOnly(t, directory, filepath.Base(path))
}

func TestTableCSVPublicationRejectsMalformedDocumentsAndLiteralPaths(t *testing.T) {
	directory := t.TempDir()
	document := tableCSVBundleFixture(t)
	for _, name := range []string{"relative.zip", "", "bad\x00.zip", "bad\xff.zip", "trail.", "trail ",
		"NUL", "con.zip", "COM1.zip", "LPT9", "COM¹.zip", "LPT².zip", "CONIN$", "CONOUT$.zip",
		"bad:stream", "bad?name", "bad*name", "bad\"name", "bad|name", "bad<name", "bad>name", "bad\x01name"} {
		t.Run(name, func(t *testing.T) {
			path := name
			if name != "relative.zip" && name != "" {
				path = filepath.Join(directory, name)
			}
			result, err := publishTableCSVBundle(context.Background(), path, document)
			if err == nil || result.Published {
				t.Fatalf("invalid path accepted: %#v %v", result, err)
			}
		})
	}
	for _, path := range []string{directory + string(filepath.Separator), directory + string(filepath.Separator) + ".." + string(filepath.Separator) + "artifact",
		directory + string(filepath.Separator) + "." + string(filepath.Separator) + "artifact",
		filepath.Join(directory, "absent", "artifact"), `\\?\C:\artifact`, `\\.\C:\artifact`} {
		result, err := publishTableCSVBundle(context.Background(), path, document)
		if err == nil || result.Published {
			t.Fatalf("ambiguous/missing-parent path accepted: %q %#v %v", path, result, err)
		}
	}
	for _, mutate := range []func(*tableCSVDocument){
		func(d *tableCSVDocument) { d.Data[0] = 'X' },
		func(d *tableCSVDocument) { d.Manifest.Columns[0].Name = "\xff" },
		func(d *tableCSVDocument) { *d.Manifest.Descriptions[4].Value.Text = "\xff" },
	} {
		bad := tableCSVBundleFixture(t)
		mutate(&bad)
		result, err := publishTableCSVBundle(context.Background(), filepath.Join(directory, "artifact"), bad)
		if err == nil || result.Published {
			t.Fatalf("invalid source accepted: %#v %v", result, err)
		}
	}
	vprotest.TableCSVPublicationOnly(t, directory)
}

func TestTableCSVPublicationExistingAndLateCollisions(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink", "late-file", "late-directory", "late-link", "case-alias"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			marker := []byte("unowned destination")
			compete := func() {
				switch kind {
				case "directory", "late-directory":
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.Symlink("missing", path); err != nil {
						t.Skipf("symlink creation unavailable: %v", err)
					}
				case "late-link":
					tableCSVPublicationWrite(t, filepath.Join(directory, "competitor"), marker)
					if err := os.Link(filepath.Join(directory, "competitor"), path); err != nil {
						t.Fatal(err)
					}
				case "case-alias":
					if runtime.GOOS != "windows" {
						t.Skip("Win32 case alias")
					}
					tableCSVPublicationWrite(t, filepath.Join(directory, "ARTIFACT"), marker)
				default:
					tableCSVPublicationWrite(t, path, marker)
				}
			}
			hooks := tableCSVPublicationHooks{}
			if strings.HasPrefix(kind, "late-") {
				hooks.Observe = func(phase, _ string) error {
					if phase == "prelink" {
						compete()
					}
					return nil
				}
			} else {
				compete()
			}
			result, err := publishTableCSVBundleWithHooks(context.Background(), path, tableCSVBundleFixture(t), hooks)
			if err == nil || result.Published {
				t.Fatalf("collision accepted %#v: %v", result, err)
			}
			info, statErr := os.Lstat(path)
			if statErr != nil {
				t.Fatal("unowned destination deleted:", statErr)
			}
			if info.Mode().IsRegular() && !bytes.Equal(vprotest.TableCSVPublicationRead(t, path), marker) {
				t.Fatal("unowned destination overwritten")
			}
			names := []string{filepath.Base(path)}
			if kind == "late-link" {
				names = append(names, "competitor")
			} else if kind == "case-alias" {
				names = []string{"ARTIFACT"}
			}
			vprotest.TableCSVPublicationOnly(t, directory, names...)
		})
	}
}

type tableCSVPublicationCountingContext struct {
	context.Context
	calls    int
	cancelAt int
	observe  func(int)
}

func (ctx *tableCSVPublicationCountingContext) Err() error {
	ctx.calls++
	if ctx.observe != nil {
		ctx.observe(ctx.calls)
	}
	if ctx.cancelAt > 0 && ctx.calls >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestTableCSVPublicationCancellationAndFreshRetry(t *testing.T) {
	// The last observed context check is the final prelink check, not an
	// arbitrary earlier encode/decode checkpoint.
	baseline := &tableCSVPublicationCountingContext{Context: context.Background()}
	if result, err := publishTableCSVBundle(baseline, filepath.Join(t.TempDir(), "baseline"), tableCSVBundleFixture(t)); err != nil || !result.Published {
		t.Fatal(result, err)
	}
	for _, phase := range []string{"initial", "write", "staged", "prelink", "final"} {
		t.Run(phase, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var used context.Context = ctx
			hooks := tableCSVPublicationHooks{
				Observe: func(current, _ string) error {
					if current == phase {
						cancel()
					}
					return nil
				},
			}
			if phase == "initial" {
				cancel()
			} else if phase == "write" {
				hooks.Write = func(file *os.File, data []byte) (int, error) {
					n, err := file.Write(data)
					cancel()
					return n, err
				}
			} else if phase == "final" {
				used = &tableCSVPublicationCountingContext{Context: context.Background(), cancelAt: baseline.calls}
			}
			result, err := publishTableCSVBundleWithHooks(used, path, tableCSVBundleFixture(t), hooks)
			if !errors.Is(err, context.Canceled) || result.Published {
				t.Fatalf("cancellation %#v: %v", result, err)
			}
			vprotest.TableCSVPublicationOnly(t, directory)
			result, err = publishTableCSVBundle(context.Background(), path, tableCSVBundleFixture(t))
			if err != nil || !result.Published {
				t.Fatal("fresh retry:", result, err)
			}
			vprotest.TableCSVPublicationOnly(t, directory, "artifact")
		})
	}
}

func TestTableCSVPublicationFinalCallbackReobservesOwnedFileAndContent(t *testing.T) {
	baseline := &tableCSVPublicationCountingContext{Context: context.Background()}
	if result, err := publishTableCSVBundle(baseline, filepath.Join(t.TempDir(), "baseline"), tableCSVBundleFixture(t)); err != nil || !result.Published {
		t.Fatal(result, err)
	}
	for _, kind := range []string{"replacement", "corruption"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			var stage string
			replacementBlocked := false
			ctx := &tableCSVPublicationCountingContext{Context: context.Background(), observe: func(call int) {
				if call != baseline.calls {
					return
				}
				if kind == "replacement" {
					if err := os.Rename(stage, filepath.Join(directory, "preserved")); err != nil {
						if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(32)) {
							replacementBlocked = true
							return
						}
						t.Fatal(err)
					}
				}
				tableCSVPublicationWrite(t, stage, []byte("final callback mutation"))
			}}
			result, err := publishTableCSVBundleWithHooks(ctx, filepath.Join(directory, "artifact"), tableCSVBundleFixture(t),
				tableCSVPublicationHooks{Observe: func(phase, temporary string) error {
					if phase == "staged" {
						stage = temporary
					}
					return nil
				}})
			if replacementBlocked {
				if err != nil || !result.Published {
					t.Fatal("pinned stage should still publish its original artifact:", result, err)
				}
				vprotest.TableCSVPublicationOnly(t, directory, "artifact")
				return
			}
			if err == nil || result.Published {
				t.Fatalf("final callback mutation accepted %#v: %v", result, err)
			}
			if kind == "replacement" {
				if !bytes.Equal(vprotest.TableCSVPublicationRead(t, stage), []byte("final callback mutation")) {
					t.Fatal("unowned final replacement deleted")
				}
				vprotest.TableCSVPublicationOnly(t, directory, filepath.Base(stage), "preserved")
			} else {
				vprotest.TableCSVPublicationOnly(t, directory)
			}
		})
	}
}

func TestTableCSVPublicationParentReplacementDuringEncodingBeforeStaging(t *testing.T) {
	root := t.TempDir()
	parent, moved := filepath.Join(root, "parent"), filepath.Join(root, "moved")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	originalMarker := []byte("original parent contents")
	replacementMarker := []byte("unowned replacement contents")
	tableCSVPublicationWrite(t, filepath.Join(parent, "original"), originalMarker)
	swapped, staged := false, false
	ctx := &tableCSVPublicationCountingContext{Context: context.Background(), observe: func(call int) {
		// The first check precedes destination acceptance; the second runs in
		// the encoder, after acceptance but before any stage is created.
		if call != 2 {
			return
		}
		if err := os.Rename(parent, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(parent, 0700); err != nil {
			t.Fatal(err)
		}
		tableCSVPublicationWrite(t, filepath.Join(parent, "unowned"), replacementMarker)
		swapped = true
	}}
	result, err := publishTableCSVBundleWithHooks(ctx, filepath.Join(parent, "artifact"), tableCSVBundleFixture(t),
		tableCSVPublicationHooks{Observe: func(phase, _ string) error {
			if phase == "staged" {
				staged = true
			}
			return nil
		}})
	if !swapped || err == nil || result.Published || staged {
		t.Fatalf("encoding-time parent swap: swapped=%v staged=%v result=%#v error=%v", swapped, staged, result, err)
	}
	if !bytes.Equal(vprotest.TableCSVPublicationRead(t, filepath.Join(parent, "unowned")), replacementMarker) ||
		!bytes.Equal(vprotest.TableCSVPublicationRead(t, filepath.Join(moved, "original")), originalMarker) {
		t.Fatal("changed unowned replacement or moved original contents")
	}
	vprotest.TableCSVPublicationOnly(t, parent, "unowned")
	vprotest.TableCSVPublicationOnly(t, moved, "original")
	vprotest.TableCSVPublicationOnly(t, root, "parent", "moved")
}

func TestTableCSVPublicationOwnedStageReplacementCorruptionAndRestorationAliases(t *testing.T) {
	for _, kind := range []string{"replacement", "missing", "symlink", "corruption", "prelink-corruption", "equivalent-alias", "restored-original"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			preserved := filepath.Join(directory, "preserved")
			var temporary string
			hooks := tableCSVPublicationHooks{Observe: func(phase, stage string) error {
				trigger := "staged"
				if kind == "prelink-corruption" {
					trigger = "prelink"
				}
				if phase != trigger {
					return nil
				}
				temporary = stage
				if kind == "corruption" || kind == "prelink-corruption" {
					tableCSVPublicationWrite(t, stage, []byte("corrupted owned file"))
					return nil
				}
				if err := os.Rename(stage, preserved); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "replacement":
					tableCSVPublicationWrite(t, stage, []byte("unowned replacement"))
				case "symlink":
					if err := os.Symlink(preserved, stage); err != nil {
						// Restore the fixture before skipping; no kernel cleanup ran yet.
						if restoreErr := os.Rename(preserved, stage); restoreErr != nil {
							t.Fatal(restoreErr)
						}
						return tableCSVPublicationSymlinkUnavailable{err}
					}
				case "equivalent-alias":
					if err := os.Link(preserved, stage); err != nil {
						t.Fatal(err)
					}
				case "restored-original":
					if err := os.Rename(preserved, stage); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			}}
			result, err := publishTableCSVBundleWithHooks(context.Background(), path, tableCSVBundleFixture(t), hooks)
			var unavailable tableCSVPublicationSymlinkUnavailable
			if errors.As(err, &unavailable) {
				vprotest.TableCSVPublicationOnly(t, directory)
				t.Skipf("symlink creation unavailable: %v", unavailable)
			}
			success := kind == "equivalent-alias" || kind == "restored-original"
			if success {
				if err != nil || !result.Published {
					t.Fatal(result, err)
				}
				names := []string{"artifact"}
				if kind == "equivalent-alias" {
					names = append(names, "preserved")
					original, _ := os.Stat(preserved)
					destination, _ := os.Stat(path)
					if !os.SameFile(original, destination) {
						t.Fatal("equivalent alias lost owned identity")
					}
				}
				vprotest.TableCSVPublicationOnly(t, directory, names...)
				return
			}
			if err == nil || result.Published {
				t.Fatalf("unsafe stage accepted: %#v %v", result, err)
			}
			if kind == "replacement" {
				if !bytes.Equal(vprotest.TableCSVPublicationRead(t, temporary), []byte("unowned replacement")) {
					t.Fatal("deleted or changed unowned replacement")
				}
				vprotest.TableCSVPublicationOnly(t, directory, filepath.Base(temporary), "preserved")
			} else if kind == "symlink" {
				info, err := os.Lstat(temporary)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("unowned symlink removed:", err)
				}
				vprotest.TableCSVPublicationOnly(t, directory, filepath.Base(temporary), "preserved")
			} else if kind == "missing" {
				vprotest.TableCSVPublicationOnly(t, directory, "preserved")
			} else {
				vprotest.TableCSVPublicationOnly(t, directory)
			}
		})
	}
}

type tableCSVPublicationSymlinkUnavailable struct{ error }

func TestTableCSVPublicationParentReplacementRetainsUnownedAndMovedStage(t *testing.T) {
	for _, phase := range []string{"staged", "prelink", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			parent, moved := filepath.Join(root, "parent"), filepath.Join(root, "moved")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			var basename string
			result, err := publishTableCSVBundleWithHooks(context.Background(), filepath.Join(parent, "artifact"), tableCSVBundleFixture(t),
				tableCSVPublicationHooks{Observe: func(current, stage string) error {
					if current != phase {
						return nil
					}
					basename = filepath.Base(stage)
					if err := os.Rename(parent, moved); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(parent, 0700); err != nil {
						t.Fatal(err)
					}
					tableCSVPublicationWrite(t, filepath.Join(parent, basename), []byte("unowned replacement"))
					return nil
				}})
			if err == nil || result.Published != (phase == "cleanup") {
				t.Fatalf("parent replacement %#v: %v", result, err)
			}
			if !bytes.Equal(vprotest.TableCSVPublicationRead(t, filepath.Join(parent, basename)), []byte("unowned replacement")) {
				t.Fatal("unowned replacement deleted or altered")
			}
			vprotest.TableCSVPublicationOnly(t, parent, basename)
			names := []string{basename}
			if result.Published {
				names = append(names, "artifact")
				if !strings.Contains(err.Error(), "do not replay") {
					t.Fatal("committed cleanup error lacks replay warning:", err)
				}
			}
			vprotest.TableCSVPublicationOnly(t, moved, names...)
		})
	}
}

func TestTableCSVPublicationFaultsAndCommittedStatus(t *testing.T) {
	fault := errors.New("injected publication fault")
	for _, kind := range []string{"snapshot", "write", "short-write", "sync", "stage-close", "unsupported-link",
		"postlink", "verify-close", "cleanup", "remove", "postlink-canceled", "postlink-replacement", "cleanup-replacement"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var stageName string
			closeCalls := 0
			hooks := tableCSVPublicationHooks{
				Observe: func(phase, stage string) error {
					if stage != "" {
						stageName = filepath.Base(stage)
					}
					if (kind == "snapshot" && phase == "snapshot") || (kind == "postlink" && phase == "published") ||
						(kind == "cleanup" && phase == "cleanup") {
						return fault
					}
					if kind == "postlink-canceled" && phase == "published" {
						cancel()
						return ctx.Err()
					}
					if (kind == "postlink-replacement" && phase == "published") || (kind == "cleanup-replacement" && phase == "cleanup") {
						target := path
						if kind == "cleanup-replacement" {
							target = stage
						}
						if err := os.Remove(target); err != nil {
							return err
						}
						tableCSVPublicationWrite(t, target, []byte("unowned postcommit replacement"))
					}
					return nil
				},
			}
			switch kind {
			case "write":
				hooks.Write = func(*os.File, []byte) (int, error) { return 0, fault }
			case "short-write":
				hooks.Write = func(*os.File, []byte) (int, error) { return 0, nil }
			case "sync":
				hooks.Sync = func(*os.File) error { return fault }
			case "stage-close", "verify-close":
				hooks.Close = func(file *os.File) error {
					closeCalls++
					err := file.Close()
					if (kind == "stage-close" && closeCalls == 1) || (kind == "verify-close" && closeCalls == 2) {
						return errors.Join(err, fault)
					}
					return err
				}
			case "unsupported-link":
				hooks.Link = func(string, string) error { return fault }
			case "remove":
				hooks.Remove = func(string) error { return fault }
			}
			result, err := publishTableCSVBundleWithHooks(ctx, path, tableCSVBundleFixture(t), hooks)
			committed := kind == "postlink" || kind == "verify-close" || kind == "cleanup" || kind == "remove" ||
				kind == "postlink-canceled" || kind == "postlink-replacement" || kind == "cleanup-replacement"
			if err == nil || result.Published != committed {
				t.Fatalf("fault result %#v: %v", result, err)
			}
			if kind == "short-write" && !errors.Is(err, io.ErrShortWrite) {
				t.Fatal("lost short write cause:", err)
			}
			if committed {
				physicalParent, resolveErr := filepath.EvalSymlinks(directory)
				if resolveErr != nil {
					t.Fatal(resolveErr)
				}
				if result.Path != filepath.Join(physicalParent, filepath.Base(path)) || len(result.SHA256) != 64 || !strings.Contains(err.Error(), "do not replay") {
					t.Fatalf("ambiguous committed result %#v: %v", result, err)
				}
				if _, err := os.Lstat(path); err != nil {
					t.Fatal("destructive rollback of published target:", err)
				}
			}
			names := []string{}
			if committed {
				names = append(names, "artifact")
			}
			if kind == "cleanup" || kind == "remove" || kind == "cleanup-replacement" {
				names = append(names, stageName)
			}
			vprotest.TableCSVPublicationOnly(t, directory, names...)
			if kind == "postlink-replacement" || kind == "cleanup-replacement" {
				target := path
				if kind == "cleanup-replacement" {
					target = filepath.Join(directory, stageName)
				}
				if !bytes.Equal(vprotest.TableCSVPublicationRead(t, target), []byte("unowned postcommit replacement")) {
					t.Fatal("unowned postcommit replacement deleted")
				}
			}
		})
	}
}
