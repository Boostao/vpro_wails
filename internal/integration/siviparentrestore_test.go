package integration

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

func TestSIVIParentDirectWriterAllDomainSubsetsAndRestorationAliases(t *testing.T) {
	service, state, db, parent, all := sftest.SiviParentWriteFixture(t, true, 3)
	for mask := 1; mask < 16; mask++ {
		t.Run(strconv.Itoa(mask), func(t *testing.T) {
			drafts := sourceform.SiviParentDirectEdits{}
			count := 0
			if mask&1 != 0 {
				drafts.Scalars = all.Scalars
				count += 7
			}
			if mask&2 != 0 {
				drafts.Options = all.Options
				count += 2
			}
			if mask&4 != 0 {
				drafts.Text = all.Text
				count += 2
			}
			if mask&8 != 0 {
				drafts.Categorical = all.Categorical
				count += 3
			}
			written, err := sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, "108050", parent, drafts)
			if err != nil || written == nil || written.ChangedCells != count || written.HistoryID == "" {
				t.Fatal("domain subset changed assignment/history shape", written, count, err)
			}
			var proposal string
			if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIParentHistory" WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
				t.Fatal(err)
			}
			var event sourceform.SiviParentHistory
			if err := json.Unmarshal([]byte(proposal), &event); err != nil {
				t.Fatal(err)
			}
			for _, change := range event.Changes {
				alias := "sAmPlE_EnV"
				if change.Table == "Sample_Admin" {
					alias = "_aDmIn"
				}
				if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE rowid=?`, alias, change.Audit.RowID); err != nil {
					t.Fatal(err)
				}
			}
			action := vprocore.AuditRestoreRetain
			if mask%2 == 0 {
				action = vprocore.AuditRestorePrune
			}
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			cancelled, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreCancel)
			if err != nil || cancelled == nil || !cancelled.Cancelled {
				t.Fatal("explicit restoration Cancel failed", cancelled, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			restored, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, action)
			if err != nil || restored == nil || restored.RestoredRows != count || restored.CleanedVegRows != 0 ||
				restored.PrunedAuditRows != map[bool]int{true: count, false: 0}[action == vprocore.AuditRestorePrune] {
				t.Fatal("typed restoration changed counts or cleaned unrelated rows", restored, err)
			}
			fresh, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(fresh, parent) {
				t.Fatal("domain restoration did not recover all literal physical originals", fresh, err)
			}
			for _, change := range event.Changes {
				var exists int
				var flag int
				if err := db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(Restore),0) FROM Sample_Audit WHERE rowid=?`, change.Audit.RowID).Scan(&exists, &flag); err != nil ||
					(action == vprocore.AuditRestoreRetain && (exists != 1 || flag != -1)) ||
					(action == vprocore.AuditRestorePrune && exists != 0) {
					t.Fatal("restore audit true=-1/prune changed", exists, flag, err)
				}
			}
			before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if got, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, action); err == nil || got != nil {
				t.Fatal("typed parent history restoration replayed", got, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
		})
	}
}

func TestSIVIParentDirectWriterAuditStrengthRestoresOnlyAuditedCells(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			service, state, db, _, _ := sftest.SiviParentWriteFixture(t, strength%2 == 0, strength)
			if _, err := db.Exec(`UPDATE Sample_Env SET SV_AhorizonType=NULL WHERE PlotNumber='108050'`); err != nil {
				t.Fatal(err)
			}
			parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			drafts := sourceform.SiviParentDirectEdits{
				Scalars:     []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, "SV_StandHeight", vprotest.SiviReal(-3.25))},
				Text:        []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, "SV_PolygonNumber", vprocore.ProjectMetadataCell{Storage: "null"})},
				Categorical: []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, "SV_AhorizonType", vprotest.MetadataText(""))},
			}
			written, err := sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, "108050", parent, drafts)
			if err != nil || written == nil || written.ChangedCells != 3 || (written.HistoryID == "") != (strength == 0) {
				t.Fatal("audit strength changed writer counts", written, err)
			}
			if strength > 0 {
				action := vprocore.AuditRestoreRetain
				if strength == 2 {
					action = vprocore.AuditRestorePrune
				}
				restored, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, action)
				if err != nil || restored == nil || restored.RestoredRows != strength {
					t.Fatal("restoration widened to unaudited assignments", restored, err)
				}
			}
			fresh, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			numeric, text, category := vprotest.SiviReal(-3.25), vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.MetadataText("")
			if strength >= 1 {
				numeric = vprotest.SiviReal(2)
			}
			if strength >= 2 {
				category = vprocore.ProjectMetadataCell{Storage: "null"}
			}
			if strength == 3 {
				text = vprotest.MetadataText("original polygon")
			}
			if !reflect.DeepEqual(sftest.SiviParentCell(t, fresh, "SV_StandHeight"), numeric) ||
				!reflect.DeepEqual(sftest.SiviParentCell(t, fresh, "SV_PolygonNumber"), text) ||
				!reflect.DeepEqual(sftest.SiviParentCell(t, fresh, "SV_AhorizonType"), category) {
				t.Fatal("audit strength lost typed NULL/empty or rewrote unaudited cells", fresh.Rows)
			}
		})
	}
}

func TestSIVIParentDirectWriterExactUTF16BooleanOptionAndNullWrites(t *testing.T) {
	service, state, _, parent, _ := sftest.SiviParentWriteFixture(t, false, 3)
	polygon, canopy := strings.Repeat("\U0001f600", 12)+"a", strings.Repeat("\U0001f600", 25)
	root, horizon := strings.Repeat("\U0001f600", 50), strings.Repeat("\U0001f600", 2)+"a"
	drafts := sourceform.SiviParentDirectEdits{
		Scalars: []sourceform.SiviParentScalarEdit{
			sftest.SiviParentEdit(t, parent, "SV_FloodPlain", vprotest.MetadataInteger("-1")),
			sftest.SiviParentEdit(t, parent, "SV_StandHeight", vprocore.ProjectMetadataCell{Storage: "null"}),
		},
		Options: []sourceform.SiviParentScalarEdit{
			sftest.SiviParentEdit(t, parent, "SV_StandAgeEstMeas", vprocore.ProjectMetadataCell{Storage: "null"}),
			sftest.SiviParentEdit(t, parent, "SV_StandHeightEstMeas", vprotest.MetadataText("2")),
		},
		Text: []sourceform.SiviParentScalarEdit{
			sftest.SiviParentEdit(t, parent, "SV_PolygonNumber", vprotest.MetadataText(polygon)),
			sftest.SiviParentEdit(t, parent, "SV_CanopyComposition", vprotest.MetadataText(canopy)),
		},
		Categorical: []sourceform.SiviParentScalarEdit{
			sftest.SiviParentEdit(t, parent, "SnowCoverregime", vprotest.MetadataText(" ")),
			sftest.SiviParentEdit(t, parent, "SV_RootZoneTexture", vprotest.MetadataText(root)),
			sftest.SiviParentEdit(t, parent, "SV_AhorizonType", vprotest.MetadataText(horizon)),
		},
	}
	result, err := sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, "108050", parent, drafts)
	if err != nil || result == nil || result.ChangedCells != 9 {
		t.Fatal("exact physical limits/NULL/source option did not commit", result, err)
	}
	fresh, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range [][]sourceform.SiviParentScalarEdit{drafts.Scalars, drafts.Options, drafts.Text, drafts.Categorical} {
		for _, edit := range domain {
			if !reflect.DeepEqual(sftest.SiviParentCell(t, fresh, edit.Column), edit.Value) {
				t.Fatal("boundary write changed literal storage", edit.Column)
			}
		}
	}
	if restored, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", result.HistoryID, vprocore.AuditRestoreRetain); err != nil || restored == nil || restored.RestoredRows != 9 {
		t.Fatal("exact boundary originals were not restored", restored, err)
	}
}

func TestSIVIParentDirectRestorationRejectsDriftCorruptHistoryAndTriggers(t *testing.T) {
	service, state, db, parent, all := sftest.SiviParentWriteFixture(t, false, 3)
	written, err := sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, "108050", parent, all)
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIParentHistory" WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var event sourceform.SiviParentHistory
	if err := json.Unmarshal([]byte(proposal), &event); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ sql, restore string }{
		{`UPDATE Sample_Env SET FieldNumber='drift' WHERE PlotNumber='108050'`, `UPDATE Sample_Env SET FieldNumber=NULL WHERE PlotNumber='108050'`},
		{`UPDATE Sample_Admin SET StrataCoverTotal=99 WHERE Plot='108050'`, `UPDATE Sample_Admin SET StrataCoverTotal=123.25 WHERE Plot='108050'`},
		{`UPDATE Sample_Audit SET Flag=-1 WHERE rowid=` + event.Changes[0].Audit.RowID,
			`UPDATE Sample_Audit SET Flag=0 WHERE rowid=` + event.Changes[0].Audit.RowID},
		{`UPDATE Sample_Audit SET ID=3 WHERE rowid=` + event.Changes[0].Audit.RowID,
			`UPDATE Sample_Audit SET ID=NULL WHERE rowid=` + event.Changes[0].Audit.RowID},
		{`UPDATE Sample_Audit SET "Table"='_Veg' WHERE rowid=` + event.Changes[0].Audit.RowID,
			`UPDATE Sample_Audit SET "Table"='_Env' WHERE rowid=` + event.Changes[0].Audit.RowID},
		{`INSERT INTO Sample_Admin(Plot) VALUES('108050 ')`, `DELETE FROM Sample_Admin WHERE Plot='108050 ' COLLATE BINARY`},
		{`CREATE TRIGGER RestoreUnexpected AFTER UPDATE OF SV_StandHeight ON Sample_Env
			BEGIN UPDATE Sample_Env SET FieldNumber='BAD' WHERE PlotNumber='108051'; END`, `DROP TRIGGER RestoreUnexpected`},
		{`CREATE TRIGGER RestoreHistoryUnexpected AFTER UPDATE OF Restored ON "__VPRO_SIVIParentHistory"
			BEGIN UPDATE Sample_Admin SET PlotType='BAD' WHERE Plot='108051'; END`, `DROP TRIGGER RestoreHistoryUnexpected`},
	} {
		if _, err := db.Exec(test.sql); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
			t.Fatal("drift/trigger authorized destructive restoration/pruning", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.restore); err != nil {
			t.Fatal(err)
		}
	}
	for _, corrupt := range []string{
		"{}", proposal + " {}", strings.Replace(proposal, `"Changes":`, `"Unknown":1,"Changes":`, 1),
		strings.Replace(proposal, "SV_StandHeight", "SpeciesListComplete", -1),
		strings.Replace(proposal, `"original polygon"`, `"\ud800"`, 1),
		strings.Replace(proposal, `"original polygon"`, `"tampered"`, 1),
	} {
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIParentHistory" SET Proposal=? WHERE ID=?`, corrupt, written.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
			t.Fatal("malformed/corrupt typed history restored/pruned", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIParentHistory" SET Proposal=? WHERE ID=?`, proposal, written.HistoryID); err != nil {
		t.Fatal(err)
	}
	if restored, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestorePrune); err != nil || restored == nil || restored.RestoredRows != 14 || restored.PrunedAuditRows != 14 {
		t.Fatal("rejected restoration poisoned complete prune retry", restored, err)
	}
}

func TestSIVIParentDirectRestorationCancellationAndReopenedOwner(t *testing.T) {
	service, state, db, parent, all := sftest.SiviParentWriteFixture(t, true, 3)
	written, err := sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, "108050", parent, all)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	for _, bad := range []struct {
		id, history string
		action      vprocore.AuditRestoreAction
	}{
		{"stale", written.HistoryID, vprocore.AuditRestoreRetain},
		{state.ContextID, "01", vprocore.AuditRestoreRetain},
		{state.ContextID, written.HistoryID, vprocore.AuditRestoreAction("invalid")},
	} {
		if got, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), bad.id, "108050", bad.history, bad.action); err == nil || got != nil {
			t.Fatal("invalid restoration owner/action/history succeeded", got, err)
		}
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if got, err := sourceform.CtxRestoreSIVIParentDirect(service, cancelled, state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled restoration succeeded", got, err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	got, restoreErr := sourceform.CtxRestoreSIVIParentDirect(service, ctx, state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain)
	cancel()
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(restoreErr, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked restoration returned success/partial result", got, restoreErr)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil || next.ContextID == state.ContextID {
		t.Fatal("reopened fixture did not rotate context identity", next, err)
	}
	restored, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), next.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain)
	if err != nil || restored == nil || restored.RestoredRows != 14 {
		t.Fatal("same physical owner could not restore after reopening", restored, err)
	}
	fresh, err := sourceform.ReadSIVIParent(service, context.Background(), next.ContextID, "108050")
	expected := *parent
	expected.ContextID = next.ContextID
	if err != nil || !reflect.DeepEqual(fresh, &expected) {
		t.Fatal("reopened restoration lost original physical storage", fresh, err)
	}
}
