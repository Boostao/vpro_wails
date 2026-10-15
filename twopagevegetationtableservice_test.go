package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func twoPageVegetationTableFixture(t *testing.T, external bool) (*TwoPageVegetationTableService, ProjectState, *sql.DB) {
	t.Helper()
	contexts, state, db := twoPageSoilReadFixture(t, external, "TEXT")
	originalColumns, err := readSQLiteStorageColumns(context.Background(), db, "main", "Sample_Veg")
	if err != nil {
		t.Fatal(err)
	}
	definitions := make([]string, 0, len(originalColumns)+1)
	for _, column := range originalColumns {
		definitions = append(definitions, quoteHeaderIdentifier(column.Name))
	}
	definitions = append(definitions, "Unmapped")
	_, err = db.Exec(`DROP TABLE Sample_Veg;
		CREATE TABLE Sample_Veg(` + strings.Join(definitions, ",") + `);
		INSERT INTO Sample_Veg(rowid,PlotNumber,Species,Cover1,Cover2,Cover8,ID,Flag,Unmapped) VALUES
		(-2,'108050','allnull',NULL,NULL,NULL,NULL,-1,X'FF00'),
		(1,'A','base',2,2,1,7,0,NULL),
		(2,'a','higher',3,NULL,NULL,7,0,NULL),
		(3,'A','lower',2,1,NULL,NULL,0,NULL),
		(4,'A','extra',2,2,8,'01',0,NULL),
		(5,'B','peer',NULL,NULL,NULL,0,0,NULL),
		(6,'X','orphan',NULL,NULL,NULL,NULL,0,NULL),
		(7,'Cafe','Unicode café',NULL,NULL,NULL,NULL,0,NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, state.SUPath, `DELETE FROM Report_SU;
		INSERT INTO Report_SU(PlotNumber) VALUES('108050'),('A'),('a'),('B'),(NULL),('absent')`)
	service, err := NewTwoPageVegetationTableService(contexts, func(name string) (string, bool) {
		return "true", name == twoPageVegetationTableEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db
}

func TestTwoPageVegetationTableSourceProjectionAndPagedOriginals(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, _ := twoPageVegetationTableFixture(t, external)
			check := assertPlotLocationFacadeReadOnly(t, service.contexts)
			defer check()
			for _, test := range []struct {
				form     string
				ids      []string
				captions []string
			}{
				{"FS882-8x6XL", []string{"-2", "2", "2", "4", "4", "1", "1", "3", "3", "5"},
					[]string{"PlotNumber", "Species", "A1", "A2", "A3", "Total A", "B1", "B2", "CHARS shrubs a",
						"CHARS shrubs b", "CHARS shrubs c", "Total B", "C", "D", "Dr/Dw", "Ep", "Cover10"}},
				{"FS882-8x6XL-CHARS", []string{"-2", "2", "2", "1", "1", "4", "4", "3", "3", "5"},
					[]string{"PlotNumber", "Species", "A1", "A2", "A3", "Total A", "B1", "B2", "CHARS shrubs a",
						"CHARS shrubs b", "CHARS shrubs c", "Total B", "C", "D"}},
			} {
				got, err := service.GetReview(context.Background(), state.ContextID, "108050", test.form, 0, 200)
				if err != nil {
					t.Fatal(err)
				}
				var ids, captions []string
				for _, column := range got.Columns {
					captions = append(captions, column.Caption)
				}
				for _, row := range got.Rows {
					ids = append(ids, row.VegRowID)
					if row.SURowID == nil || len(row.Cells) != len(test.captions) {
						t.Fatal("selected physical witness/projected shape lost", row)
					}
				}
				if !reflect.DeepEqual(ids, test.ids) || !reflect.DeepEqual(captions, test.captions) ||
					got.TotalRows != 10 || got.ContextID != state.ContextID || got.ProjectPath != state.ProjectPath ||
					got.SUPath != state.SUPath || got.Table != "Sample_Veg" || got.SUTable != "Report_SU" ||
					got.GeneralLocaleVerified || got.OrderingPolicy != twoPageVegetationTableOrdering ||
					got.SelectedOriginal == nil || len(got.Original.Columns) < 20 {
					t.Fatal("source scope/variant/explicit ordering policy changed", got, ids, captions)
				}
				original := got.Original.Rows[0]
				index := map[string]int{}
				for i, column := range got.Original.Columns {
					index[column.Name] = i
				}
				if original.RowID != "-2" || len(original.Cells) != len(got.Original.Columns) ||
					original.Cells[index["ID"]].Storage != "null" ||
					original.Cells[index["Flag"]].Integer == nil || *original.Cells[index["Flag"]].Integer != "-1" ||
					original.Cells[index["Unmapped"]].BlobHex == nil || *original.Cells[index["Unmapped"]].BlobHex != "ff00" {
					t.Fatal("complete source NULL ID/BOOLEAN/blob repaired", original)
				}
				page, err := service.GetReview(context.Background(), state.ContextID, "108050", test.form, 2, 2)
				if err != nil || page == nil || len(page.Rows) != 2 || page.TotalRows != 10 ||
					page.Rows[0].VegRowID != test.ids[2] || page.Rows[1].VegRowID != test.ids[3] ||
					len(page.Original.Rows) != 2 || len(page.SelectedOriginal.Rows) != 2 {
					t.Fatal("slice inside duplicate expansion lost independent witnesses", page, err)
				}
			}
		})
	}
}

func TestTwoPageVegetationTableNoneScopeAndVariantOmissions(t *testing.T) {
	service, state, db := twoPageVegetationTableFixture(t, true)
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 200); err == nil || got != nil {
		t.Fatal("retired context returned table", got, err)
	}
	got, err := service.GetReview(context.Background(), next.ContextID, "108050", "FS882-8x6XL", 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, row := range got.Rows {
		ids = append(ids, row.VegRowID)
		if row.SURowID != nil {
			t.Fatal("None scope invented SU witness")
		}
	}
	if !reflect.DeepEqual(ids, []string{"-2", "2", "4", "1", "3", "5", "7", "6"}) ||
		got.TotalRows != 8 || got.SelectedOriginal != nil || got.SUTable != "" {
		t.Fatal("all-project non-current/orphan/null-cover rows lost", got, ids)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover8='historical unsupported',Cover9=X'FF' WHERE rowid=1`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), next.ContextID, "108050", "FS882-8x6XL-CHARS", 0, 200); err != nil || got == nil {
		t.Fatal("CHARS inherited omitted normal fields", got, err)
	}
	if got, err := service.GetReview(context.Background(), next.ContextID, "108050", "FS882-8x6XL", 0, 200); err == nil || got != nil {
		t.Fatal("normal coerced unsupported numeric storage", got, err)
	}
}

func TestTwoPageVegetationTableBoundsAndDuplicatePages(t *testing.T) {
	service, state, db := twoPageVegetationTableFixture(t, false)
	mutateContextFixture(t, state.SUPath, `DELETE FROM Report_SU;
		WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<500)
		INSERT INTO Report_SU(PlotNumber) SELECT '108050' FROM n`)
	for _, test := range []struct{ offset, limit, rows int }{{0, 200, 200}, {199, 200, 200}, {499, 200, 1}, {500, 200, 0}} {
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", test.offset, test.limit)
		if err != nil || got == nil || len(got.Rows) != test.rows || got.TotalRows != 500 || len(got.Original.Rows) > 1 ||
			len(got.SelectedOriginal.Rows) != test.rows {
			t.Fatal("bounded duplicate page/witness shape changed", test, got, err)
		}
	}
	for _, test := range []struct{ offset, limit int }{{-1, 1}, {0, 0}, {0, 201}, {501, 200}} {
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", test.offset, test.limit); err == nil || got != nil {
			t.Fatal("invalid page returned data", test, got, err)
		}
	}
	if _, err := db.Exec(`DELETE FROM Sample_Veg`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 1); err != nil ||
		got == nil || got.TotalRows != 0 || len(got.Rows) != 0 || len(got.Columns) != 17 {
		t.Fatal("legitimate empty source lost schema", got, err)
	}
	if _, err := db.Exec(`WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<100001)
		INSERT INTO Sample_Veg(PlotNumber) SELECT '108050' FROM n`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 1); err == nil ||
		got != nil || !strings.Contains(err.Error(), "100000-row") {
		t.Fatal("max+1 physical source returned partial review", got, err)
	}
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE rowid=(SELECT MAX(rowid) FROM Sample_Veg)`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 1); err != nil ||
		got == nil || got.TotalRows != 50000000 || len(got.Rows) != 1 || len(got.Original.Rows) != 1 ||
		len(got.SelectedOriginal.Rows) != 1 {
		t.Fatal("exact100000 input/fifty-million virtual join bound failed", got, err)
	}
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE rowid<>(SELECT MIN(rowid) FROM Sample_Veg)`); err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, state.SUPath, `DELETE FROM Report_SU;
		WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<100000)
		INSERT INTO Report_SU(PlotNumber) SELECT '108050' FROM n`)
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 200); err != nil ||
		got == nil || got.TotalRows != 100000 || len(got.Rows) != 200 || len(got.Original.Rows) != 1 ||
		len(got.SelectedOriginal.Rows) != 200 {
		t.Fatal("exact selected-source/page maximum failed", got, err)
	}
	mutateContextFixture(t, state.SUPath, `INSERT INTO Report_SU(PlotNumber) VALUES('108050')`)
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 1); err == nil ||
		got != nil || !strings.Contains(err.Error(), "100000-row") {
		t.Fatal("max+1 selected source returned partial review", got, err)
	}
}

func TestTwoPageVegetationTableInvalidSourceAndRetry(t *testing.T) {
	for _, test := range []struct{ name, sql, repair string }{
		{"numeric-plot", `UPDATE Sample_Veg SET PlotNumber=7 WHERE rowid=1`, `UPDATE Sample_Veg SET PlotNumber='A' WHERE rowid=1`},
		{"malformed-key", `UPDATE Sample_Veg SET PlotNumber=CAST(X'FF' AS TEXT) WHERE rowid=1`, `UPDATE Sample_Veg SET PlotNumber='A' WHERE rowid=1`},
		{"malformed-species", `UPDATE Sample_Veg SET Species=CAST(X'FF' AS TEXT) WHERE rowid=1`, `UPDATE Sample_Veg SET Species='base' WHERE rowid=1`},
		{"numeric-cover-text", `UPDATE Sample_Veg SET Cover1='2' WHERE rowid=1`, `UPDATE Sample_Veg SET Cover1=2 WHERE rowid=1`},
		{"missing-column", `ALTER TABLE Sample_Veg RENAME COLUMN Cover10 TO Missing`, `ALTER TABLE Sample_Veg RENAME COLUMN Missing TO Cover10`},
		{"literal-parent", `UPDATE Sample_Admin SET Plot='0108050' WHERE Plot='108050'`, `UPDATE Sample_Admin SET Plot='108050' WHERE Plot='0108050'`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, state, db := twoPageVegetationTableFixture(t, false)
			if test.name == "missing-column" {
				if _, err := db.Exec(`DROP VIEW IF EXISTS qryHerbariumReportData`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(test.sql); err != nil {
				t.Fatal(err)
			}
			check := assertPlotLocationFacadeReadOnly(t, service.contexts)
			if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 200); err == nil || got != nil {
				t.Fatal("invalid source returned repaired/partial data", got, err)
			}
			check()
			if _, err := db.Exec(test.repair); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 200); err != nil || got == nil {
				t.Fatal("same owned context retry unavailable", got, err)
			}
		})
	}
}

func TestTwoPageVegetationTableGatesAndRequestGuards(t *testing.T) {
	service, state, _ := twoPageVegetationTableFixture(t, false)
	if got, err := NewTwoPageVegetationTableService(nil, func(string) (string, bool) { return "true", true }); err == nil || got != nil {
		t.Fatal("unowned constructor succeeded", got, err)
	}
	if got, err := NewTwoPageVegetationTableService(service.contexts, nil); err == nil || got != nil {
		t.Fatal("missing explicit gate lookup succeeded", got, err)
	}
	for _, value := range []string{"", "false", "true", "TRUE", "1"} {
		got, err := NewTwoPageVegetationTableService(service.contexts, func(string) (string, bool) { return value, value != "" })
		if value == "TRUE" || value == "1" {
			if err == nil {
				t.Fatal("malformed gate accepted", value)
			}
			continue
		}
		if err != nil || got.review != (value == "true") {
			t.Fatal("independent gate default changed", value, got, err)
		}
	}
	for _, unavailable := range []*TwoPageVegetationTableService{nil, {}, {contexts: service.contexts}} {
		if got, err := unavailable.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 1); err == nil || got != nil {
			t.Fatal("disabled service returned source", got, err)
		}
	}
	for _, test := range []struct{ plot, form string }{{"", "FS882-8x6XL"}, {"\xff", "FS882-8x6XL"}, {"108050", "FS882-6x4XL"}} {
		if got, err := service.GetReview(context.Background(), state.ContextID, test.plot, test.form, 0, 1); err == nil || got != nil {
			t.Fatal("unsupported literal request returned source", got, err)
		}
		if got, err := service.GetReview(nil, state.ContextID, "108050", "FS882-8x6XL", 0, 1); err == nil || got != nil {
			t.Fatal("nil request context returned source", got, err)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if got, err := service.GetReview(cancelled, state.ContextID, "108050", "FS882-8x6XL", 0, 1); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("pre-cancelled request returned source", got, err)
		}
	}
}

func TestTwoPageVegetationTableInFlightCancellationAndOwnedRetry(t *testing.T) {
	service, state, _ := twoPageVegetationTableFixture(t, false)
	owner := service.contexts.projects.sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("vegetation table cancellation requires actual SQLite")
		}
		return conn.RegisterFunc("sourceTableReadBarrier", func(value any) any {
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
		CREATE TEMP VIEW USysEnv AS SELECT sourceTableReadBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := service.GetReview(ctx, state.ContextID, "108050", "FS882-8x6XL", 0, 200)
		if got != nil {
			err = errors.Join(err, errors.New("cancelled table read returned data"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("owned table read did not reach actual SQLite")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("owned table cancellation did not settle")
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;`+
		strings.Replace(original, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", 0, 200); err != nil || got == nil {
		t.Fatal("cancelled coordinator lost attachments/views or owned retry", got, err)
	}
}
