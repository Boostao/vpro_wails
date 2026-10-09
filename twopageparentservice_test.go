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

func TestTwoPageParentExtraServiceIndependentGatesAndOwnedReads(t *testing.T) {
	contexts, state, _, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	for _, review := range []bool{false, true} {
		for _, editing := range []bool{false, true} {
			service, err := NewTwoPageParentExtraService(contexts, func(name string) (string, bool) {
				if name == twoPageParentReviewEnvironment {
					return strconv.FormatBool(review), true
				}
				if name == twoPageParentExtraEditingEnvironment {
					return strconv.FormatBool(editing), true
				}
				t.Fatal("unexpected inherited authorization lookup", name)
				return "", false
			})
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
			if review {
				if err != nil || !reflect.DeepEqual(got, parent) {
					t.Fatal("two-page review depends on an unrelated SIVI gate", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatal("disabled review published original", got, err)
			}
			if !review || !editing {
				if got, err := service.Save(context.Background(), state.ContextID, "108050", parent.Form, TwoPageParentExtraWrite{parent, edits}); err == nil || got != nil {
					t.Fatal("disabled service authorized writes", got, err)
				}
				if got, err := service.Restore(context.Background(), state.ContextID, "108050", parent.Form, "1", AuditRestoreCancel); err == nil || got != nil {
					t.Fatal("disabled service bypassed restoration authorization", got, err)
				}
			}
			assertProfileSUFiles(t, contexts, before)
		}
	}
	if _, err := NewTwoPageParentExtraService(nil, func(string) (string, bool) { return "true", true }); err == nil {
		t.Fatal("nil owner accepted")
	}
	if _, err := NewTwoPageParentExtraService(contexts, nil); err == nil {
		t.Fatal("implicit environment lookup accepted")
	}
	for _, name := range []string{twoPageParentReviewEnvironment, twoPageParentExtraEditingEnvironment} {
		if _, err := NewTwoPageParentExtraService(contexts, func(key string) (string, bool) {
			if key == name {
				return "invalid boolean", true
			}
			return "true", true
		}); err == nil {
			t.Fatal("malformed flag silently defaulted", name)
		}
	}
	service, err := NewTwoPageParentExtraService(contexts, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form); err == nil || got != nil {
		t.Fatal("missing flags enabled review", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.Save(ctx, state.ContextID, "108050", parent.Form, TwoPageParentExtraWrite{parent, edits}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled service returned a gate error or committed", got, err)
	}
}

func TestTwoPageParentExtraServiceTypedRawUnicodeAndStrictTransport(t *testing.T) {
	_, _, _, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	request := TwoPageParentExtraWrite{parent, edits}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`"  cArD  "`, strconv.Quote(parent.ContextID), `"column":"GIS_BGC"`} {
		for _, replacement := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\udfff\ud800"`, "\"" + string([]byte{0xff}) + "\""} {
			value := replacement
			if strings.HasPrefix(marker, `"column"`) {
				value = `"column":` + replacement
			}
			bad := strings.Replace(string(wire), marker, value, 1)
			if bad == string(wire) {
				t.Fatal("raw ingestion marker missing", marker)
			}
			decoded := request
			if err := json.Unmarshal([]byte(bad), &decoded); err == nil || !reflect.DeepEqual(decoded, request) {
				t.Fatal("malformed raw Unicode repaired or partially replaced request", marker, replacement, err)
			}
		}
	}
	valid := strings.Replace(string(wire), `"  cArD  "`, `"\ud83d\ude00"`, 1)
	var decoded TwoPageParentExtraWrite
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil || *decoded.Edits[2].Value.Text != "\U0001f600" {
		t.Fatal("valid raw Unicode pair changed", err)
	}
	if _, err := planTwoPageParentExtraProjection(context.Background(), parent, decoded.Edits); err != nil {
		t.Fatal("valid transport cannot reach source-typed planning", err)
	}
	for _, bad := range []string{
		`null`, `{}`, `{"original":null}`, `{"edits":[]}`, `{"original":null,"edits":[],"unknown":1}`,
		`{"original":null,"edits":[],"edits":[]}`, string(wire) + `{}`,
	} {
		decoded := request
		if err := json.Unmarshal([]byte(bad), &decoded); err == nil || !reflect.DeepEqual(decoded, request) {
			t.Fatal("malformed request defaulted or partially replaced receiver", bad, err)
		}
	}
}

func TestTwoPageParentExtraServiceEnabledSaveRestoreAndScopeDenial(t *testing.T) {
	contexts, state, _, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", true)
	service, err := NewTwoPageParentExtraService(contexts, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	written, err := service.Save(context.Background(), state.ContextID, "108050", parent.Form, TwoPageParentExtraWrite{parent, edits})
	if err != nil || written == nil || written.ChangedCells != 8 || written.HistoryID == "" {
		t.Fatal("facade changed private transaction result", written, err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := service.Restore(context.Background(), state.ContextID, "108050", "FS882-8x6XL", written.HistoryID, AuditRestorePrune); err == nil || got != nil {
		t.Fatal("normal source consumed CHARS history", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
	restored, err := service.Restore(context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 8 || restored.PrunedAuditRows != 8 {
		t.Fatal("facade changed typed restoration", restored, err)
	}
	got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
	if err != nil || !reflect.DeepEqual(got, parent) {
		t.Fatal("facade restoration changed physical originals", err)
	}
}
