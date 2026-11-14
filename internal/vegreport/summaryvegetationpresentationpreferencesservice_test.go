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
	"strings"
	"testing"
)

func summaryPresentationRequest(t *testing.T, expected, proposed SummaryVegetationPresentationPreferenceValues, applied bool) string {
	t.Helper()
	raw, err := json.Marshal(SummaryVegetationPresentationPreferencesRequest{
		Expected: expected, Proposed: proposed, ConstancyApplied: applied,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSummaryPresentationFacadeStrictRequiredWire(t *testing.T) {
	value := SummaryVegetationPresentationPreferenceValues{
		Title: "Short veg report title", CreateSummary: true, GroupBy: 1, Display: 1,
	}
	valid := summaryPresentationRequest(t, value, value, false)
	var decoded SummaryVegetationPresentationPreferencesRequest
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil || decoded.Expected != value ||
		decoded.Proposed != value || decoded.ConstancyApplied {
		t.Fatal(decoded, err)
	}
	for _, invalid := range []string{
		`{}`, strings.Replace(valid, `"constancyApplied":false`, `"constancyApplied":null`, 1),
		strings.Replace(valid, `"constancyApplied":false`, `"constancyApplied":0`, 1),
		strings.Replace(valid, `"constancyApplied":false`, `"constancyApplied":false,"constancyApplied":true`, 1),
		strings.Replace(valid, `"constancyApplied":false`, `"constancyApplied":false,"\u0063onstancyApplied":true`, 1),
		strings.Replace(valid, `"constancyApplied":false`, `"ConstancyApplied":false`, 1),
		strings.Replace(valid, `"title":"Short veg report title"`, `"title":null`, 1),
		strings.Replace(valid, `"title":"Short veg report title"`, `"title":"\ud800"`, 1),
		strings.Replace(valid, `"title":"Short veg report title"`, `"title":"`+string([]byte{0xff})+`"`, 1),
		strings.Replace(valid, `"title":"Short veg report title"`, `"title":"first","\u0074itle":"second"`, 1),
		strings.Replace(valid, `"showCommon":false`, `"showCommon":1`, 1),
		strings.Replace(valid, `"groupBy":1`, `"groupBy":1.5`, 1),
		strings.Replace(valid, `"display":1`, `"display":"1"`, 1),
		strings.Replace(valid, `"valueLimit":0`, `"valueLimit":null`, 1),
		strings.Replace(valid, `"expected":`, `"extra":{},"expected":`, 1),
		valid + `{}`,
	} {
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatal("invalid strict presentation request accepted", invalid)
		}
	}
	for _, key := range []string{"title", "showCommon", "showGuide", "hierarchyHeader", "includeEnv", "createSummary",
		"combineVariants", "constancy", "groupBy", "display", "valueLimit"} {
		var fields map[string]json.RawMessage
		raw, err := json.Marshal(value)
		if err != nil || json.Unmarshal(raw, &fields) != nil {
			t.Fatal(err)
		}
		delete(fields, key)
		missing, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		request := `{"expected":` + string(missing) + `,"proposed":` + string(raw) + `,"constancyApplied":false}`
		if err := json.Unmarshal([]byte(request), &decoded); err == nil {
			t.Fatal("missing presentation value defaulted", key)
		}
	}
}

func TestSummaryPresentationFacadeIndependentGateAndOwner(t *testing.T) {
	for _, service := range []*SummaryVegetationPresentationPreferencesService{nil, {}, NewSummaryVegetationPresentationPreferencesService(nil)} {
		if got, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), "id"); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveSummaryVegetationPresentationPreferences(context.Background(), "id", "{}"); got.Changed || got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewSummaryVegetationPresentationPreferencesService(contexts)
	read, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID)
	if err != nil || read.ContextID != state.ContextID || read.Project != "Sample" || read.ProjectPath != state.ProjectPath ||
		read.SU != "None" || read.SUPath != state.SUPath || read.Source != summaryVegetationPresentationPreferencesSource {
		t.Fatal(read, err)
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveSummaryVegetationPresentationPreferences(context.Background(), id, "{}"); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	if got, err := service.GetSummaryVegetationPresentationPreferences(nil, state.ContextID); got != nil || err == nil {
		t.Fatal(got, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.GetSummaryVegetationPresentationPreferences(cancelled, state.ContextID); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}

func TestSummaryPresentationFacadeCleanupCancellationReplacementAndCommittedReceipt(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewSummaryVegetationPresentationPreferencesService(contexts)
	store := contexts.Projects.Preferences
	read, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	expected, proposed := read.Values, read.Values
	proposed.Title, proposed.Constancy, proposed.ShowCommon, proposed.GroupBy, proposed.Display =
		" source literal ", true, false, 3, 1
	request := summaryPresentationRequest(t, expected, proposed, true)
	before := summaryPresentationBytes(t, store)
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveSummaryVegetationPresentationPreferences(ctx, state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	owner.Attachments["project"] = path + ".unowned"
	got := service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = path
	if got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal("unowned context authorized config mutation", got)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("replacement denied") }
	got = service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "denied") || !bytes.Equal(before, summaryPresentationBytes(t, store)) {
		t.Fatal("refusal altered source", got)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.Replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	got = service.SaveSummaryVegetationPresentationPreferences(ctx, state.ContextID, request)
	store.Replace = replace
	if !got.Changed || !got.Committed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("irreversible config outcome lost", got)
	}
	read, err = service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID)
	if err != nil || read.Values != proposed {
		t.Fatal("cancelled persistence lost actual plan or poisoned coordinator", read, err)
	}
	stale := service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID, request)
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "changed") {
		t.Fatal("stale request replayed", stale)
	}
	retry := service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID,
		summaryPresentationRequest(t, proposed, proposed, false))
	if retry.Committed || retry.Changed || retry.ErrorMessage != "" {
		t.Fatal("acknowledged actual plan replayed", retry)
	}
}

func TestSummaryPresentationFacadeConstancyAwayBackAndNoDatabaseChanges(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewSummaryVegetationPresentationPreferencesService(contexts)
	store := contexts.Projects.Preferences
	if err := store.Update("ReportOptions", map[string]any{
		"SVGroupBy": 4, "SVValue": 6, "SVShowEnglishName": -1, "SVShowValOpt": 7.25,
	}); err != nil {
		t.Fatal(err)
	}
	databases := map[string][]byte{}
	for role, path := range contexts.Projects.Sqlite.Attachments {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(role, err)
		}
		databases[path] = raw
	}
	read, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	proposed := read.Values
	proposed.GroupBy, proposed.Display = 3, 1
	proposed.Constancy = false
	proposed.ShowCommon = true
	outcome := service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID,
		summaryPresentationRequest(t, read.Values, proposed, true))
	if !outcome.Committed || !outcome.Changed || outcome.ErrorMessage != "" {
		t.Fatal("on/off/source-reenabled common edit refused", outcome)
	}
	reopened, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID)
	if err != nil || reopened.Values != proposed || reopened.Values.ValueLimit != 7.25 {
		t.Fatal("away/back source plan lost or limit cleared", reopened, err)
	}
	for path, before := range databases {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("saved presentation mutated database/history/audit storage", path, err)
		}
	}
	options, err := ReadSummaryVegetationPreferences(store, context.Background())
	if err != nil || options.GroupBy != 3 || options.Display != 1 || options.ValueLimit != 7.25 {
		t.Fatal("peer read DTO missed shared plan", options, err)
	}
}

func TestSummaryPresentationFacadeUnavailableHierarchyGuardAndHistoricalOmission(t *testing.T) {
	for _, historical := range []int{0, -1} {
		contexts, state := vprotest.PrepareKMLContextFixture(t)
		service := NewSummaryVegetationPresentationPreferencesService(contexts)
		store := contexts.Projects.Preferences
		if err := store.Update("ReportOptions", map[string]any{"SVShowHierarchyHeader": historical}); err != nil {
			t.Fatal(err)
		}
		read, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID)
		if err != nil || read.Values.HierarchyHeader != (historical == -1) {
			t.Fatal(read, err)
		}
		before := summaryPresentationBytes(t, store)
		proposed := read.Values
		proposed.HierarchyHeader = !proposed.HierarchyHeader
		got := service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID,
			summaryPresentationRequest(t, read.Values, proposed, false))
		if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "unavailable hierarchy report scope") ||
			!bytes.Equal(before, summaryPresentationBytes(t, store)) {
			t.Fatal("disabled source hierarchy control authorized mutation", got)
		}
		proposed = read.Values
		proposed.Title = "unrelated title update"
		got = service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID,
			summaryPresentationRequest(t, read.Values, proposed, false))
		if !got.Committed || !got.Changed || got.ErrorMessage != "" {
			t.Fatal("unavailable historical header should not block unrelated literal title", got)
		}
		values, err := store.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		section, err := vprocore.ConfigSection(values, "ReportOptions")
		if err != nil || section["SVShowHierarchyHeader"] != historical {
			t.Fatal("unchanged header storage normalized", historical, section, err)
		}
	}
}

func TestSummaryPresentationFacadeEmitActualGoWire(t *testing.T) {
	directory := t.TempDir()
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewSummaryVegetationPresentationPreferencesService(contexts)
	read, err := service.GetSummaryVegetationPresentationPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	proposed := read.Values
	proposed.Title, proposed.Constancy, proposed.ShowCommon, proposed.GroupBy, proposed.Display = "literal title", true, false, 3, 1
	request := SummaryVegetationPresentationPreferencesRequest{Expected: read.Values, Proposed: proposed, ConstancyApplied: true}
	outcome := service.SaveSummaryVegetationPresentationPreferences(context.Background(), state.ContextID,
		summaryPresentationRequest(t, request.Expected, request.Proposed, request.ConstancyApplied))
	if !outcome.Committed || !outcome.Changed || outcome.ErrorMessage != "" {
		t.Fatal(outcome)
	}
	raw, err := json.MarshalIndent(struct {
		Review  *SummaryVegetationPresentationPreferencesReview  `json:"review"`
		Request SummaryVegetationPresentationPreferencesRequest  `json:"request"`
		Outcome *SummaryVegetationPresentationPreferencesOutcome `json:"outcome"`
	}{read, request, outcome}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(directory, "summaryVegetationPresentationPreferencesWire.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal(writeErr, closeErr)
	}
}
