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

func TestTwoPageParentCommonServiceOwnedReadsAndCancellation(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	request := twopage.TwoPageParentCommonWrite{Original: parent, Edits: twoPageCommonEdits(t, parent)}
	if _, err := twopage.NewTwoPageParentCommonService(nil); err == nil {
		t.Fatal("common nil context accepted")
	}
	service, err := twopage.NewTwoPageParentCommonService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
	if err != nil || !reflect.DeepEqual(got, parent) {
		t.Fatal("common review did not preserve exact source", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.Save(ctx, state.ContextID, "108050", parent.Form, request); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled common save returned a gate error or committed", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestTwoPageParentCommonServiceStrictRawTransport(t *testing.T) {
	_, _, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", false)
	request := twopage.TwoPageParentCommonWrite{Original: parent, Edits: twoPageCommonEdits(t, parent)}
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
	var decoded twopage.TwoPageParentCommonWrite
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil {
		t.Fatal("valid Unicode pair rejected", err)
	}
	if _, err := twopage.PlanTwoPageParentCommonProjection(context.Background(), parent, decoded.Edits); err != nil {
		t.Fatal("common strict transport cannot reach typed planning", err)
	}
}

func TestTwoPageParentCommonServiceEnabledSaveRestoreAndVariantIsolation(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", true)
	service, err := twopage.NewTwoPageParentCommonService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	written, err := service.Save(context.Background(), state.ContextID, "108050", parent.Form,
		twopage.TwoPageParentCommonWrite{Original: parent, Edits: twoPageCommonEdits(t, parent)})
	if err != nil || written == nil || written.ChangedCells != 14 {
		t.Fatal("common facade changed typed transaction scope", written, err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := service.Restore(context.Background(), state.ContextID, "108050", "FS882-8x6XL", written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
		t.Fatal("normal variant consumed CHARS common history", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	restored, err := service.Restore(context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 14 || restored.PrunedAuditRows != 14 {
		t.Fatal("common facade changed typed restoration scope", restored, err)
	}
	got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", parent.Form)
	if err != nil || !reflect.DeepEqual(got, parent) {
		t.Fatal("common facade did not restore original physical values", err)
	}
}
