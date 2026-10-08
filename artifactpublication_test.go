package main

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

func testArtifactPublicationFormat(data []byte) artifactPublicationFormat {
	expected := bytes.Clone(data)
	return artifactPublicationFormat{
		label: "test artifact publication", artifact: "document",
		commitLabel: "test artifact", stagePattern: ".vpro-test-artifact-*",
		encode: func(context.Context) ([]byte, error) { return data, nil },
		validate: func(_ context.Context, staged []byte) error {
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
	result, err := publishArtifactChecked(context.Background(), path, testArtifactPublicationFormat(data), func() error {
		phases = append(phases, "authority")
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.New("authority check ran after publication")
		}
		return nil
	}, artifactPublicationHooks{observe: func(phase, _ string) error {
		phases = append(phases, phase)
		if phase == "staged" {
			data[0] = 'X'
		}
		return nil
	}})
	hash := sha256.Sum256(expected)
	if err != nil || !result.Published || result.SHA256 != hex.EncodeToString(hash[:]) ||
		!bytes.Equal(tableCSVPublicationRead(t, path), expected) {
		t.Fatalf("detached artifact: %#v %v", result, err)
	}
	if !reflect.DeepEqual(phases, []string{"snapshot", "staged", "prelink", "authority", "published", "cleanup"}) {
		t.Fatalf("publication phases: %v", phases)
	}
	tableCSVPublicationOnly(t, directory, filepath.Base(path))
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
			hooks := artifactPublicationHooks{}
			var authority func() error
			switch failure {
			case "missing-contract":
				format.validate = nil
			case "encode":
				format.encode = func(context.Context) ([]byte, error) { return nil, rejected }
			case "validate":
				format.validate = func(context.Context, []byte) error { return rejected }
			case "authority":
				authority = func() error { return rejected }
			case "cancel":
				authority = func() error { cancel(); return nil }
			case "late-stage-change":
				var temporary string
				hooks.observe = func(phase, stage string) error {
					if phase == "prelink" {
						temporary = stage
					}
					return nil
				}
				authority = func() error { return os.WriteFile(temporary, []byte("changed"), 0600) }
			}
			result, err := publishArtifactChecked(ctx, path, format, authority, hooks)
			if err == nil || result.Published || strings.Contains(err.Error(), "table CSV") {
				t.Fatalf("non-CSV rejection: %#v %v", result, err)
			}
			if (failure == "encode" || failure == "validate" || failure == "authority") && !errors.Is(err, rejected) {
				t.Fatalf("cause lost: %v", err)
			}
			if failure == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			tableCSVPublicationOnly(t, directory)
		})
	}
}

func TestArtifactPublicationOperationLabelsAndCleanupFailureKeepCommittedResult(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "output")
	data := []byte("committed")
	format := testArtifactPublicationFormat(data)
	rejected := errors.New("owned cleanup failed")
	result, err := publishArtifactChecked(context.Background(), path, format, nil, artifactPublicationHooks{
		remove: func(string) error { return rejected },
	})
	if !result.Published || !errors.Is(err, rejected) ||
		!strings.Contains(err.Error(), "test artifact published; do not replay publication") ||
		strings.Contains(err.Error(), "table CSV") || !bytes.Equal(tableCSVPublicationRead(t, path), data) {
		t.Fatalf("committed cleanup outcome: %#v %v", result, err)
	}
	retry, err := publishArtifactChecked(context.Background(), path, format, nil, artifactPublicationHooks{})
	if err == nil || retry.Published || !strings.Contains(err.Error(), "test artifact publication") ||
		!bytes.Equal(tableCSVPublicationRead(t, path), data) {
		t.Fatalf("committed retry replaced bytes: %#v %v", retry, err)
	}
	invalid, err := publishArtifactChecked(context.Background(), "relative", format, nil, artifactPublicationHooks{})
	if err == nil || invalid.Published || !strings.Contains(err.Error(), "test artifact publication") {
		t.Fatalf("literal path label: %#v %v", invalid, err)
	}
}
