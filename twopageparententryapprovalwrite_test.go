package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTwoPageEntryOwnedReferenceWriterModesAliasesAtomicRetryAndRestore(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, external := range []bool{false, true} {
			for _, projectSource := range []int{1, 2} {
				for _, workingSource := range []int{1, 2, 3} {
					provider, contexts, state := twoPageEntryProviderSUFixture(t, form, external)
					if err := contexts.projects.preferences.update("Current", map[string]any{
						"ProjectIdSource": projectSource, "AssignedSuSource": workingSource,
					}); err != nil {
						t.Fatal(err)
					}
					snapshot, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", form,
						metadataText("BG"), metadataText("xh1"), provider.readers)
					if err != nil {
						t.Fatal(err)
					}
					edits, acknowledgements := twoPageEntryApprovalDraft(t, snapshot)
					unit := siviParentEdit(t, snapshot.Original, "UserSiteUnit", metadataText("ZZ"))
					edits = append(edits, unit)
					for _, reference := range snapshot.Fields {
						if reference.Column == "UserSiteUnit" {
							acknowledgements = append(acknowledgements, twoPageEntryCodeAcknowledgement{
								ContextID: state.ContextID, Project: snapshot.Original.Project, Plot: "108050", Form: form,
								Table: unit.Table, RowID: unit.RowID, Column: unit.Column, Expected: unit.Expected, Value: unit.Value, Reference: reference,
							})
						}
					}
					selection := twoPageEntryReferenceSelection{projectSource, workingSource}
					before := databaseBytes(t, contexts.projects.sqlite.attachments)
					if result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", form,
						snapshot.Original, edits, false, provider.readers, selection, nil); err == nil || result != nil {
						t.Fatal("fixed owned writer accepted absent optional acknowledgement", result, err)
					}
					assertProfileSUFiles(t, contexts, before)
					if !contexts.projects.preferences.mu.TryLock() {
						t.Fatal("rejected owned writer leaked source preference lease")
					}
					contexts.projects.preferences.mu.Unlock()
					result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", form,
						snapshot.Original, edits, false, provider.readers, selection, acknowledgements)
					if err != nil || result == nil || result.ChangedCells != len(edits) || result.HistoryID == "" {
						t.Fatal("owned source modes/aliases could not atomically retry", form, external, selection, result, err)
					}
					if !contexts.projects.preferences.mu.TryLock() {
						t.Fatal("successful owned writer leaked source preference lease")
					}
					contexts.projects.preferences.mu.Unlock()
					restored, err := contexts.restoreTwoPageEntry(context.Background(), state.ContextID, "108050", form,
						result.HistoryID, AuditRestorePrune, false)
					if err != nil || restored == nil || restored.RestoredRows != len(edits) {
						t.Fatal("owned source modes/aliases could not restore mixed history", restored, err)
					}
					original, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
					if err != nil || !reflect.DeepEqual(original, snapshot.Original) {
						t.Fatal("owned writer restoration changed physical original", original, err)
					}
				}
			}
		}
	}
}

type twoPageEntryLeaseProbeBEC struct {
	original interface {
		ListBECZones(context.Context) ([]BECZone, error)
		ListBECSubZones(context.Context, *string) ([]BECSubZone, error)
	}
	probe func(context.Context) error
}

func (s twoPageEntryLeaseProbeBEC) ListBECZones(ctx context.Context) ([]BECZone, error) {
	if err := s.probe(ctx); err != nil {
		return nil, err
	}
	return s.original.ListBECZones(ctx)
}

func (s twoPageEntryLeaseProbeBEC) ListBECSubZones(ctx context.Context, zone *string) ([]BECSubZone, error) {
	return s.original.ListBECSubZones(ctx, zone)
}

func TestTwoPageEntryOwnedReferenceWriterPreferenceLeaseDriftCancellationAndRetry(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderSUFixture(t, "FS882-8x6XL", true)
	if err := contexts.projects.preferences.update("Current", map[string]any{"AssignedSuSource": 3}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
		metadataText("BG"), metadataText("xh1"), provider.readers)
	if err != nil {
		t.Fatal(err)
	}
	edits, acknowledgements := twoPageEntryApprovalDraft(t, snapshot)
	selection := twoPageEntryReferenceSelection{snapshot.ProjectSource, snapshot.WorkingSource}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	wrong := selection
	wrong.projectSource = 3 - selection.projectSource
	if result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, edits, false, provider.readers, wrong, acknowledgements); err == nil || result != nil || !strings.Contains(err.Error(), "preferences changed") {
		t.Fatal("owned writer accepted stale source preferences", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
	readers := provider.readers
	probed := false
	readers.shared.bec = twoPageEntryLeaseProbeBEC{provider.readers.shared.bec, func(ctx context.Context) error {
		probed = true
		deadline, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
		defer cancel()
		if err := contexts.projects.preferences.compareAndSetProjectIDSource(deadline, selection.projectSource, 3-selection.projectSource); !errors.Is(err, context.DeadlineExceeded) {
			return errors.New("Project source preference escaped the held complete-entry lease")
		}
		for _, source := range []struct{ role, query string }{
			{"VLists", `UPDATE USysTableOfLists SET Item=Item`},
			{"su", `UPDATE ` + quoteHeaderIdentifier(contexts.projects.sqlite.selection.SU+"_SU") + ` SET SiteUnit=SiteUnit`},
		} {
			db, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments[source.role], "rw")+"&_busy_timeout=50")
			if err != nil {
				return err
			}
			_, writeErr := db.ExecContext(ctx, source.query)
			if err := db.Close(); err != nil {
				return err
			}
			if writeErr == nil || (!strings.Contains(writeErr.Error(), "locked") && !strings.Contains(writeErr.Error(), "busy")) {
				t.Fatalf("owned source reference escaped a transaction-held read lock: %s: %v", source.role, writeErr)
			}
		}
		return errors.New("bounded independent reader failure")
	}}
	if result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, edits, false, readers, selection, acknowledgements); err == nil || result != nil || !probed {
		t.Fatal("source preference lease was not observed inside fixed reference approval", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
	contexts.projects.preferences.mu.Lock()
	cancelled, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	result, writeErr := contexts.writeTwoPageEntryWithOwnedReferences(cancelled, state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, edits, false, provider.readers, selection, acknowledgements)
	cancel()
	contexts.projects.preferences.mu.Unlock()
	if !errors.Is(writeErr, context.DeadlineExceeded) || result != nil {
		t.Fatal("source lease wait ignored cancellation", result, writeErr)
	}
	assertProfileSUFiles(t, contexts, before)
	result, err = contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, edits, false, provider.readers, selection, acknowledgements)
	if err != nil || result == nil || result.HistoryID == "" {
		t.Fatal("owned writer failure/cancellation leaked context or source lease", result, err)
	}
}

func TestTwoPageEntryOwnedReferenceWriterCollisionCommitsOneAuditedHistory(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	snapshot, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
		metadataText("BG"), metadataText("xh1"), provider.readers)
	if err != nil {
		t.Fatal(err)
	}
	edits, acknowledgements := twoPageEntryApprovalDraft(t, snapshot)
	selection := twoPageEntryReferenceSelection{snapshot.ProjectSource, snapshot.WorkingSource}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var originalAudits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit`).Scan(&originalAudits); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		result *siviParentWriteResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	for index := 0; index < 2; index++ {
		go func() {
			result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
				snapshot.Original, edits, false, provider.readers, selection, acknowledgements)
			outcomes <- outcome{result, err}
		}()
	}
	success := 0
	for index := 0; index < 2; index++ {
		current := <-outcomes
		if current.err == nil {
			if current.result == nil || current.result.ChangedCells != len(edits) || current.result.HistoryID == "" {
				t.Fatal("colliding writer returned an incomplete commit", current)
			}
			success++
		} else if current.result != nil || !strings.Contains(current.err.Error(), "parents changed") {
			t.Fatal("colliding writer failed outside original ownership guard", current)
		}
	}
	if success != 1 {
		t.Fatal("colliding owned writers committed more or fewer than one original", success)
	}
	history, err := twoPageEntryHistory(snapshot.Original.Form, false)
	if err != nil {
		t.Fatal(err)
	}
	var histories, audits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + quoteHeaderIdentifier(history.table)).Scan(&histories); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if histories != 1 || audits != originalAudits+len(edits) {
		t.Fatal("rejected collision added typed history or phantom audits", histories, audits-originalAudits, len(edits))
	}
}

func TestTwoPageEntryOwnedReferenceWriterExternalSUWALDeniedAndCorrectionRetry(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderSUFixture(t, "FS882-8x6XL", true)
	if err := contexts.projects.preferences.update("Current", map[string]any{"AssignedSuSource": 3}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
		ProjectMetadataCell{Storage: "null"}, ProjectMetadataCell{Storage: "null"}, provider.readers)
	if err != nil {
		t.Fatal(err)
	}
	edit := siviParentEdit(t, snapshot.Original, "UserSiteUnit", metadataText("ZZ"))
	var acknowledgements []twoPageEntryCodeAcknowledgement
	for _, reference := range snapshot.Fields {
		if reference.Column == "UserSiteUnit" {
			acknowledgements = []twoPageEntryCodeAcknowledgement{{ContextID: state.ContextID, Project: snapshot.Original.Project,
				Plot: "108050", Form: snapshot.Original.Form, Table: edit.Table, RowID: edit.RowID, Column: edit.Column,
				Expected: edit.Expected, Value: edit.Value, Reference: reference}}
		}
	}
	su, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["su"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer su.Close()
	var journal string
	if err := su.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&journal); err != nil || journal != "wal" {
		t.Fatal("disposable external SU WAL fixture failed", journal, err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	selection := twoPageEntryReferenceSelection{snapshot.ProjectSource, 3}
	result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, []siviParentScalarEdit{edit}, false, provider.readers, selection, acknowledgements)
	if err == nil || result != nil || !strings.Contains(err.Error(), "sivi_su references require rollback-journal") {
		t.Fatal("external SU WAL snapshot became a stable Working Unit read lock", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if err := su.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&journal); err != nil || journal != "delete" {
		t.Fatal("disposable external SU journal correction failed", journal, err)
	}
	result, err = contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, []siviParentScalarEdit{edit}, false, provider.readers, selection, acknowledgements)
	if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
		t.Fatal("external SU journal correction could not retry Working Unit assignment", result, err)
	}
}

func TestTwoPageEntryOwnedReferenceWriterWALDeniedAndJournalCorrectionRetry(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	snapshot, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
		metadataText("BG"), metadataText("xh1"), provider.readers)
	if err != nil {
		t.Fatal(err)
	}
	edits, acknowledgements := twoPageEntryApprovalDraft(t, snapshot)
	selection := twoPageEntryReferenceSelection{snapshot.ProjectSource, snapshot.WorkingSource}
	lists, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer lists.Close()
	var journal string
	if err := lists.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&journal); err != nil || journal != "wal" {
		t.Fatal("disposable reference WAL fixture failed", journal, err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	noop, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, []siviParentScalarEdit{siviParentEdit(t, snapshot.Original, "FieldNumber",
			siviParentCell(t, snapshot.Original, "FieldNumber"))}, false, provider.readers, selection, nil)
	if err != nil || noop == nil || noop.ChangedCells != 0 || noop.HistoryID != "" {
		t.Fatal("untouched references imposed mutation-only restrictions on a historical noop", noop, err)
	}
	assertProfileSUFiles(t, contexts, before)
	result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, edits, false, provider.readers, selection, acknowledgements)
	if err == nil || result != nil || !strings.Contains(err.Error(), "rollback-journal") {
		t.Fatal("owned writer treated WAL snapshot as a through-commit reference lock", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if err := lists.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&journal); err != nil || journal != "delete" {
		t.Fatal("disposable reference journal correction failed", journal, err)
	}
	result, err = contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", snapshot.Original.Form,
		snapshot.Original, edits, false, provider.readers, selection, acknowledgements)
	if err != nil || result == nil || result.HistoryID == "" {
		t.Fatal("owned writer journal rejection leaked lease/attachment and blocked retry", result, err)
	}
}
