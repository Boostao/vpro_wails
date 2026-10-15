package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func subzoneMatrixServiceFixture(t *testing.T) (*SubzoneMatrixService, ProjectState) {
	t.Helper()
	contexts, state := subzoneMatrixFixture(t)
	service, err := NewSubzoneMatrixService(contexts, func(name string) (string, bool) {
		if name != subzoneMatrixFeatureEnvironment {
			t.Fatal("Subzone Matrix borrowed another workflow gate", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func subzoneMatrixExportJSON(t *testing.T, approval, destination string) string {
	t.Helper()
	return workbookRequest(t, map[string]any{"approvalHash": approval, "destination": destination})
}

func TestSubzoneMatrixServiceOwnedPagingReviewPublicationAndCollision(t *testing.T) {
	service, state := subzoneMatrixServiceFixture(t)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	config, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.GetPreview(context.Background(), state.ContextID, `{"offset":0,"limit":1}`)
	if err != nil || first == nil || first.Total != 3 || first.Offset != 0 || first.Limit != 1 ||
		len(first.Rows) != 1 || first.Rows[0].Name != nil || first.Rows[0].Values[0] != nil ||
		first.HierarchyPath != state.HierarchyPath || first.LabelsPath != service.contexts.projects.sqlite.attachments["VPro64"] {
		t.Fatal("NULL-first physical page/scope differs", first, err)
	}
	second, err := service.GetPreview(context.Background(), state.ContextID, `{"offset":1,"limit":1}`)
	if err != nil || second == nil || len(second.Rows) != 1 || *second.Rows[0].Name != "Association Alpha" ||
		*second.Rows[0].Values[0] != "01" || second.SourceSHA256 != first.SourceSHA256 {
		t.Fatal("literal paged row/source identity differs", second, err)
	}
	last, err := service.GetPreview(context.Background(), state.ContextID, `{"offset":9223372036854775807,"limit":100}`)
	if err != nil || last == nil || len(last.Rows) != 0 || last.Rows == nil || last.Total != 3 {
		t.Fatal("out-of-range page overflowed or invented rows", last, err)
	}
	review, err := service.GetReview(context.Background(), state.ContextID)
	if err != nil || review == nil || review.Bytes <= 0 || review.Preview.Total != 3 ||
		review.Preview.SourceSHA256 != first.SourceSHA256 || len(review.ApprovalHash) != 64 {
		t.Fatal("owned workbook review differs", review, err)
	}
	again, err := service.GetReview(context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(review, again) {
		t.Fatal("unchanged source review is nondeterministic", err)
	}
	wire, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SubzoneMatrixWorkbookReview
	decoder := json.NewDecoder(bytes.NewReader(wire))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(review, &decoded) ||
		!bytes.Contains(wire, []byte(`"sourceSHA256":`)) || !bytes.Contains(wire, []byte(`"name":null,"values":[null,null,null]`)) ||
		bytes.Contains(wire, []byte(`"HierarchyOriginal"`)) {
		t.Fatal("production wire repaired NULLs/exposed originals/changed shape", string(wire), err)
	}
	destination := filepath.Join(t.TempDir(), "literal # é.xlsx")
	request := subzoneMatrixExportJSON(t, review.ApprovalHash, destination)
	outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request)
	if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
		t.Fatal("reviewed workbook did not publish", outcome, err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || len(data) != review.Bytes {
		t.Fatal("published bytes missing", err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != review.WorkbookSHA256 {
		t.Fatal("published bytes differ from reviewed content")
	}
	collision, err := service.ExportReviewed(context.Background(), state.ContextID, request)
	if err != nil || collision == nil || collision.Status != "not-published" || collision.ErrorMessage == "" {
		t.Fatal("destination collision overwritten or hidden", collision, err)
	}
	after, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("collision changed workbook", err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
		t.Fatal("report preparation/publication changed owned database family")
	}
	afterConfig, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(config, afterConfig) {
		t.Fatal("report changed configuration", err)
	}
}

func TestSubzoneMatrixServiceGateStrictRequestsAndCancellation(t *testing.T) {
	for _, gate := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSubzoneMatrixService(nil, func(string) (string, bool) { return gate, gate != "" })
		if gate == "" || gate == "false" || gate == "true" {
			if err != nil || service.enabled != (gate == "true") {
				t.Fatal("independent literal flag differs", gate, err)
			}
			if preview, err := service.GetPreview(context.Background(), "owned", `{"offset":0,"limit":25}`); err == nil || preview != nil {
				t.Fatal("disabled/unowned service returned data")
			}
			if review, err := service.GetReview(context.Background(), "owned"); err == nil || review != nil {
				t.Fatal("disabled/unowned service returned workbook")
			}
			if outcome, err := service.ExportReviewed(context.Background(), "owned", `{}`); err == nil || outcome != nil {
				t.Fatal("disabled/unowned service published")
			}
		} else if err == nil {
			t.Fatal("malformed flag accepted", gate)
		}
	}
	service, state := subzoneMatrixServiceFixture(t)
	for _, request := range []string{
		`{}`, `null`, `{"offset":null,"limit":25}`, `{"offset":0,"limit":null}`,
		`{"offset":-1,"limit":25}`, `{"offset":0,"limit":0}`, `{"offset":0,"limit":101}`,
		`{"offset":0.5,"limit":25}`, `{"offset":"0","limit":25}`,
		`{"offset":0,"limit":25,"unknown":true}`, `{"Offset":0,"limit":25}`,
		`{"offset":0,"offset":1,"limit":25}`, `{"offset":0,"\u006fffset":1,"limit":25}`,
		`{"offset":0,"limit":25} {}`, `{"offset":0,"limit":25,"unknown":"\ud800"}`,
	} {
		if preview, err := service.GetPreview(context.Background(), state.ContextID, request); err == nil || preview != nil {
			t.Fatal("raw request defaulted/repaired", request, preview, err)
		}
	}
	review, err := service.GetReview(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []string{
		`{}`, `{"approvalHash":null,"destination":"bad.xlsx"}`,
		subzoneMatrixExportJSON(t, "BAD", filepath.Join(t.TempDir(), "bad.xlsx")),
		subzoneMatrixExportJSON(t, review.ApprovalHash, filepath.Join(t.TempDir(), "bad.db")),
		`{"approvalHash":"` + review.ApprovalHash + `","destination":"\ud800.xlsx"}`,
		`{"approvalHash":"` + review.ApprovalHash + `","destination":"a.xlsx","destination":"b.xlsx"}`,
		`{"approvalHash":"` + review.ApprovalHash + `","destination":"a.xlsx","\u0064estination":"b.xlsx"}`,
	} {
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err == nil || outcome != nil {
			t.Fatal("raw export request repaired/published", request, outcome, err)
		}
	}
	if _, err := service.GetReview(nil, state.ContextID); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetReview(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled review returned data", err)
	}
	if _, err := service.GetPreview(ctx, state.ContextID, `{"offset":0,"limit":25}`); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled page returned data", err)
	}
	if _, err := service.ExportReviewed(ctx, state.ContextID, `{}`); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled export continued", err)
	}
}

func TestSubzoneMatrixServiceSnapshotFailuresAndIrreversibleReceipts(t *testing.T) {
	for _, phase := range []string{"first-commit", "first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel", "remove-stage"} {
		t.Run(phase, func(t *testing.T) {
			service, state := subzoneMatrixServiceFixture(t)
			review, err := service.GetReview(context.Background(), state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "matrix.xlsx")
			request := subzoneMatrixExportJSON(t, review.ApprovalHash, destination)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rejected, reads := errors.New("matrix snapshot cleanup refused"), 0
			hooks := subzoneMatrixPublicationHooks{
				snapshot: publicationReadSnapshotHooks{
					commitRead: func(*sql.Tx) error {
						if phase == "first-commit" {
							return rejected
						}
						return nil
					},
					rollbackRead: func(*sql.Tx) error {
						reads++
						if phase == "first-cleanup" && reads == 1 || phase == "prelink-cleanup" && reads == 2 ||
							phase == "published-cleanup" && reads == 3 {
							return rejected
						}
						return nil
					},
				},
				publication: artifactPublicationHooks{observe: func(current, _ string) error {
					if phase == "prelink-cancel" && current == "prelink" || phase == "published-cancel" && current == "published" {
						cancel()
					}
					return nil
				}},
			}
			if phase == "remove-stage" {
				hooks.publication.remove = func(string) error { return rejected }
			}
			outcome, err := service.exportReviewed(ctx, state.ContextID, request, hooks)
			switch phase {
			case "first-commit", "first-cleanup":
				if !errors.Is(err, rejected) || outcome != nil {
					t.Fatal("failed initial observation invented receipt", outcome, err)
				}
			case "prelink-cleanup", "prelink-cancel":
				if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
					t.Fatal("known no-publication result hidden", outcome, err)
				}
			default:
				if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
					outcome.SHA256 != review.WorkbookSHA256 || !strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("irreversible publication lost receipt/replay guard", outcome, err)
				}
			}
			published := strings.HasPrefix(phase, "published-") || phase == "remove-stage"
			if published {
				data, err := os.ReadFile(destination)
				if err != nil || len(data) != review.Bytes {
					t.Fatal("committed workbook absent", err)
				}
			} else {
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed export left a workbook", err)
				}
				retry, err := service.ExportReviewed(context.Background(), state.ContextID, request)
				if err != nil || retry == nil || retry.Status != "published" {
					t.Fatal("failed operation leaked lease/snapshot or prevented retry", retry, err)
				}
			}
		})
	}
}

func TestSubzoneMatrixServiceCompleteOriginalDriftAndRetiredOwner(t *testing.T) {
	service, state := subzoneMatrixServiceFixture(t)
	review, err := service.GetReview(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}

	mutateContextFixture(t, state.HierarchyPath, `UPDATE Sample_Hierarchy SET Tag=X'ABCD' WHERE ID=1`)
	destination := filepath.Join(t.TempDir(), "matrix.xlsx")
	if outcome, err := service.ExportReviewed(context.Background(), state.ContextID,
		subzoneMatrixExportJSON(t, review.ApprovalHash, destination)); err == nil || outcome != nil || !strings.Contains(err.Error(), "original source") {
		t.Fatal("unused original-column drift accepted", outcome, err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source drift refusal created output", err)
	}
	next, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetReview(context.Background(), state.ContextID); err == nil {
		t.Fatal("retired context returned review")
	}
	if _, err := service.GetPreview(context.Background(), state.ContextID, `{"offset":0,"limit":25}`); err == nil {
		t.Fatal("retired context returned page")
	}
	if _, err := service.GetReview(context.Background(), next.ContextID); err != nil {
		t.Fatal("fresh context could not recover", err)
	}
}

func TestSubzoneMatrixServiceSourceDriftAtPublicationBoundary(t *testing.T) {
	for _, phase := range []string{"prelink", "published"} {
		t.Run(phase, func(t *testing.T) {
			service, state := subzoneMatrixServiceFixture(t)
			review, err := service.GetReview(context.Background(), state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "matrix.xlsx")
			observed := false
			outcome, err := service.exportReviewed(context.Background(), state.ContextID,
				subzoneMatrixExportJSON(t, review.ApprovalHash, destination),
				subzoneMatrixPublicationHooks{publication: artifactPublicationHooks{observe: func(current, _ string) error {
					if current == phase {
						observed = true
						mutateContextFixture(t, service.contexts.projects.sqlite.attachments["VPro64"],
							`UPDATE USysSubzoneMatrixLabels SET Len=42 WHERE rowid=1`)
					}
					return nil
				}}})
			if !observed || err != nil || outcome == nil || !strings.Contains(outcome.ErrorMessage, "original source changed") {
				t.Fatal("unrendered source drift not surfaced at publication boundary", observed, outcome, err)
			}
			if phase == "prelink" {
				if outcome.Status != "not-published" {
					t.Fatal("changed source published", outcome)
				}
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("prelink drift left output", err)
				}
			} else {
				if outcome.Status != "published-with-errors" || !strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("postcommit drift lost irreversible receipt", outcome)
				}
				data, err := os.ReadFile(destination)
				digest := sha256.Sum256(data)
				if err != nil || hex.EncodeToString(digest[:]) != review.WorkbookSHA256 {
					t.Fatal("postcommit drift changed committed bytes", err)
				}
			}
			staging, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".vpro-subzone-matrix-*"))
			if err != nil || len(staging) != 0 {
				t.Fatal("source drift leaked output staging", staging, err)
			}
		})
	}
}

func TestSubzoneMatrixServicePreviewIndependentFromWorkbookAndDTOOwnership(t *testing.T) {
	service, state := subzoneMatrixServiceFixture(t)
	mutateContextFixture(t, state.HierarchyPath, `UPDATE Sample_Hierarchy SET Name='AAAAAAAAA1bad'||char(1)||'text' WHERE ID=11`)
	preview, err := service.GetPreview(context.Background(), state.ContextID, `{"offset":1,"limit":100}`)
	if err != nil || preview == nil || *preview.Rows[0].Values[0] != "bad\x01text" {
		t.Fatal("ordinary preview unnecessarily serialized XLSX", preview, err)
	}
	if review, err := service.GetReview(context.Background(), state.ContextID); err == nil || review != nil {
		t.Fatal("XLSX-invalid source was published as workbook review", review, err)
	}
	source, err := service.contexts.readSubzoneMatrix(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	page, err := subzoneMatrixPage(source, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	page.Columns[0] = "changed"
	*page.Rows[0].Name = "changed"
	*page.Rows[0].Values[0] = "changed"
	if source.Matrix.Columns[0] != "AAAAAAAAA" || *source.Matrix.Rows[1].Name != "Association Alpha" ||
		*source.Matrix.Rows[1].Values[0] != "bad\x01text" {
		t.Fatal("transport DTO mutated complete retained source")
	}
	mutateContextFixture(t, state.HierarchyPath, `
			UPDATE Sample_Hierarchy SET Name='AAAAAAAAA101' WHERE ID=11;
			WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<101)
			INSERT INTO Sample_Hierarchy(ID,Name,Parent,Level)
			SELECT 100+x,printf('Extra %03d',x),NULL,10 FROM n;
			WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<101)
			INSERT INTO Sample_Hierarchy(ID,Name,Parent,Level)
			SELECT 1000+x,printf('AAAAAAAAA1%03d',x),100+x,11 FROM n;`)
	maximum, err := service.GetPreview(context.Background(), state.ContextID, `{"offset":0,"limit":100}`)
	if err != nil || maximum == nil || maximum.Total != 104 || len(maximum.Rows) != 100 {
		t.Fatal("exact maximum page limit differs", maximum, err)
	}
	tail, err := service.GetPreview(context.Background(), state.ContextID, `{"offset":100,"limit":100}`)
	if err != nil || tail == nil || len(tail.Rows) != 4 || tail.Total != 104 {
		t.Fatal("maximum-limit final page dropped/duplicated rows", tail, err)
	}
}
