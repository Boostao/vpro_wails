package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestSummaryVegetationFieldListSaveWireCommitReplayAndDrift(t *testing.T) {
	service, id := summaryFieldListOwnedFixture(t)
	review, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	request := SummaryVegetationFieldListRequest{
		RequestID: "2e6b14c0-77b2-49e7-8fa5-9d1b4bb6a854", Review: *review,
		Actions: []SummaryVegetationFieldListAction{{Kind: "all", Selected: true}},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	committed := service.SaveSummaryVegetationFieldList(t.Context(), id, string(raw))
	if !committed.Changed || !committed.Committed || committed.Replayed || !committed.Verified ||
		committed.ErrorMessage != "" || committed.Plan == nil || len(committed.Plan.Assignments) != 3 ||
		committed.ContextID != id || committed.RequestID != request.RequestID {
		t.Fatal("Save lost exact newly committed plan", committed)
	}
	wire, err := json.Marshal(committed)
	var fields map[string]json.RawMessage
	if err != nil || json.Unmarshal(wire, &fields) != nil || len(fields) != 8 {
		t.Fatal("outcome wire shape differs", string(wire), err)
	}
	for _, key := range []string{"contextId", "requestId", "changed", "committed", "replayed", "verified", "plan", "errorMessage"} {
		if fields[key] == nil {
			t.Fatal("missing actual outcome wire field", key)
		}
	}
	replay := service.SaveSummaryVegetationFieldList(t.Context(), id, string(raw))
	if !replay.Committed || !replay.Replayed || !replay.Verified || replay.ErrorMessage != "" ||
		!reflect.DeepEqual(replay.Plan, committed.Plan) {
		t.Fatal("historical committed receipt lost on verified replay", replay)
	}
	path := review.SourcePath
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, updateErr := db.Exec(`UPDATE USysShortVegSummaryFieldList SET Source='external drift' WHERE rowid=1`)
	if err := errors.Join(updateErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	drift := service.SaveSummaryVegetationFieldList(t.Context(), id, string(raw))
	if !drift.Committed || !drift.Changed || !drift.Replayed || drift.Verified || drift.ErrorMessage == "" ||
		!reflect.DeepEqual(drift.Plan, committed.Plan) {
		t.Fatal("historical committed result erased by later drift error", drift)
	}
}

func TestSummaryVegetationFieldListSaveNoopAndCommittedCleanupError(t *testing.T) {
	for _, cleanupFailure := range []bool{false, true} {
		service, id := summaryFieldListOwnedFixture(t)
		review, err := service.GetSummaryVegetationFieldList(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		request := SummaryVegetationFieldListRequest{
			RequestID: "2e6b14c0-77b2-49e7-8fa5-9d1b4bb6a854", Review: *review,
			Actions: []SummaryVegetationFieldListAction{},
		}
		hooks := summaryFieldListWriterHooks{}
		if cleanupFailure {
			request.Actions = []SummaryVegetationFieldListAction{{Kind: "none"}}
			hooks.afterClose = func() error { return errors.New("actual close completed; injected cleanup error") }
		}
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		result := service.saveSummaryVegetationFieldList(t.Context(), id, string(raw), hooks)
		if result.Plan == nil || !result.Verified || result.Replayed {
			t.Fatal("verified result plan lost", result)
		}
		if cleanupFailure {
			if !result.Committed || !result.Changed || result.ErrorMessage == "" || len(result.Plan.Assignments) != 3 {
				t.Fatal("committed cleanup error became success-shaped or zeroed", result)
			}
		} else if result.Committed || result.Changed || result.ErrorMessage != "" || len(result.Plan.Assignments) != 0 ||
			!reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
			t.Fatal("no-op wrote receipt or mutated physical data", result)
		}
	}
}

func TestSummaryVegetationFieldListSaveGateContextAndStrictRefusal(t *testing.T) {
	service, id := summaryFieldListOwnedFixture(t)
	review, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	request := SummaryVegetationFieldListRequest{
		RequestID: "2e6b14c0-77b2-49e7-8fa5-9d1b4bb6a854", Review: *review,
		Actions: []SummaryVegetationFieldListAction{{Kind: "all", Selected: true}},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	for _, call := range []func() *SummaryVegetationFieldListOutcome{
		func() *SummaryVegetationFieldListOutcome {
			return service.SaveSummaryVegetationFieldList(t.Context(), "foreign", string(raw))
		},
		func() *SummaryVegetationFieldListOutcome {
			return service.SaveSummaryVegetationFieldList(t.Context(), id, `{}`)
		},
		func() *SummaryVegetationFieldListOutcome {
			return NewSummaryVegetationFieldListService(service.contexts, false).SaveSummaryVegetationFieldList(t.Context(), id, string(raw))
		},
		func() *SummaryVegetationFieldListOutcome {
			return (*SummaryVegetationFieldListService)(nil).SaveSummaryVegetationFieldList(t.Context(), id, string(raw))
		},
		func() *SummaryVegetationFieldListOutcome {
			return service.SaveSummaryVegetationFieldList(nil, id, string(raw))
		},
	} {
		result := call()
		if result.Committed || result.Changed || result.Replayed || result.Verified || result.Plan != nil || result.ErrorMessage == "" {
			t.Fatal("refused Save returned partial/committed authority", result)
		}
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
		t.Fatal("rejected Save mutated source/history")
	}
}
