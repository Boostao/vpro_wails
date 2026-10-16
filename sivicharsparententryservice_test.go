package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func charsEntryServiceFixture(t *testing.T) (*SIVICHARSParentEntryService, ProjectState, *SIVICHARSParentEntryReview) {
	t.Helper()
	shared, state := charsReferenceFixture(t)
	service, err := newSIVICHARSParentEntryService(shared.contexts, shared.references, true)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.Review(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	return service, state, review
}

func charsMandatoryEdit(t *testing.T, review *SIVICHARSParentEntryReview, column string) SIVIParentCellEdit {
	t.Helper()
	for _, field := range review.Fields {
		if field.Column == column {
			for _, choice := range field.Choices {
				if choice.Selectable && choice.Code != nil &&
					!metadataCellEqual(siviParentCell(t, review.Original, column), metadataText(*choice.Code)) {
					return charsEntryEdit(t, review.Original, column, metadataText(*choice.Code))
				}
			}
		}
	}
	t.Fatal("independent source reference has no changed selectable code", column)
	return SIVIParentCellEdit{}
}

func TestSIVICHARSEntryServiceActualWireMandatoryReferencesAndRestoration(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	edits := []SIVIParentCellEdit{charsEntryEdit(t, review.Original, "SV_WaterTableCM", siviReal(.1))}
	for _, column := range []string{"Exposure1", "Exposure2", "SuccessionalStatus", "SoilDrainage"} {
		edits = append(edits, charsMandatoryEdit(t, review, column))
	}
	raw, err := json.Marshal(SIVICHARSParentEntryWrite{Original: review.Original, Edits: edits})
	if err != nil {
		t.Fatal(err)
	}
	var request SIVICHARSParentEntryWrite
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal("actual Go wire did not round trip", err)
	}
	written, err := service.Save(context.Background(), state.ContextID, "108050", request)
	if err != nil || written == nil || written.ChangedCells != 5 || written.HistoryID == "" {
		t.Fatal("owned service did not commit exact source references", written, err)
	}
	after, err := service.Review(context.Background(), state.ContextID, "108050")
	if err != nil || !metadataCellEqual(siviParentCell(t, after.Original, "SV_WaterTableCM"), siviReal(0.10000000149011612)) {
		t.Fatal("physical SINGLE result did not survive service review", err)
	}
	for _, edit := range edits[1:] {
		if !metadataCellEqual(siviParentCell(t, after.Original, edit.Column), edit.Value) {
			t.Fatal("mandatory literal source identity changed", edit.Column)
		}
	}
	restored, err := service.Restore(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 5 {
		t.Fatal("owned service restoration failed", restored, err)
	}
	final, err := service.Review(context.Background(), state.ContextID, "108050")
	if err != nil || !sourceParentProjectionStorageEqual(final.Original, review.Original) {
		t.Fatal("service restoration changed unrelated source", err)
	}
}

func TestSIVICHARSEntryServiceIndependentGateStaleAndCancellation(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	if _, err := newSIVICHARSParentEntryService(nil, service.readers, true); err == nil {
		t.Fatal("missing context owner accepted")
	}
	disabled, err := newSIVICHARSParentEntryService(service.contexts, service.readers, false)
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []SIVIParentCellEdit{
		charsEntryEdit(t, review.Original, "SV_WaterTableCM", siviReal(.1)),
	}}

	if got, err := disabled.Review(context.Background(), state.ContextID, "108050"); err == nil || got != nil {
		t.Fatal("disabled review leaked source", got, err)
	}
	if got, err := disabled.Save(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
		t.Fatal("disabled Save wrote data", got, err)
	}
	if got, err := disabled.Restore(context.Background(), state.ContextID, "108050", "1", AuditRestoreCancel); err == nil || got != nil {
		t.Fatal("disabled restore bypassed independent gate", got, err)
	}
	if got, err := service.Save(context.Background(), "stale", "108050", request); err == nil || got != nil {
		t.Fatal("stale context wrote data", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.Review(ctx, state.ContextID, "108050"); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled review returned data", got, err)
	}
	if got, err := service.Save(ctx, state.ContextID, "108050", request); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled Save returned success", got, err)
	}
	assertProfileSUFiles(t, service.contexts, before)
}

func TestSIVICHARSEntryTransportRejectsDuplicatesUnknownKeysAndRawUnicode(t *testing.T) {
	original := charsEntryOriginal(t)
	request := SIVICHARSParentEntryWrite{Original: original, Edits: []SIVIParentCellEdit{
		charsEntryEdit(t, original, "RootZoneParticleSize", metadataText("abc")),
	}}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	valid := string(data)
	for _, raw := range []string{
		strings.Replace(valid, `"original":`, `"Original":`, 1),
		strings.Replace(valid, `"edits":`, `"edits":[],"edits":`, 1),
		strings.Replace(valid, `"edits":`, `"edits":[],"\u0065dits":`, 1),
		strings.Replace(valid, `"edits":`, `"other":[],"edits":`, 1),
		strings.Replace(valid, `"ContextID":`, `"ContextID":"ignored","ContextID":`, 1),
		strings.Replace(valid, `"column":`, `"column":"ignored","\u0063olumn":`, 1),
		strings.Replace(valid, `"abc"`, `"\ud800"`, 1),
		strings.Replace(valid, `"abc"`, "\"\xff\"", 1),
		`{"original":null,"edits":[]}`,
		`{"original":{},"edits":null}`,
		valid + `{}`,
	} {
		var decoded SIVICHARSParentEntryWrite
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("malformed/duplicate/recased transport accepted", raw)
		}
	}
	var decoded SIVICHARSParentEntryWrite
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("valid wire rejected after refusals", err)
	}
}

func TestSIVICHARSEntryMandatoryFailuresRollbackThenCorrect(t *testing.T) {
	for _, column := range []string{"Exposure1", "Exposure2", "SuccessionalStatus", "SoilDrainage"} {
		t.Run(column, func(t *testing.T) {
			service, state, review := charsEntryServiceFixture(t)
			request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []SIVIParentCellEdit{
				charsEntryEdit(t, review.Original, "SV_WaterTableCM", siviReal(.1)),
				charsEntryEdit(t, review.Original, column, metadataText("?")),
			}}

			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
				t.Fatal("absent mandatory source code committed", got, err)
			}
			assertProfileSUFiles(t, service.contexts, before)
			request.Edits[1] = charsMandatoryEdit(t, review, column)
			if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err != nil || got == nil || got.ChangedCells != 2 {
				t.Fatal("corrected mandatory source code cannot retry", got, err)
			}
		})
	}
}

func TestSIVICHARSEntryHistoricalOmissionNullAndOptionalValuesDoNotInventCallbacks(t *testing.T) {
	service, state, _ := charsEntryServiceFixture(t)
	mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Exposure1='?',Exposure2='?',SuccessionalStatus='?',SoilDrainage='?' WHERE PlotNumber='108050'`)
	review, err := service.Review(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []SIVIParentCellEdit{
		charsEntryEdit(t, review.Original, "Exposure1", metadataText("?")),
		charsEntryEdit(t, review.Original, "Exposure2", ProjectMetadataCell{Storage: "null"}),
		charsEntryEdit(t, review.Original, "SuccessionalStatus", ProjectMetadataCell{Storage: "null"}),
		charsEntryEdit(t, review.Original, "SoilDrainage", ProjectMetadataCell{Storage: "null"}),
		charsEntryEdit(t, review.Original, "RootZoneParticleSize", metadataText("  Z  ")),
		charsEntryEdit(t, review.Original, "SV_AhorizonType", metadataText("Other")),
	}}

	service.readers = siviParentSharedReferenceReaders{}
	written, err := service.Save(context.Background(), state.ContextID, "108050", request)
	if err != nil || written == nil || written.ChangedCells != 5 {
		t.Fatal("NULL/unchanged/optional values invented mandatory lookups or acknowledgements", written, err)
	}
	fresh, err := service.contexts.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
	if err != nil || !metadataCellEqual(siviParentCell(t, fresh, "Exposure1"), metadataText("?")) ||
		!metadataCellEqual(siviParentCell(t, fresh, "RootZoneParticleSize"), metadataText("  Z  ")) ||
		!metadataCellEqual(siviParentCell(t, fresh, "SV_AhorizonType"), metadataText("Other")) {
		t.Fatal("optional literal source data was normalized", err)
	}
}

func TestSIVICHARSEntryBorrowedFailureAndInFlightCancellationRetry(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	originalReaders := service.readers
	failure := errors.New("independent parent reference failure")
	for _, kind := range []string{"failure", "cancel", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service.readers = originalReaders
			service.readers.parent = charsReferenceParentStub{func(context.Context, string) ([]ParentCodeChoice, error) {
				if kind == "failure" {
					return nil, failure
				}
				if kind == "cancel" {
					cancel()
					return nil, ctx.Err()
				}
				rows, err := originalReaders.parent.ListChoices(ctx, "SoilDrainage")
				if err != nil {
					return nil, err
				}
				bad := string([]byte{0xff})
				rows[0].Description = &bad
				return rows, nil
			}}
			request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []SIVIParentCellEdit{charsMandatoryEdit(t, review, "SoilDrainage")}}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			got, err := service.Save(ctx, state.ContextID, "108050", request)
			if err == nil || got != nil || kind == "failure" && !errors.Is(err, failure) ||
				kind == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("borrowed failure became successful mutation", got, err)
			}
			assertProfileSUFiles(t, service.contexts, before)
		})
	}
	service.readers = originalReaders
	if got, err := service.Save(context.Background(), state.ContextID, "108050",
		SIVICHARSParentEntryWrite{Original: review.Original, Edits: []SIVIParentCellEdit{charsMandatoryEdit(t, review, "SoilDrainage")}}); err != nil || got == nil {
		t.Fatal("failed/cancelled borrowed reference leaked writer lifecycle", got, err)
	}
}

func TestSIVICHARSEntryFamilySchemaAndJournalFailuresRecover(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	path := service.contexts.projects.sqlite.attachments["VLists"]
	request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []SIVIParentCellEdit{charsMandatoryEdit(t, review, "SuccessionalStatus")}}
	for _, kind := range []string{"missing", "malformed", "view", "wal"} {
		t.Run(kind, func(t *testing.T) {
			switch kind {
			case "missing":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ListName='Unrelated' WHERE ListName='SuccessionalStatus'`)
			case "malformed":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ItemDescription=CAST(X'FF' AS TEXT) WHERE ListName='SuccessionalStatus'`)
			case "view":
				mutateContextFixture(t, path, `ALTER TABLE USysTableOfLists RENAME TO SavedLists; CREATE VIEW USysTableOfLists AS SELECT * FROM SavedLists`)
			case "wal":
				mutateContextFixture(t, path, `PRAGMA journal_mode=WAL`)
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
				t.Fatal("bad family source committed", got, err)
			}
			assertProfileSUFiles(t, service.contexts, before)
			switch kind {
			case "missing":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ListName='SuccessionalStatus' WHERE ListName='Unrelated'`)
			case "malformed":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ItemDescription='corrected' WHERE ListName='SuccessionalStatus'`)
			case "view":
				mutateContextFixture(t, path, `DROP VIEW USysTableOfLists; ALTER TABLE SavedLists RENAME TO USysTableOfLists`)
			case "wal":
				mutateContextFixture(t, path, `PRAGMA journal_mode=DELETE`)
			}
		})
	}
	if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err != nil || got == nil {
		t.Fatal("corrected family cannot retry", got, err)
	}
}

func TestSIVICHARSEntryAliasReaderRejectsForeignScope(t *testing.T) {
	service, state, _ := charsEntryServiceFixture(t)
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		t.Fatal(err)
	}
	_, err = withContextPlotRequest(context.Background(), service.contexts, state.ContextID, func(plots *PlotService) (bool, error) {
		return withOwnedSIVISnapshot(context.Background(), plots, func(_ *sqliteContext, tx *sql.Tx) (bool, error) {
			if _, err := readSIVICHARSParentReferenceAtAlias(context.Background(), tx, "foreign", service.readers, fields[0]); err == nil {
				t.Fatal("unknown source alias accepted")
			}
			return true, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}
