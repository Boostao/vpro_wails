package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func siviPublicActionRequest(original *sourceform.SiviParentProjection, actions []sourceform.SiviParentActionEdit) SIVIParentActionWrite {
	request := SIVIParentActionWrite{Original: original, Actions: []SIVIParentActionInput{}}
	for _, action := range actions {
		request.Actions = append(request.Actions, SIVIParentActionInput{
			action.ContextID, action.ControlID, action.Table, action.RowID, vprocore.CloneSiteUnitCell(action.Expected), action.Option,
		})
	}
	return request
}

func TestSIVIParentPublicActionOriginalAndCancellation(t *testing.T) {
	service, state, original, _ := siviParentActionWriteFixture(t, false, 3)
	got, err := GetSIVIParentActionOriginal(service, context.Background(), state.ContextID, original.Plot)
	if err != nil || !reflect.DeepEqual(got, original) {
		t.Fatal("owned action original differs", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GetSIVIParentActionOriginal(service, ctx, state.ContextID, original.Plot); !errors.Is(err, context.Canceled) {
		t.Fatal("action original discarded caller cancellation", err)
	}
}

func TestSIVIParentPublicActionStrictWireSaveRestore(t *testing.T) {
	service, state, original, actions := siviParentActionWriteFixture(t, true, 3)
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
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	result, err := SaveSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, request)
	if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") {
		t.Fatal("strict wire silently accepted a new source Species NULL", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	no := 2
	request.Actions[1].Option = &no
	result, err = SaveSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, request)
	if err != nil || result == nil || result.ChangedCells != 2 || result.HistoryID == "" || !result.SourceRefreshRequired {
		t.Fatal("raw explicit action request failed", result, err)
	}
	restored, err := RestoreSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || restored.RestoredRows != 2 || restored.PrunedAuditRows != 2 || restored.CleanedVegRows != 0 {
		t.Fatal("public source action history restoration failed", restored, err)
	}
	fresh, err := GetSIVIParentActionOriginal(service, context.Background(), state.ContextID, original.Plot)
	if err != nil || !reflect.DeepEqual(fresh, original) {
		t.Fatal("public action original did not recover exact historical values", fresh, err)
	}
	if _, err := RestoreSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestorePrune); err == nil {
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
