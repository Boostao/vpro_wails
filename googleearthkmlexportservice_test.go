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

func TestGoogleEarthKMLExportIndependentGateAndUnavailableAuthorization(t *testing.T) {
	for _, value := range []string{"", "true", "false", "TRUE", "1", " true", "true "} {
		enabled, err := googleEarthKMLExportFeature(func(key string) (string, bool) {
			if key == googleEarthKMLExportFeatureEnvironment {
				return value, value != ""
			}
			t.Fatal("export flag inherited another environment key", key)
			return "true", true
		})
		if enabled != (value == "true") || (err != nil) != (value != "" && value != "true" && value != "false") {
			t.Fatal("literal gate", value, enabled, err)
		}
	}
	for _, service := range []*GoogleEarthKMLExportService{nil, NewGoogleEarthKMLExportService(nil, false), NewGoogleEarthKMLExportService(nil, true)} {
		if value, err := service.GetGoogleEarthKMLExportReview(context.Background(), "context", "malformed"); value != nil || err == nil {
			t.Fatal("unavailable review authorization", value, err)
		}
		if value, err := service.ExportReviewedGoogleEarthKML(context.Background(), "context", "malformed"); value != nil || err == nil {
			t.Fatal("unavailable publication authorization", value, err)
		}
	}
	service := NewGoogleEarthKMLExportService(nil, true)
	if value, err := service.GetGoogleEarthKMLExportReview(nil, "", "{}"); value != nil || err == nil {
		t.Fatal("nil request context", value, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := service.ExportReviewedGoogleEarthKML(ctx, "", "{}"); value != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("authorization cancellation", value, err)
	}
}

func TestGoogleEarthKMLExportReviewHashBindsRawSourceAndPublishesOnce(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewGoogleEarthKMLExportService(contexts, true)
	request := `{"descriptionField":"Zone","title":" Literal <&\r\n "}`
	review, err := service.GetGoogleEarthKMLExportReview(context.Background(), state.ContextID, request)
	if err != nil || review == nil || len(review.ApprovalHash) != 64 {
		t.Fatal("export source review", err)
	}
	data := []byte(review.Review.KML)
	hash := sha256.Sum256(data)
	if review.KMLSHA256 != hex.EncodeToString(hash[:]) {
		t.Fatal("exact KML byte hash differs")
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	config := tableCSVPublicationRead(t, contexts.projects.preferences.path)
	path := filepath.Join(t.TempDir(), "literal output")
	payload, err := json.Marshal(GoogleEarthKMLExportRequest{
		DescriptionField: "Zone", Title: review.Review.Title, ApprovalHash: review.ApprovalHash, Destination: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := service.ExportReviewedGoogleEarthKML(context.Background(), state.ContextID, string(payload))
	if err != nil || outcome == nil || outcome.Status != "published" || outcome.RequestedDestination != path ||
		outcome.SHA256 != review.KMLSHA256 || outcome.ErrorMessage != "" || !bytes.Equal(tableCSVPublicationRead(t, path), data) {
		t.Fatal("typed committed publication", outcome, err)
	}
	retry, err := service.ExportReviewedGoogleEarthKML(context.Background(), state.ContextID, string(payload))
	if err != nil || retry == nil || retry.Status != "not-published" || retry.ErrorMessage == "" || !bytes.Equal(tableCSVPublicationRead(t, path), data) {
		t.Fatal("no-replace failure hidden or replaced", retry, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if !bytes.Equal(config, tableCSVPublicationRead(t, contexts.projects.preferences.path)) {
		t.Fatal("export wrote preferences")
	}
	mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Zone=''`)
	after, err := service.GetGoogleEarthKMLExportReview(context.Background(), state.ContextID, request)
	if err != nil || after.ApprovalHash == review.ApprovalHash || after.KMLSHA256 != review.KMLSHA256 || after.Review.KML != review.Review.KML {
		t.Fatal("same XML raw NULL/empty source drift not bound", after, err)
	}
	directory := t.TempDir()
	var changed GoogleEarthKMLExportRequest
	if err := json.Unmarshal(payload, &changed); err != nil {
		t.Fatal(err)
	}
	changed.Destination = filepath.Join(directory, "old-approval.kml")
	stale, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := service.ExportReviewedGoogleEarthKML(context.Background(), state.ContextID, string(stale)); outcome != nil || err == nil || !strings.Contains(err.Error(), "raw source") {
		t.Fatal("stale raw approval allowed", outcome, err)
	}
	tableCSVPublicationOnly(t, directory)
}

func TestGoogleEarthKMLExportStrictRawRequestsRejectRepairAliasesAndDuplicates(t *testing.T) {
	for _, request := range []string{
		`{}`, `null`, `[]`, `{"descriptionField":"Zone"}`, `{"descriptionField":"Zone","title":null}`,
		`{"descriptionField":"Zone","title":"\ud800"}`, "{\"descriptionField\":\"Zone\",\"title\":\"\xff\"}",
		`{"descriptionField":"Zone","title":"","title":"again"}`,
		`{"descriptionField":"Zone","title":"","\u0074itle":"again"}`,
		`{"descriptionField":"Zone","title":"","Title":"again"}`,
		`{"descriptionField":"Zone","title":"","extra":""}`,
		`{"descriptionField":"Zone","title":""} {}`,
	} {
		var decoded GoogleEarthKMLExportReviewRequest
		if err := json.Unmarshal([]byte(request), &decoded); err == nil {
			t.Fatal("malformed export review accepted", request)
		}
	}
	for _, request := range []string{
		`{"descriptionField":"Zone","title":"","approvalHash":"","destination":"literal","destination":"other"}`,
		`{"descriptionField":"Zone","title":"","approvalHash":"","destination":"\ud800"}`,
		`{"descriptionField":"Zone","title":"","approvalHash":"","destination":null}`,
		`{"descriptionField":"Zone","title":"","approvalHash":"","Destination":"literal"}`,
	} {
		var decoded GoogleEarthKMLExportRequest
		if err := json.Unmarshal([]byte(request), &decoded); err == nil {
			t.Fatal("malformed publication request accepted", request)
		}
	}
}

func TestGoogleEarthKMLExportExplicitCommittedErrorsNeverBecomeRPCFailure(t *testing.T) {
	rejected := errors.New("postcommit read cleanup failed")
	for _, published := range []bool{false, true} {
		result := artifactPublication{Path: "actual", SHA256: strings.Repeat("a", 64), Published: published}
		outcome := googleEarthKMLExportOutcome("requested", result, rejected)
		status := "not-published"
		if published {
			status = "published-with-errors"
		}
		if outcome.Status != status || outcome.Path != result.Path || outcome.SHA256 != result.SHA256 ||
			outcome.RequestedDestination != "requested" || outcome.ErrorMessage != rejected.Error() {
			t.Fatal("explicit irreversible receipt lost", outcome)
		}
	}
	if outcome := googleEarthKMLExportOutcome("requested", artifactPublication{}, nil); outcome.Status != "not-published" || outcome.ErrorMessage == "" {
		t.Fatal("no artifact became a success-shaped receipt", outcome)
	}
}

func TestGoogleEarthKMLExportFacadeReturnsActualCommittedReadCleanupFailure(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewGoogleEarthKMLExportService(contexts, true)
	review, err := service.GetGoogleEarthKMLExportReview(context.Background(), state.ContextID, `{"descriptionField":"Zone","title":""}`)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "committed.kml")
	payload, err := json.Marshal(GoogleEarthKMLExportRequest{
		DescriptionField: "Zone", Title: "", ApprovalHash: review.ApprovalHash, Destination: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("actual postcommit owned read cleanup failure")
	reads := 0
	outcome, err := service.exportReviewedGoogleEarthKML(context.Background(), state.ContextID, string(payload),
		googleEarthKMLOwnedPublicationHooks{snapshot: publicationReadSnapshotHooks{rollbackRead: func(*sql.Tx) error {
			reads++
			if reads == 3 {
				return rejected
			}
			return nil
		}}})
	if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
		outcome.SHA256 != review.KMLSHA256 || !strings.Contains(outcome.ErrorMessage, rejected.Error()) ||
		!strings.Contains(outcome.ErrorMessage, "do not replay publication") ||
		!bytes.Equal(tableCSVPublicationRead(t, path), []byte(review.Review.KML)) {
		t.Fatal("actual irreversible facade result became missing/RPC error", outcome, err)
	}
}
