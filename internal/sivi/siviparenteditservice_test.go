package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
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

func TestSIVIParentDirectOwnedJSONWriteAndRestoration(t *testing.T) {
	service, state, _, original, edits := sftest.SiviParentWriteFixture(t, true, 3)
	request := sourceform.SIVIParentDirectWrite{Original: original, Scalars: edits.Scalars, Options: edits.Options, Text: edits.Text, Categorical: edits.Categorical}
	ctx := context.Background()
	if got, err := sourceform.GetSIVIParentDirectOriginal(service, ctx, state.ContextID, "108050"); err != nil || !reflect.DeepEqual(got, original) {
		t.Fatal("direct editor original lost owned source shape", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := sourceform.SaveSIVIParentDirect(service, cancelled, state.ContextID, "108050", request); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("public save ignored pre-cancellation", got, err)
	}
	if got, err := sourceform.SaveSIVIParentDirect(service, ctx, "stale", "108050", request); err == nil || got != nil {
		t.Fatal("public save ignored stale context", got, err)
	}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded sourceform.SIVIParentDirectWrite
	if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("public raw fourteen-field JSON roundtrip changed storage/ownership", err)
	}
	written, err := sourceform.SaveSIVIParentDirect(service, ctx, state.ContextID, "108050", decoded)
	if err != nil || written == nil || written.ChangedCells != 14 || written.HistoryID == "" {
		t.Fatal("public decoded fourteen-field write failed", written, err)
	}
	if got, err := sourceform.RestoreSIVIParentDirect(service, ctx, "stale", "108050", written.HistoryID, vprocore.AuditRestoreRetain); err == nil || got != nil {
		t.Fatal("public restoration ignored stale context", got, err)
	}
	restored, err := sourceform.RestoreSIVIParentDirect(service, ctx, state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain)
	if err != nil || restored == nil || restored.RestoredRows != 14 {
		t.Fatal("public restoration failed typed owned history", restored, err)
	}
	fresh, err := GetSIVIParentOriginal(service, ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(fresh, original) {
		t.Fatal("public save/restore lost raw parent originals", err)
	}
}

func TestSIVIParentDirectTransportRejectsOmissionsUnicodeAndActions(t *testing.T) {
	service, state, _, original, edits := sftest.SiviParentWriteFixture(t, false, 3)
	request := sourceform.SIVIParentDirectWrite{Original: original, Scalars: edits.Scalars, Options: edits.Options, Text: edits.Text, Categorical: edits.Categorical}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(wire, &properties); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"original", "scalars", "options", "text", "categorical"} {
		for _, omit := range []bool{false, true} {
			t.Run(key, func(t *testing.T) {
				copy := map[string]json.RawMessage{}
				for name, value := range properties {
					copy[name] = value
				}
				if omit {
					delete(copy, key)
				} else {
					copy[key] = json.RawMessage("null")
				}
				bad, err := json.Marshal(copy)
				if err != nil {
					t.Fatal(err)
				}
				var decoded sourceform.SIVIParentDirectWrite
				if err := json.Unmarshal(bad, &decoded); err == nil {
					t.Fatal("public request omitted/defaulted original/domain", key)
				}
			})
		}
	}
	for _, bad := range []string{
		string(wire) + "{}",
		strings.Replace(string(wire), `"original":`, `"actions":[],"original":`, 1),
		strings.Replace(string(wire), `"Literal canopy"`, `"\ud800"`, 1),
		strings.Replace(string(wire), `"Literal canopy"`, "\""+string([]byte{0xff})+"\"", 1),
		strings.Replace(string(wire), `"contextId":`, `"Unknown":0,"contextId":`, 1),
		strings.Replace(string(wire), `"column":`, `"COLUMN":`, 1),
	} {
		var decoded sourceform.SIVIParentDirectWrite
		if err := json.Unmarshal([]byte(bad), &decoded); err == nil {
			t.Fatal("unknown/malformed/defaulted transport decoded")
		}
	}
	cellWire, err := json.Marshal(edits.Text[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"contextId", "table", "rowId", "column", "expected", "value"} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(cellWire, &fields); err != nil {
			t.Fatal(err)
		}
		delete(fields, key)
		bad, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		var decoded sourceform.SIVIParentCellEdit
		if err := json.Unmarshal(bad, &decoded); err == nil {
			t.Fatal("direct cell omitted its explicit ownership/value", key)
		}
	}
	request.Categorical = append(request.Categorical, sftest.SiviParentEdit(t, original, "PlotType", vprotest.MetadataText("Ground")))
	if got, err := sourceform.SaveSIVIParentDirect(service, context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
		t.Fatal("public direct transport widened to callback target", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}
