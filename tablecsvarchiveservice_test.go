package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestTableCSVArchiveServiceIndependentGateAndAuthorization(t *testing.T) {
	for _, value := range []string{"", "true", "false", "TRUE", "1", " true", "true "} {
		enabled, err := tableCSVArchiveFeature(func(key string) (string, bool) {
			if key != tableCSVArchiveFeatureEnvironment {
				t.Fatal("archive export inherited another flag", key)
			}
			return value, value != ""
		})
		if enabled != (value == "true") || (err != nil) != (value != "" && value != "true" && value != "false") {
			t.Fatal("literal flag semantics", value, enabled, err)
		}
	}
	for _, service := range []*TableCSVArchiveService{nil, NewTableCSVArchiveService(nil, false), NewTableCSVArchiveService(nil, true)} {
		if result, err := service.GetTableCSVArchiveReview(context.Background(), "owned", "{}"); result != nil || err == nil {
			t.Fatal("unavailable review accepted", result, err)
		}
		if result, err := service.ExportReviewedTableCSVArchive(context.Background(), "owned", "{}"); result != nil || err == nil {
			t.Fatal("unavailable export accepted", result, err)
		}
	}
	service := NewTableCSVArchiveService(nil, true)
	if result, err := service.GetTableCSVArchiveReview(nil, "", "{}"); result != nil || err == nil {
		t.Fatal("nil request context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.ExportReviewedTableCSVArchive(ctx, "", "{}"); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
}

func TestTableCSVArchiveServiceStrictRawRequests(t *testing.T) {
	for _, request := range []string{
		`{}`, `null`, `[]`, `{"table":null}`, `{"table":1}`, `{"table":"\ud800"}`,
		"{\"table\":\"\xff\"}", `{"table":"one","table":"two"}`,
		`{"table":"one","\u0074able":"two"}`, `{"table":"one","Table":"two"}`,
		`{"Table":"one"}`, `{"table":"one","extra":""}`, `{"table":"one"} {}`,
	} {
		var decoded TableCSVArchiveReviewRequest
		if err := json.Unmarshal([]byte(request), &decoded); err == nil {
			t.Fatal("malformed review accepted", request)
		}
	}
	for _, request := range []string{
		`{"table":"one","approvalHash":"a","destination":null}`,
		`{"table":"one","approvalHash":"a","destination":"\ud800"}`,
		`{"table":"one","approvalHash":"a","destination":"x","destination":"y"}`,
		`{"table":"one","approvalHash":"a","Destination":"x"}`,
		`{"table":"one","destination":"x"}`,
	} {
		var decoded TableCSVArchiveExportRequest
		if err := json.Unmarshal([]byte(request), &decoded); err == nil {
			t.Fatal("malformed export accepted", request)
		}
	}
	contexts, state, _ := ownedTableCSVPublicationFixture(t)
	service := NewTableCSVArchiveService(contexts, true)
	for _, table := range []string{"", "sample_Other", "Sample_SU", "Sample_Env;SELECT 1", "Sample_\x00Other"} {
		raw, err := json.Marshal(TableCSVArchiveReviewRequest{Table: table})
		if err != nil {
			t.Fatal(err)
		}
		if result, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, string(raw)); result != nil || err == nil {
			t.Fatal("arbitrary or malformed table accepted", table, result, err)
		}
	}
}

func TestTableCSVArchiveServiceReviewPublicationCollisionAndNoWrites(t *testing.T) {
	contexts, state, source := ownedTableCSVPublicationFixture(t)
	// The archive's independent gate does not require the read-only UI gate.
	contexts.tableCSVReviewEnabled = false
	service := NewTableCSVArchiveService(contexts, true)
	review, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, `{"table":"Sample_Other"}`)
	if err != nil {
		t.Fatal(err)
	}
	want, err := encodeOwnedTableCSVArchive(context.Background(), ownedTableCSVArchive{source.DescriptionMetadataPresent, source.Document})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(want)
	if review.ArchiveSHA256 != hex.EncodeToString(digest[:]) || review.ByteCount != len(want) ||
		review.Format != ownedTableCSVArchiveFormat || review.Version != 1 ||
		review.Review.ContextID != state.ContextID || review.Review.CSV != string(source.Document.Data) ||
		len(review.ApprovalHash) != 64 {
		t.Fatal("review does not describe exact artifact and owned source", review)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	configBefore := tableCSVPublicationRead(t, contexts.projects.preferences.path)
	destination := filepath.Join(t.TempDir(), "exact.zip")
	raw, err := json.Marshal(TableCSVArchiveExportRequest{source.Document.Manifest.Table, review.ApprovalHash, destination})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ExportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw))
	actualDestination, pathErr := filepath.EvalSymlinks(destination)
	if err != nil || result == nil || result.Status != "published" || result.SHA256 != review.ArchiveSHA256 ||
		result.RequestedDestination != destination || pathErr != nil || result.Path != actualDestination || result.ErrorMessage != "" ||
		!bytes.Equal(want, tableCSVPublicationRead(t, destination)) {
		t.Fatal("reviewed archive receipt differs", result, err)
	}
	collision, err := service.ExportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw))
	if err != nil || collision == nil || collision.Status != "not-published" || collision.ErrorMessage == "" ||
		!bytes.Equal(want, tableCSVPublicationRead(t, destination)) {
		t.Fatal("collision lost typed outcome or replaced artifact", collision, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if !bytes.Equal(configBefore, tableCSVPublicationRead(t, contexts.projects.preferences.path)) {
		t.Fatal("archive publication changed preferences")
	}
}

func TestTableCSVArchiveServiceApprovalBindsMetadataPresenceAndOwnership(t *testing.T) {
	contexts, state, _ := ownedTableCSVPublicationFixture(t)
	tableCSVProjectWriter(t, contexts, `DELETE FROM _table_metadata`)
	service := NewTableCSVArchiveService(contexts, true)
	review, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, `{"table":"Sample_Other"}`)
	if err != nil {
		t.Fatal(err)
	}
	tableCSVProjectWriter(t, contexts, `DROP TABLE _table_metadata`)
	fresh, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, `{"table":"Sample_Other"}`)
	if err != nil || fresh.ApprovalHash == review.ApprovalHash || fresh.ArchiveSHA256 == review.ArchiveSHA256 ||
		fresh.Review.Manifest.SHA256 != review.Review.Manifest.SHA256 {
		t.Fatal("metadata presence collapsed with unchanged CSV", fresh, err)
	}
	directory := t.TempDir()
	raw, err := json.Marshal(TableCSVArchiveExportRequest{"Sample_Other", review.ApprovalHash, filepath.Join(directory, "stale.zip")})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.ExportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw)); result != nil || err == nil {
		t.Fatal("stale approval published", result, err)
	}
	tableCSVPublicationOnly(t, directory)
	source, err := contexts.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
	if err != nil {
		t.Fatal(err)
	}
	original, err := tableCSVArchiveApprovalHash(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ownedTableCSVReview){
		func(r *ownedTableCSVReview) { r.ContextID += "-other" },
		func(r *ownedTableCSVReview) { r.Project += "-other" },
		func(r *ownedTableCSVReview) { r.ProjectPath += "-other" },
		func(r *ownedTableCSVReview) { r.Document.Manifest.Columns[0].DeclaredType += " " },
	} {
		changed := source
		changed.Document = snapshotTableCSVBundleDocument(source.Document)
		mutate(&changed)
		hash, err := tableCSVArchiveApprovalHash(context.Background(), changed)
		if err != nil || hash == original {
			t.Fatal("raw ownership/schema change not bound", hash, err)
		}
	}
}

func TestTableCSVArchiveServiceAllCoreTablesAndLiteralHashRefusal(t *testing.T) {
	contexts, state, _ := ownedTableCSVPublicationFixture(t)
	service := NewTableCSVArchiveService(contexts, true)
	for _, suffix := range coreTables {
		raw, err := json.Marshal(TableCSVArchiveReviewRequest{Table: state.ActiveProject + "_" + suffix})
		if err != nil {
			t.Fatal(err)
		}
		review, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, string(raw))
		if err != nil || review == nil || review.Review.Manifest.Table != state.ActiveProject+"_"+suffix {
			t.Fatal("literal core table review failed", suffix, err)
		}
	}
	directory := t.TempDir()
	for _, hash := range []string{"", strings.Repeat("A", 64), strings.Repeat("g", 64), strings.Repeat("0", 63), strings.Repeat("0", 64)} {
		raw, err := json.Marshal(TableCSVArchiveExportRequest{"Sample_Other", hash, filepath.Join(directory, "refused.zip")})
		if err != nil {
			t.Fatal(err)
		}
		if result, err := service.ExportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw)); result != nil || err == nil {
			t.Fatal("malformed or unapproved hash accepted", hash, result, err)
		}
	}
	tableCSVPublicationOnly(t, directory)
}

func TestTableCSVArchiveServiceRawSourceDriftAndStaleContext(t *testing.T) {
	for name, sql := range map[string]string{
		"rows":     `DELETE FROM Sample_Other`,
		"schema":   `ALTER TABLE Sample_Other ADD COLUMN Extra TEXT`,
		"metadata": `UPDATE _table_metadata SET description='changed' WHERE table_name='Sample_Other'`,
	} {
		t.Run(name, func(t *testing.T) {
			contexts, state, _ := ownedTableCSVPublicationFixture(t)
			service := NewTableCSVArchiveService(contexts, true)
			review, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, `{"table":"Sample_Other"}`)
			if err != nil {
				t.Fatal(err)
			}
			tableCSVProjectWriter(t, contexts, sql)
			directory := t.TempDir()
			raw, err := json.Marshal(TableCSVArchiveExportRequest{"Sample_Other", review.ApprovalHash, filepath.Join(directory, "refused.zip")})
			if err != nil {
				t.Fatal(err)
			}
			if result, err := service.ExportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw)); result != nil || err == nil {
				t.Fatal("changed raw source accepted", result, err)
			}
			if result, err := service.ExportReviewedTableCSVArchive(context.Background(), "stale-context", string(raw)); result != nil || err == nil {
				t.Fatal("stale ownership accepted", result, err)
			}
			tableCSVPublicationOnly(t, directory)
		})
	}
}

func TestTableCSVArchiveServicePrecommitCleanupReturnsTypedRefusalAndRetry(t *testing.T) {
	contexts, state, _ := ownedTableCSVPublicationFixture(t)
	service := NewTableCSVArchiveService(contexts, true)
	review, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, `{"table":"Sample_Other"}`)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	raw, err := json.Marshal(TableCSVArchiveExportRequest{"Sample_Other", review.ApprovalHash, filepath.Join(directory, "retry.zip")})
	if err != nil {
		t.Fatal(err)
	}
	fault := errors.New("initial snapshot cleanup failure")
	result, err := service.exportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw),
		tableCSVOwnedPublicationHooks{rollbackRead: func(*sql.Tx) error { return fault }})
	if err != nil || result == nil || result.Status != "not-published" || !strings.Contains(result.ErrorMessage, fault.Error()) {
		t.Fatal("precommit failure not explicit", result, err)
	}
	tableCSVPublicationOnly(t, directory)
	result, err = service.ExportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw))
	if err != nil || result == nil || result.Status != "published" {
		t.Fatal("cleanup failure poisoned retry", result, err)
	}
}

func TestTableCSVArchiveApprovalCancellationAndCallerDetachment(t *testing.T) {
	_, _, source := ownedTableCSVPublicationFixture(t)
	counter := tableCSVBundleCancellation(0)
	expected, err := tableCSVArchiveApprovalHash(counter, source)
	counter.cancel()
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= counter.calls; at++ {
		ctx := tableCSVBundleCancellation(at)
		actual, err := tableCSVArchiveApprovalHash(ctx, source)
		ctx.cancel()
		if !errors.Is(err, context.Canceled) || actual != "" {
			t.Fatalf("approval cancellation checkpoint %d returned partial hash: %v", at, err)
		}
	}
	changed := source
	changed.Document = snapshotTableCSVBundleDocument(source.Document)
	ctx := &tableCSVBundleMutationContext{Context: context.Background(), at: 1, mutate: func() {
		changed.Document.Data[0] = 'X'
		changed.Document.Manifest.Columns[0].Name = "changed"
		*changed.Document.Manifest.Descriptions[2].Value.Text = "changed"
	}}
	actual, err := tableCSVArchiveApprovalHash(ctx, changed)
	if err != nil || actual != expected {
		t.Fatal("approval hash kept caller aliases", actual, err)
	}
}
func TestTableCSVArchiveServiceCommittedReadCleanupWarning(t *testing.T) {
	contexts, state, _ := ownedTableCSVPublicationFixture(t)
	service := NewTableCSVArchiveService(contexts, true)
	review, err := service.GetTableCSVArchiveReview(context.Background(), state.ContextID, `{"table":"Sample_Other"}`)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "committed.zip")
	raw, err := json.Marshal(TableCSVArchiveExportRequest{"Sample_Other", review.ApprovalHash, destination})
	if err != nil {
		t.Fatal(err)
	}
	fault := errors.New("postcommit owned snapshot cleanup fault")
	reads := 0
	result, err := service.exportReviewedTableCSVArchive(context.Background(), state.ContextID, string(raw),
		tableCSVOwnedPublicationHooks{rollbackRead: func(*sql.Tx) error {
			reads++
			if reads == 3 {
				return fault
			}
			return nil
		}})
	actualDestination, pathErr := filepath.EvalSymlinks(destination)
	if err != nil || result == nil || result.Status != "published-with-errors" ||
		result.SHA256 != review.ArchiveSHA256 || !strings.Contains(result.ErrorMessage, fault.Error()) ||
		!strings.Contains(result.ErrorMessage, "do not replay") || pathErr != nil || result.Path != actualDestination ||
		result.RequestedDestination != destination {
		t.Fatal("committed warning became RPC failure", result, err)
	}
	data := tableCSVPublicationRead(t, destination)
	if _, err := decodeOwnedTableCSVArchive(context.Background(), data, int64(len(data))); err != nil {
		t.Fatal("committed artifact missing or corrupt", err)
	}
}
