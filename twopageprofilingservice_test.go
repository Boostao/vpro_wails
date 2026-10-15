package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func twoPageProfilingFixture(t *testing.T, externalSU, externalProfile bool) (*TwoPageProfilingService, ProjectState, *sql.DB) {
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
	next, err := environment.contexts.SelectPlotProfile(state.ContextID, PlotProfileSource{"Sample", path})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewTwoPageProfilingService(environment.contexts, func(name string) (string, bool) {
		return "true", name == twoPageProfilingEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, next, db
}

func TestTwoPageProfilingOwnedPhysicalOriginalsNoWrites(t *testing.T) {
	for _, externalSU := range []bool{false, true} {
		for _, externalProfile := range []bool{false, true} {
			t.Run(map[bool]string{false: "embeddedSU", true: "externalSU"}[externalSU]+
				map[bool]string{false: "-embeddedProfile", true: "-externalProfile"}[externalProfile], func(t *testing.T) {
				service, state, _ := twoPageProfilingFixture(t, externalSU, externalProfile)
				files := databaseBytes(t, service.contexts.projects.sqlite.attachments)
				config, err := os.ReadFile(service.contexts.projects.preferences.path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 200)
				if err != nil {
					t.Fatal(err)
				}
				if got.ContextID != state.ContextID || got.Project != state.ActiveProject || got.ProjectPath != state.ProjectPath ||
					got.SU != state.ActiveSU || got.SUPath != state.SUPath || got.Plot != "108050" ||
					got.TargetForm != "USysPlotProfiling" || got.ProfileTable != "Sample_Profile" ||
					got.Source != state.PlotProfile.Source || got.OrderingPolicy != twoPageProfilingOrdering ||
					got.TotalRows != 4 || !got.DescriptionsPresent || len(got.DescriptionOriginal.Rows) != 3 ||
					len(got.EnvOriginal.Rows) != 1 || got.EnvOriginal.Rows[0].RowID != "-1" ||
					len(got.AdminOriginal.Rows) != 1 || got.AdminOriginal.Rows[0].RowID != "-2" ||
					len(got.ProfileOriginal.Columns) != 10 {
					t.Fatal("source ownership/full physical witnesses changed", got)
				}
				indices, err := siteUnitTransferColumns(got.ProfileOriginal, append(plotProfileColumns, "Extra")...)
				if err != nil {
					t.Fatal(err)
				}
				rows := got.ProfileOriginal.Rows
				if len(rows) != 4 || rows[0].RowID != "-7" || rows[1].RowID != "2" || rows[2].RowID != "3" || rows[3].RowID != "4" ||
					rows[0].Cells[indices["Order"]].Storage != "null" ||
					*rows[1].Cells[indices["Order"]].Integer != "10" || *rows[2].Cells[indices["Order"]].Integer != "10" ||
					*rows[3].Cells[indices["Order"]].Text != "not integer" ||
					*rows[0].Cells[indices["Criteria"]].Text != strings.Repeat("x", 256) ||
					*rows[0].Cells[indices["Extra"]].BlobHex != "ff00" ||
					*rows[1].Cells[indices["PlotCount"]].Integer != "-1" ||
					*rows[2].Cells[indices["PlotCount"]].Text != "historical count" ||
					*rows[3].Cells[indices["Table"]].BlobHex != "4142" {
					t.Fatal("source historical/duplicate/NULL/storage repaired", rows)
				}
				description, err := siteUnitTransferColumns(got.DescriptionOriginal, "description", "Extra")
				if err != nil {
					t.Fatal(err)
				}
				dr := got.DescriptionOriginal.Rows
				if dr[0].RowID != "-1" || dr[0].Cells[description["description"]].Storage != "null" ||
					dr[1].RowID != "2" || *dr[1].Cells[description["description"]].Text != "" ||
					dr[2].RowID != "3" || *dr[2].Cells[description["description"]].Text != "source description" ||
					*dr[0].Cells[description["Extra"]].BlobHex != "ff00" {
					t.Fatal("Description NULL/empty/duplicates/extras repaired", dr)
				}
				page, err := service.GetReview(context.Background(), state.ContextID, "108050", 1, 2)
				if err != nil || page.TotalRows != 4 || !reflect.DeepEqual(page.ProfileOriginal.Rows, rows[1:3]) ||
					!reflect.DeepEqual(page.DescriptionOriginal, got.DescriptionOriginal) {
					t.Fatal("physical page differs from complete original", page, err)
				}
				if !reflect.DeepEqual(files, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
					t.Fatal("saved-data review wrote family/source/scratch/audit bytes")
				}
				after, err := os.ReadFile(service.contexts.projects.preferences.path)
				if err != nil || !reflect.DeepEqual(config, after) {
					t.Fatal("saved-data review wrote effective config", err)
				}
			})
		}
	}
}

func TestTwoPageProfilingExplicitSelectionScopeAndRetirement(t *testing.T) {
	environment, state, _ := twoPageEnvironmentTableFixture(t, false)
	service, err := NewTwoPageProfilingService(environment.contexts, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil ||
		!strings.Contains(err.Error(), "fallback") {
		t.Fatal("implicit project-local profile granted mapped authority", got, err)
	}
	selected, err := environment.contexts.SelectPlotProfile(state.ContextID, PlotProfileSource{"Sample", state.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("retired context returned mapped profile", got, err)
	}
	for _, plot := range []string{"", "108050 ", "outside", "A", "a", "orphan", string([]byte{0xff})} {
		if got, err := service.GetReview(context.Background(), selected.ContextID, plot, 0, 1); err == nil || got != nil {
			t.Fatal("invalid/unselected/ambiguous/unpaired literal parent returned rules", plot, got, err)
		}
	}
	none, err := environment.contexts.SelectPlotProfile(selected.ContextID, PlotProfileSource{"None", ""})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), none.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("None profile fell back to project rules", got, err)
	}
}

func TestTwoPageProfilingGateInputsAndPaging(t *testing.T) {
	service, state, db := twoPageProfilingFixture(t, false, false)
	for _, value := range []string{"", "false", "TRUE", "1", " true "} {
		gated, err := NewTwoPageProfilingService(service.contexts, func(string) (string, bool) { return value, value != "" })
		if value == "" || value == "false" {
			if err != nil {
				t.Fatal(err)
			}
			if got, err := gated.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
				t.Fatal("default/off service returned mapped data", got, err)
			}
		} else if err == nil {
			t.Fatal("nonliteral gate accepted", value)
		}
	}
	if _, err := NewTwoPageProfilingService(nil, os.LookupEnv); err == nil {
		t.Fatal("missing owner accepted")
	}
	if _, err := NewTwoPageProfilingService(service.contexts, nil); err == nil {
		t.Fatal("missing lookup accepted")
	}
	if got, err := service.GetReview(nil, state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("nil request accepted", got, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.GetReview(cancelled, state.ContextID, "108050", 0, 1); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled request accepted", got, err)
	}
	for _, page := range []struct{ offset, limit int }{{-1, 1}, {100001, 1}, {0, 0}, {0, 201}, {int(^uint(0) >> 1), 1}} {
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", page.offset, page.limit); err == nil || got != nil {
			t.Fatal("invalid bounds returned data", page, got, err)
		}
	}
	if _, err := db.Exec(`DELETE FROM Sample_Profile;
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<201)
		INSERT INTO Sample_Profile("Order") SELECT i FROM n`); err != nil {
		t.Fatal(err)
	}
	for _, page := range []struct{ offset, limit, count int }{{0, 200, 200}, {200, 1, 1}, {201, 50, 0}, {100000, 200, 0}} {
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", page.offset, page.limit)
		if err != nil || got == nil || got.TotalRows != 201 || len(got.ProfileOriginal.Rows) != page.count ||
			got.Offset != page.offset || got.Limit != page.limit {
			t.Fatal("exact/end/beyond-end paging changed", page, got, err)
		}
	}
	if _, err := db.Exec(`DELETE FROM Sample_Profile;DELETE FROM _table_metadata`); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got.TotalRows != 0 || len(got.ProfileOriginal.Columns) != 10 ||
		!got.DescriptionsPresent || len(got.DescriptionOriginal.Columns) != 3 || len(got.DescriptionOriginal.Rows) != 0 {
		t.Fatal("empty physical source or present-empty Description lost", got, err)
	}
	if _, err := db.Exec(`DROP TABLE _table_metadata`); err != nil {
		t.Fatal(err)
	}
	got, err = service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got.DescriptionsPresent || len(got.DescriptionOriginal.Columns) != 0 {
		t.Fatal("absent Description treated as present-empty", got, err)
	}
}

func TestTwoPageProfilingNoneSUAndLiteralNamedSource(t *testing.T) {
	service, state, db := twoPageProfilingFixture(t, true, false)
	name := " Profile O'Brien #é "
	table := name + "_Profile"
	if _, err := db.Exec(`ALTER TABLE Sample_Profile RENAME TO ` + quoteHeaderIdentifier(table)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE _table_metadata SET table_name=? WHERE table_name='Sample_Profile'`, table); err != nil {
		t.Fatal(err)
	}
	named, err := service.contexts.SelectPlotProfile(state.ContextID, PlotProfileSource{name, state.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	selection := contextSelection(named)
	selection.SU, selection.SUPath = "None", ""
	none, err := service.contexts.SwitchContext(named.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), none.ContextID, "108050", 0, 200)
	if err != nil || got == nil || got.SU != "None" || got.SUPath != "" || got.Source.Name != name ||
		got.ProfileTable != table || got.TotalRows != 4 || len(got.DescriptionOriginal.Rows) != 3 {
		t.Fatal("None SU changed literal named profile authority or filtered its rules", got, err)
	}
}

func TestTwoPageProfilingSnapshotRetainsPreMutationWitnesses(t *testing.T) {
	service, state, db := twoPageProfilingFixture(t, false, false)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	owner := service.contexts.projects.sqlite
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("snapshot witness requires actual SQLite")
		}
		return conn.RegisterFunc("profilingSnapshotWitness", func(value any) (any, error) {
			_, err := db.Exec(`UPDATE Sample_Env SET FieldNumber='changed';
				UPDATE Sample_Admin SET UserSiteUnit='changed';
				UPDATE Sample_Profile SET Criteria='changed';
				UPDATE _table_metadata SET description='changed'`)
			return value, err
		}, false)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
		CREATE TEMP VIEW USysEnv AS SELECT profilingSnapshotWitness(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	env, err := siteUnitTransferColumns(got.EnvOriginal, "FieldNumber")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := siteUnitTransferColumns(got.AdminOriginal, "UserSiteUnit")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := siteUnitTransferColumns(got.ProfileOriginal, "Criteria")
	if err != nil {
		t.Fatal(err)
	}
	descriptions, err := siteUnitTransferColumns(got.DescriptionOriginal, "description")
	if err != nil {
		t.Fatal(err)
	}
	if got.EnvOriginal.Rows[0].Cells[env["FieldNumber"]].Storage != "null" ||
		got.AdminOriginal.Rows[0].Cells[admin["UserSiteUnit"]].Storage != "null" ||
		*got.ProfileOriginal.Rows[0].Cells[rules["Criteria"]].Text != strings.Repeat("x", 256) ||
		got.DescriptionOriginal.Rows[0].Cells[descriptions["description"]].Storage != "null" {
		t.Fatal("parent/profile/Description witnesses crossed snapshots", got)
	}
	var changed string
	if err := db.QueryRow(`SELECT Criteria FROM Sample_Profile WHERE rowid=-7`).Scan(&changed); err != nil || changed != "changed" {
		t.Fatal("independent concurrent fixture mutation did not commit", changed, err)
	}
}

func TestTwoPageProfilingSourceSchemaTextAndCorrection(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE Sample_Profile SET Criteria=CAST(X'FF' AS TEXT) WHERE rowid=4`,
		`UPDATE _table_metadata SET description=CAST(X'FF' AS TEXT) WHERE rowid=4`,
		`UPDATE _table_metadata SET table_name=X'4142' WHERE rowid=4`,
		`ALTER TABLE Sample_Profile RENAME COLUMN Species TO Missing`,
		`ALTER TABLE Sample_Profile ADD COLUMN rowid`,
		`ALTER TABLE Sample_Profile ADD COLUMN Derived GENERATED ALWAYS AS ("Order"+1) VIRTUAL`,
		`ALTER TABLE Sample_Profile RENAME TO Original; CREATE VIEW Sample_Profile AS SELECT * FROM Original`,
		`DROP TABLE Sample_Profile;CREATE TABLE Sample_Profile("Order" PRIMARY KEY,"Table","Field","Operator","Layer","Species","Criteria","Operation","PlotCount") WITHOUT ROWID`,
		`ALTER TABLE _table_metadata RENAME COLUMN description TO Missing`,
		`ALTER TABLE _table_metadata ADD COLUMN rowid`,
		`ALTER TABLE _table_metadata RENAME TO Original; CREATE VIEW _table_metadata AS SELECT * FROM Original`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, state, db := twoPageProfilingFixture(t, false, false)
			if _, err := db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
				t.Fatal("malformed source/schema returned partial or repaired page", got, err)
			}
		})
	}
	service, state, db := twoPageProfilingFixture(t, false, false)
	if _, err := db.Exec(`UPDATE Sample_Profile SET Criteria=CAST(X'FF' AS TEXT) WHERE rowid=4`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("malformed off-page text was hidden", got, err)
	}
	if _, err := db.Exec(`UPDATE Sample_Profile SET Criteria='' WHERE rowid=4`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err != nil || got == nil {
		t.Fatal("corrected source could not retry", got, err)
	}
}

func TestTwoPageProfilingMaximumPhysicalBounds(t *testing.T) {
	for _, table := range []string{"Sample_Profile", "_table_metadata"} {
		t.Run(table, func(t *testing.T) {
			service, state, db := twoPageProfilingFixture(t, false, false)
			if _, err := db.Exec(`DELETE FROM ` + table + `;
				WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<100000)
				INSERT INTO ` + table + `(rowid) SELECT i FROM n`); err != nil {
				t.Fatal(err)
			}
			got, err := service.GetReview(context.Background(), state.ContextID, "108050", 100000, 1)
			if err != nil || got == nil || len(got.ProfileOriginal.Rows) != 0 ||
				table == "Sample_Profile" && got.TotalRows != 100000 {
				t.Fatal("exact100000 physical source refused", got, err)
			}
			if _, err := db.Exec(`INSERT INTO ` + table + `(rowid) VALUES(100001)`); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
				t.Fatal("over100000 returned partial review", got, err)
			}
		})
	}
}

func TestTwoPageProfilingInFlightCancellationAndRetry(t *testing.T) {
	service, state, _ := twoPageProfilingFixture(t, false, false)
	owner := service.contexts.projects.sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("profiling cancellation requires actual SQLite")
		}
		return conn.RegisterFunc("profilingReadBarrier", func(value any) any {
			once.Do(func() { close(entered) })
			<-release
			return value
		}, false)
	}); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := owner.conn.QueryRowContext(context.Background(), `SELECT sql FROM sqlite_temp_master WHERE name='USysEnv'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
		CREATE TEMP VIEW USysEnv AS SELECT profilingReadBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := service.GetReview(ctx, state.ContextID, "108050", 0, 1)
		if got != nil {
			err = errors.Join(err, errors.New("cancelled profiling read returned data"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("owned profiling read did not reach SQLite")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled profiling read did not settle")
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;`+
		strings.Replace(original, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got == nil || got.TotalRows != 4 {
		t.Fatal("cancelled pinned coordinator lost source/attachments/TEMP views", got, err)
	}
}
