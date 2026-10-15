package main

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

	"gopkg.in/yaml.v3"
)

func vegetationQualityMinimumRequest(t *testing.T, expected, proposed LongVegetationQualityMinimumPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationQualityMinimumPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationQualityMinimumPreferencesOwnershipCleanupCancellationCollisionAndRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	store := contexts.projects.preferences
	expected := LongVegetationQualityMinimumPreferenceValues{"Poor", "Poor", "Poor"}
	proposed := LongVegetationQualityMinimumPreferenceValues{"Fair", "Good", "Excellent"}
	request := vegetationQualityMinimumRequest(t, expected, proposed)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "foreign"} {
		if got := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), id, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationQualityMinimumPreferences(ctx, state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	owner := contexts.projects.sqlite
	path := owner.attachments["project"]
	owner.attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = path
	if got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "collision") || err != nil ||
		!bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed source or retained staging", got, err, stageErr)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	got = service.SaveLongVegetationQualityMinimumPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost", got)
	}
	current, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values != proposed {
		t.Fatal("cancelled snapshot/persistence poisoned the coordinator or lost the commit", current, err)
	}
	stale := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID, request)
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "changed") {
		t.Fatal("stale expected minima accepted", stale)
	}
	retry := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID,
		vegetationQualityMinimumRequest(t, proposed, proposed))
	if retry.Committed || retry.Changed || retry.ErrorMessage != "" {
		t.Fatal("observed committed retry was not a no-op", retry)
	}
}
func TestLongVegetationQualityMinimumPreferencesGateAndOwnedRead(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}} {
		enabled, err := longVegetationQualityMinimumPreferencesFeature(func(key string) (string, bool) {
			if key != "VPRO_LONG_VEGETATION_QUALITY_MINIMUM_PREFERENCES" {
				t.Fatal(key)
			}
			return test.value, test.present
		})
		if enabled != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, enabled, err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongVegetationQualityMinimumPreferencesService{nil, {}, NewLongVegetationQualityMinimumPreferencesService(nil, true)} {
		for _, ctx := range []context.Context{nil, context.Background(), cancelled} {
			if read, err := service.GetLongVegetationQualityMinimumPreferences(ctx, "id"); read != nil || err == nil {
				t.Fatal(read, err)
			}
			if saved := service.SaveLongVegetationQualityMinimumPreferences(ctx, "id", "{}"); saved.Committed || saved.Changed || saved.ErrorMessage == "" {
				t.Fatal(saved)
			}
		}
	}
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if read.ContextID != state.ContextID || read.Project != "Sample" || read.ProjectPath != state.ProjectPath ||
		read.SU != "None" || read.SUPath != state.SUPath || read.Values != (LongVegetationQualityMinimumPreferenceValues{"Poor", "Poor", "Poor"}) ||
		read.Source != longVegetationQualityMinimumSuggestionsSource || len(read.Definitions.Columns) != 10 || len(read.Definitions.Rows) == 0 {
		t.Fatal(read)
	}
	for _, id := range []string{"", "foreign"} {
		if read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), id); read != nil || err == nil {
			t.Fatal(read, err)
		}
	}
}

func TestLongVegetationQualityMinimumPreferencesStrictGoClientWire(t *testing.T) {
	var wire struct {
		Review  LongVegetationQualityMinimumPreferencesReview  `json:"review"`
		Request LongVegetationQualityMinimumPreferencesRequest `json:"request"`
		Outcome LongVegetationQualityMinimumPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/src/longVegetationQualityMinimumWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictRequiredJSON(raw, &wire, "quality minimum Go/client wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	contexts, state := prepareKMLContextFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE ListName='PlotQualitySite'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	read.ContextID, read.ProjectPath = wire.Review.ContextID, wire.Review.ProjectPath
	if !reflect.DeepEqual(*read, wire.Review) {
		t.Fatal("actual Go read differs from client wire", read)
	}
	result := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID,
		vegetationQualityMinimumRequest(t, wire.Request.Expected, wire.Request.Proposed))
	result.ContextID = wire.Outcome.ContextID
	if *result != wire.Outcome {
		t.Fatal("actual Go save differs from client wire", result)
	}
	wire.Review, wire.Outcome = *read, *result
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var expectedJSON, actualJSON any
	if err := json.Unmarshal(raw, &expectedJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actualJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expectedJSON, actualJSON) {
		t.Fatal("strict Go round trip changed required source/declaredType wire")
	}
}

func TestLongVegetationQualityMinimumPreferencesHistoricalNULOmissionAndUnavailableStorage(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	store := contexts.projects.preferences
	values, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	fields := values["ReportOptions"].(map[string]any)
	fields["DataQualityFilterSoilLV"] = "historical\x00value"
	raw, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.commit(raw); err != nil {
		t.Fatal(err)
	}
	read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	next := read.Values
	next.SiteMinimum = "Fair"
	result := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID,
		vegetationQualityMinimumRequest(t, read.Values, next))
	if result.ErrorMessage != "" || !result.Committed || !result.Changed {
		t.Fatal(result)
	}
	fields["DataQualityFilterSiteLV"] = "Fair"
	after, err := store.snapshot()
	if err != nil || !reflect.DeepEqual(values, after) {
		t.Fatal("changed Save repaired an unchanged historical minimum", err)
	}
	for _, key := range []string{"DataQualityFilterSiteLV", "DataQualityFilterVegLV", "DataQualityFilterSoilLV"} {
		for _, invalid := range []any{nil, 1, false} {
			current, err := store.snapshot()
			if err != nil {
				t.Fatal(err)
			}
			current["ReportOptions"].(map[string]any)[key] = invalid
			raw, err := yaml.Marshal(current)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.commit(raw); err != nil {
				t.Fatal(err)
			}
			if read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID); read != nil || err == nil {
				t.Fatal("invalid storage invented a source default or string conversion", key, invalid, read, err)
			}
			current["ReportOptions"].(map[string]any)[key] = "Poor"
			raw, err = yaml.Marshal(current)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.commit(raw); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestLongVegetationQualityMinimumPreferencesLiteralSaveAndChangedOnlyOmission(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, literal := range []string{"", " poor ", "CUSTOM-not-a-suggestion", strings.Repeat("界", 32), "Fair", "Poor"} {
		read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		before, err := contexts.projects.preferences.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		next := read.Values
		next.SiteMinimum = literal
		result := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID,
			vegetationQualityMinimumRequest(t, read.Values, next))
		changed := read.Values != next
		if result.Changed != changed || result.Committed != changed || result.ErrorMessage != "" {
			t.Fatal(literal, result)
		}
		before["ReportOptions"].(map[string]any)["DataQualityFilterSiteLV"] = literal
		after, err := contexts.projects.preferences.snapshot()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("literal Save normalized or changed unrelated settings", literal, err)
		}
	}
	read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	result := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID,
		vegetationQualityMinimumRequest(t, read.Values, read.Values))
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || result.Committed || result.Changed || result.ErrorMessage != "" || !bytes.Equal(before, after) {
		t.Fatal("no-op changed bytes", result, err)
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("quality minimum Save changed data/audits")
	}
}

func TestLongVegetationQualityMinimumPreferencesStrictRequestsAndStaleRefusal(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	values := LongVegetationQualityMinimumPreferenceValues{"Poor", "Poor", "Poor"}
	base := vegetationQualityMinimumRequest(t, values, values)
	requests := []string{"{}", "null", "[]", strings.Replace(base, `"expected":`, `"Expected":`, 1)}
	proposedAt := strings.Index(base, `"proposed":`)
	for _, field := range []string{"siteMinimum", "vegMinimum", "soilMinimum"} {
		for _, invalid := range []string{"null", "1", "true", "{}", `"new\u0000value"`, `"\ud800"`} {
			requests = append(requests, base[:proposedAt]+
				strings.Replace(base[proposedAt:], `"`+field+`":"Poor"`, `"`+field+`":`+invalid, 1))
		}
		requests = append(requests, strings.Replace(base, `"`+field+`":"Poor"`, `"foreign":"Poor"`, 1),
			strings.Replace(base, `"`+field+`":"Poor"`, `"`+field+`":"Poor","`+field+`":"Fair"`, 1))
	}
	requests = append(requests, strings.Replace(base, `"siteMinimum":"Poor"`, `"siteMinimum":"Poor","\u0073iteMinimum":"Fair"`, 1),
		strings.Replace(base, `"siteMinimum":"Poor"`, `"siteMinimum":"`+string([]byte{0xff})+`"`, 1))
	stale := values
	stale.SiteMinimum = "Fair"
	requests = append(requests, vegetationQualityMinimumRequest(t, stale, values))
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range requests {
		result := service.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID, raw)
		if result.Committed || result.Changed || result.ErrorMessage == "" {
			t.Fatal("invalid request accepted", raw, result)
		}
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed YAML", err)
	}
}

func TestLongVegetationQualityMinimumSuggestionsEmptyDuplicatesNullAndMalformedRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	db, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE ListName='PlotQualitySite'`); err != nil {
		t.Fatal(err)
	}
	read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
	if err != nil || read == nil || len(read.Definitions.Rows) != 0 || read.Definitions.Rows == nil {
		t.Fatal("empty suggestions invented an unavailable membership requirement", read, err)
	}
	if _, err := db.Exec(`INSERT INTO USysTableOfLists(rowid,ListName,ItemOrder,Item,ItemDescription)
		VALUES (-4,'PlotQualitySite',1,'Same','first'),(-3,'PlotQualitySite',1,'Same',NULL),
		(-2,'PlotQualitySite',2,NULL,'null item'),(-1,'PlotQualitySite',3,'','empty item'),
		(-5,'DifferentList',0,'Foreign','not owned')`); err != nil {
		t.Fatal(err)
	}
	read, err = service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Definitions.Rows) != 4 {
		t.Fatal(read.Definitions)
	}
	for i, id := range []string{"-4", "-3", "-2", "-1"} {
		if read.Definitions.Rows[i].RowID != id {
			t.Fatal("source ItemOrder/rowid order changed", read.Definitions)
		}
	}
	if read.Definitions.Rows[2].Cells[3].Storage != "null" || read.Definitions.Rows[3].Cells[3].Text == nil ||
		*read.Definitions.Rows[3].Cells[3].Text != "" || read.Definitions.Rows[1].Cells[4].Storage != "null" {
		t.Fatal("NULL/empty/duplicate metadata collapsed", read.Definitions)
	}
	if _, err := db.Exec(`UPDATE USysTableOfLists SET ItemDescription=CAST(X'FF' AS TEXT) WHERE rowid=-3`); err != nil {
		t.Fatal(err)
	}
	if read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID); read != nil || err == nil {
		t.Fatal("malformed source metadata was serialized or made merely nonselectable", read, err)
	}
	if _, err := db.Exec(`UPDATE USysTableOfLists SET ItemDescription='corrected' WHERE rowid=-3`); err != nil {
		t.Fatal(err)
	}
	if read, err := service.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID); read == nil || err != nil {
		t.Fatal("corrected reference retry failed", read, err)
	}
}
