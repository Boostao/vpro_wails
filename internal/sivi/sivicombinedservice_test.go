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
	"strconv"
	"strings"
	"testing"
	"time"
)

func siviCombinedServiceFixture(t *testing.T, contexts *vprocore.ContextService) *SIVICombinedService {
	t.Helper()
	service, err := NewSIVICombinedService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func siviCombinedRequestJSON(t *testing.T, original []sourceform.SiviVegetationProjection, edits []sourceform.SiviHeightEdit) string {
	t.Helper()
	request := SIVICombinedWrite{Original: original, Edits: []SIVICoverEdit{}}
	for _, edit := range edits {
		request.Edits = append(request.Edits, SIVICoverEdit{edit.RowID, edit.Form, edit.Column, edit.Expected, edit.Value})
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func siviCombinedFixtureEdits(heights []sourceform.SiviHeightEdit, extended bool) []sourceform.SiviHeightEdit {
	form := "SubVegA-SIVI_BC"
	if extended {
		form = "SubVegA-SIVI"
	}
	row := heights[0].RowID
	return []sourceform.SiviHeightEdit{
		{row, form, "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(100)},
		{row, form, "Cover1", vprotest.SiviReal(0), vprocore.ProjectMetadataCell{Storage: "null"}},
		{row, form, "HeightB", vprotest.MetadataText("before"), vprotest.MetadataText("")},
		{row, "SubVegC-SIVI", "Cover6", vprotest.SiviReal(0), vprocore.ProjectMetadataCell{Storage: "null"}},
		{row, "SubVegC-SIVI", "Height6", vprotest.SiviReal(-3), vprotest.SiviReal(100)},
	}
}

func TestSIVICombinedServiceNilAndCanceled(t *testing.T) {
	contexts, state, _, original, heights := sftest.SiviWriteFixture(t, false, 3)
	valid := siviCombinedRequestJSON(t, original, siviCombinedFixtureEdits(heights, false))
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if service, err := NewSIVICombinedService(nil); err == nil || service != nil {
		t.Fatal("constructor accepted nil owner", service, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*SIVICombinedService{nil, {}, siviCombinedServiceFixture(t, contexts)} {
		for _, requestContext := range []context.Context{nil, ctx} {
			if result, err := service.GetOriginal(requestContext, state.ContextID, "108050", false); err == nil || result != nil {
				t.Fatal("nil/canceled read succeeded", result, err)
			}
			if result, err := service.SaveReviewed(requestContext, state.ContextID, "108050", false, valid); err == nil || result != nil {
				t.Fatal("nil/canceled Save succeeded", result, err)
			}
			if result, err := service.RestoreReviewed(requestContext, state.ContextID, "108050", "1", vprocore.AuditRestorePrune); err == nil || result != nil {
				t.Fatal("nil/canceled restore succeeded", result, err)
			}
		}
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestSIVICombinedServiceMixedTransactionHistoryAndTypedRecovery(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for _, action := range []vprocore.AuditRestoreAction{vprocore.AuditRestoreRetain, vprocore.AuditRestorePrune} {
			t.Run(strconv.FormatBool(extended)+"/"+string(action), func(t *testing.T) {
				contexts, state, db, original, heights := sftest.SiviWriteFixture(t, extended, 3)
				service := siviCombinedServiceFixture(t, contexts)
				original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", extended)
				if err != nil {
					t.Fatal(err)
				}
				edits := siviCombinedFixtureEdits(heights, extended)
				saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", extended, siviCombinedRequestJSON(t, original, edits))
				if err != nil || saved == nil || saved.ChangedCells != 5 || saved.HistoryID == "" {
					t.Fatal("mixed facade Save failed", saved, err)
				}
				var a, c float64
				var coverA, coverC *float64
				var b, species string
				if err := db.QueryRow(`SELECT Cover1,Cover6,HeightA,HeightB,Height6,Species FROM Sample_Veg WHERE ID=10000001`).
					Scan(&coverA, &coverC, &a, &b, &c, &species); err != nil || coverA != nil || coverC != nil || a != 100 || c != 100 || b != "" || species != "RAW" {
					t.Fatal("mixed physical transaction changed unintended state", coverA, coverC, a, b, c, species, err)
				}
				fresh, err := service.GetOriginal(context.Background(), state.ContextID, "108050", extended)
				if err != nil {
					t.Fatal(err)
				}
				for _, group := range fresh {
					for _, row := range group.Rows {
						if row.RowID == heights[0].RowID {
							t.Fatal("height-only row invented cover query membership", group.Form)
						}
					}
				}
				var count int
				var proposal string
				if err := db.QueryRow(`SELECT COUNT(*),Proposal FROM "__VPRO_SIVICombinedHistory"`).Scan(&count, &proposal); err != nil || count != 1 {
					t.Fatal("mixed Save did not create exactly one combined history", count, err)
				}
				var event sourceform.SiviHeightHistory
				if err := json.Unmarshal([]byte(proposal), &event); err != nil || len(event.Changes) != 5 || len(event.Committed) != 1 {
					t.Fatal("combined typed history incomplete", event, err)
				}
				for i, change := range event.Changes {
					if change.Column != edits[i].Column || change.Audit.EditField != edits[i].Column {
						t.Fatal("combined history/audit order differs", i, change)
					}
				}
				if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001`).Scan(&count); err != nil || count != 5 {
					t.Fatal("mixed audit count differs", count, err)
				}
				for _, table := range []string{sourceform.SiviHeightHistoryTable, sourceform.SiviCoverHistoryTable} {
					if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, table).Scan(&count); err != nil || count != 0 {
						t.Fatal("combined Save entered predecessor history", table, count, err)
					}
				}
				before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
				if result, err := sourceform.RestoreSIVIHeights(contexts, context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || result != nil {
					t.Fatal("height restoration entered combined history", result, err)
				}
				if result, err := sourceform.RestoreSIVICovers(contexts, context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || result != nil {
					t.Fatal("cover restoration entered combined history", result, err)
				}
				cancelled, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreCancel)
				if err != nil || cancelled == nil || !cancelled.Cancelled {
					t.Fatal("review cancel failed", cancelled, err)
				}
				vprotest.AssertProfileSUFiles(t, contexts, before)
				if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='Sample_Veg' WHERE ID=10000001`); err != nil {
					t.Fatal(err)
				}
				restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action)
				if err != nil || restored == nil || restored.RestoredRows != 5 || restored.CleanedVegRows != 0 ||
					action == vprocore.AuditRestorePrune && restored.PrunedAuditRows != 5 {
					t.Fatal("combined typed recovery failed", restored, err)
				}
				again, err := service.GetOriginal(context.Background(), state.ContextID, "108050", extended)
				if err != nil || !reflect.DeepEqual(again, original) {
					t.Fatal("combined recovery lost originals/query membership", again, err)
				}
				if action == vprocore.AuditRestoreRetain {
					if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001 AND Restore=-1`).Scan(&count); err != nil || count != 5 {
						t.Fatal("retained audits lost Access true=-1", count, err)
					}
				} else if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001`).Scan(&count); err != nil || count != 0 {
					t.Fatal("pruned audits remain", count, err)
				}
				before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
				if result, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || result != nil {
					t.Fatal("combined recovery replayed", result, err)
				}
				vprotest.AssertProfileSUFiles(t, contexts, before)
			})
		}
	}
}

func TestSIVICombinedServiceStrictRawTransportAndOwnership(t *testing.T) {
	contexts, state, _, original, heights := sftest.SiviWriteFixture(t, false, 3)
	service := siviCombinedServiceFixture(t, contexts)
	valid := siviCombinedRequestJSON(t, original, siviCombinedFixtureEdits(heights, false))
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, body := range []string{
		"{}", "null", "[]", valid + "{}",
		strings.Replace(valid, `"original":`, `"Original":`, 1),
		strings.Replace(valid, `"original":`, `"extra":true,"original":`, 1),
		strings.Replace(valid, `"edits":`, `"edits":[],"edits":`, 1),
		strings.Replace(valid, `"rowId":`, `"extra":true,"rowId":`, 1),
		strings.Replace(valid, `"column":"HeightA"`, `"column":"HeightB","column":"HeightA"`, 1),
		strings.Replace(valid, `"column":"HeightA"`, `"column":"Species"`, 1),
		strings.Replace(valid, `"text":""`, `"text":"\ud800"`, 1),
		strings.Replace(valid, `"text":""`, `"text":"\udc00"`, 1),
		strings.Replace(valid, `"text":""`, `"text":"`+"\xff"+`"`, 1),
		siviCombinedRequestJSON(t, original[:2], siviCombinedFixtureEdits(heights, false)),
		siviCombinedRequestJSON(t, original, nil),
	} {
		if body == valid {
			t.Fatal("malformed wire case did not change payload")
		}
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, body); err == nil || result != nil {
			t.Fatal("malformed/raw repaired combined wire accepted", body, result, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	if result, err := service.GetOriginal(context.Background(), "foreign", "108050", false); err == nil || result != nil {
		t.Fatal("foreign owned read succeeded", result, err)
	}
	if result, err := service.SaveReviewed(context.Background(), "foreign", "108050", false, valid); err == nil || result != nil {
		t.Fatal("foreign owned Save succeeded", result, err)
	}
	if result, err := service.RestoreReviewed(context.Background(), "foreign", "108050", "1", vprocore.AuditRestoreCancel); err == nil || result != nil {
		t.Fatal("foreign owned restore succeeded", result, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestSIVICombinedServiceAuditStrengthsAndNullableHeightB(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			contexts, state, db, original, heights := sftest.SiviWriteFixture(t, strength%2 == 0, strength)
			service := siviCombinedServiceFixture(t, contexts)
			edits := []sourceform.SiviHeightEdit{
				{heights[0].RowID, "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataText("before"), vprocore.ProjectMetadataCell{Storage: "null"}},
				{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)},
				{heights[0].RowID, "SubVegC-SIVI", "Height6", vprotest.SiviReal(-3), vprotest.SiviReal(100)},
			}
			saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCombinedRequestJSON(t, original, edits))
			if err != nil || saved == nil || saved.ChangedCells != 3 {
				t.Fatal("mixed audit-strength Save failed", saved, err)
			}
			var b *string
			var count int
			if err := db.QueryRow(`SELECT HeightB FROM Sample_Veg WHERE ID=10000001`).Scan(&b); err != nil || b != nil {
				t.Fatal("nullable HeightB was repaired to empty", b, err)
			}
			expected := []int{0, 2, 2, 3}[strength]
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001`).Scan(&count); err != nil || count != expected ||
				(saved.HistoryID == "") != (expected == 0) {
				t.Fatal("combined audit strength differs", expected, count, saved, err)
			}
			if expected == 0 {
				if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, siviCombinedHistoryTable).Scan(&count); err != nil || count != 0 {
					t.Fatal("unaudited mixed Save invented restoration history", count, err)
				}
				return
			}
			restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreRetain)
			if err != nil || restored == nil || restored.RestoredRows != expected {
				t.Fatal("combined strength recovery differs", restored, err)
			}
			if err := db.QueryRow(`SELECT HeightB FROM Sample_Veg WHERE ID=10000001`).Scan(&b); err != nil ||
				strength == 3 && (b == nil || *b != "before") || strength != 3 && b != nil {
				t.Fatal("recovery changed unaudited HeightB or lost text original", b, err)
			}
		})
	}
}

func TestSIVICombinedServiceAtomicRollbackCollisionNoopAndRetry(t *testing.T) {
	contexts, state, db, original, heights := sftest.SiviWriteFixture(t, false, 3)
	service := siviCombinedServiceFixture(t, contexts)
	edits := siviCombinedFixtureEdits(heights, false)
	valid := siviCombinedRequestJSON(t, original, edits)
	if _, err := db.Exec(`CREATE TRIGGER SIVICombinedReject BEFORE UPDATE OF HeightB ON Sample_Veg BEGIN SELECT RAISE(ABORT,'controlled late rejection'); END`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || result != nil {
		t.Fatal("late height rejection committed preceding mixed cells", result, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`DROP TRIGGER SIVICombinedReject;
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('108050','DUP',10000001,0)`); err != nil {
		t.Fatal(err)
	}
	collision, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCombinedRequestJSON(t, collision, edits)); err == nil || result != nil {
		t.Fatal("ambiguous application ID committed mixed writes", result, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE ID=10000001 AND Species='DUP';
		UPDATE Sample_Veg SET Cover1=100,HeightB=? WHERE ID=10000001`, strings.Repeat("x", 256)); err != nil {
		t.Fatal(err)
	}
	historical, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	noops := []sourceform.SiviHeightEdit{
		{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(100), vprotest.SiviReal(100)},
		{heights[0].RowID, "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataText(strings.Repeat("x", 256)), vprotest.MetadataText(strings.Repeat("x", 256))},
	}
	before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCombinedRequestJSON(t, historical, noops))
	if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("historical mixed noop produced assignments/history", result, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=0,HeightB='before' WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	result, err = service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid)
	if err != nil || result == nil || result.ChangedCells != 5 {
		t.Fatal("rejected mixed transactions poisoned retry", result, err)
	}
	before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || result != nil {
		t.Fatal("stale mixed original replayed", result, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestSIVICombinedServiceBlockedCancellationAndRetry(t *testing.T) {
	contexts, state, db, original, heights := sftest.SiviWriteFixture(t, false, 3)
	service := siviCombinedServiceFixture(t, contexts)
	valid := siviCombinedRequestJSON(t, original, siviCombinedFixtureEdits(heights, false))
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}
	}()
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if result, err := service.SaveReviewed(ctx, state.ContextID, "108050", false, valid); !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("blocked mixed Save returned partial success", result, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err != nil || result == nil || result.ChangedCells != 5 {
		t.Fatal("canceled mixed Save poisoned retry", result, err)
	}
}
