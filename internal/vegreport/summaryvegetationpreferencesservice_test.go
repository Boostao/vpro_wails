package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func summaryPreferencesRequest(t *testing.T, expected, proposed SummaryVegetationPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(SummaryVegetationPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSummaryVegetationPreferencesFacadeGatesStrictWire(t *testing.T) {
	for _, service := range []*SummaryVegetationPreferencesService{nil, {}, NewSummaryVegetationPreferencesService(nil)} {
		for _, ctx := range []context.Context{nil, context.Background()} {
			if got, err := service.GetSummaryVegetationPreferences(ctx, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveSummaryVegetationPreferences(ctx, "id", "{}"); got.Committed || got.Changed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewSummaryVegetationPreferencesService(contexts)
	base := SummaryVegetationPreferenceValues{GroupBy: 1, Average: 1, Display: 1, ShowClass: 1}
	valid := summaryPreferencesRequest(t, base, base)
	before, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []string{`{}`, `null`, `[]`,
		`{"Expected":{},"proposed":{}}`,
		`{"expected":{},"expected":{},"proposed":{}}`,
		`{"expected":null,"proposed":null}`,
		strings.Replace(valid, `"groupBy":1`, `"GroupBy":1`, 1),
		strings.Replace(valid, `"groupBy":1`, `"groupBy":1,"\u0067roupBy":1`, 1),
		strings.Replace(valid, `"groupBy":1`, `"groupBy":1,"unknown":1`, 1),
		strings.Replace(valid, `"groupBy":1,`, ``, 1),
		valid[:len(valid)-1] + `,"\ud800":0}`,
		valid[:len(valid)-1] + `,"` + string([]byte{0xff}) + `":0}`,
	}
	for _, field := range []string{"groupBy", "average", "display", "showClass"} {
		for _, value := range []string{`null`, `"1"`, `true`, `1.0`, `1e0`, `0`, `-1`, `2147483648`} {
			invalid = append(invalid, strings.Replace(valid, `"`+field+`":1`, `"`+field+`":`+value, 1))
		}
	}
	for _, value := range []string{`null`, `"0"`, `true`, `1e1000`} {
		invalid = append(invalid, strings.Replace(valid, `"valueLimit":0`, `"valueLimit":`+value, 1))
	}
	for _, raw := range invalid {
		if got := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, raw); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(raw, got)
		}
	}
	after, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("invalid wire wrote YAML", err)
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetSummaryVegetationPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveSummaryVegetationPreferences(context.Background(), id, valid); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
}

func TestSummaryVegetationPreferencesFacadeOwnedTransitionsAndAvailability(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	protected := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	service := NewSummaryVegetationPreferencesService(contexts)
	for _, group := range []int{1, 2, 3, 4} {
		for _, display := range []int{1, 2, 3, 4, 5, 6} {
			original := SummaryVegetationPreferenceValues{GroupBy: group, Average: 1, Display: display, ShowClass: 10, ValueLimit: 3.5}
			if err := store.Update("ReportOptions", map[string]any{"SVGroupBy": group, "SVAvgType": 1, "SVValue": display, "SVShowClass": 10, "SVShowValOpt": 3.5}); err != nil {
				t.Fatal(err)
			}
			read, err := service.GetSummaryVegetationPreferences(context.Background(), state.ContextID)
			available := !((group == 2 || group == 3) && display == 2)
			if err != nil || read.Values != original || !read.PreviewEnabled || read.SelectionAvailable != available ||
				read.ContextID != state.ContextID || read.ProjectPath != state.ProjectPath || read.SUPath != state.SUPath {
				t.Fatal(read, err)
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			noop := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, summaryPreferencesRequest(t, original, original))
			after, err := os.ReadFile(store.Path)
			if err != nil || noop.Committed || noop.Changed || noop.ErrorMessage != "" || !bytes.Equal(before, after) {
				t.Fatal("historical no-op changed", original, noop, err)
			}
			proposed := original
			proposed.GroupBy, proposed.Display, proposed.ValueLimit = 4, 6, 0
			got := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, summaryPreferencesRequest(t, original, proposed))
			if !got.Committed || !got.Changed || got.ErrorMessage != "" {
				t.Fatal(got)
			}
			reopened := &vprocore.DesktopConfig{Path: store.Path, Replace: os.Rename}
			actual, err := ReadSummaryVegetationPreferences(reopened, context.Background())
			if err != nil || SummaryVegetationPreferenceValues(actual) != proposed {
				t.Fatal(actual, err)
			}
			if stale := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, summaryPreferencesRequest(t, original, proposed)); stale.Committed || stale.ErrorMessage == "" {
				t.Fatal(stale)
			}
		}
	}
	afterDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if !reflect.DeepEqual(protected, afterDB) {
		t.Fatal("saved preferences changed a database")
	}
}

func TestSummaryVegetationPreferencesFacadeCleanupCollisionCancellationReceipt(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewSummaryVegetationPreferencesService(contexts)
	store := contexts.Projects.Preferences
	base := SummaryVegetationPreferenceValues{GroupBy: 1, Average: 1, Display: 1, ShowClass: 1}
	next := base
	next.GroupBy = 4
	request := summaryPreferencesRequest(t, base, next)
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("read rollback failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetSummaryVegetationPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveSummaryVegetationPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	read, err := service.GetSummaryVegetationPreferences(context.Background(), state.ContextID)
	if err != nil || read.Values != base {
		t.Fatal("read after snapshot cancellation failed", read, err)
	}
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	owner.Attachments["project"] = path + ".unowned"
	got := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned preference save", got)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	after, err := os.ReadFile(store.Path)
	staged, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") || err != nil || stageErr != nil || len(staged) != 0 || !bytes.Equal(before, after) {
		t.Fatal(got, err, stageErr)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.Replace = func(source, destination string) error {
		if err := replace(source, destination); err != nil {
			return err
		}
		cancel()
		return nil
	}
	got = service.SaveSummaryVegetationPreferences(ctx, state.ContextID, request)
	store.Replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed receipt lost", got)
	}
	read, err = service.GetSummaryVegetationPreferences(context.Background(), state.ContextID)
	if err != nil || read.Values != next {
		t.Fatal(read, err)
	}
	if noop := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, summaryPreferencesRequest(t, next, next)); noop.Committed || noop.Changed || noop.ErrorMessage != "" {
		t.Fatal("acknowledged retry wrote", noop)
	}
}

func TestSummaryVegetationPreferencesFacadeProductionWire(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewSummaryVegetationPreferencesService(contexts)
	review, err := service.GetSummaryVegetationPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request := SummaryVegetationPreferencesRequest{Expected: review.Values, Proposed: review.Values}
	request.Proposed.GroupBy, request.Proposed.Display, request.Proposed.ShowClass = 4, 6, 20
	outcome := service.SaveSummaryVegetationPreferences(context.Background(), state.ContextID, summaryPreferencesRequest(t, request.Expected, request.Proposed))
	if !outcome.Committed || outcome.ErrorMessage != "" {
		t.Fatal(outcome)
	}
	review.ContextID, review.ProjectPath, review.SUPath = "wire-context", `C:\fixture\Project.sqlite`, `C:\fixture\SU.sqlite`
	outcome.ContextID = review.ContextID
	wire := struct {
		Review  SummaryVegetationPreferencesReview  `json:"review"`
		Request SummaryVegetationPreferencesRequest `json:"request"`
		Outcome SummaryVegetationPreferencesOutcome `json:"outcome"`
	}{*review, request, *outcome}
	raw, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var reopened struct {
		Review  SummaryVegetationPreferencesReview  `json:"review"`
		Request SummaryVegetationPreferencesRequest `json:"request"`
		Outcome SummaryVegetationPreferencesOutcome `json:"outcome"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &reopened, "Summary preference wire", "review", "request", "outcome"); err != nil || !reflect.DeepEqual(wire, reopened) {
		t.Fatal("strict actual Go round-trip differs", err)
	}
	expectedBytes, err := os.ReadFile("frontend/src/summaryVegetationPreferencesWire.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual any
	if err := json.Unmarshal(expectedBytes, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &actual); err != nil || !reflect.DeepEqual(expected, actual) {
		t.Fatal("production Go wire differs from the client fixture", err)
	}
}
