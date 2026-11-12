package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testArtifactPublicationFormat(data []byte) vprocore.ArtifactPublicationFormat {
	expected := bytes.Clone(data)
	return vprocore.ArtifactPublicationFormat{
		Label: "test artifact publication", Artifact: "document",
		CommitLabel: "test artifact", StagePattern: ".vpro-test-artifact-*",
		Encode: func(context.Context) ([]byte, error) { return data, nil },
		Validate: func(_ context.Context, staged []byte) error {
			if !bytes.Equal(staged, expected) {
				return errors.New("test format bytes differ")
			}
			return nil
		},
	}
}

func TestArtifactPublicationDetachesEncoderBytesAndPreservesCommitOrder(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "literal.no-added-extension")
	data := []byte{0, 255, '<', '&', '\r', '\n'}
	expected := bytes.Clone(data)
	var phases []string
	result, err := vprocore.PublishArtifactChecked(context.Background(), path, testArtifactPublicationFormat(data), func() error {
		phases = append(phases, "authority")
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.New("authority check ran after publication")
		}
		return nil
	}, vprocore.ArtifactPublicationHooks{Observe: func(phase, _ string) error {
		phases = append(phases, phase)
		if phase == "staged" {
			data[0] = 'X'
		}
		return nil
	}})
	hash := sha256.Sum256(expected)
	if err != nil || !result.Published || result.SHA256 != hex.EncodeToString(hash[:]) ||
		!bytes.Equal(vprotest.TableCSVPublicationRead(t, path), expected) {
		t.Fatalf("detached artifact: %#v %v", result, err)
	}
	if !reflect.DeepEqual(phases, []string{"snapshot", "staged", "prelink", "authority", "published", "cleanup"}) {
		t.Fatalf("publication phases: %v", phases)
	}
	vprotest.TableCSVPublicationOnly(t, directory, filepath.Base(path))
}

func TestArtifactPublicationFormatAuthorityAndCancellationErrorsDoNotPublish(t *testing.T) {
	for _, failure := range []string{"missing-contract", "encode", "validate", "authority", "cancel", "late-stage-change"} {
		t.Run(failure, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "output")
			data := []byte("validated non-CSV bytes")
			format := testArtifactPublicationFormat(data)
			rejected := errors.New("explicit rejection")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			hooks := vprocore.ArtifactPublicationHooks{}
			var authority func() error
			switch failure {
			case "missing-contract":
				format.Validate = nil
			case "encode":
				format.Encode = func(context.Context) ([]byte, error) { return nil, rejected }
			case "validate":
				format.Validate = func(context.Context, []byte) error { return rejected }
			case "authority":
				authority = func() error { return rejected }
			case "cancel":
				authority = func() error { cancel(); return nil }
			case "late-stage-change":
				var temporary string
				hooks.Observe = func(phase, stage string) error {
					if phase == "prelink" {
						temporary = stage
					}
					return nil
				}
				authority = func() error { return os.WriteFile(temporary, []byte("changed"), 0600) }
			}
			result, err := vprocore.PublishArtifactChecked(ctx, path, format, authority, hooks)
			if err == nil || result.Published || strings.Contains(err.Error(), "table CSV") {
				t.Fatalf("non-CSV rejection: %#v %v", result, err)
			}
			if (failure == "encode" || failure == "validate" || failure == "authority") && !errors.Is(err, rejected) {
				t.Fatalf("cause lost: %v", err)
			}
			if failure == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			vprotest.TableCSVPublicationOnly(t, directory)
		})
	}
}

func TestArtifactPublicationOperationLabelsAndCleanupFailureKeepCommittedResult(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "output")
	data := []byte("committed")
	format := testArtifactPublicationFormat(data)
	rejected := errors.New("owned cleanup failed")
	result, err := vprocore.PublishArtifactChecked(context.Background(), path, format, nil, vprocore.ArtifactPublicationHooks{
		Remove: func(string) error { return rejected },
	})
	if !result.Published || !errors.Is(err, rejected) ||
		!strings.Contains(err.Error(), "test artifact published; do not replay publication") ||
		strings.Contains(err.Error(), "table CSV") || !bytes.Equal(vprotest.TableCSVPublicationRead(t, path), data) {
		t.Fatalf("committed cleanup outcome: %#v %v", result, err)
	}
	retry, err := vprocore.PublishArtifactChecked(context.Background(), path, format, nil, vprocore.ArtifactPublicationHooks{})
	if err == nil || retry.Published || !strings.Contains(err.Error(), "test artifact publication") ||
		!bytes.Equal(vprotest.TableCSVPublicationRead(t, path), data) {
		t.Fatalf("committed retry replaced bytes: %#v %v", retry, err)
	}
	invalid, err := vprocore.PublishArtifactChecked(context.Background(), "relative", format, nil, vprocore.ArtifactPublicationHooks{})
	if err == nil || invalid.Published || !strings.Contains(err.Error(), "test artifact publication") {
		t.Fatalf("literal path label: %#v %v", invalid, err)
	}
}
