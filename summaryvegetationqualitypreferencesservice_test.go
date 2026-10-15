package main

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

func summaryQualityRequest(t *testing.T, expected, proposed SummaryVegetationQualityPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(SummaryVegetationQualityPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSummaryQualityFacadeStrictRequiredWire(t *testing.T) {
	value := SummaryVegetationQualityPreferenceValues{
		SiteMinimum: "Poor", VegMinimum: "Poor", SoilMinimum: "Poor", BECMinimum: "True",
		SiteIncludeNulls: true, VegIncludeNulls: true, SoilIncludeNulls: true, BECIncludeNulls: true,
	}
	valid := summaryQualityRequest(t, value, value)
	var decoded SummaryVegetationQualityPreferencesRequest
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil || decoded.Expected != value || decoded.Proposed != value {
		t.Fatal(decoded, err)
	}
	for _, invalid := range []string{
		`{}`, strings.Replace(valid, `"enforce":false`, `"enforce":null`, 1),
		strings.Replace(valid, `"becMinimum":"True"`, `"becMinimum":true`, 1),
		strings.Replace(valid, `"becIncludeNulls":true`, `"becIncludeNulls":1`, 1),
		strings.Replace(valid, `"siteMinimum":"Poor"`, `"siteMinimum":"Poor","siteMinimum":"Good"`, 1),
		strings.Replace(valid, `"siteMinimum":"Poor"`, `"siteMinimum":"Poor","\u0073iteMinimum":"Good"`, 1),
		strings.Replace(valid, `"siteMinimum":"Poor"`, `"SiteMinimum":"Poor"`, 1),
		strings.Replace(valid, `"siteMinimum":"Poor"`, `"siteMinimum":"\ud800"`, 1),
		strings.Replace(valid, `"siteMinimum":"Poor"`, `"siteMinimum":"`+string([]byte{0xff})+`"`, 1),
		strings.Replace(valid, `"siteMinimum":"Poor"`, `"siteMinimum":"Poor","extra":1`, 1),
		strings.Replace(valid, `"expected":`, `"extra":{},"expected":`, 1),
		valid + `{}`,
	} {
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatal("invalid strict request accepted", invalid)
		}
	}
}

func TestSummaryQualityFacadeIndependentGateAndOwnedRead(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}} {
		enabled, err := summaryVegetationQualityPreferencesFeature(func(key string) (string, bool) {
			if key != "VPRO_SUMMARY_VEGETATION_QUALITY_PREFERENCES" {
				t.Fatal(key)
			}
			return test.value, test.present
		})
		if enabled != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, enabled, err)
		}
	}
	for _, service := range []*SummaryVegetationQualityPreferencesService{nil, {}, NewSummaryVegetationQualityPreferencesService(nil, true)} {
		if read, err := service.GetSummaryVegetationQualityPreferences(context.Background(), "id"); read != nil || err == nil {
			t.Fatal(read, err)
		}
		if saved := service.SaveSummaryVegetationQualityPreferences(context.Background(), "id", "{}"); saved.Committed || saved.Changed || saved.ErrorMessage == "" {
			t.Fatal(saved)
		}
	}
	contexts, state := prepareKMLContextFixture(t)
	service := NewSummaryVegetationQualityPreferencesService(contexts, true)
	read, err := service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if read.ContextID != state.ContextID || read.Project != "Sample" || read.ProjectPath != state.ProjectPath ||
		read.SU != "None" || read.SUPath != state.SUPath || read.Source != summaryVegetationQualitySuggestionsSource ||
		len(read.QualityDefinitions.Columns) != 10 || len(read.BECDefinitions.Columns) != 10 {
		t.Fatal(read)
	}
	for _, id := range []string{"", "foreign"} {
		if read, err := service.GetSummaryVegetationQualityPreferences(context.Background(), id); read != nil || err == nil {
			t.Fatal(read, err)
		}
		if saved := service.SaveSummaryVegetationQualityPreferences(context.Background(), id, "{}"); saved.Committed || saved.Changed || saved.ErrorMessage == "" {
			t.Fatal(saved)
		}
	}
}

func TestSummaryQualityFacadeOwnedCleanupCollisionCancellationAndReceipt(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewSummaryVegetationQualityPreferencesService(contexts, true)
	store := contexts.projects.preferences
	read, err := service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	expected, proposed := read.Values, read.Values
	proposed.BECMinimum = "literal nonmember"
	proposed.SoilIncludeNulls = !expected.SoilIncludeNulls
	request := summaryQualityRequest(t, expected, proposed)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveSummaryVegetationQualityPreferences(ctx, state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	owner := contexts.projects.sqlite
	path := owner.attachments["project"]
	owner.attachments["project"] = path + ".unowned"
	got := service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = path
	if got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("replacement collision") }
	got = service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "collision") || err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed configuration", got, err)
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
	got = service.SaveSummaryVegetationQualityPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost", got)
	}
	read, err = service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil || read.Values != proposed {
		t.Fatal("cancelled persistence poisoned coordinator or lost commit", read, err)
	}
	stale := service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID, request)
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "changed") {
		t.Fatal("stale expected values accepted", stale)
	}
	retry := service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID, summaryQualityRequest(t, proposed, proposed))
	if retry.Committed || retry.Changed || retry.ErrorMessage != "" {
		t.Fatal("observed committed retry was not a no-op", retry)
	}
}

func TestSummaryQualityFacadeBECSuggestionsEmptyAndMalformedCorrection(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewSummaryVegetationQualityPreferencesService(contexts, true)
	db, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE ListName='BEC_Use'`); err != nil {
		t.Fatal(err)
	}
	read, err := service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil || len(read.BECDefinitions.Rows) != 0 {
		t.Fatal("empty suggestions became unavailable", read, err)
	}
	if _, err := db.Exec(`INSERT INTO USysTableOfLists(rowid,ListName,ItemOrder,Item,ItemDescription)
		VALUES (-5,'BEC_Use',1,'Same','first'),(-4,'BEC_Use',1,'Same',NULL),
		(-3,'BEC_Use',2,NULL,'null item'),(-2,'BEC_Use',3,'','empty item'),
		(-1,'BEC_Use',4,CAST(X'FF' AS TEXT),'malformed')`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal("malformed physical UTF-8 became serializable metadata", got, err)
	}
	if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE rowid=-1`); err != nil {
		t.Fatal(err)
	}
	read, err = service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil || len(read.BECDefinitions.Rows) != 4 {
		t.Fatal("correction/retry lost duplicates/NULL/empty rows", read, err)
	}
	for i, identity := range []string{"-5", "-4", "-3", "-2"} {
		if read.BECDefinitions.Rows[i].RowID != identity {
			t.Fatal("source BEC ItemOrder/rowid order changed", read.BECDefinitions)
		}
	}
	if read.BECDefinitions.Rows[2].Cells[3].Storage != "null" ||
		read.BECDefinitions.Rows[3].Cells[3].Text == nil || *read.BECDefinitions.Rows[3].Cells[3].Text != "" ||
		read.BECDefinitions.Rows[1].Cells[4].Storage != "null" ||
		read.BECDefinitions.Rows[0].Cells[3].Text == nil || read.BECDefinitions.Rows[1].Cells[3].Text == nil ||
		*read.BECDefinitions.Rows[0].Cells[3].Text != "Same" || *read.BECDefinitions.Rows[1].Cells[3].Text != "Same" {
		t.Fatal("BEC NULL/empty/duplicate definitions collapsed", read.BECDefinitions)
	}
}

func TestSummaryQualityFacadeSharedBECExternalCASAndNoDatabaseWrites(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewSummaryVegetationQualityPreferencesService(contexts, true)
	read, err := service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	physical := map[string][]byte{}
	for name, path := range contexts.projects.sqlite.attachments {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(name, err)
		}
		physical[path] = raw
	}
	expected, proposed := read.Values, read.Values
	proposed.SiteMinimum = "literal changed"
	if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterBECUseLV": "external shared edit"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	got := service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID, summaryQualityRequest(t, expected, proposed))
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "changed") {
		t.Fatal("other report's shared BEC edit was overwritten", got)
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("stale shared key changed configuration", err)
	}
	read, err = service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	proposed = read.Values
	proposed.BECMinimum = "explicit recovered"
	got = service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID, summaryQualityRequest(t, read.Values, proposed))
	if !got.Committed || !got.Changed || got.ErrorMessage != "" {
		t.Fatal(got)
	}
	for path, before := range physical {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("preference Save wrote SQLite/audit", path, err)
		}
	}
}

func TestSummaryQualityFacadeEmitActualGoWire(t *testing.T) {
	directory := os.Getenv("VPRO_SUMMARY_QUALITY_PRIVATE_WIRE_DIR")
	if directory == "" {
		t.Skip("private actual-Go wire generation requires an explicit session directory")
	}
	contexts, state := prepareKMLContextFixture(t)
	service := NewSummaryVegetationQualityPreferencesService(contexts, true)
	review, err := service.GetSummaryVegetationQualityPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	proposed := review.Values
	proposed.SiteMinimum = " literal nonmember "
	proposed.BECMinimum = "10"
	proposed.BECIncludeNulls = !proposed.BECIncludeNulls
	request := SummaryVegetationQualityPreferencesRequest{Expected: review.Values, Proposed: proposed}
	outcome := service.SaveSummaryVegetationQualityPreferences(context.Background(), state.ContextID,
		summaryQualityRequest(t, request.Expected, request.Proposed))
	if !outcome.Committed || !outcome.Changed || outcome.ErrorMessage != "" {
		t.Fatal(outcome)
	}
	raw, err := json.MarshalIndent(struct {
		Review  *SummaryVegetationQualityPreferencesReview  `json:"review"`
		Request SummaryVegetationQualityPreferencesRequest  `json:"request"`
		Outcome *SummaryVegetationQualityPreferencesOutcome `json:"outcome"`
	}{review, request, outcome}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(directory, "summaryVegetationQualityPreferencesWire.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
