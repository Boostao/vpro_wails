package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOwnedTableCSVArchiveMetadataStatesAndLegacySeparation(t *testing.T) {
	ctx := context.Background()
	document := tableCSVBundleFixture(t)
	var emptyEncodings [][]byte
	for _, present := range []bool{false, true} {
		for _, candidates := range []bool{false, true} {
			t.Run(fmt.Sprintf("present=%t/candidates=%t", present, candidates), func(t *testing.T) {
				source := ownedTableCSVArchive{present, snapshotTableCSVBundleDocument(document)}
				if !candidates {
					source.Document.Manifest.Descriptions = []TableCSVDescription{}
				}
				data, err := encodeOwnedTableCSVArchive(ctx, source)
				if !present && candidates {
					if err == nil || data != nil {
						t.Fatal("absent metadata accepted physical candidates:", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				again, err := encodeOwnedTableCSVArchive(ctx, source)
				if err != nil || !bytes.Equal(data, again) {
					t.Fatal("owned archive is not deterministic:", err)
				}
				actual, err := decodeOwnedTableCSVArchive(ctx, data, int64(len(data)))
				if err != nil || !reflect.DeepEqual(actual, source) {
					t.Fatal("metadata state or document lost:", err)
				}
				independent, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil || len(independent.File) != 2 {
					t.Fatal("independent ZIP read failed:", err)
				}
				reader, err := independent.File[1].Open()
				if err != nil {
					t.Fatal(err)
				}
				raw, readErr := io.ReadAll(reader)
				closeErr := reader.Close()
				var manifest ownedTableCSVArchiveManifest
				if err := json.Unmarshal(raw, &manifest); err != nil || readErr != nil || closeErr != nil ||
					manifest.Format != ownedTableCSVArchiveFormat || manifest.Version != 1 ||
					manifest.DescriptionMetadataPresent != present || !reflect.DeepEqual(manifest.Document, source.Document.Manifest) {
					t.Fatal("independent manifest observation differs:", err, readErr, closeErr)
				}
				tableCSVBundleAssertRejected(t, data, int64(len(data)))
				if !candidates {
					emptyEncodings = append(emptyEncodings, data)
				}
			})
		}
	}
	if len(emptyEncodings) != 2 || bytes.Equal(emptyEncodings[0], emptyEncodings[1]) {
		t.Fatal("absent and present-empty archive bytes collapsed")
	}
	legacy, err := encodeTableCSVBundle(ctx, document)
	if err != nil {
		t.Fatal(err)
	}
	assertOwnedTableCSVArchiveRejected(t, legacy, int64(len(legacy)))
}

func assertOwnedTableCSVArchiveRejected(t *testing.T, data []byte, budget int64) {
	t.Helper()
	actual, err := decodeOwnedTableCSVArchive(context.Background(), data, budget)
	if err == nil || err.Error() == "" || !reflect.DeepEqual(actual, ownedTableCSVArchive{}) {
		t.Fatal("expected explicit refusal with zero output:", actual, err)
	}
}

func TestOwnedTableCSVArchiveStrictEnvelopeAndIntegrity(t *testing.T) {
	document := tableCSVBundleFixture(t)
	rawBytes, err := json.Marshal(ownedTableCSVArchiveManifest{ownedTableCSVArchiveFormat, 1, true, document.Manifest})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(rawBytes)
	for name, manifest := range map[string]string{
		"unknown":          strings.Replace(raw, `"format":`, `"extra":0,"format":`, 1),
		"duplicate":        strings.Replace(raw, `"version":1`, `"version":1,"version":1`, 1),
		"missing presence": strings.Replace(raw, `"descriptionMetadataPresent":true,`, "", 1),
		"null presence":    strings.Replace(raw, `"descriptionMetadataPresent":true`, `"descriptionMetadataPresent":null`, 1),
		"string presence":  strings.Replace(raw, `"descriptionMetadataPresent":true`, `"descriptionMetadataPresent":"true"`, 1),
		"number presence":  strings.Replace(raw, `"descriptionMetadataPresent":true`, `"descriptionMetadataPresent":1`, 1),
		"false candidates": strings.Replace(raw, `"descriptionMetadataPresent":true`, `"descriptionMetadataPresent":false`, 1),
		"wrong format":     strings.Replace(raw, ownedTableCSVArchiveFormat, "unrecognized", 1),
		"wrong version":    strings.Replace(raw, `"version":1`, `"version":2`, 1),
		"null document":    strings.Replace(raw, `"document":{`, `"document":null,"extra":{`, 1),
		"nested duplicate": strings.Replace(raw, `"table":`, `"table":"other","table":`, 1),
		"nested missing":   strings.Replace(raw, `"rowIds":`, `"unexpected":`, 1),
		"nested Unicode":   strings.Replace(raw, `"text":""`, `"text":"\ud800"`, 1),
		"key Unicode":      strings.Replace(raw, `"format"`, `"\ud800"`, 1),
		"invalid UTF8":     strings.Replace(raw, "Literal_", "Literal_"+string([]byte{0xff}), 1),
		"checksum":         strings.Replace(raw, document.Manifest.SHA256, strings.Repeat("0", 64), 1),
		"trailing JSON":    raw + "{}",
	} {
		t.Run(name, func(t *testing.T) {
			data := tableCSVBundleTestArchive(t,
				tableCSVBundleTestMember{"table.csv", document.Data, zip.Store, 0},
				tableCSVBundleTestMember{"manifest.json", []byte(manifest), zip.Store, 0})
			assertOwnedTableCSVArchiveRejected(t, data, int64(len(data)))
		})
	}
	data, err := encodeOwnedTableCSVArchive(context.Background(), ownedTableCSVArchive{true, document})
	if err != nil {
		t.Fatal(err)
	}
	assertOwnedTableCSVArchiveRejected(t, data, int64(len(data))-1)
	assertOwnedTableCSVArchiveRejected(t, data, 0)
	corrupt := bytes.Clone(data)
	corrupt[30+len("table.csv")] ^= 1
	assertOwnedTableCSVArchiveRejected(t, corrupt, int64(len(corrupt)))
}

func TestOwnedTableCSVArchiveCancellationAndSnapshots(t *testing.T) {
	original := ownedTableCSVArchive{true, tableCSVBundleFixture(t)}
	encodeCount := tableCSVBundleCancellation(0)
	encoded, err := encodeOwnedTableCSVArchive(encodeCount, original)
	encodeCount.cancel()
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= encodeCount.calls; at++ {
		ctx := tableCSVBundleCancellation(at)
		actual, err := encodeOwnedTableCSVArchive(ctx, original)
		ctx.cancel()
		if !errors.Is(err, context.Canceled) || actual != nil {
			t.Fatalf("encode checkpoint %d returned partial success: %v", at, err)
		}
	}
	decodeCount := tableCSVBundleCancellation(0)
	if _, err := decodeOwnedTableCSVArchive(decodeCount, encoded, int64(len(encoded))); err != nil {
		t.Fatal(err)
	}
	decodeCount.cancel()
	for at := 1; at <= decodeCount.calls; at++ {
		ctx := tableCSVBundleCancellation(at)
		actual, err := decodeOwnedTableCSVArchive(ctx, encoded, int64(len(encoded)))
		ctx.cancel()
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(actual, ownedTableCSVArchive{}) {
			t.Fatalf("decode checkpoint %d returned partial success: %v", at, err)
		}
	}
	source := ownedTableCSVArchive{true, tableCSVBundleFixture(t)}
	ctx := &tableCSVBundleMutationContext{Context: context.Background(), at: 1, mutate: func() {
		source.Document.Data[0] = 'X'
		*source.Document.Manifest.Descriptions[4].Value.Text = "changed"
	}}
	actual, err := encodeOwnedTableCSVArchive(ctx, source)
	if err != nil || !bytes.Equal(actual, encoded) {
		t.Fatal("encoder retained caller aliases:", err)
	}
	input := bytes.Clone(encoded)
	ctx = &tableCSVBundleMutationContext{Context: context.Background(), at: 1, mutate: func() { clear(input) }}
	decoded, err := decodeOwnedTableCSVArchive(ctx, input, int64(len(input)))
	if err != nil || !reflect.DeepEqual(decoded, original) {
		t.Fatal("decoder retained caller archive alias:", err)
	}
}

func TestOwnedTableCSVArchivePublicationPreservesMetadataAndCollision(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprint(present), func(t *testing.T) {
			service, state, _ := ownedTableCSVPublicationFixture(t)
			tableCSVProjectWriter(t, service, `DELETE FROM _table_metadata`)
			if !present {
				tableCSVProjectWriter(t, service, `DROP TABLE _table_metadata`)
			}
			review, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			directory := t.TempDir()
			destination := filepath.Join(directory, "migration.zip")
			result, err := service.publishOwnedProjectTableCSVArchive(context.Background(), state.ContextID, review, destination)
			if err != nil || !result.Published {
				t.Fatal("owned publication failed:", result, err)
			}
			bytesBefore := tableCSVPublicationRead(t, destination)
			actual, err := decodeOwnedTableCSVArchive(context.Background(), bytesBefore, int64(len(bytesBefore)))
			if err != nil || actual.DescriptionMetadataPresent != present || !reflect.DeepEqual(actual.Document, review.Document) {
				t.Fatal("publication lost source metadata presence:", err)
			}
			collision, err := service.publishOwnedProjectTableCSVArchive(context.Background(), state.ContextID, review, destination)
			if err == nil || collision.Published || !bytes.Equal(bytesBefore, tableCSVPublicationRead(t, destination)) {
				t.Fatal("collision replaced destination:", collision, err)
			}
			tableCSVPublicationOnly(t, directory, "migration.zip")
			if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
				t.Fatal("read-only source changed")
			}
		})
	}
}

func TestOwnedTableCSVArchivePublicationDriftCancellationAndCommittedWarning(t *testing.T) {
	for _, phase := range []string{"precommit", "staged", "published"} {
		t.Run(phase, func(t *testing.T) {
			service, state, review := ownedTableCSVPublicationFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			directory := t.TempDir()
			result, err := service.publishOwnedProjectTableCSVArchiveWithHooks(ctx, state.ContextID, review, filepath.Join(directory, "migration.zip"),
				tableCSVOwnedPublicationHooks{
					observe: func(current string) error {
						if current == phase {
							tableCSVProjectWriter(t, service, `DELETE FROM _table_metadata`)
						}
						return nil
					},
					publication: tableCSVPublicationHooks{observe: func(current, _ string) error {
						if current == "staged" && phase == "staged" {
							cancel()
						}
						return nil
					}},
				})
			if err == nil || result.Published != (phase == "published") {
				t.Fatal("source drift/cancellation outcome incorrect:", result, err)
			}
			if phase == "published" {
				expected, encodeErr := encodeOwnedTableCSVArchive(context.Background(), ownedTableCSVArchive{review.DescriptionMetadataPresent, review.Document})
				if encodeErr != nil || !strings.Contains(err.Error(), "do not replay") ||
					!bytes.Equal(expected, tableCSVPublicationRead(t, result.Path)) {
					t.Fatal("postcommit warning lost original committed artifact:", encodeErr, err)
				}
				tableCSVPublicationOnly(t, directory, "migration.zip")
			} else {
				tableCSVPublicationOnly(t, directory)
			}
			fresh, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
			if err != nil {
				t.Fatal(err)
			}
			retry, err := service.publishOwnedProjectTableCSVArchive(context.Background(), state.ContextID, fresh, filepath.Join(directory, "retry.zip"))
			if err != nil || !retry.Published {
				t.Fatal("fresh retry after failure failed:", retry, err)
			}
		})
	}
}
