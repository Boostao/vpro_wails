package integration

import (
	"github.com/boostao/vpro-wails/internal/classification"
	"github.com/boostao/vpro-wails/internal/datasheet"
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/spatial"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"

	"os"
	"path/filepath"
	"reflect"
	"strings"

	"testing"
)

func twoPageEnvironmentTableFixture(t *testing.T, external bool) (*datasheet.TwoPageEnvironmentTableService, vprocore.ProjectState, *sql.DB) {
	t.Helper()
	contexts, state, db := vprotest.TwoPageSoilReadFixture(t, external, "TEXT")
	preferences, err := vprocore.OpenDesktopConfig(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	contexts.Projects.Preferences = preferences
	definitions := func(table string, extras ...string) string {
		columns, err := vprocore.ReadSQLiteStorageColumns(context.Background(), db, "main", table)
		if err != nil {
			t.Fatal(err)
		}
		var fields []string
		seen := map[string]bool{}
		for _, column := range columns {
			fields = append(fields, vprocore.QuoteHeaderIdentifier(column.Name)+" "+column.DeclaredType)
			seen[strings.ToLower(column.Name)] = true
		}
		for _, name := range extras {
			if !seen[strings.ToLower(name)] {
				fields = append(fields, vprocore.QuoteHeaderIdentifier(name))
			}
		}
		return strings.Join(fields, ",")
	}
	envDefinition := definitions("Sample_Env", "Unmapped")
	adminDefinition := definitions("Sample_Admin", "Flag", "ID", "Unmapped")
	if _, err := db.Exec(`DROP TABLE Sample_Env; DROP TABLE Sample_Admin;
		CREATE TABLE Sample_Env(` + envDefinition + `);
		CREATE TABLE Sample_Admin(` + adminDefinition + `);
		INSERT INTO Sample_Env(rowid,PlotNumber,RootingDepth,SeepageDepth,FieldNumber,Unmapped) VALUES
		(-1,'108050',0,NULL,NULL,X'FF00'),
		(2,'A',10,2,'same',NULL),(3,'a',10,2,'same',NULL),(4,'A',NULL,NULL,'same',NULL),
		(5,'B',20,3,'historical long text retained',NULL),
		(6,'007',5,1,'',NULL),(7,'7',5,0,NULL,NULL),(8,'orphan',99,99,NULL,NULL),(9,NULL,0,0,NULL,NULL);
		INSERT INTO Sample_Admin(rowid,Plot,UserSiteUnit,Flag,ID,Unmapped) VALUES
		(-2,'108050',NULL,-1,NULL,X'FF00'),
		(2,'A',NULL,0,7,NULL),(3,'a',NULL,0,7,NULL),(4,'A',NULL,0,7,NULL),
		(5,'B',NULL,0,NULL,NULL),(6,'007','',0,'01',NULL),(7,'7',NULL,0,0,NULL),
		(8,'unmatched',NULL,0,NULL,NULL),(9,NULL,NULL,0,NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	service, err := datasheet.NewTwoPageEnvironmentTableService(contexts, func(name string) (string, bool) {
		return "true", name == datasheet.TwoPageEnvironmentTableEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db
}

func twoPageGoogleEarthFixture(t *testing.T, external bool) (*spatial.TwoPageGoogleEarthService, vprocore.ProjectState, *sql.DB) {
	t.Helper()
	contexts, state, db := vprotest.TwoPageSoilReadFixture(t, external, "TEXT")
	if _, err := db.Exec(`UPDATE Sample_Env SET Longitude=123.25,Latitude=53.5,
		PlotRepresenting='<b>Historical & café</b>' WHERE PlotNumber='108050';
		UPDATE Sample_Env SET PlotRepresenting=CAST(X'FF' AS TEXT) WHERE PlotNumber<>'108050'`); err != nil {
		t.Fatal(err)
	}
	vprotest.MutateContextFixture(t, state.SUPath, `INSERT INTO Report_SU VALUES('108050','duplicate membership')`)
	service, err := spatial.NewTwoPageGoogleEarthService(contexts, func(name string) (string, bool) {
		return "true", name == spatial.TwoPageGoogleEarthReviewEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db
}

func twoPageEntryApprovalDraft(t *testing.T, snapshot *twopage.TwoPageEntryReferenceSnapshotInternal) ([]sourceform.SiviParentScalarEdit, []twopage.TwoPageEntryCodeAcknowledgementInternal) {
	t.Helper()
	original := snapshot.Original
	edits := []sourceform.SiviParentScalarEdit{
		sftest.SiviParentEdit(t, original, "FieldNumber", vprotest.MetadataText("  Literal field  ")),
		sftest.SiviParentEdit(t, original, "Zone", vprotest.MetadataText("BG")),
		sftest.SiviParentEdit(t, original, "SubZone", vprotest.MetadataText("xh1")),
	}
	acknowledgements := []twopage.TwoPageEntryCodeAcknowledgementInternal{}
	for _, reference := range snapshot.Fields {
		if reference.Required {
			found := false
			for _, choice := range reference.Choices {
				if choice.Selectable && choice.Code != nil && *choice.Code != "" {
					edits = append(edits, sftest.SiviParentEdit(t, original, reference.Column, vprotest.MetadataText(*choice.Code)))
					found = true
					break
				}
			}
			if !found {
				t.Fatal("required actual fixture lacks a listed code", reference.Column)
			}
		}
		if reference.Column == "RealmClass" || reference.Column == "SiteSeries" {
			edit := sftest.SiviParentEdit(t, original, reference.Column, vprotest.MetadataText("ZZ"))
			edits = append(edits, edit)
			acknowledgements = append(acknowledgements, twopage.TwoPageEntryCodeAcknowledgementInternal{
				ContextID: original.ContextID, Project: original.Project, Plot: original.Plot, Form: original.Form,
				Table: edit.Table, RowID: edit.RowID, Column: edit.Column, Expected: edit.Expected, Value: edit.Value, Reference: reference,
			})
		}
	}
	return edits, acknowledgements
}

func assertTwoPageEntryRestoredPhysicalTables(t *testing.T, form string, before, after map[string]vprocore.ProjectMetadataTable) {
	t.Helper()
	history, err := twopage.TwoPageEntryHistory(form, false)
	if err != nil {
		t.Fatal(err)
	}
	columns, err := vprocore.SiteUnitTransferColumns(after[history.Table], "Restored")
	if err != nil || len(after[history.Table].Rows) != 1 {
		t.Fatal("typed restoration lost its retained historical event", err)
	}
	restored := after[history.Table].Rows[0].Cells[columns["Restored"]]
	if restored.Storage != "text" || restored.Text == nil || *restored.Text == "" {
		t.Fatal("typed restoration failed to mark its retained event", restored)
	}
	delete(after, history.Table)
	schema := after["sqlite_master"]
	schemaColumns, err := vprocore.SiteUnitTransferColumns(schema, "name", "type")
	if err != nil {
		t.Fatal(err)
	}
	schemaRows := []vprocore.ProjectMetadataRow{}
	found := 0
	for _, row := range schema.Rows {
		if reflect.DeepEqual(row.Cells[schemaColumns["name"]], vprotest.MetadataText(history.Table)) {
			if !reflect.DeepEqual(row.Cells[schemaColumns["type"]], vprotest.MetadataText("table")) {
				t.Fatal("retained event schema is not a physical history table")
			}
			found++
		} else {
			schemaRows = append(schemaRows, row)
		}
	}
	if found != 1 {
		t.Fatal("retained history schema identity changed", found)
	}
	schema.Rows = schemaRows
	after["sqlite_master"] = schema
	if !reflect.DeepEqual(before, after) {
		t.Fatal("typed restoration changed original physical tables/schema")
	}
}

func twoPageEntryProviderFixture(t *testing.T, form string) (*twopage.TwoPageEntryReferenceProvider, *vprocore.ContextService, vprocore.ProjectState) {
	t.Helper()
	return twoPageEntryProviderSUFixture(t, form, false)
}

func twoPageEntryProviderSUFixture(t *testing.T, form string, external bool) (*twopage.TwoPageEntryReferenceProvider, *vprocore.ContextService, vprocore.ProjectState) {
	t.Helper()
	contexts, state, _, _, _ := twoPageWriteFixture(t, form, external)
	return twoPageEntryCatalogueProviderFixture(t, form), contexts, state
}

func twoPageEntryCatalogueProviderFixture(t *testing.T, form string) *twopage.TwoPageEntryReferenceProvider {
	t.Helper()
	region, err := references.NewRegionCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	quality, err := references.NewQualityService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	soil, err := references.NewSoilCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bec, err := references.NewBECService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	geology, err := references.NewGeologyCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, close := range []func() error{region.Close, quality.Close, soil.Close, bec.Close, geology.Close} {
			if err := close(); err != nil {
				t.Error(err)
			}
		}
	})
	shared := sftest.TwoPageEntryBorrowedReferenceReaders(t)
	shared.Region, shared.Bec, shared.Geology = region, bec, geology
	provider, err := twopage.NewTwoPageEntryReferenceProvider(form, twopage.TwoPageEntryReferenceReaders{
		Shared: shared, Ecosection: region, Quality: quality, Soil: soil, Series: bec,
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func twoPageWriteFixture(t *testing.T, form string, external bool) (*vprocore.ContextService, vprocore.ProjectState, *sql.DB, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarEdit) {
	t.Helper()
	service, state, db, _, _ := sftest.SiviParentWriteFixture(t, external, 3)
	parent, err := twopage.ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	values := []struct {
		column string
		value  vprocore.ProjectMetadataCell
	}{
		{"SV_WaterTableCM", vprotest.SiviReal(-3.25)}, {"ActiveLayerDepth", vprotest.SiviReal(0.1)},
		{"SV_FullCruiseCard", vprotest.MetadataText("  cArD  ")}, {"PlotSize", vprotest.SiviReal(12.5)},
		{"ProvinceStateTerritory", vprotest.MetadataText("  État  ")}, {"SiteUnitLongName", vprotest.MetadataText("e\u0301 Long Name")},
		{"GIS_BGC", vprotest.MetadataText("  MiXeD  ")}, {"GIS_BGC_VER", vprotest.MetadataInteger("-32768")},
		{"BEC_Use", vprotest.MetadataText("Case Sensitive")},
	}
	edits := []sourceform.SiviParentScalarEdit{}
	for _, field := range values {
		if form == "FS882-8x6XL-CHARS" && field.column == "BEC_Use" {
			continue
		}
		edits = append(edits, sftest.SiviParentEdit(t, parent, field.column, field.value))
	}
	return service, state, db, parent, edits
}

func twoPageProfilingFixture(t *testing.T, externalSU, externalProfile bool) (*classification.TwoPageProfilingService, vprocore.ProjectState, *sql.DB) {
	t.Helper()
	environment, state, db := twoPageEnvironmentTableFixture(t, externalSU)
	if _, err := db.Exec(`DROP TABLE Sample_Profile;
		CREATE TABLE Sample_Profile("Order","Table","Field","Operator","Layer","Species","Criteria","Operation","PlotCount","Extra");
		INSERT INTO Sample_Profile(rowid,"Order","Table","Field","Operator","Layer","Species","Criteria","Operation","PlotCount","Extra") VALUES
		(-7,NULL,'Env',NULL,'=','',NULL,'historical','And',NULL,X'FF00'),
		(2,10,'Veg','Species','=','1','AB','','Or',-1,1.25),
		(3,10,NULL,NULL,NULL,NULL,NULL,NULL,NULL,'historical count',NULL),
		(4,'not integer',X'4142',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL);
		CREATE TEMP TABLE OriginalDescriptions AS SELECT table_name,description FROM _table_metadata;
		DROP TABLE _table_metadata;
		CREATE TABLE _table_metadata(table_name,description,Extra);
		INSERT INTO _table_metadata(rowid,table_name,description,Extra) VALUES
		(-1,'Sample_Profile',NULL,X'FF00'),(2,'Sample_Profile','',NULL),
		(3,'Sample_Profile','source description',-1),(4,'sample_profile','not literal',NULL),
		(5,NULL,'no identity',NULL),(6,'Sample_Profile ','not literal',NULL);
		INSERT INTO _table_metadata(table_name,description)
		SELECT table_name,description FROM OriginalDescriptions WHERE table_name<>'Sample_Profile';
		DROP TABLE OriginalDescriptions`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Profile SET Criteria=? WHERE rowid=-7`, strings.Repeat("x", 256)); err != nil {
		t.Fatal(err)
	}
	path := state.ProjectPath
	if externalProfile {
		path = filepath.Join(t.TempDir(), "profile O'Brien # é.db")
		data, err := os.ReadFile(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	next, err := environment.Contexts.SelectPlotProfile(state.ContextID, vprocore.PlotProfileSource{Name: "Sample", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	service, err := classification.NewTwoPageProfilingService(environment.Contexts, func(name string) (string, bool) {
		return "true", name == classification.TwoPageProfilingEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, next, db
}

func twoPageProjectMetadataFixture(t *testing.T, external bool) (*classification.TwoPageProjectMetadataService, vprocore.ProjectState, *sql.DB) {
	t.Helper()
	environment, state, db := twoPageEnvironmentTableFixture(t, external)
	contexts := environment.Contexts
	columns, err := vprocore.ReadSQLiteStorageColumns(context.Background(), db, "main", "Sample_Metadata")
	if err != nil {
		t.Fatal(err)
	}
	var definitions []string
	for _, column := range columns {
		definitions = append(definitions, vprocore.QuoteHeaderIdentifier(column.Name)+" "+column.DeclaredType)
	}

	if _, err := db.Exec(`DROP TABLE Sample_Metadata;CREATE TABLE Sample_Metadata(` +
		strings.Join(definitions, ",") + `);CREATE UNIQUE INDEX MetadataApplicationID ON Sample_Metadata(ID)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID='Alpha' WHERE PlotNumber='108050';
		DELETE FROM Sample_Metadata;
		ALTER TABLE Sample_Metadata ADD COLUMN Unmapped;
		INSERT INTO Sample_Metadata(rowid,ProjectID,ID,ProjectTitle,Notes,CollectedSite,Unmapped) VALUES
		(-7,'Alpha',NULL,NULL,'',-1,X'FF00'),
		(2,'Alpha',-2147483648,'duplicate',NULL,3,NULL),
		(3,'alpha','historical invalid ID','lowercase',NULL,NULL,NULL),
		(4,'007',7,'leading zero',NULL,NULL,NULL),
		(5,'7',8,'numeric looking',NULL,NULL,NULL),
		(6,'',9,'empty',NULL,NULL,NULL),
		(7,NULL,10,'null',NULL,NULL,NULL),
		(8,'spaced',11,'no terminal space',NULL,NULL,NULL),
		(9,'spaced ',12,'terminal space',NULL,NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	service, err := classification.NewTwoPageProjectMetadataService(contexts, func(name string) (string, bool) {
		return "true", name == classification.TwoPageProjectMetadataEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db
}
