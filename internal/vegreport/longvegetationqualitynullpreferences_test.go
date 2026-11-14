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

	"gopkg.in/yaml.v3"
)

func vegetationQualityNullRequest(t *testing.T, expected, proposed LongVegetationQualityNullPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationQualityNullPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationQualityNullPreferencesStrictWire(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongVegetationQualityNullPreferencesService{nil, {}, NewLongVegetationQualityNullPreferencesService(nil)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if got, err := service.GetLongVegetationQualityNullPreferences(requestCtx, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveLongVegetationQualityNullPreferences(requestCtx, "id", "{}"); got.Committed || got.Changed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	var wire struct {
		Review  LongVegetationQualityNullPreferencesReview  `json:"review"`
		Request LongVegetationQualityNullPreferencesRequest `json:"request"`
		Outcome LongVegetationQualityNullPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/src/longVegetationQualityNullWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "quality Include Nulls wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationQualityNullPreferencesService(contexts)
	read, err := service.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID)
	if err != nil || read.ContextID != state.ContextID || read.Project != "Sample" || read.SU != "None" ||
		read.ProjectPath != state.ProjectPath || read.SUPath != state.SUPath {
		t.Fatal(read, err)
	}
	read.ContextID, read.ProjectPath = wire.Review.ContextID, wire.Review.ProjectPath
	if !reflect.DeepEqual(*read, wire.Review) {
		t.Fatal("Go/client review wire differs", read)
	}
	outcome := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
		vegetationQualityNullRequest(t, wire.Request.Expected, wire.Request.Proposed))
	outcome.ContextID = wire.Outcome.ContextID
	if *outcome != wire.Outcome {
		t.Fatal("Go/client outcome differs", outcome)
	}
	encoded, err := json.Marshal(struct {
		Review  LongVegetationQualityNullPreferencesReview  `json:"review"`
		Request LongVegetationQualityNullPreferencesRequest `json:"request"`
		Outcome LongVegetationQualityNullPreferencesOutcome `json:"outcome"`
	}{*read, wire.Request, *outcome})
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
		t.Fatal("strict-Go round trip changed client wire")
	}
}

func TestLongVegetationQualityNullPreferencesRejectMalformedRequests(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationQualityNullPreferencesService(contexts)
	values := LongVegetationQualityNullPreferenceValues{true, true, true}
	base := vegetationQualityNullRequest(t, values, values)
	before, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	requests := []string{"{}", "null", "[]", strings.Replace(base, `"expected":`, `"Expected":`, 1),
		strings.Replace(base, `"expected":`, `"expected":null,"extra":`, 1),
		strings.Replace(base, `"proposed":`, `"proposed":null,"extra":`, 1)}
	for _, field := range []string{"siteIncludeNulls", "vegIncludeNulls", "soilIncludeNulls"} {
		for _, invalid := range []string{"null", "1", "-1", `"True"`, `"false"`, "{}"} {
			requests = append(requests, strings.Replace(base, `"`+field+`":true`, `"`+field+`":`+invalid, 1))
		}
		requests = append(requests, strings.Replace(base, `"`+field+`":true`, `"foreign":true`, 1),
			strings.Replace(base, `"`+field+`":true`, `"`+field+`":true,"`+field+`":false`, 1))
	}
	requests = append(requests, strings.Replace(base, `"siteIncludeNulls":true`, `"siteIncludeNulls":true,"\u0073iteIncludeNulls":false`, 1),
		strings.Replace(base, `"siteIncludeNulls":true`, `"siteIncludeNulls":true,"\ud800":0`, 1),
		strings.Replace(base, `"siteIncludeNulls":true`, `"siteIncludeNulls":true,"`+string([]byte{0xff})+`":0`, 1))
	for _, raw := range requests {
		if got := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID, raw); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal("malformed request accepted", raw, got)
		}
	}
	after, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed YAML", err)
	}
}

func TestLongVegetationQualityNullPreferencesEightCombinationsAndQualification(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationQualityNullPreferencesService(contexts)
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for mask := 0; mask < 8; mask++ {
		next := LongVegetationQualityNullPreferenceValues{mask&1 != 0, mask&2 != 0, mask&4 != 0}
		current, err := service.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		before, err := contexts.Projects.Preferences.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		result := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
			vegetationQualityNullRequest(t, current.Values, next))
		changed := current.Values != next
		if result.Committed != changed || result.Changed != changed || result.ErrorMessage != "" {
			t.Fatal(mask, result)
		}
		fields := before["ReportOptions"].(map[string]any)
		for _, field := range []struct {
			key      string
			old, new bool
		}{{"DataQualityFilterSiteNullLV", current.Values.SiteIncludeNulls, next.SiteIncludeNulls},
			{"DataQualityFilterVegNullLV", current.Values.VegIncludeNulls, next.VegIncludeNulls},
			{"DataQualityFilterSoilNullLV", current.Values.SoilIncludeNulls, next.SoilIncludeNulls}} {
			if field.old != field.new {
				fields[field.key] = field.new
			}
		}
		after, err := contexts.Projects.Preferences.Snapshot()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("Save changed unrelated configuration or normalized unchanged values", mask, err)
		}
		fields["DataQualityFilterEnforceLV"] = true
		fields["DataQualityFilterSiteLV"], fields["DataQualityFilterVegLV"], fields["DataQualityFilterSoilLV"] = "Good", "Good", "Good"
		criteria, err := decodeLongVegetationQualityCriteria(before)
		if err != nil {
			t.Fatal(criteria, err)
		}
		for nullMask := 0; nullMask < 8; nullMask++ {
			su, env, admin, lists := vegetationQualityFixture()
			for domain := 0; domain < 3; domain++ {
				if nullMask&(1<<domain) != 0 {
					admin.Rows[0].Cells[domain+1] = vprocore.ProjectMetadataCell{Storage: "null"}
				}
			}
			result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, criteria)
			pass := nullMask & ^mask == 0
			if err != nil || pass && (len(result.Occurrences) != 1 || len(result.ExcludedMembershipIDs) != 0) ||
				!pass && (len(result.Occurrences) != 0 || !reflect.DeepEqual(result.ExcludedMembershipIDs, []string{"11"})) {
				t.Fatal("saved flag qualification differs from independent domain mask", mask, nullMask, result, err)
			}
		}
		result = service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
			vegetationQualityNullRequest(t, next, next))
		if result.Committed || result.Changed || result.ErrorMessage != "" {
			t.Fatal("retry was not a no-op", result)
		}
	}
	if !reflect.DeepEqual(beforeDB, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("flag Save changed data/audits")
	}
}

func TestLongVegetationQualityNullPreferencesLegacyStorageNoopOmissionAndInvalid(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationQualityNullPreferencesService(contexts)
	store := contexts.Projects.Preferences
	values, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	fields := values["ReportOptions"].(map[string]any)
	keys := []string{"DataQualityFilterSiteNullLV", "DataQualityFilterVegNullLV", "DataQualityFilterSoilNullLV"}
	for _, key := range keys {
		for _, representation := range []any{true, false, -1, 0, "True", "False"} {
			for _, name := range keys {
				fields[name] = "True"
			}
			fields[key] = representation
			raw, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Commit(raw); err != nil {
				t.Fatal(err)
			}
			read, err := service.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID)
			if err != nil {
				t.Fatal(key, representation, err)
			}
			actual := map[string]bool{keys[0]: read.Values.SiteIncludeNulls, keys[1]: read.Values.VegIncludeNulls, keys[2]: read.Values.SoilIncludeNulls}
			want := representation == true || representation == -1 || representation == "True"
			if actual[key] != want {
				t.Fatal("legacy representation decoded incorrectly", key, representation, actual)
			}
			result := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
				vegetationQualityNullRequest(t, read.Values, read.Values))
			saved, err := os.ReadFile(store.Path)
			if err != nil || result.Committed || result.Changed || result.ErrorMessage != "" || !bytes.Equal(raw, saved) {
				t.Fatal("no-op normalized legacy storage", result, err)
			}
			proposed := read.Values
			if key == keys[0] {
				proposed.VegIncludeNulls = false
				fields[keys[1]] = false
			} else {
				proposed.SiteIncludeNulls = false
				fields[keys[0]] = false
			}
			result = service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
				vegetationQualityNullRequest(t, read.Values, proposed))
			after, err := store.Snapshot()
			if err != nil || !result.Committed || result.ErrorMessage != "" || !reflect.DeepEqual(values, after) {
				t.Fatal("unrelated changed key normalized unchanged legacy flag", key, representation, result, err)
			}
		}
		for _, invalid := range []any{nil, "", "true", "FALSE", " True", 1, -2, 0.5, []any{true}, map[string]any{"value": true}} {
			for _, name := range keys {
				fields[name] = true
			}
			fields[key] = invalid
			raw, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Commit(raw); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID); got != nil || err == nil {
				t.Fatal("malformed flag accepted", key, invalid, got, err)
			}
			got := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
				vegetationQualityNullRequest(t, LongVegetationQualityNullPreferenceValues{true, true, true}, LongVegetationQualityNullPreferenceValues{}))
			after, err := os.ReadFile(store.Path)
			if got.Committed || got.ErrorMessage == "" || err != nil || !bytes.Equal(raw, after) {
				t.Fatal("malformed flag became a default", key, invalid, got, err)
			}
		}
		delete(fields, key)
		raw, err := yaml.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(raw); err != nil {
			t.Fatal(err)
		}
		if got, err := service.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal("missing flag promoted source default", key, got, err)
		}
		fields[key] = true
	}
	if _, err := longVegetationQualityNullOption(values, "LVQuickReport"); err == nil {
		t.Fatal("quality-specific reader widened ordinary switch contract")
	}
}

func TestLongVegetationQualityNullPreferencesOwnershipCleanupCancellationCollisionAndRetry(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationQualityNullPreferencesService(contexts)
	store := contexts.Projects.Preferences
	expected, proposed := LongVegetationQualityNullPreferenceValues{true, true, true}, LongVegetationQualityNullPreferenceValues{false, false, false}
	request := vegetationQualityNullRequest(t, expected, proposed)
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongVegetationQualityNullPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationQualityNullPreferences(context.Background(), id, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationQualityNullPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	owner.Attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	after, err := os.ReadFile(store.Path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") || err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed source or retained staging", got, err, stageErr)
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
	got = service.SaveLongVegetationQualityNullPreferences(ctx, state.ContextID, request)
	store.Replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost", got)
	}
	current, err := service.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values != proposed {
		t.Fatal(current, err)
	}
	stale := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID, request)
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "changed") {
		t.Fatal("stale expected flags accepted", stale)
	}
	retry := service.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
		vegetationQualityNullRequest(t, proposed, proposed))
	if retry.Committed || retry.Changed || retry.ErrorMessage != "" {
		t.Fatal("observed committed retry was not a no-op", retry)
	}
}
