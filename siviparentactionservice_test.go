package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func siviPublicActionRequest(original *siviParentProjection, actions []siviParentActionEdit) SIVIParentActionWrite {
	request := SIVIParentActionWrite{Original: original, Actions: []SIVIParentActionInput{}}
	for _, action := range actions {
		request.Actions = append(request.Actions, SIVIParentActionInput{
			action.ContextID, action.ControlID, action.Table, action.RowID, cloneSiteUnitCell(action.Expected), action.Option,
		})
	}
	return request
}

func TestSIVIParentActionEditingFeatureLiteralAndIndependent(t *testing.T) {
	for _, test := range []struct {
		value   string
		present bool
		enabled bool
		valid   bool
	}{{"", false, false, true}, {"true", true, true, true}, {"false", true, false, true},
		{"TRUE", true, false, false}, {" true", true, false, false}, {"1", true, false, false}, {"", true, false, false}} {
		result, err := siviParentActionEditingFeature(func(name string) (string, bool) {
			if name != siviParentActionEditingFeatureEnvironment {
				t.Fatal("source action flag inherited another workflow", name)
			}
			return test.value, test.present
		})
		if (err == nil) != test.valid || result != test.enabled {
			t.Fatal("source action feature accepted a silent/coerced flag", test, result, err)
		}
	}
}

func TestSIVIParentPublicActionGatesAllThreeSurfaces(t *testing.T) {
	service, state, original, actions := siviParentActionWriteFixture(t, false, 3)
	request := siviPublicActionRequest(original, actions)
	for mask := 0; mask < 7; mask++ {
		service.siviParentReviewEnabled = mask&1 != 0
		service.siviParentEditingEnabled = mask&2 != 0
		service.siviParentActionEditingEnabled = mask&4 != 0
		files := databaseBytes(t, service.projects.sqlite.attachments)
		if result, err := service.GetSIVIParentActionOriginal(context.Background(), state.ContextID, original.Plot); err == nil || result != nil {
			t.Fatal("partially enabled action original initialized", mask, result, err)
		}
		if result, err := service.SaveSIVIParentActions(context.Background(), state.ContextID, original.Plot, request); err == nil || result != nil {
			t.Fatal("partially enabled source action wrote", mask, result, err)
		}
		if result, err := service.RestoreSIVIParentActions(context.Background(), state.ContextID, original.Plot, "1", AuditRestoreCancel); err == nil || result != nil {
			t.Fatal("partially enabled source action restore succeeded", mask, result, err)
		}
		assertProfileSUFiles(t, service, files)
	}
	service.siviParentReviewEnabled, service.siviParentEditingEnabled, service.siviParentActionEditingEnabled = true, true, true
	got, err := service.GetSIVIParentActionOriginal(context.Background(), state.ContextID, original.Plot)
	if err != nil || !reflect.DeepEqual(got, original) {
		t.Fatal("fully enabled owned action original differs", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetSIVIParentActionOriginal(ctx, state.ContextID, original.Plot); !errors.Is(err, context.Canceled) {
		t.Fatal("action original discarded caller cancellation", err)
	}
}

func TestSIVIParentPublicActionStrictWireSaveRestore(t *testing.T) {
	service, state, original, actions := siviParentActionWriteFixture(t, true, 3)
	service.siviParentReviewEnabled, service.siviParentEditingEnabled, service.siviParentActionEditingEnabled = true, true, true
	actions[1].Option = nil
	body, err := json.Marshal(siviPublicActionRequest(original, actions))
	if err != nil {
		t.Fatal(err)
	}
	var request SIVIParentActionWrite
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	if request.Actions[1].Option != nil || request.Actions[0].ControlID != "form:frmSIVIsite/optPlotType" {
		t.Fatal("nullable option/source identity changed in transport", request)
	}
	result, err := service.SaveSIVIParentActions(context.Background(), state.ContextID, original.Plot, request)
	if err != nil || result.ChangedCells != 2 || result.HistoryID == "" || !result.SourceRefreshRequired {
		t.Fatal("raw explicit action request failed", result, err)
	}
	restored, err := service.RestoreSIVIParentActions(context.Background(), state.ContextID, original.Plot, result.HistoryID, AuditRestorePrune)
	if err != nil || restored.RestoredRows != 2 || restored.PrunedAuditRows != 2 || restored.CleanedVegRows != 0 {
		t.Fatal("public source action history restoration failed", restored, err)
	}
	fresh, err := service.GetSIVIParentActionOriginal(context.Background(), state.ContextID, original.Plot)
	if err != nil || !reflect.DeepEqual(fresh, original) {
		t.Fatal("public action original did not recover exact historical values", fresh, err)
	}
	if _, err := service.RestoreSIVIParentActions(context.Background(), state.ContextID, original.Plot, result.HistoryID, AuditRestorePrune); err == nil {
		t.Fatal("public action replay accepted")
	}
}

func TestSIVIParentPublicActionTransportRejectsMissingForeignAndMalformedFields(t *testing.T) {
	_, _, original, actions := siviParentActionWriteFixture(t, false, 3)
	body, err := json.Marshal(siviPublicActionRequest(original, actions))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"original", "actions"} {
		for _, mode := range []string{"missing", "null"} {
			var object map[string]json.RawMessage
			if err := json.Unmarshal(body, &object); err != nil {
				t.Fatal(err)
			}
			if mode == "missing" {
				delete(object, field)
			} else {
				object[field] = json.RawMessage("null")
			}
			raw, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			var request SIVIParentActionWrite
			if err := json.Unmarshal(raw, &request); err == nil {
				t.Fatal("missing/non-NULL action envelope requirement ignored", field, mode)
			}
		}
	}
	input, err := json.Marshal(siviPublicActionRequest(original, actions).Actions[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"contextId", "controlId", "table", "rowId", "expected", "option"} {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(input, &object); err != nil {
			t.Fatal(err)
		}
		delete(object, field)
		raw, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		var action SIVIParentActionInput
		if err := json.Unmarshal(raw, &action); err == nil {
			t.Fatal("source action silently defaulted a missing property", field)
		}
	}
	for _, raw := range []string{
		strings.TrimSuffix(string(body), "}") + `,"scalars":[]}`,
		strings.Replace(string(body), `"controlId":`, `"column":"SV_StandHeight","controlId":`, 1),
		strings.Replace(string(body), `"option":1`, `"option":"1"`, 1),
		strings.Replace(string(body), `"option":1`, `"option":1.5`, 1),
		strings.Replace(string(body), `"controlId":"form:frmSIVIsite/optPlotType"`, `"controlId":"\ud800"`, 1),
		string(body) + `{}`,
	} {
		var request SIVIParentActionWrite
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("unknown/coerced/malformed action transport accepted", raw[:100])
		}
	}
}
