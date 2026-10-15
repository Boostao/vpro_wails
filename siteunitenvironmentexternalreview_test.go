package main

import (
	"database/sql"
	"errors"
	"testing"
)

func TestExternalSUEnvironmentServiceRetainsOneCurrentImmutableApproval(t *testing.T) {
	service, state, request, raw := externalSUEnvironmentServiceFixture(t)
	for i := 0; i < 8; i++ {
		review, err := service.GetReview(t.Context(), state.ContextID)
		if err != nil || review.ApprovalHash != request.ApprovalHash || len(service.reviews) != 1 {
			t.Fatal("unchanged source created unbounded or different review ownership", review, err)
		}
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(service.contexts.projects.sqlite.attachments["su"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := db.Exec(`UPDATE External_SU SET PlotNumber='Changed foreign peer' WHERE PlotNumber='Foreign'`)
	if err := errors.Join(writeErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	review, err := service.GetReview(t.Context(), state.ContextID)
	if err != nil || review.ApprovalHash == request.ApprovalHash || len(service.reviews) != 1 {
		t.Fatal("changed source did not replace current approval explicitly", review, err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if result, err := service.TransferReviewed(t.Context(), state.ContextID, raw); result != nil || err == nil {
		t.Fatal("retired source review retained write authority", result, err)
	}
	assertProfileSUFiles(t, service.contexts, before)
}
