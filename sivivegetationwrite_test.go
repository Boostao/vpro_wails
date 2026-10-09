package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func siviWriteFixture(t *testing.T, external bool, strength int) (*ContextService, ProjectState, *sql.DB, []siviVegetationProjection, []siviHeightEdit) {
	t.Helper()
	service, state := reportServiceFixture(t, external)
	if err := service.plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["project"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1,HeightA,HeightB,Cover6,Height6)
		VALUES ('108050','RAW',10000001,0,2,'before',0,-3)`); err != nil {
		t.Fatal(err)
	}
	original, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	var rowID string
	for _, row := range original[0].Rows {
		if row.Cells[0].Integer != nil && *row.Cells[0].Integer == "10000001" {
			rowID = row.RowID
		}
	}
	if rowID == "" {
		t.Fatal("controlled source row unavailable")
	}
	return service, state, db, original, []siviHeightEdit{
		{rowID, "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(3)},
		{rowID, "SubVegA-SIVI_BC", "HeightB", metadataText("before"), metadataText("")},
		{rowID, "SubVegC-SIVI", "Height6", siviReal(-3), ProjectMetadataCell{Storage: "null"}},
	}
}

func TestSIVIWriteHooksRejectBeforeMutationAndRetry(t *testing.T) {
	service, state, _, original, edits := siviWriteFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	rejected := errors.New("reviewed reference definitions changed")
	for _, stage := range []string{"prepare", "validate"} {
		t.Run(stage, func(t *testing.T) {
			var prepared, validated bool
			hooks := siviVegetationWriteHooks{
				prepare: func(ctx context.Context, conn *sql.Conn, owner *sqliteContext) error {
					prepared = true
					if owner != service.projects.sqlite || conn == nil || ctx.Err() != nil {
						t.Fatal("hook lost its leased owner or writer")
					}
					if stage == "prepare" {
						return rejected
					}
					return nil
				},
				validate: func(ctx context.Context, tx *sql.Tx, owner *sqliteContext, planned []siviHeightAssignment) error {
					validated = true
					if owner != service.projects.sqlite || len(planned) != 3 {
						t.Fatal("validation lost its complete source plan")
					}
					var audits int
					if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001`).Scan(&audits); err != nil || audits != 0 {
						t.Fatal("reference validation ran after audits", audits, err)
					}
					if _, err := tx.ExecContext(ctx, `UPDATE Sample_Veg SET HeightA=99 WHERE ID=10000001`); err != nil {
						t.Fatal(err)
					}
					return rejected
				},
			}
			result, err := service.writeSIVIVegetationCellsWithHooks(context.Background(), state.ContextID, "108050",
				false, original, edits, siviHeightWritePolicy(), hooks)
			if result != nil || !errors.Is(err, rejected) || !prepared || validated != (stage == "validate") {
				t.Fatal("hook rejection escaped its boundary", result, err, prepared, validated)
			}
			assertProfileSUFiles(t, service, before)
		})
	}
	var validated bool
	result, err := service.writeSIVIVegetationCellsWithHooks(context.Background(), state.ContextID, "108050",
		false, original, edits, siviHeightWritePolicy(), siviVegetationWriteHooks{
			validate: func(context.Context, *sql.Tx, *sqliteContext, []siviHeightAssignment) error {
				validated = true
				return nil
			},
		})
	if err != nil || result == nil || result.ChangedCells != 3 || !validated {
		t.Fatal("verified reference retry failed", result, err, validated)
	}
}

func TestSIVIWriteHooksNoopOmitsReferenceValidationAndHistory(t *testing.T) {
	service, state, _, original, edits := siviWriteFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for index := range edits {
		edits[index].Value = cloneSiteUnitCell(edits[index].Expected)
	}
	result, err := service.writeSIVIVegetationCellsWithHooks(context.Background(), state.ContextID, "108050",
		false, original, edits, siviHeightWritePolicy(), siviVegetationWriteHooks{
			validate: func(context.Context, *sql.Tx, *sqliteContext, []siviHeightAssignment) error {
				t.Fatal("unchanged assignments inherited phantom reference work")
				return nil
			},
		})
	if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("noop created a result-shaped mutation", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIOwnedHeightWritesAuditStrengthsAndTypedRestoration(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			service, state, db, original, edits := siviWriteFixture(t, strength%2 == 0, strength)
			files := databaseBytes(t, service.projects.sqlite.attachments)
			for role, path := range service.projects.sqlite.attachments {
				if path == service.projects.sqlite.attachments["project"] {
					delete(files, role)
				}
			}
			result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits)
			if err != nil || result == nil || result.ChangedCells != 3 {
				t.Fatal(result, err)
			}
			var a float64
			var b string
			var c *float64
			if err := db.QueryRow(`SELECT HeightA,HeightB,Height6 FROM Sample_Veg WHERE ID=10000001`).Scan(&a, &b, &c); err != nil ||
				a != 3 || b != "" || c != nil {
				t.Fatal("typed height write changed NULL/empty/numeric semantics", a, b, c, err)
			}
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			expected := []int{0, 2, 2, 3}[strength]
			if count != expected || (result.HistoryID == "") != (expected == 0) {
				t.Fatal("audit strength or typed history differs", count, result)
			}
			if expected > 0 {
				action := AuditRestoreRetain
				if strength == 2 {
					action = AuditRestorePrune
				}
				if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='Sample_Veg' WHERE ID=10000001`); err != nil {
					t.Fatal(err)
				}
				restored, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", result.HistoryID, action)
				if err != nil || restored == nil || restored.RestoredRows != expected || restored.CleanedVegRows != 0 {
					t.Fatal(restored, err)
				}
				if err := db.QueryRow(`SELECT HeightA,HeightB,Height6 FROM Sample_Veg WHERE ID=10000001`).Scan(&a, &b, &c); err != nil ||
					a != 2 || b != "before" || (c == nil) != (strength != 3) || c != nil && *c != -3 {
					t.Fatal("restoration changed unaudited values or failed typed originals", a, b, c, err)
				}
				if restored.PrunedAuditRows != map[bool]int{true: expected, false: 0}[action == AuditRestorePrune] {
					t.Fatal("pruning count changed", restored)
				}
				beforeReplay := databaseBytes(t, service.projects.sqlite.attachments)
				if again, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", result.HistoryID, action); err == nil || again != nil {
					t.Fatal("completed restoration replayed", again, err)
				}
				assertProfileSUFiles(t, service, beforeReplay)
			}
			assertProfileSUFiles(t, service, files)
		})
	}
}

func TestSIVIOwnedHeightRejectsStaleCollisionTriggerAndRetries(t *testing.T) {
	service, state, db, original, edits := siviWriteFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, contextID := range []string{"stale", state.ContextID} {
		drafts := append([]siviHeightEdit{}, edits...)
		if contextID == state.ContextID {
			drafts[1].Value = metadataText(strings.Repeat("x", 256))
		}
		if result, err := service.writeSIVIHeights(context.Background(), contextID, "108050", false, original, drafts); err == nil || result != nil {
			t.Fatal("stale/invalid batch returned success", result, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('108050','DUP',10000001,0)`); err != nil {
		t.Fatal(err)
	}
	collision, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	collisionBytes := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, collision, edits); err == nil || result != nil {
		t.Fatal("ambiguous application ID was edited through physical rowid", result, err)
	}
	assertProfileSUFiles(t, service, collisionBytes)
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE ID=10000001 AND Species='DUP';
		CREATE TRIGGER SIVIUnexpected AFTER UPDATE OF HeightA ON Sample_Veg
		BEGIN UPDATE Sample_Veg SET Species='BAD' WHERE ID=10000001; END`); err != nil {
		t.Fatal(err)
	}
	triggerBytes := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits); err == nil || result != nil {
		t.Fatal("unexpected trigger change committed", result, err)
	}
	assertProfileSUFiles(t, service, triggerBytes)
	if _, err := db.Exec(`DROP TRIGGER SIVIUnexpected`); err != nil {
		t.Fatal(err)
	}
	result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits)
	if err != nil || result == nil || result.ChangedCells != 3 {
		t.Fatal("rejected transaction poisoned retry", result, err)
	}
}

func TestSIVIOwnedHeightBlockedCancellationAndRetry(t *testing.T) {
	service, state, db, original, edits := siviWriteFixture(t, false, 3)
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
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err := service.writeSIVIHeights(ctx, state.ContextID, "108050", false, original, edits)
	if !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("blocked cancellation returned success/partial result", result, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	assertProfileSUFiles(t, service, before)
	if result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits); err != nil || result == nil {
		t.Fatal("cancelled write discarded coordinator attachments", result, err)
	}
}

func TestSIVIOwnedHeightUnchangedHistoricalDraftIsWriteFree(t *testing.T) {
	service, state, db, _, edits := siviWriteFixture(t, false, 3)
	historical := strings.Repeat("x", 256)
	if _, err := db.Exec(`UPDATE Sample_Veg SET HeightB=? WHERE ID=10000001`, historical); err != nil {
		t.Fatal(err)
	}
	original, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	edits = []siviHeightEdit{{edits[0].RowID, "SubVegA-SIVI_BC", "HeightB", metadataText(historical), metadataText(historical)}}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits)
	if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("unchanged historical draft produced a mutation/history", result, err)
	}
	assertProfileSUFiles(t, service, before)
	edits[0].Value = metadataText("")
	result, err = service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits)
	if err != nil || result == nil {
		t.Fatal(result, err)
	}
	if restored, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestoreRetain); err != nil || restored == nil {
		t.Fatal("typed restoration rejected the proven historical overlength original", restored, err)
	}
	again, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(original, again) {
		t.Fatal("historical literal storage did not round-trip", err)
	}
}
