package sivi

import (
	referencespkg "github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func charsEntryServiceFixture(t *testing.T) (*SIVICHARSParentEntryService, vprocore.ProjectState, *SIVICHARSParentEntryReview) {
	t.Helper()
	shared, state := charsReferenceFixture(t)
	service, err := NewSIVICHARSParentEntryService(shared.Contexts, shared.References, true)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.Review(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	return service, state, review
}

func charsMandatoryEdit(t *testing.T, review *SIVICHARSParentEntryReview, column string) sourceform.SIVIParentCellEdit {
	t.Helper()
	for _, field := range review.Fields {
		if field.Column == column {
			for _, choice := range field.Choices {
				if choice.Selectable && choice.Code != nil &&
					!vprocore.MetadataCellEqual(sftest.SiviParentCell(t, review.Original, column), vprotest.MetadataText(*choice.Code)) {
					return charsEntryEdit(t, review.Original, column, vprotest.MetadataText(*choice.Code))
				}
			}
		}
	}
	t.Fatal("independent source reference has no changed selectable code", column)
	return sourceform.SIVIParentCellEdit{}
}

func TestSIVICHARSEntryServiceActualWireMandatoryReferencesAndRestoration(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	edits := []sourceform.SIVIParentCellEdit{charsEntryEdit(t, review.Original, "SV_WaterTableCM", vprotest.SiviReal(.1))}
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
	if err != nil || !vprocore.MetadataCellEqual(sftest.SiviParentCell(t, after.Original, "SV_WaterTableCM"), vprotest.SiviReal(0.10000000149011612)) {
		t.Fatal("physical SINGLE result did not survive service review", err)
	}
	for _, edit := range edits[1:] {
		if !vprocore.MetadataCellEqual(sftest.SiviParentCell(t, after.Original, edit.Column), edit.Value) {
			t.Fatal("mandatory literal source identity changed", edit.Column)
		}
	}
	restored, err := service.Restore(context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 5 {
		t.Fatal("owned service restoration failed", restored, err)
	}
	final, err := service.Review(context.Background(), state.ContextID, "108050")
	if err != nil || !sourceform.SourceParentProjectionStorageEqual(final.Original, review.Original) {
		t.Fatal("service restoration changed unrelated source", err)
	}
}

func TestSIVICHARSEntryServiceIndependentGateStaleAndCancellation(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	if _, err := NewSIVICHARSParentEntryService(nil, service.readers, true); err == nil {
		t.Fatal("missing context owner accepted")
	}
	disabled, err := NewSIVICHARSParentEntryService(service.contexts, service.readers, false)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.contexts.Projects.Sqlite.Attachments)
	request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []sourceform.SIVIParentCellEdit{
		charsEntryEdit(t, review.Original, "SV_WaterTableCM", vprotest.SiviReal(.1)),
	}}

	if got, err := disabled.Review(context.Background(), state.ContextID, "108050"); err == nil || got != nil {
		t.Fatal("disabled review leaked source", got, err)
	}
	if got, err := disabled.Save(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
		t.Fatal("disabled Save wrote data", got, err)
	}
	if got, err := disabled.Restore(context.Background(), state.ContextID, "108050", "1", vprocore.AuditRestoreCancel); err == nil || got != nil {
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
	vprotest.AssertProfileSUFiles(t, service.contexts, before)
}

func TestSIVICHARSEntryTransportRejectsDuplicatesUnknownKeysAndRawUnicode(t *testing.T) {
	original := charsEntryOriginal(t)
	request := SIVICHARSParentEntryWrite{Original: original, Edits: []sourceform.SIVIParentCellEdit{
		charsEntryEdit(t, original, "RootZoneParticleSize", vprotest.MetadataText("abc")),
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
			request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []sourceform.SIVIParentCellEdit{
				charsEntryEdit(t, review.Original, "SV_WaterTableCM", vprotest.SiviReal(.1)),
				charsEntryEdit(t, review.Original, column, vprotest.MetadataText("?")),
			}}

			before := vprotest.DatabaseBytes(t, service.contexts.Projects.Sqlite.Attachments)
			if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
				t.Fatal("absent mandatory source code committed", got, err)
			}
			vprotest.AssertProfileSUFiles(t, service.contexts, before)
			request.Edits[1] = charsMandatoryEdit(t, review, column)
			if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err != nil || got == nil || got.ChangedCells != 2 {
				t.Fatal("corrected mandatory source code cannot retry", got, err)
			}
		})
	}
}

func TestSIVICHARSEntryHistoricalOmissionNullAndOptionalValuesDoNotInventCallbacks(t *testing.T) {
	service, state, _ := charsEntryServiceFixture(t)
	vprotest.MutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Exposure1='?',Exposure2='?',SuccessionalStatus='?',SoilDrainage='?' WHERE PlotNumber='108050'`)
	review, err := service.Review(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []sourceform.SIVIParentCellEdit{
		charsEntryEdit(t, review.Original, "Exposure1", vprotest.MetadataText("?")),
		charsEntryEdit(t, review.Original, "Exposure2", vprocore.ProjectMetadataCell{Storage: "null"}),
		charsEntryEdit(t, review.Original, "SuccessionalStatus", vprocore.ProjectMetadataCell{Storage: "null"}),
		charsEntryEdit(t, review.Original, "SoilDrainage", vprocore.ProjectMetadataCell{Storage: "null"}),
		charsEntryEdit(t, review.Original, "RootZoneParticleSize", vprotest.MetadataText("  Z  ")),
		charsEntryEdit(t, review.Original, "SV_AhorizonType", vprotest.MetadataText("Other")),
	}}

	service.readers = sourceform.SiviParentSharedReferenceReaders{}
	written, err := service.Save(context.Background(), state.ContextID, "108050", request)
	if err != nil || written == nil || written.ChangedCells != 5 {
		t.Fatal("NULL/unchanged/optional values invented mandatory lookups or acknowledgements", written, err)
	}
	fresh, err := ReadSIVICHARSParent(service.contexts, context.Background(), state.ContextID, "108050")
	if err != nil || !vprocore.MetadataCellEqual(sftest.SiviParentCell(t, fresh, "Exposure1"), vprotest.MetadataText("?")) ||
		!vprocore.MetadataCellEqual(sftest.SiviParentCell(t, fresh, "RootZoneParticleSize"), vprotest.MetadataText("  Z  ")) ||
		!vprocore.MetadataCellEqual(sftest.SiviParentCell(t, fresh, "SV_AhorizonType"), vprotest.MetadataText("Other")) {
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
			service.readers.Parent = charsReferenceParentStub{func(context.Context, string) ([]referencespkg.ParentCodeChoice, error) {
				if kind == "failure" {
					return nil, failure
				}
				if kind == "cancel" {
					cancel()
					return nil, ctx.Err()
				}
				rows, err := originalReaders.Parent.ListChoices(ctx, "SoilDrainage")
				if err != nil {
					return nil, err
				}
				bad := string([]byte{0xff})
				rows[0].Description = &bad
				return rows, nil
			}}
			request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []sourceform.SIVIParentCellEdit{charsMandatoryEdit(t, review, "SoilDrainage")}}
			before := vprotest.DatabaseBytes(t, service.contexts.Projects.Sqlite.Attachments)
			got, err := service.Save(ctx, state.ContextID, "108050", request)
			if err == nil || got != nil || kind == "failure" && !errors.Is(err, failure) ||
				kind == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("borrowed failure became successful mutation", got, err)
			}
			vprotest.AssertProfileSUFiles(t, service.contexts, before)
		})
	}
	service.readers = originalReaders
	if got, err := service.Save(context.Background(), state.ContextID, "108050",
		SIVICHARSParentEntryWrite{Original: review.Original, Edits: []sourceform.SIVIParentCellEdit{charsMandatoryEdit(t, review, "SoilDrainage")}}); err != nil || got == nil {
		t.Fatal("failed/cancelled borrowed reference leaked writer lifecycle", got, err)
	}
}

func TestSIVICHARSEntryFamilySchemaAndJournalFailuresRecover(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	path := service.contexts.Projects.Sqlite.Attachments["VLists"]
	request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []sourceform.SIVIParentCellEdit{charsMandatoryEdit(t, review, "SuccessionalStatus")}}
	for _, kind := range []string{"missing", "malformed", "view", "wal"} {
		t.Run(kind, func(t *testing.T) {
			switch kind {
			case "missing":
				vprotest.MutateContextFixture(t, path, `UPDATE USysTableOfLists SET ListName='Unrelated' WHERE ListName='SuccessionalStatus'`)
			case "malformed":
				vprotest.MutateContextFixture(t, path, `UPDATE USysTableOfLists SET ItemDescription=CAST(X'FF' AS TEXT) WHERE ListName='SuccessionalStatus'`)
			case "view":
				vprotest.MutateContextFixture(t, path, `ALTER TABLE USysTableOfLists RENAME TO SavedLists; CREATE VIEW USysTableOfLists AS SELECT * FROM SavedLists`)
			case "wal":
				vprotest.MutateContextFixture(t, path, `PRAGMA journal_mode=WAL`)
			}
			before := vprotest.DatabaseBytes(t, service.contexts.Projects.Sqlite.Attachments)
			if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
				t.Fatal("bad family source committed", got, err)
			}
			vprotest.AssertProfileSUFiles(t, service.contexts, before)
			switch kind {
			case "missing":
				vprotest.MutateContextFixture(t, path, `UPDATE USysTableOfLists SET ListName='SuccessionalStatus' WHERE ListName='Unrelated'`)
			case "malformed":
				vprotest.MutateContextFixture(t, path, `UPDATE USysTableOfLists SET ItemDescription='corrected' WHERE ListName='SuccessionalStatus'`)
			case "view":
				vprotest.MutateContextFixture(t, path, `DROP VIEW USysTableOfLists; ALTER TABLE SavedLists RENAME TO USysTableOfLists`)
			case "wal":
				vprotest.MutateContextFixture(t, path, `PRAGMA journal_mode=DELETE`)
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
	_, err = vprocore.WithContextPlotRequest(context.Background(), service.contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
		return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(_ *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
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
