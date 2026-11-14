package sftest

import (
	referencespkg "github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"

	"errors"
	"fmt"

	"os"
	"path/filepath"
	"reflect"

	"strings"

	"testing"
)

func EnvironmentReportFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	env, admin := vprocore.ProjectMetadataTable{}, vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Plot"}}}
	for _, field := range reportcore.LongEnvironmentFields() {
		if !field.Heading {
			table := &env
			if field.Source == "Admin" {
				table = &admin
			}
			table.Columns = append(table.Columns, vprocore.ProjectMetadataColumn{Name: field.Key})
		}
	}
	for _, plot := range []string{"P1", "P2", "P4"} {
		for _, table := range []*vprocore.ProjectMetadataTable{&env, &admin} {
			if table == &admin && plot == "P2" {
				continue
			}
			row := vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(table.Rows) + 1), Cells: make([]vprocore.ProjectMetadataCell, len(table.Columns))}
			for i, column := range table.Columns {
				row.Cells[i] = vprocore.ProjectMetadataCell{Storage: "null"}
				switch column.Name {
				case "PlotNumber", "Plot":
					row.Cells[i] = vprotest.MetadataText(plot)
				case "Location":
					row.Cells[i] = vprotest.MetadataText("")
				case "Elevation":
					row.Cells[i] = vprotest.MetadataInteger("0")
				case "UserSiteUnit":
					row.Cells[i] = vprotest.MetadataText("OTHER")
				case "SitePlotQuality":
					row.Cells[i] = vprotest.MetadataText("Poor")
				}
			}
			table.Rows = append(table.Rows, row)
		}
	}
	su := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}, Rows: []vprocore.ProjectMetadataRow{
		{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("U")}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P2"), vprotest.MetadataText("U")}},
		{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P3"), vprotest.MetadataText("V")}},
		{RowID: "4", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText("")}},
		{RowID: "5", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("W")}},
		{RowID: "6", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P4"), {Storage: "null"}}},
	}}
	master := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []vprocore.ProjectMetadataRow{
		{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("U"), vprotest.MetadataText("Unit U")}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("U"), {Storage: "null"}}},
		{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("U"), vprotest.MetadataText("Unit U")}},
		{RowID: "4", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("V"), vprotest.MetadataText("Alpha")}},
		{RowID: "5", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("V"), vprotest.MetadataText("Beta")}},
	}}
	return env, admin, su, master
}

func SiviCreationFixture(t *testing.T, external bool, strength int) (*vprocore.ContextService, vprocore.ProjectState, *sql.DB, sourceform.SiviCreationRequest) {
	t.Helper()
	service, state, db, _, _ := SiviWriteFixture(t, external, strength)
	vprotest.MutateContextFixture(t, service.Projects.Sqlite.Attachments["VLists"], `DELETE FROM USysAllSpecs;
		INSERT INTO USysAllSpecs(Code,Lifeform,Codetype,OldCode) VALUES
		('A',3,'U',NULL),('A',3,'U',NULL),('C',12,'x',NULL),('D',9,'u',NULL),
		('NEW',99,'S','old'),('OTHER',3,'U','old')`)
	vprotest.MutateContextFixture(t, service.Projects.Sqlite.Attachments["VUser"], `DELETE FROM USysUserSpp;
		INSERT INTO USysUserSpp(Code,LifeForm,Codetype) VALUES ('Personal',99,'S'),('old',1,'U')`)
	return service, state, db, sourceform.SiviCreationRequest{RequestID: "creation-1", ContextID: state.ContextID,
		Project: "Sample", Plot: "108050", Form: "SubVegA-SIVI", Species: "A",
		Covers: []sourceform.SiviCreationCover{{Column: "Cover1", Value: vprotest.SiviReal(0)}}}
}

func SiviDeletionFixture(t *testing.T, external bool, strength int) (*vprocore.ContextService, vprocore.ProjectState, *sql.DB, sourceform.SiviDeletionRequest) {
	t.Helper()
	service, state, db, _, edits := SiviWriteFixture(t, external, strength)
	original, err := sourceform.ReadSIVIDeletionOriginal(service, context.Background(), state.ContextID, "108050", "SubVegA-SIVI", edits[0].RowID)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db, sourceform.SiviDeletionRequest{RequestID: "abcdef01-2345-4678-8abc-012345678901",
		ContextID: state.ContextID, Project: original.Project, Plot: original.Plot, Form: original.Form,
		Columns: original.Columns, Original: original.Original}
}

func SiviActionProposalEdits(parent *sourceform.SiviParentProjection) []sourceform.SiviParentActionEdit {
	edits := []sourceform.SiviParentActionEdit{}
	for _, source := range []struct {
		control, column, table string
		row                    vprocore.ProjectMetadataRow
		columns                []vprocore.ProjectMetadataColumn
	}{
		{"form:frmSIVIsite/optPlotType", "PlotType", parent.AdminTable, parent.Rows[0].Admin, parent.AdminColumns},
		{"form:frmSIVIsite/optSpeciesListComplete", "SpeciesListComplete", parent.EnvTable, parent.Rows[0].Env, parent.EnvColumns},
	} {
		for index, column := range source.columns {
			if column.Name == source.column {
				option := 1
				edits = append(edits, sourceform.SiviParentActionEdit{
					ContextID: parent.ContextID, ControlID: source.control, Table: source.table,
					RowID: source.row.RowID, Expected: vprocore.CloneSiteUnitCell(source.row.Cells[index]), Option: &option,
				})
			}
		}
	}
	return edits
}

func SiviCategoricalProposalEdits(parent *sourceform.SiviParentProjection) []sourceform.SiviParentScalarEdit {
	edits := []sourceform.SiviParentScalarEdit{}
	for _, field := range []struct{ column, value string }{
		{"SnowCoverregime", "Z"}, {"SV_RootZoneTexture", "  MiXeD \t "}, {"SV_AhorizonType", ""},
	} {
		for index, column := range parent.EnvColumns {
			if column.Name == field.column {
				edits = append(edits, sourceform.SiviParentScalarEdit{
					ContextID: parent.ContextID, Table: parent.EnvTable, RowID: parent.Rows[0].Env.RowID,
					Column: field.column, Expected: vprocore.CloneSiteUnitCell(parent.Rows[0].Env.Cells[index]), Value: vprotest.MetadataText(field.value),
				})
			}
		}
	}
	return edits
}

func SiviProposalEdits(parent *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarEdit, []sourceform.SiviParentScalarEdit) {
	scalars, options := []sourceform.SiviParentScalarEdit{}, []sourceform.SiviParentScalarEdit{}
	for _, binding := range parent.Bindings {
		_, scalar := sourceform.SiviParentScalarOwners[binding.Binding]
		option := binding.Binding == "SV_StandAgeEstMeas" || binding.Binding == "SV_StandHeightEstMeas"
		if !scalar && !option {
			continue
		}
		row := parent.Rows[0].Env
		if binding.Table == parent.AdminTable {
			row = parent.Rows[0].Admin
		}
		value := vprotest.SiviReal(123.25)
		if binding.Binding == "SV_FloodPlain" {
			value = vprotest.MetadataInteger("-1")
		}
		if option {
			value = vprotest.MetadataText("2")
		}
		before := row.Cells[binding.Column]
		if reflect.DeepEqual(value, before) {
			value = vprocore.ProjectMetadataCell{Storage: "null"}
		}
		edit := sourceform.SiviParentScalarEdit{
			ContextID: parent.ContextID, Table: binding.Table, RowID: row.RowID,
			Column: binding.Binding, Expected: vprocore.CloneSiteUnitCell(before), Value: value,
		}
		if scalar {
			scalars = append(scalars, edit)
		} else {
			options = append(options, edit)
		}
	}
	return scalars, options
}

func SiviScalarFixture(t *testing.T, column string, before vprocore.ProjectMetadataCell) (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, sourceform.SiviParentScalarEdit) {
	t.Helper()
	env, admin := vprotest.SiviParentTables(t)
	table := &env
	suffix := "Env"
	if column == "StrataCoverTotal" {
		table, suffix = &admin, "Admin"
	}
	found := false
	for index, field := range table.Columns {
		if field.Name == column {
			table.Rows[0].Cells[index] = before
			found = true
		}
	}
	if !found {
		t.Fatal("scalar fixture missing source field", column)
	}
	return env, admin, sourceform.SiviParentScalarEdit{
		ContextID: "owned", Table: "Sample_" + suffix, RowID: table.Rows[0].RowID,
		Column: column, Expected: before, Value: vprocore.ProjectMetadataCell{Storage: "null"},
	}
}

func SharedReferenceFixture(t *testing.T) (*sourceform.SIVIParentSharedService, vprocore.ProjectState, *sourceform.SIVIParentProjection) {
	t.Helper()
	service, state, original, _ := SharedServiceFixture(t)
	dir := t.TempDir()
	region, err := referencespkg.NewRegionCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	site, err := referencespkg.NewSiteCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := referencespkg.NewParentCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	geology, err := referencespkg.NewGeologyCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	bec, err := referencespkg.NewBECService(dir)
	if err != nil {
		t.Fatal(err)
	}
	service.References = sourceform.SiviParentSharedReferenceReaders{Region: region, Site: site, Parent: parent, Geology: geology, Bec: bec}
	t.Cleanup(func() {
		for _, close := range []func() error{region.Close, site.Close, parent.Close, geology.Close, bec.Close} {
			if err := close(); err != nil {
				t.Error(err)
			}
		}
	})
	return service, state, original
}

func SharedServiceFixture(t *testing.T) (*sourceform.SIVIParentSharedService, vprocore.ProjectState, *sourceform.SIVIParentProjection, sourceform.SIVIParentSharedWrite) {
	t.Helper()
	contexts, state, db, original, _ := SiviParentWriteFixture(t, true, 3)
	service, err := sourceform.NewSIVIParentSharedService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET rowid=901 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	original, err = sourceform.ReadSIVIParent(contexts, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := []sourceform.SIVIParentCellEdit{}
	for _, column := range []string{"AirPhotoNum", "XCoord", "YCoord", "StrataCoverTree", "StrataCoverShrub", "StrataCoverHerb", "StrataCoverMoss", "VegNotes"} {
		value := vprocore.ProjectMetadataCell{Storage: "real", Real: vprotest.SiviSharedTestPointer(3.25)}
		if column == "AirPhotoNum" || column == "VegNotes" {
			value = vprocore.ProjectMetadataCell{Storage: "text", Text: vprotest.SiviSharedTestPointer("  Literal  ")}
		}
		edits = append(edits, SiviParentEdit(t, original, column, value))
	}
	return service, state, original, sourceform.SIVIParentSharedWrite{Original: original, Edits: edits}
}

func SiviStorageTextEdits(parent *sourceform.SiviParentProjection) []sourceform.SiviParentScalarEdit {
	edits := []sourceform.SiviParentScalarEdit{}
	for _, field := range []struct{ column, value string }{
		{"SV_PolygonNumber", "  New Polygon  "}, {"SV_CanopyComposition", "Literal canopy"},
	} {
		for index, column := range parent.EnvColumns {
			if column.Name == field.column {
				edits = append(edits, sourceform.SiviParentScalarEdit{
					ContextID: parent.ContextID, Table: parent.EnvTable, RowID: parent.Rows[0].Env.RowID,
					Column: field.column, Expected: vprocore.CloneSiteUnitCell(parent.Rows[0].Env.Cells[index]), Value: vprotest.MetadataText(field.value),
				})
			}
		}
	}
	return edits
}

func SiviParentWriteFixture(t *testing.T, external bool, strength int) (*vprocore.ContextService, vprocore.ProjectState, *sql.DB, *sourceform.SiviParentProjection, sourceform.SiviParentDirectEdits) {
	t.Helper()
	service, state := vprotest.ReportServiceFixture(t, external)
	if err := service.Plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin;
		INSERT INTO Sample_Env(PlotNumber,SV_StandHeight,SV_AhorizonDepth,SV_GleyingMottlingCM,SV_PercentCoarseFrags,
			SV_SoilDepth,SV_FloodPlain,SV_StandAgeEstMeas,SV_StandHeightEstMeas,SV_PolygonNumber,SV_CanopyComposition,
			SnowCoverregime,SV_RootZoneTexture,SV_AhorizonType)
		VALUES('108050',2,2,2,2,2,0,'1','1','original polygon','original canopy','S','original texture','Ah');
		INSERT INTO Sample_Admin(Plot,StrataCoverTotal) VALUES('108050',3);
		INSERT INTO Sample_Env(PlotNumber) VALUES('108051'); INSERT INTO Sample_Admin(Plot) VALUES('108051')`); err != nil {
		t.Fatal(err)
	}
	parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := SiviProposalEdits(parent)
	return service, state, db, parent, sourceform.SiviParentDirectEdits{Scalars: scalars, Options: options, Text: SiviStorageTextEdits(parent), Categorical: SiviCategoricalProposalEdits(parent)}
}

func SiviParentEdit(t *testing.T, parent *sourceform.SiviParentProjection, column string, value vprocore.ProjectMetadataCell) sourceform.SiviParentScalarEdit {
	t.Helper()
	for _, binding := range parent.Bindings {
		if binding.Binding != column {
			continue
		}
		row := parent.Rows[0].Env
		if binding.Table == parent.AdminTable {
			row = parent.Rows[0].Admin
		}
		return sourceform.SiviParentScalarEdit{parent.ContextID, binding.Table, row.RowID, column, vprocore.CloneSiteUnitCell(row.Cells[binding.Column]), value}
	}
	t.Fatal("controlled parent target missing", column)
	return sourceform.SiviParentScalarEdit{}
}

func SiviParentCell(t *testing.T, parent *sourceform.SiviParentProjection, column string) vprocore.ProjectMetadataCell {
	t.Helper()
	return SiviParentEdit(t, parent, column, vprocore.ProjectMetadataCell{Storage: "null"}).Expected
}

func SiviProjectAssignmentWriteFixture(t *testing.T, external bool, source, strength int, before any) (*vprocore.ContextService, vprocore.ProjectState, *sql.DB, *sourceform.SiviParentProjection, sourceform.SiviProjectSelection) {
	t.Helper()
	service, state := vprotest.ContextServiceFixture(t)
	if external {
		path := filepath.Join(t.TempDir(), "external # assignment.db")
		data, err := os.ReadFile(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		selected := vprotest.ContextSelection(state)
		selected.ProjectPath = path
		state, err = service.SwitchContext(state.ContextID, selected)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	if err := service.Projects.Preferences.Update("Current", map[string]any{"ProjectIdSource": source}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID=? WHERE PlotNumber='108050';
		DELETE FROM Sample_Metadata; INSERT INTO Sample_Metadata(ID,ProjectID,ProjectTitle)
		VALUES(1,'  Existing 🌱  ',''),(2,'  Existing 🌱  ',NULL)`, before); err != nil {
		t.Fatal(err)
	}
	vprotest.MutateContextFixture(t, service.Projects.SupportPaths["VMetaData"], `DELETE FROM ProjectMetadata;
		INSERT INTO ProjectMetadata(ProjectID,ProjectTitle) VALUES('other',''),('  Existing 🌱  ',NULL)`)
	parent, selection := SiviProjectAssignmentWriteSelection(t, service, state)
	return service, state, db, parent, selection
}

func SiviProjectAssignmentWriteSelection(t *testing.T, service *vprocore.ContextService, state vprocore.ProjectState) (*sourceform.SiviParentProjection, sourceform.SiviProjectSelection) {
	t.Helper()
	parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	choices, err := sourceform.ReadSIVIProjectChoices(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return parent, sourceform.SiviProjectSelection{
		ContextID: state.ContextID, ControlID: "form:frmSIVIsite/ProjectID", Table: parent.EnvTable,
		RowID: parent.Rows[0].Env.RowID, Expected: SiviParentCell(t, parent, "ProjectID"),
		SourceOption: choices.SourceOption, MetadataAlias: choices.Alias, MetadataTable: choices.Table,
		MetadataColumns:  append([]vprocore.ProjectMetadataColumn{}, choices.Choices.Columns...),
		MetadataOriginal: choices.Choices.Rows[len(choices.Choices.Rows)-1],
	}
}

func SiviSpeciesTestReferences(t *testing.T) sourceform.SIVISpeciesReferences {
	t.Helper()
	table := func(personal bool) vprocore.ProjectMetadataTable {
		names := sourceform.SiviSpeciesMasterColumns
		if personal {
			names = sourceform.SiviSpeciesPersonalColumns
		}
		result := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
		for _, name := range names {
			kind := "TEXT"
			if strings.EqualFold(name, "Lifeform") {
				kind = "INTEGER"
			}
			result.Columns = append(result.Columns, vprocore.ProjectMetadataColumn{Name: name, DeclaredType: kind})
		}
		return result
	}
	result := sourceform.SIVISpeciesReferences{ContextID: "ctx", Project: "Sample", Plot: "108050", Master: table(false), Personal: table(true)}
	result.Master.Rows = []vprocore.ProjectMetadataRow{
		{RowID: "-9223372036854775808", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), {Storage: "null"}, vprotest.MetadataInteger("3"), vprotest.MetadataText(""), vprotest.MetadataText("U"), {Storage: "null"}}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("C"), vprotest.MetadataText(""), vprotest.MetadataInteger("12"), {Storage: "null"}, vprotest.MetadataText("x"), {Storage: "null"}}},
		{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("D"), {Storage: "null"}, vprotest.MetadataInteger("9"), {Storage: "null"}, vprotest.MetadataText("u"), {Storage: "null"}}},
		{RowID: "4", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("new_a"), {Storage: "null"}, vprotest.MetadataInteger("99"), {Storage: "null"}, vprotest.MetadataText("S"), vprotest.MetadataText("olddup")}},
		{RowID: "5", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("NEW_B"), vprotest.MetadataText("duplicate"), vprotest.MetadataInteger("3"), vprotest.MetadataText(""), vprotest.MetadataText("U"), vprotest.MetadataText("olddup")}},
		{RowID: "6", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("é😀😀😀"), {Storage: "null"}, vprotest.MetadataInteger("1"), {Storage: "null"}, vprotest.MetadataText("X"), {Storage: "null"}}},
		{RowID: "7", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("éé😀😀😀"), {Storage: "null"}, vprotest.MetadataInteger("1"), {Storage: "null"}, vprotest.MetadataText("X"), {Storage: "null"}}},
	}
	result.Personal.Rows = []vprocore.ProjectMetadataRow{
		{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Personal"), {Storage: "null"}, vprotest.MetadataInteger("99"), vprotest.MetadataText(""), vprotest.MetadataText("S")}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("olddup"), {Storage: "null"}, vprotest.MetadataInteger("1"), {Storage: "null"}, vprotest.MetadataText("U")}},
	}
	return result
}

func SiviWriteFixture(t *testing.T, external bool, strength int) (*vprocore.ContextService, vprocore.ProjectState, *sql.DB, []sourceform.SiviVegetationProjection, []sourceform.SiviHeightEdit) {
	t.Helper()
	service, state := vprotest.ReportServiceFixture(t, external)
	if err := service.Plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.Sqlite.Attachments["project"], "rw"))
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
	original, err := sourceform.ReadSIVIVegetation(service, context.Background(), state.ContextID, "108050", false)
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
	return service, state, db, original, []sourceform.SiviHeightEdit{
		{rowID, "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(3)},
		{rowID, "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataText("before"), vprotest.MetadataText("")},
		{rowID, "SubVegC-SIVI", "Height6", vprotest.SiviReal(-3), vprocore.ProjectMetadataCell{Storage: "null"}},
	}
}

func TwoPageEntryReferenceChoice(value string) sourceform.SIVIParentSharedReferenceChoice {
	return sourceform.SIVIParentSharedReferenceChoice{Code: &value, Selectable: true}
}

func TwoPageEntryBorrowedReferenceReaders(t *testing.T) sourceform.SiviParentSharedReferenceReaders {
	t.Helper()
	dir := t.TempDir()
	site, err := referencespkg.NewSiteCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := referencespkg.NewParentCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(site.Close(), parent.Close()); err != nil {
			t.Error(err)
		}
	})
	return sourceform.SiviParentSharedReferenceReaders{Site: site, Parent: parent}
}

func LifeformReferenceFixture(rows ...[]vprocore.ProjectMetadataCell) vprocore.ProjectMetadataTable {
	result := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	for _, name := range reportcore.VegetationLifeformReferenceColumns {
		result.Columns = append(result.Columns, vprocore.ProjectMetadataColumn{Name: name})
	}
	for i, cells := range rows {
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: reportcore.SiteUnitNumericString(float64(i + 1)), Cells: cells})
	}
	return result
}
