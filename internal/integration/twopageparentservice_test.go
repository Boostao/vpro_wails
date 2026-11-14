package integration

import (
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestTwoPageParentExtraServiceOwnedReadsAndCancellation(t *testing.T) {
	contexts, state, _, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	if _, err := twopage.NewTwoPageParentExtraService(nil); err == nil {
		t.Fatal("nil owner accepted")
	}
	service, err := twopage.NewTwoPageParentExtraService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
	if err != nil || !reflect.DeepEqual(got, parent) {
		t.Fatal("two-page review did not preserve source", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.Save(ctx, state.ContextID, "108050", parent.Form, twopage.TwoPageParentExtraWrite{Original: parent, Edits: edits}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled service returned a gate error or committed", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestTwoPageParentExtraServiceTypedRawUnicodeAndStrictTransport(t *testing.T) {
	_, _, _, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	request := twopage.TwoPageParentExtraWrite{Original: parent, Edits: edits}
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
	var decoded twopage.TwoPageParentExtraWrite
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil || *decoded.Edits[2].Value.Text != "\U0001f600" {
		t.Fatal("valid raw Unicode pair changed", err)
	}
	if _, err := twopage.PlanTwoPageParentExtraProjection(context.Background(), parent, decoded.Edits); err != nil {
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
	service, err := twopage.NewTwoPageParentExtraService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	written, err := service.Save(context.Background(), state.ContextID, "108050", parent.Form, twopage.TwoPageParentExtraWrite{Original: parent, Edits: edits})
	if err != nil || written == nil || written.ChangedCells != 8 || written.HistoryID == "" {
		t.Fatal("facade changed private transaction result", written, err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := service.Restore(context.Background(), state.ContextID, "108050", "FS882-8x6XL", written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
		t.Fatal("normal source consumed CHARS history", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	restored, err := service.Restore(context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 8 || restored.PrunedAuditRows != 8 {
		t.Fatal("facade changed typed restoration", restored, err)
	}
	got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
	if err != nil || !reflect.DeepEqual(got, parent) {
		t.Fatal("facade restoration changed physical originals", err)
	}
}
