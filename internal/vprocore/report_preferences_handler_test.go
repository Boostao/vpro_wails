package vprocore_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

type samplePrefRequest struct {
	Expected string `json:"expected"`
	Proposed string `json:"proposed"`
}

func (r *samplePrefRequest) UnmarshalJSON(data []byte) error {
	type plain samplePrefRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "sample preference save", "expected", "proposed"); err != nil {
		return err
	}
	*r = samplePrefRequest(decoded)
	return nil
}

func TestReportPreferenceHandlerGuardsSnapshotAndPostCommitReceipt(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	for _, reqCtx := range []context.Context{nil, context.Background(), cancelled} {
		if got, err := vprocore.GetOwnedReportPreferences(reqCtx, vprocore.ReportPreferenceHelper{}, "ctx", "Sample",
			func(*vprocore.PlotService, *vprocore.SqliteContext, *sql.Tx) (*string, error) {
				v := "unexpected"
				return &v, nil
			}); got != nil || err == nil {
			t.Fatalf("expected unready GetOwnedReportPreferences rejection, got %v %v", got, err)
		}
		out := vprocore.SaveOwnedReportPreferences(reqCtx, vprocore.ReportPreferenceHelper{}, "ctx", `{"expected":"a","proposed":"b"}`, "Sample", nil,
			func(*vprocore.PlotService, samplePrefRequest) (vprocore.ReportPreferenceUpdate, error) {
				return vprocore.ReportPreferenceUpdate{Changed: true, Committed: true}, nil
			})
		if out.Changed || out.Committed || out.ErrorMessage == "" {
			t.Fatalf("expected unready SaveOwnedReportPreferences rejection, got %+v", out)
		}
	}

	contexts, state := vprotest.PrepareKMLContextFixture(t)
	helper := vprocore.ReportPreferenceHelper{Contexts: contexts}

	// Malformed UTF-8 contextID is rejected before touching SQLite.
	if _, err := vprocore.GetOwnedReportPreferences(context.Background(), helper, string([]byte{0xff}), "Sample",
		func(*vprocore.PlotService, *vprocore.SqliteContext, *sql.Tx) (*string, error) {
			v := "unexpected"
			return &v, nil
		}); err == nil {
		t.Fatal("expected invalid UTF-8 contextID rejection")
	}

	// Snapshot failure blocks apply.
	applied := false
	failHelper := vprocore.ReportPreferenceHelper{
		Contexts: contexts,
		Snapshot: vprocore.PublicationReadSnapshotHooks{
			CommitRead: func(*sql.Tx) error { return errors.New("snapshot commit failed") },
		},
	}
	blocked := vprocore.SaveOwnedReportPreferences(context.Background(), failHelper, state.ContextID, `{"expected":"a","proposed":"b"}`, "Sample", nil,
		func(*vprocore.PlotService, samplePrefRequest) (vprocore.ReportPreferenceUpdate, error) {
			applied = true
			return vprocore.ReportPreferenceUpdate{Changed: true, Committed: true}, nil
		})
	if applied || blocked.Committed || !strings.Contains(blocked.ErrorMessage, "snapshot commit failed") {
		t.Fatalf("snapshot error did not block apply: applied=%v outcome=%+v", applied, blocked)
	}

	// Post-commit failure retains Changed/Committed receipt alongside ErrorMessage.
	postCommit := vprocore.SaveOwnedReportPreferences(context.Background(), helper, state.ContextID, `{"expected":"a","proposed":"b"}`, "Sample", nil,
		func(*vprocore.PlotService, samplePrefRequest) (vprocore.ReportPreferenceUpdate, error) {
			return vprocore.ReportPreferenceUpdate{Changed: true, Committed: true}, errors.New("post-commit refresh failed")
		})
	if !postCommit.Changed || !postCommit.Committed || !strings.Contains(postCommit.ErrorMessage, "post-commit refresh failed") {
		t.Fatalf("post-commit failure lost committed receipt: %+v", postCommit)
	}
}
