package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func summaryWorkbookFixtureSource(ctx context.Context, service *SummaryVegetationEnvironmentalLifeformService,
	contextID string, options summaryVegetationWorkbookOptions, hooks vprocore.PublicationReadSnapshotHooks) (summaryVegetationWorkbookSource, error) {
	return vprocore.WithContextPlotRequest(ctx, service.environment.report.contexts, contextID, func(plots *vprocore.PlotService) (summaryVegetationWorkbookSource, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (summaryVegetationWorkbookSource, error) {
			return readSummaryVegetationWorkbookSource(ctx, service, plots, owner, tx, contextID, options)
		})
	})
}

func TestSummaryWorkbookOwnedSourcePhysicalInputsAndBytes(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprintf("external=%t", external), func(t *testing.T) {
			service, state := summaryEnvironmentalLifeformServiceFixture(t, external)
			owner := service.environment.report.contexts.Projects.Sqlite
			vprotest.MutateContextFixture(t, owner.Attachments["VUser"], `INSERT INTO USysUserSpp
				(Code,ScientificName,LifeForm,CodeType) VALUES('UNUSED','Private one',1,'U')`)
			vprotest.MutateContextFixture(t, owner.Attachments["VPro64"], `INSERT INTO LayerCode
				(LayerText,Layer1234567,Layer) VALUES('7','7','U7')`)
			original, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0)
			if err != nil {
				t.Fatal(err)
			}
			before := vprotest.DatabaseBytes(t, owner.Attachments)
			config, err := os.ReadFile(service.environment.report.contexts.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			for _, includeEnv := range []bool{false, true} {
				snapshots := 0
				options := summaryVegetationWorkbookOptions{Title: "=SUM(A1)", Average: 1, Display: 6, IncludeEnv: includeEnv}
				got, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options,
					vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { snapshots++; return nil }})
				if err != nil || snapshots != 1 || !reflect.DeepEqual(got.Input.Vegetation, original.Vegetation) ||
					len(got.Raw.SU.Rows) != 8 || len(got.Raw.Veg.Rows) != 27 || len(got.Raw.Master.Rows) != 13 ||
					len(got.Raw.Personal.Rows) != 1 || len(got.Raw.Layers.Rows) != 3 || len(got.Paths) != 5 {
					t.Fatal("complete physical source or one-snapshot composition differs", err, snapshots, got)
				}
				for i, role := range []string{"project", "su", "VLists", "VUser", "VPro64"} {
					if got.Paths[i] != (summaryVegetationWorkbookSourcePath{role, owner.Attachments[role]}) {
						t.Fatal("literal attachment source identity lost", got.Paths)
					}
				}
				expected := summaryVegetationWorkbookInput{Title: options.Title, Vegetation: original.Vegetation}
				first := 6
				if includeEnv {
					expected.Environment = &summaryEnvironmentalWorkbookInput{FirstRow: 5,
						FieldList: original.FieldList, Units: summaryEnvironmentalPreviewUnits(original.Vegetation), Rows: original.Rows}
					first = 36
					if len(got.Environment.FieldList.Rows) != 31 || len(got.Environment.Env.Rows) != 3 {
						t.Fatal("full selected and unselected definitions or physical Env lost", got.Environment)
					}
				} else if got.Input.Environment != nil || !reflect.DeepEqual(got.Environment, summaryEnvironmentalPhysicalInput{}) {
					t.Fatal("vegetation-only source read environmental definitions", got.Environment)
				}
				workbook, err := prepareSummaryVegetationWorkbook(t.Context(), got.Input)
				expectedBook, expectedErr := prepareSummaryVegetationWorkbook(t.Context(), expected)
				if err != nil || expectedErr != nil || workbook.FirstVegetationRow != first ||
					!bytes.Equal(workbook.Bytes, expectedBook.Bytes) {
					t.Fatal("owned compositor differs from qualified original preview", err, expectedErr)
				}
				book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: workbook.Bytes})
				title, err := book.GetCellValue(workbook.Sheet, "A1")
				formula, formulaErr := book.GetCellFormula(workbook.Sheet, "A1")
				if err != nil || formulaErr != nil || title != "=SUM(A1)" || formula != "" {
					t.Fatal("owned literal title changed or executed", title, formula, err, formulaErr)
				}
			}
			afterConfig, err := os.ReadFile(service.environment.report.contexts.Projects.Preferences.Path)
			if err != nil || !bytes.Equal(config, afterConfig) || !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, owner.Attachments)) {
				t.Fatal("owned composition changed physical data/audits/configuration", err)
			}
		})
	}
}

func TestSummaryWorkbookOwnedSourceRetainsInvisiblePeerChanges(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	owner := service.environment.report.contexts.Projects.Sqlite
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], `INSERT INTO USysUserSpp
		(Code,ScientificName,LifeForm,CodeType) VALUES('UNUSED','Private one',1,'U')`)
	vprotest.MutateContextFixture(t, owner.Attachments["VPro64"], `INSERT INTO LayerCode
		(LayerText,Layer1234567,Layer) VALUES('7','7','U7')`)
	options := summaryVegetationWorkbookOptions{Average: 1, Display: 6}
	first, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err != nil {
		t.Fatal(err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], `UPDATE USysUserSpp SET ScientificName='Private two' WHERE Code='UNUSED'`)
	second, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err != nil || !reflect.DeepEqual(first.Input, second.Input) || reflect.DeepEqual(first.Raw.Personal, second.Raw.Personal) {
		t.Fatal("unused physical personal peer drift was lost or invented a rendered difference", err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VPro64"], `UPDATE LayerCode SET Layer='V7' WHERE LayerText='7'`)
	third, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err != nil || !reflect.DeepEqual(second.Input, third.Input) || reflect.DeepEqual(second.Raw.Layers, third.Raw.Layers) {
		t.Fatal("unused physical layer peer drift was lost or invented a rendered difference", err)
	}
}

func TestSummaryWorkbookOwnedSourceCanonicalSelection(t *testing.T) {
	definitions, err := csv.NewReader(strings.NewReader(summaryEnvironmentalCanonicalFieldListCSV)).ReadAll()
	if err != nil || len(definitions) != 55 {
		t.Fatal("54 independent canonical field definitions required", err)
	}
	for _, external := range []bool{false, true} {
		service, state := summaryEnvironmentalLifeformServiceFixture(t, external)
		owner := service.environment.report.contexts.Projects.Sqlite
		vprotest.MutateContextFixture(t, owner.Attachments["project"], "DELETE FROM "+vprocore.QuoteHeaderIdentifier(state.ActiveProject+"_Env"))
		vprotest.MutateContextFixture(t, owner.Attachments["VPro64"], "DELETE FROM USysShortVegSummaryFieldList")
		quoted := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
		for _, row := range definitions[1:] {
			report, source, argument := "-1", "NULL", "NULL"
			if row[5] == "SVHumusDepth" {
				report = "0"
			}
			if row[1] != "" {
				source = quoted(row[1])
			}
			if row[6] != "" {
				argument = quoted(row[6])
			}
			vprotest.MutateContextFixture(t, owner.Attachments["VPro64"], "INSERT INTO USysShortVegSummaryFieldList VALUES("+
				row[0]+","+source+","+quoted(row[2])+","+quoted(row[3])+","+report+","+quoted(row[5])+","+argument+")")
		}
		snapshots := 0
		got, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID,
			summaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: true},
			vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { snapshots++; return nil }})
		if err != nil || snapshots != 1 || len(got.Environment.FieldList.Rows) != 54 || got.Input.Environment == nil ||
			len(got.Input.Environment.Rows) != 53 || got.Environment.Env.Rows == nil || len(got.Environment.Env.Rows) != 0 {
			t.Fatal("complete canonical owned snapshot differs", err, snapshots)
		}
		workbook, err := prepareSummaryVegetationWorkbook(t.Context(), got.Input)
		if err != nil || workbook.FirstVegetationRow != 59 {
			t.Fatal("canonical source-related body placement differs", err)
		}
		book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: workbook.Bytes})
		if marker, err := book.GetCellValue(workbook.Sheet, "A58"); err != nil || marker != "StartVeg" {
			t.Fatal("canonical source separator differs", marker, err)
		}
	}
}

func TestSummaryWorkbookOwnedSourceCleanupCancellationAndRetry(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	options := summaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: true}
	failure := errors.New("owned source cleanup refused")
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return failure }},
		{RollbackRead: func(*sql.Tx) error { return failure }},
	} {
		got, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, hooks)
		if !errors.Is(err, failure) || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
			t.Fatal("failed source cleanup returned a partial success", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	got, err := summaryWorkbookFixtureSource(ctx, service, state.ContextID, options,
		vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
		t.Fatal("in-flight cancellation returned a source", err)
	}
	got, err = summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err != nil || got.Input.Environment == nil || len(got.Input.Environment.Rows) != 30 {
		t.Fatal("owned attachments/source did not recover after cancellation/cleanup", err)
	}
}
