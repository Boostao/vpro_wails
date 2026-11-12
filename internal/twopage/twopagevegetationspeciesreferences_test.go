package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func TestTwoPageVegetationSpeciesReferencesPhysicalDefinitionsAndFreshRead(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprint("external-", external), func(t *testing.T) {
			service, state, _ := twoPageVegetationReadFixture(t, external, "TEXT")
			vprotest.MutateContextFixture(t, service.Projects.Sqlite.Attachments["VLists"], `DELETE FROM USysAllSpecs;
				INSERT INTO USysAllSpecs(rowid,Code,ScientificName,Lifeform,EnglishName,Codetype,OldCode) VALUES
				(-9,'A',NULL,3,'','U','old'),
				(-8,'A','',3,NULL,'U','old'),
				(-7,NULL,'historical',NULL,'',NULL,NULL),
				(-6,'historical overlength',NULL,99,NULL,'s','')`)
			// Fixture-only legacy shape; ordinary personal Code is unique and NOT NULL.
			vprotest.MutateContextFixture(t, service.Projects.Sqlite.Attachments["VUser"], `DROP TABLE USysUserSpp;
				CREATE TABLE USysUserSpp(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
				INSERT INTO USysUserSpp(rowid,Code,ScientificName,LifeForm,EnglishName,Codetype) VALUES
				(-9,'A',NULL,3,'','U'),
				(-8,'A','',3,NULL,'U'),
				(-7,NULL,NULL,NULL,NULL,NULL),
				(-6,'',NULL,99,NULL,'s')`)
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			var retained *twoPageVegetationSpeciesReferences
			for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
				got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", form)
				if err != nil || got == nil {
					t.Fatal("owned physical references unavailable", got, err)
				}
				if got.ContextID != state.ContextID || got.Project != "Sample" || got.Plot != "108050" ||
					got.Form != form || !strings.Contains(got.Source, "VLists.USysAllSpecs/VUser.USysUserSpp") ||
					len(got.Master.Rows) != 4 || len(got.Personal.Rows) != 4 {
					t.Fatalf("source owner/physical duplicates lost: %+v", got)
				}
				if !reflect.DeepEqual(got.Master.Columns, []vprocore.ProjectMetadataColumn{
					{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "Lifeform", DeclaredType: "INTEGER"},
					{Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Codetype", DeclaredType: "TEXT"}, {Name: "OldCode", DeclaredType: "TEXT"},
				}) || !reflect.DeepEqual(got.Personal.Columns, []vprocore.ProjectMetadataColumn{
					{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "Lifeform", DeclaredType: "INTEGER"},
					{Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Codetype", DeclaredType: "TEXT"},
				}) {
					t.Fatalf("physical declared columns were replaced with union display order: %+v %+v", got.Master.Columns, got.Personal.Columns)
				}
				for _, table := range []vprocore.ProjectMetadataTable{got.Master, got.Personal} {
					if table.Rows[0].RowID != "-9" || table.Rows[1].RowID != "-8" ||
						!reflect.DeepEqual(table.Rows[0].Cells[0], vprotest.MetadataText("A")) ||
						!reflect.DeepEqual(table.Rows[1].Cells[0], vprotest.MetadataText("A")) ||
						table.Rows[0].Cells[1].Storage != "null" ||
						!reflect.DeepEqual(table.Rows[1].Cells[1], vprotest.MetadataText("")) ||
						!reflect.DeepEqual(table.Rows[0].Cells[3], vprotest.MetadataText("")) ||
						table.Rows[1].Cells[3].Storage != "null" ||
						table.Rows[2].Cells[0].Storage != "null" ||
						!reflect.DeepEqual(table.Rows[3].Cells[2], vprotest.MetadataInteger("99")) {
						t.Fatal("physical identities, duplicate/NULL/empty metadata or unlisted definitions collapsed", table)
					}
				}
				if !reflect.DeepEqual(got.Master.Rows[0].Cells[5], vprotest.MetadataText("old")) ||
					!reflect.DeepEqual(got.Master.Rows[3].Cells[5], vprotest.MetadataText("")) ||
					!reflect.DeepEqual(got.Master.Rows[3].Cells[0], vprotest.MetadataText("historical overlength")) {
					t.Fatal("alias/empty/historical code was repaired or filtered")
				}
				retained = got
			}
			if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
				t.Fatal("reference reads mutated an owned database")
			}
			vprotest.MutateContextFixture(t, service.Projects.Sqlite.Attachments["VLists"], `UPDATE USysAllSpecs SET ScientificName='corrected' WHERE rowid=-9`)
			fresh, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", "SubVegDXL")
			if err != nil || fresh == nil || !reflect.DeepEqual(fresh.Master.Rows[0].Cells[1], vprotest.MetadataText("corrected")) ||
				retained.Master.Rows[0].Cells[1].Storage != "null" {
				t.Fatal("fresh definitions replaced with frozen catalogue or prior response mutated", fresh, err)
			}
		})
	}
}

func TestTwoPageVegetationSpeciesReferencesMalformedSourceCorrection(t *testing.T) {
	service, state, _ := twoPageVegetationReadFixture(t, false, "TEXT")
	for _, source := range []struct {
		role, table string
		columns     []string
	}{
		{"VLists", "USysAllSpecs", []string{"Code", "ScientificName", "Lifeform", "EnglishName", "Codetype", "OldCode"}},
		{"VUser", "USysUserSpp", []string{"Code", "ScientificName", "Lifeform", "EnglishName", "Codetype"}},
	} {
		for _, column := range source.columns {
			t.Run(source.role+"/"+column, func(t *testing.T) {
				path := service.Projects.Sqlite.Attachments[source.role]
				vprotest.MutateContextFixture(t, path, `DELETE FROM `+source.table+`; INSERT INTO `+source.table+
					`(rowid,Code) VALUES(-9,'A'); UPDATE `+source.table+` SET `+vprocore.QuoteHeaderIdentifier(column)+`=CAST(X'FF' AS TEXT) WHERE rowid=-9`)
				before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", "SubVegAXL_BC")
				if got != nil || err == nil || !strings.Contains(err.Error(), "malformed Unicode") ||
					!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
					t.Fatal("malformed source was repaired/partially transported or refusal wrote data", got, err)
				}
				correction := "NULL"
				if source.role == "VUser" && column == "Code" {
					correction = "'A'"
				}
				vprotest.MutateContextFixture(t, path, `UPDATE `+source.table+` SET `+vprocore.QuoteHeaderIdentifier(column)+`=`+correction+` WHERE rowid=-9`)
				if got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", "SubVegAXL_BC"); got == nil || err != nil {
					t.Fatal("corrected source could not be retried", got, err)
				}
			})
		}
	}
}

func TestTwoPageVegetationSpeciesReferencesOwnerRefusalsAndContextRotation(t *testing.T) {
	service, state, db := vprotest.TwoPageSoilReadFixture(t, true, "TEXT")
	for _, test := range []struct{ contextID, plot, form string }{
		{state.ContextID, "", "SubVegAXL"}, {state.ContextID, "\xff", "SubVegAXL"},
		{state.ContextID, "0108050", "SubVegAXL"}, {state.ContextID, "108050 ", "SubVegCXL"},
		{state.ContextID, "outside", "SubVegDXL"}, {"stale", "108050", "SubVegAXL"},
		{state.ContextID, "108050", "SubVegA"}, {state.ContextID, "108050", "unknown"},
	} {
		if got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), test.contextID, test.plot, test.form); got != nil || err == nil {
			t.Fatal("invalid literal owner/source accepted", test, got, err)
		}
	}
	var absent *vprocore.ContextService
	if got, err := ReadTwoPageVegetationSpeciesReferences(absent, context.Background(), state.ContextID, "108050", "SubVegAXL"); got != nil || err == nil {
		t.Fatal("nil service accepted", got, err)
	}
	if got, err := ReadTwoPageVegetationSpeciesReferences(service, nil, state.ContextID, "108050", "SubVegAXL"); got != nil || err == nil {
		t.Fatal("nil context accepted", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := ReadTwoPageVegetationSpeciesReferences(service, ctx, state.ContextID, "108050", "SubVegAXL"); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request accepted", got, err)
	}
	old, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", "SubVegAXL")
	if err != nil {
		t.Fatal(err)
	}
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil || next.ContextID == state.ContextID {
		t.Fatal("context did not actually rotate", next, err)
	}
	if got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", "SubVegAXL"); got != nil || err == nil {
		t.Fatal("stale context inherited read authority", got, err)
	}
	fresh, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), next.ContextID, "108050", "SubVegAXL")
	if err != nil || fresh == nil || fresh.ContextID != next.ContextID ||
		!reflect.DeepEqual(fresh.Master, old.Master) || !reflect.DeepEqual(fresh.Personal, old.Personal) {
		t.Fatal("new owner changed reference storage", fresh, err)
	}
	if _, err := db.Exec(`CREATE TABLE Duplicate_SpeciesParent AS SELECT * FROM Sample_Admin;
		INSERT INTO Duplicate_SpeciesParent SELECT * FROM Sample_Admin WHERE Plot='108050';
		ALTER TABLE Sample_Admin RENAME TO Archived_SpeciesParent;
		ALTER TABLE Duplicate_SpeciesParent RENAME TO Sample_Admin`); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), next.ContextID, "108050", "SubVegAXL"); got != nil || err == nil {
		t.Fatal("nonunique physical parent granted references", got, err)
	}
}

func TestTwoPageVegetationSpeciesReferencesInFlightCancellationRetainsCoordinator(t *testing.T) {
	service, state, _ := twoPageVegetationReadFixture(t, false, "TEXT")
	owner := service.Projects.Sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.Conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("vegetation Species cancellation requires the actual SQLite driver")
		}
		return conn.RegisterFunc("vegetationSpeciesBarrier", func(value any) any {
			once.Do(func() { close(entered) })
			<-release
			return value
		}, false)
	}); err != nil {
		t.Fatal(err)
	}
	var originalView string
	if err := owner.Conn.QueryRowContext(context.Background(), `SELECT sql FROM sqlite_temp_master WHERE name='USysEnv'`).Scan(&originalView); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
		CREATE TEMP VIEW USysEnv AS SELECT vegetationSpeciesBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := ReadTwoPageVegetationSpeciesReferences(service, ctx, state.ContextID, "108050", "SubVegAXL")
		if got != nil {
			err = errors.Join(err, errors.New("cancelled Species reference read returned definitions"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("actual reference read did not reach the cancellation barrier")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight reference read did not cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled reference read did not finish")
	}
	if _, err := owner.Conn.ExecContext(context.Background(), `DROP VIEW USysEnv; `+
		strings.Replace(originalView, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatal("read snapshot cancellation discarded pinned coordinator", err)
	}
	if got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", "SubVegAXL"); got == nil || err != nil {
		t.Fatal("correction/retry lost attachments or TEMP views", got, err)
	}
}

func TestTwoPageVegetationSpeciesReferencesPhysicalSchemaRefusals(t *testing.T) {
	for _, source := range []struct{ role, table string }{{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"}} {
		for _, drift := range []string{"missing", "view", "missing-column", "rowid", "_rowid_", "oid", "generated"} {
			t.Run(source.role+"/"+drift, func(t *testing.T) {
				service, state, _ := vprotest.TwoPageSoilReadFixture(t, false, "TEXT")
				table := vprocore.QuoteHeaderIdentifier(source.table)
				var query string
				switch drift {
				case "missing":
					query = `DROP TABLE ` + table
				case "view":
					query = `ALTER TABLE ` + table + ` RENAME TO Archived_SpeciesDefinitions;
						CREATE VIEW ` + table + ` AS SELECT rowid AS rowid,* FROM Archived_SpeciesDefinitions`
				case "missing-column":
					query = `ALTER TABLE ` + table + ` RENAME COLUMN ScientificName TO WrongName`
				case "rowid", "_rowid_", "oid":
					query = `ALTER TABLE ` + table + ` ADD COLUMN ` + vprocore.QuoteHeaderIdentifier(drift) + ` INTEGER`
				case "generated":
					query = `ALTER TABLE ` + table + ` ADD COLUMN Synthetic TEXT GENERATED ALWAYS AS ('fake') VIRTUAL`
				}
				vprotest.MutateContextFixture(t, service.Projects.Sqlite.Attachments[source.role], query)
				before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				got, err := ReadTwoPageVegetationSpeciesReferences(service, context.Background(), state.ContextID, "108050", "SubVegAXL")
				if got != nil || err == nil || !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
					t.Fatal("missing/view/ambiguous/synthetic source schema accepted or refusal wrote data", got, err)
				}
			})
		}
	}
}
