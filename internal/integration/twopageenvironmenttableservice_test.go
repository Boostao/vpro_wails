package integration

import (
	"github.com/boostao/vpro-wails/internal/datasheet"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"

	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

func TestTwoPageEnvironmentTableTargetsPairsAndCompleteOriginals(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, _ := twoPageEnvironmentTableFixture(t, external)
			before := vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
			for _, target := range []string{"FS882-6x4", "FS882-6x4XL", "FS882-8x6XL", "FS882-8x6XL-CHARS"} {
				if err := service.Contexts.Projects.Preferences.Update("Current", map[string]any{"DataFormName": target}); err != nil {
					t.Fatal(err)
				}
				configBefore, err := os.ReadFile(service.Contexts.Projects.Preferences.Path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 200)
				if err != nil {
					t.Fatal(err)
				}
				var ids []string
				for _, row := range got.Rows {
					ids = append(ids, row.EnvRowID+":"+row.AdminRowID)
				}
				want := []string{"6:6", "-1:-2", "7:7", "2:2", "2:3", "2:4", "3:2", "3:3", "3:4", "4:2", "4:3", "4:4", "5:5"}
				if target == "FS882-6x4" {
					want = []string{"5:5", "2:2", "2:3", "2:4", "3:2", "3:3", "3:4", "6:6", "7:7", "-1:-2", "4:2", "4:3", "4:4"}
				}
				if !reflect.DeepEqual(ids, want) || got.TotalRows != 13 || got.TargetForm != target ||
					got.ContextID != state.ContextID || got.ProjectPath != state.ProjectPath || got.SUPath != state.SUPath ||
					got.EnvTable != "Sample_Env" || got.AdminTable != "Sample_Admin" || got.Plot != "108050" ||
					got.OrderingPolicy != datasheet.TwoPageEnvironmentTableOrdering || got.GeneralLocaleVerified ||
					len(got.EnvOriginal.Rows) != 7 || len(got.AdminOriginal.Rows) != 7 ||
					len(got.EnvOriginal.Columns) < 5 || len(got.AdminOriginal.Columns) < 5 {
					t.Fatal("dynamic target/whole-project physical pair ownership changed", target, ids, got)
				}
				adminIndices, err := vprocore.SiteUnitTransferColumns(got.AdminOriginal, "Flag", "ID", "Unmapped")
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range got.AdminOriginal.Rows {
					flag, id, blob := row.Cells[adminIndices["Flag"]], row.Cells[adminIndices["ID"]], row.Cells[adminIndices["Unmapped"]]
					if row.RowID == "-2" && (flag.Integer == nil || *flag.Integer != "-1" ||
						id.Storage != "null" || blob.BlobHex == nil || *blob.BlobHex != "ff00") {
						t.Fatal("complete nullable/BOOLEAN/blob Admin witness changed", row)
					}
				}
				page, err := service.GetReview(context.Background(), state.ContextID, "108050", 4, 2)
				if err != nil || page == nil || !reflect.DeepEqual(page.Rows, got.Rows[4:6]) || page.TotalRows != 13 {
					t.Fatal("slice inside duplicate expansion changed", page, err)
				}
				end, err := service.GetReview(context.Background(), state.ContextID, "108050", 13, 200)
				if err != nil || end == nil || len(end.Rows) != 0 || len(end.EnvOriginal.Rows) != 0 || len(end.AdminOriginal.Rows) != 0 {
					t.Fatal("empty end page retains unrelated originals", end, err)
				}
				configAfter, err := os.ReadFile(service.Contexts.Projects.Preferences.Path)
				if err != nil || !reflect.DeepEqual(configBefore, configAfter) {
					t.Fatal("readonly review changed config", err)
				}
			}
			if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)) {
				t.Fatal("readonly pair review changed original databases")
			}
			nextSelection := vprotest.ContextSelection(state)
			nextSelection.SU, nextSelection.SUPath = "None", ""
			next, err := service.Contexts.SwitchContext(state.ContextID, nextSelection)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
				t.Fatal("retired context returned pair review", got, err)
			}
			got, err := service.GetReview(context.Background(), next.ContextID, "108050", 0, 200)
			if err != nil || got == nil || got.TotalRows != 13 || got.SU != "None" || got.SUPath != "" {
				t.Fatal("None changed whole-project Env/Admin collection", got, err)
			}
		})
	}
}

func TestTwoPageEnvironmentTableConfigStorageErrorsAndRetry(t *testing.T) {
	service, state, db := twoPageEnvironmentTableFixture(t, false)
	for _, value := range []any{"", "FS882-6x4 ", "unknown", nil, 1} {
		if err := service.Contexts.Projects.Preferences.Update("Current", map[string]any{"DataFormName": value}); err != nil {
			t.Fatal(err)
		}
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
			t.Fatal("invalid saved target silently repaired", value, got, err)
		}
	}
	if err := service.Contexts.Projects.Preferences.Update("Current", map[string]any{"DataFormName": "FS882-6x4"}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE Sample_Env SET RootingDepth='not numeric' WHERE rowid=2`,
		`UPDATE Sample_Env SET RootingDepth=10, FieldNumber=CAST(X'FF' AS TEXT) WHERE rowid=2`,
		`UPDATE Sample_Env SET FieldNumber='same', PlotNumber=X'41' WHERE rowid=2`,
		`UPDATE Sample_Admin SET Plot=X'41' WHERE rowid=2`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
			t.Fatal("unsupported original storage returned review", statement, got, err)
		}
		if _, err := db.Exec(`UPDATE Sample_Env SET PlotNumber='A', RootingDepth=10, FieldNumber='same' WHERE rowid=2;
			UPDATE Sample_Admin SET Plot='A' WHERE rowid=2`); err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
		if err != nil || got == nil {
			t.Fatal("corrected saved source cannot retry", got, err)
		}
	}
}

func TestTwoPageEnvironmentTableMissingTargetDefaultIsReadonly(t *testing.T) {
	service, state, _ := twoPageEnvironmentTableFixture(t, false)
	preferences := service.Contexts.Projects.Preferences
	values, err := preferences.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	delete(values["Current"].(map[string]any), "DataFormName")
	data, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preferences.Path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got == nil || got.TargetForm != "FS882-6x4" {
		t.Fatal("source missing-setting default was lost", got, err)
	}
	after, err := os.ReadFile(preferences.Path)
	if err != nil || !reflect.DeepEqual(data, after) {
		t.Fatal("readonly default persisted an invented setting", err)
	}
	service.Contexts.Projects.Preferences = nil
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("unavailable preferences replaced by assumed target", got, err)
	}
}

func TestTwoPageEnvironmentTableBoundsAndVirtualJoin(t *testing.T) {
	service, state, db := twoPageEnvironmentTableFixture(t, false)
	if _, err := db.Exec(`DELETE FROM Sample_Env WHERE rowid != -1; DELETE FROM Sample_Admin WHERE rowid != -2;
		WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<100000)
		INSERT INTO Sample_Env(PlotNumber,RootingDepth) SELECT 'A',1 FROM n`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 200); err == nil ||
		got != nil || !strings.Contains(err.Error(), "100000") {
		t.Fatal("max+1 physical Env input returned partial review", got, err)
	}
	if _, err := db.Exec(`DELETE FROM Sample_Env WHERE rowid=(SELECT MAX(rowid) FROM Sample_Env);
		WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<100000)
		INSERT INTO Sample_Admin(Plot) SELECT 'A' FROM n`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 200); err == nil ||
		got != nil || !strings.Contains(err.Error(), "100000") {
		t.Fatal("max+1 physical Admin input returned partial review", got, err)
	}
	if _, err := db.Exec(`DELETE FROM Sample_Admin WHERE rowid=(SELECT MAX(rowid) FROM Sample_Admin)`); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 9999800000, 200)
	if err != nil || got == nil || got.TotalRows != 9999800002 || len(got.Rows) != 2 ||
		len(got.EnvOriginal.Rows) != 2 || len(got.AdminOriginal.Rows) != 2 {
		t.Fatal("exact max physical input / virtual ten-billion-scale join bound changed", got, err)
	}
	for _, page := range []struct{ offset, limit int }{{-1, 1}, {0, 0}, {0, 201}, {9999800003, 200}, {int(^uint(0) >> 1), 200}} {
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", page.offset, page.limit); err == nil || got != nil {
			t.Fatal("invalid page returned review", page, got, err)
		}
	}
	got, err = service.GetReview(context.Background(), state.ContextID, "108050", 0, 200)
	if err != nil || got == nil || len(got.Rows) != 200 || len(got.EnvOriginal.Rows) != 1 || len(got.AdminOriginal.Rows) != 200 {
		t.Fatal("200-row max page expands source collection", got, err)
	}
}

func TestTwoPageEnvironmentTableInFlightCancellationPreservesCoordinator(t *testing.T) {
	service, state, _ := twoPageEnvironmentTableFixture(t, false)
	owner := service.Contexts.Projects.Sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.Conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("environment table cancellation requires actual SQLite")
		}
		return conn.RegisterFunc("environmentTableReadBarrier", func(value any) any {
			once.Do(func() { close(entered) })
			<-release
			return value
		}, false)
	}); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := owner.Conn.QueryRowContext(context.Background(), `SELECT sql FROM sqlite_temp_master WHERE name='USysEnv'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
		CREATE TEMP VIEW USysEnv AS SELECT environmentTableReadBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := service.GetReview(ctx, state.ContextID, "108050", 0, 200)
		if got != nil {
			err = errors.Join(err, errors.New("cancelled environment table read returned data"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("owned environment read did not reach SQLite")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled environment read did not settle")
	}
	if _, err := owner.Conn.ExecContext(context.Background(), `DROP VIEW USysEnv;`+
		strings.Replace(original, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 200)
	if err != nil || got == nil || got.TotalRows != 13 {
		t.Fatal("cancelled coordinator lost attachments/views", got, err)
	}
}

func TestTwoPageEnvironmentTableGateCancellationAndOpeningScope(t *testing.T) {
	service, state, _ := twoPageEnvironmentTableFixture(t, false)
	if _, err := datasheet.NewTwoPageEnvironmentTableService(nil); err == nil {
		t.Fatal("nil context accepted")
	}
	for _, plot := range []string{"", "A", "108050 ", "\xff"} {
		if got, err := service.GetReview(context.Background(), state.ContextID, plot, 0, 1); err == nil || got != nil {
			t.Fatal("invalid/out-of-context opening parent returned review", plot, got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.Contexts.Projects.Sqlite.Mu.Lock()
	result := make(chan error, 1)
	go func() {
		got, err := service.GetReview(ctx, state.ContextID, "108050", 0, 1)
		if got != nil {
			result <- errors.New("cancelled request returned review")
		} else {
			result <- err
		}
	}()
	cancel()
	err := <-result
	service.Contexts.Projects.Sqlite.Mu.Unlock()
	if !errors.Is(err, context.Canceled) {
		t.Fatal("waiting request did not preserve cancellation", err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got == nil {
		t.Fatal("cancelled coordinator cannot retry", got, err)
	}
	if got, err := service.GetReview(nil, state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("nil context returned review", got, err)
	}
}
