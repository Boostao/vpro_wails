package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestTwoPageParentCommonServiceIndependentAuthorization(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	request := TwoPageParentCommonWrite{parent, twoPageCommonEdits(t, parent)}
	for _, review := range []bool{false, true} {
		for _, editing := range []bool{false, true} {
			service, err := NewTwoPageParentCommonService(contexts, func(name string) (string, bool) {
				switch name {
				case twoPageParentReviewEnvironment:
					return strconv.FormatBool(review), true
				case twoPageParentCommonEditingEnvironment:
					return strconv.FormatBool(editing), true
				default:
					t.Fatal("common service inherited additional-field or SIVI authorization", name)
					return "", false
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
			if review {
				if err != nil || !reflect.DeepEqual(got, parent) {
					t.Fatal("common review did not preserve exact source", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatal("disabled common review returned an original", got, err)
			}
			if !review || !editing {
				if got, err := service.Save(context.Background(), state.ContextID, "108050", parent.Form, request); err == nil || got != nil {
					t.Fatal("disabled common service wrote", got, err)
				}
				if got, err := service.Restore(context.Background(), state.ContextID, "108050", parent.Form, "1", AuditRestoreCancel); err == nil || got != nil {
					t.Fatal("disabled common service bypassed restoration authorization", got, err)
				}
			}
			assertProfileSUFiles(t, contexts, before)
		}
	}
	for _, name := range []string{twoPageParentReviewEnvironment, twoPageParentCommonEditingEnvironment} {
		if _, err := NewTwoPageParentCommonService(contexts, func(key string) (string, bool) {
			if key == name {
				return "invalid boolean", true
			}
			return "true", true
		}); err == nil {
			t.Fatal("common malformed flag silently defaulted", name)
		}
	}
	if _, err := NewTwoPageParentCommonService(nil, func(string) (string, bool) { return "true", true }); err == nil {
		t.Fatal("common nil context accepted")
	}
	if _, err := NewTwoPageParentCommonService(contexts, nil); err == nil {
		t.Fatal("common implicit environment lookup accepted")
	}
	service, err := NewTwoPageParentCommonService(contexts, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form); err == nil || got != nil {
		t.Fatal("missing common flags enabled review", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.Save(ctx, state.ContextID, "108050", parent.Form, request); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled common save returned a gate error or committed", got, err)
	}
}

func TestTwoPageParentCommonServiceStrictRawTransport(t *testing.T) {
	_, _, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", false)
	request := TwoPageParentCommonWrite{parent, twoPageCommonEdits(t, parent)}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`"  Custom  "`, strconv.Quote(parent.ContextID), `"column":"PlotType"`} {
		for _, replacement := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\udfff\ud800"`, "\"" + string([]byte{0xff}) + "\""} {
			value := replacement
			if strings.HasPrefix(marker, `"column"`) {
				value = `"column":` + replacement
			}
			bad := strings.Replace(string(wire), marker, value, 1)
			if bad == string(wire) {
				t.Fatal("common raw ingestion marker missing", marker)
			}
			decoded := request
			if err := json.Unmarshal([]byte(bad), &decoded); err == nil || !reflect.DeepEqual(decoded, request) {
				t.Fatal("common raw Unicode repaired or receiver partially replaced", err)
			}
		}
	}
	for _, bad := range []string{`null`, `{}`, `{"original":null}`, `{"edits":[]}`,
		`{"original":null,"edits":[],"unknown":1}`, `{"original":null,"edits":[],"edits":[]}`, string(wire) + `{}`} {
		decoded := request
		if err := json.Unmarshal([]byte(bad), &decoded); err == nil || !reflect.DeepEqual(decoded, request) {
			t.Fatal("common malformed request defaulted or receiver partially replaced", err)
		}
	}
	valid := strings.Replace(string(wire), `"  Custom  "`, `"\ud83d\ude00"`, 1)
	var decoded TwoPageParentCommonWrite
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil {
		t.Fatal("valid Unicode pair rejected", err)
	}
	if _, err := planTwoPageParentCommonProjection(context.Background(), parent, decoded.Edits); err != nil {
		t.Fatal("common strict transport cannot reach typed planning", err)
	}
}

func TestTwoPageParentCommonServiceEnabledSaveRestoreAndVariantIsolation(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", true)
	service, err := NewTwoPageParentCommonService(contexts, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	written, err := service.Save(context.Background(), state.ContextID, "108050", parent.Form,
		TwoPageParentCommonWrite{parent, twoPageCommonEdits(t, parent)})
	if err != nil || written == nil || written.ChangedCells != 14 {
		t.Fatal("common facade changed typed transaction scope", written, err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := service.Restore(context.Background(), state.ContextID, "108050", "FS882-8x6XL", written.HistoryID, AuditRestorePrune); err == nil || got != nil {
		t.Fatal("normal variant consumed CHARS common history", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
	restored, err := service.Restore(context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 14 || restored.PrunedAuditRows != 14 {
		t.Fatal("common facade changed typed restoration scope", restored, err)
	}
	got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
	if err != nil || !reflect.DeepEqual(got, parent) {
		t.Fatal("common facade did not restore original physical values", err)
	}
}
