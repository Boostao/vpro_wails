package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type profilePresenceNative struct {
	Scope         []string            `json:"scope"`
	SelectedRules [][]*string         `json:"selectedRules"`
	VegMatches    map[string][]string `json:"vegMatches"`
	LumpMatches   map[string][]string `json:"lumpMatches"`
	Physical      map[string]struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	} `json:"physical"`
}

func profilePresenceFixture(t *testing.T) (*sql.Conn, profilePresenceNative) {
	t.Helper()
	raw, err := os.ReadFile("testdata/profiling-presence-native.json")
	if err != nil {
		t.Fatal(err)
	}
	var native profilePresenceNative
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	if len(native.VegMatches) != 23 || len(native.LumpMatches) != 9 || len(native.Scope) != 23 || len(native.Physical) != 6 {
		t.Fatal("incomplete independently reopened source presence fixture")
	}
	conn := profileSQLFixture(t)
	ctx := context.Background()
	_, err = conn.ExecContext(ctx, `DROP TABLE USysEnv; DROP TABLE USysVeg;
		CREATE TABLE Sample_Env(PlotNumber TEXT);
		CREATE TABLE Sample_Admin(Plot TEXT);
		CREATE TABLE Sample_Veg(PlotNumber TEXT,Species TEXT,Cover1 REAL,Cover2 REAL,Cover3 REAL,
			Cover4 REAL,Cover5 REAL,Cover6 REAL,Cover7 REAL,TotalA REAL,TotalB REAL,
			Cover5a REAL,Cover5b REAL,Cover5c REAL,Cover8 REAL,Cover9 REAL,Cover10 REAL);
		CREATE TABLE Sample_Lump(LumpCode TEXT,SppCode TEXT,"Use" BOOLEAN);
		CREATE TABLE Sample_Profile("Order" SMALLINT,"Table" TEXT,Species TEXT);
		CREATE TABLE USysPlotProfilePlots(PlotNumber TEXT);
		CREATE VIEW USysEnv AS SELECT e.PlotNumber FROM Sample_Env e JOIN Sample_Admin a
			ON CAST(e.PlotNumber AS BLOB)=CAST(a.Plot AS BLOB);
		CREATE VIEW USysVeg AS SELECT * FROM Sample_Veg;`)
	if err != nil {
		t.Fatal(err)
	}
	for name, table := range native.Physical {
		columns := make([]string, len(table.Columns))
		parameters := make([]string, len(table.Columns))
		for index, column := range table.Columns {
			columns[index] = quoteHeaderIdentifier(column)
			parameters[index] = "?"
		}
		for _, row := range table.Rows {
			values := append([]any{}, row...)
			for index, value := range values {
				if boolean, ok := value.(bool); ok {
					values[index] = 0
					if boolean {
						values[index] = -1
					}
				}
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO `+quoteHeaderIdentifier(name)+
				`(`+strings.Join(columns, ",")+`) VALUES(`+strings.Join(parameters, ",")+`)`, values...); err != nil {
				t.Fatal(name, err)
			}
		}
	}
	return conn, native
}

func profilePresenceRules(values ...[2]string) ProjectMetadataTable {
	result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Table"}, {Name: "Species"}}}
	for index, value := range values {
		table, species := value[0], value[1]
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: strconv.Itoa(index + 1),
			Cells: []ProjectMetadataCell{{Storage: "text", Text: &table}, {Storage: "text", Text: &species}}})
	}
	return result
}

func profilePresenceSnapshot(t *testing.T, conn *sql.Conn) *sql.Tx {
	t.Helper()
	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Error(err)
		}
	})
	return tx
}

func TestProfileSpeciesPresenceNativeMatchingSQL(t *testing.T) {
	conn, native := profilePresenceFixture(t)
	before := map[string]ProjectMetadataTable{}
	for name := range native.Physical {
		original, err := readSQLiteStorageRows(context.Background(), conn, "main", name, "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		before[name] = original
	}
	if _, err := conn.ExecContext(context.Background(), `PRAGMA query_only=ON`); err != nil {
		t.Fatal(err)
	}
	tx := profilePresenceSnapshot(t, conn)
	for table, cases := range map[string]map[string][]string{"Veg": native.VegMatches, "Lump": native.LumpMatches} {
		for species, expected := range cases {
			t.Run(table+"/"+species, func(t *testing.T) {
				plan, err := compilePlotProfilePresence(profilePresenceRules([2]string{table, species}))
				if err != nil {
					t.Fatal(err)
				}
				result, err := executePlotProfilePresence(context.Background(), tx, plan, "Sample_Lump")
				if err != nil || result.totalPlots != len(native.Scope) || !reflect.DeepEqual(result.plotNumbers, expected) {
					t.Fatal("original USysTVeg matching SQL differs", result, expected, err)
				}
			})
		}
	}
	var rules ProjectMetadataTable
	rules.Columns = []ProjectMetadataColumn{{Name: "Table"}, {Name: "Species"}}
	rules.Rows = []ProjectMetadataRow{}
	for index, row := range native.Physical["Sample_Profile"].Rows {
		cells := []ProjectMetadataCell{{Storage: "null"}, {Storage: "null"}}
		for column := range cells {
			if value, ok := row[column+1].(string); ok {
				cells[column] = ProjectMetadataCell{Storage: "text", Text: &value}
			}
		}
		rules.Rows = append(rules.Rows, ProjectMetadataRow{RowID: strconv.Itoa(index + 1), Cells: cells})
	}
	plan, err := compilePlotProfilePresence(rules)
	if err != nil || !plan.seed || len(plan.selectors) != 2 {
		t.Fatal("original selectedRules/unknown-with-NULL branch differs", plan, native.SelectedRules, err)
	}
	result, err := executePlotProfilePresence(context.Background(), tx, plan, "Sample_Lump")
	if err != nil || !reflect.DeepEqual(result.plotNumbers, []string{"MIX", "P00", "P01", "P02", "P03", "P04", "P05", "P06", "P07", "P08"}) {
		t.Fatal("Veg/Lump intersection differs", result, err)
	}
	var counts []int
	for _, name := range []string{"Sample_Env", "Sample_Admin", "Sample_Veg", "Sample_Lump", "Sample_Profile", "USysPlotProfilePlots"} {
		var count int
		if err := tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(name)).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, count)
		if count != len(native.Physical[name].Rows) {
			t.Fatal("readonly matching changed stored data", name, counts)
		}
		current, err := readSQLiteStorageRows(context.Background(), tx, "main", name, "", nil, "")
		if err != nil || !reflect.DeepEqual(current, before[name]) {
			t.Fatal("readonly matching changed complete typed schema/row identities/cells", name, err)
		}
	}
}

func TestProfileSpeciesPresenceIgnoresInactiveRuleFields(t *testing.T) {
	conn, native := profilePresenceFixture(t)
	tx := profilePresenceSnapshot(t, conn)
	rules := numericProfileCriterionTable("Veg", "0e309", ">")
	for index, column := range rules.Columns {
		if column.Name != "Table" && column.Name != "Species" {
			rules.Rows[0].Cells[index] = ProjectMetadataCell{Storage: "null"}
		}
	}
	if _, err := compilePlotProfileRules(rules, nil); err == nil {
		t.Fatal("ordered numeric runner unexpectedly accepted inactive-field witness")
	}
	plan, err := compilePlotProfilePresence(rules)
	if err != nil {
		t.Fatal("species-only reused numeric/order/layer/operation validation", err)
	}
	result, err := executePlotProfilePresence(context.Background(), tx, plan, "")
	if err != nil || !reflect.DeepEqual(result.plotNumbers, native.VegMatches["TARGET"]) {
		t.Fatal(result, err)
	}
	for _, table := range []string{"Env", "env", "Env "} {
		rules := profilePresenceRules([2]string{table, ""})
		rules.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "null"}
		plan, err := compilePlotProfilePresence(rules)
		if err != nil || plan.seed {
			t.Fatal("original SQL Env exclusion changed", table, plan, err)
		}
		result, err := executePlotProfilePresence(context.Background(), tx, plan, "")
		if err != nil || result.totalPlots != 23 || len(result.plotNumbers) != 0 {
			t.Fatal("Env-only source early-exit changed", result, err)
		}
	}
	for _, rules := range []ProjectMetadataTable{
		{Columns: []ProjectMetadataColumn{{Name: "Table"}, {Name: "Species"}}},
		{Columns: []ProjectMetadataColumn{{Name: "Table"}, {Name: "Species"}},
			Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{{Storage: "null"}, {Storage: "null"}}}}},
	} {
		plan, err := compilePlotProfilePresence(rules)
		if err != nil || plan.seed {
			t.Fatal("zero selected-rule result changed", plan, err)
		}
	}
	unknown := profilePresenceRules([2]string{"unknown", ""})
	unknown.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "null"}
	plan, err = compilePlotProfilePresence(unknown)
	if err != nil {
		t.Fatal(err)
	}
	result, err = executePlotProfilePresence(context.Background(), tx, plan, "")
	if err != nil || !reflect.DeepEqual(result.plotNumbers, native.Scope) {
		t.Fatal("selected unmatched Table must retain seeded scope, not read Species", result, err)
	}
	plan, err = compilePlotProfilePresence(profilePresenceRules([2]string{"Veg", "OTHER"}, [2]string{"Veg", "TARGET"}))
	if err != nil {
		t.Fatal(err)
	}
	result, err = executePlotProfilePresence(context.Background(), tx, plan, "")
	if err != nil || !reflect.DeepEqual(result.plotNumbers, []string{"MIX"}) {
		t.Fatal("multiple species require intersection, not union", result, err)
	}
}

func TestProfileSpeciesPresenceRejectsUnqualifiedRuleDomains(t *testing.T) {
	for name, edit := range map[string]func(*ProjectMetadataTable){
		"missing":          func(table *ProjectMetadataTable) { table.Columns[1].Name = "Other" },
		"duplicate-column": func(table *ProjectMetadataTable) { table.Columns[1].Name = "Table" },
		"duplicate-row":    func(table *ProjectMetadataTable) { table.Rows = append(table.Rows, table.Rows[0]) },
		"identity":         func(table *ProjectMetadataTable) { table.Rows[0].RowID = "01" },
		"shape":            func(table *ProjectMetadataTable) { table.Rows[0].Cells = table.Rows[0].Cells[:1] },
		"table-storage":    func(table *ProjectMetadataTable) { table.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "integer"} },
		"recased-table":    func(table *ProjectMetadataTable) { *table.Rows[0].Cells[0].Text = "veg" },
		"spaced-table":     func(table *ProjectMetadataTable) { *table.Rows[0].Cells[0].Text = "Lump " },
		"unicode-table":    func(table *ProjectMetadataTable) { *table.Rows[0].Cells[0].Text = "Vég" },
		"long-table":       func(table *ProjectMetadataTable) { *table.Rows[0].Cells[0].Text = strings.Repeat("x", 256) },
		"null-species":     func(table *ProjectMetadataTable) { table.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "null"} },
		"nil-species":      func(table *ProjectMetadataTable) { table.Rows[0].Cells[1].Text = nil },
		"sql-species":      func(table *ProjectMetadataTable) { *table.Rows[0].Cells[1].Text = "x' OR 1=1" },
		"unicode-species":  func(table *ProjectMetadataTable) { *table.Rows[0].Cells[1].Text = "TÁRGET" },
		"nul-species":      func(table *ProjectMetadataTable) { *table.Rows[0].Cells[1].Text = "T\x00ARGET" },
		"long-species":     func(table *ProjectMetadataTable) { *table.Rows[0].Cells[1].Text = strings.Repeat("x", 256) },
	} {
		t.Run(name, func(t *testing.T) {
			table := profilePresenceRules([2]string{"Veg", "TARGET"})
			edit(&table)
			plan, err := compilePlotProfilePresence(table)
			if err == nil || !reflect.DeepEqual(plan, plotProfilePresencePlan{}) {
				t.Fatal("unqualified input returned partial/success-shaped plan", plan, err)
			}
		})
	}
}

func TestProfileSpeciesPresenceComparisonPreservesOriginalText(t *testing.T) {
	conn, native := profilePresenceFixture(t)
	tx := profilePresenceSnapshot(t, conn)
	if _, err := tx.ExecContext(context.Background(), `UPDATE Sample_Veg SET Species='TARGET ' WHERE Species='TARGET';
		UPDATE Sample_Lump SET SppCode='TARGET  ',LumpCode='LT ' WHERE LumpCode='LT'`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"Veg", "Lump"} {
		species := "TARGET  "
		expected := native.VegMatches["TARGET "]
		if table == "Lump" {
			species = "lt  "
			expected = native.LumpMatches["LT"]
		}
		rules := profilePresenceRules([2]string{table, species})
		before, err := json.Marshal(rules)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := compilePlotProfilePresence(rules)
		if err != nil {
			t.Fatal(err)
		}
		result, err := executePlotProfilePresence(context.Background(), tx, plan, "Sample_Lump")
		if err != nil || !reflect.DeepEqual(result.plotNumbers, expected) || plan.selectors[0].species != species {
			t.Fatal("comparison-time terminal spaces changed literal plan or matches", result, plan, expected, err)
		}
		after, err := json.Marshal(rules)
		if err != nil || string(before) != string(after) {
			t.Fatal("comparison repaired reviewed original data", err)
		}
	}
	var stored string
	if err := tx.QueryRowContext(context.Background(), `SELECT Species FROM Sample_Veg WHERE PlotNumber='MIX' AND Species LIKE 'TARGET%' LIMIT 1`).Scan(&stored); err != nil || stored != "TARGET " {
		t.Fatal("comparison changed stored species text", stored, err)
	}
}

func TestProfileSpeciesPresenceCancellationOwnershipAndRetry(t *testing.T) {
	for _, failure := range []string{"cancel", "missing-lump", "bad-boolean", "case-child", "spaced-child", "numeric-child", "case-scope", "spaced-scope", "unicode-scope"} {
		t.Run(failure, func(t *testing.T) {
			conn, native := profilePresenceFixture(t)
			tx := profilePresenceSnapshot(t, conn)
			rules := profilePresenceRules([2]string{"Veg", "TARGET"})
			lump := "Sample_Lump"
			ctx := context.Background()
			correction := ""
			switch failure {
			case "cancel":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			case "missing-lump":
				rules = profilePresenceRules([2]string{"Lump", "LT"})
				lump = ""
			case "bad-boolean":
				rules = profilePresenceRules([2]string{"Lump", "LT"})
				if _, err := tx.ExecContext(ctx, `UPDATE Sample_Lump SET "Use"=2 WHERE LumpCode='LT'`); err != nil {
					t.Fatal(err)
				}
				correction = `UPDATE Sample_Lump SET "Use"=-1 WHERE LumpCode='LT'`
			case "case-child":
				if _, err := tx.ExecContext(ctx, `UPDATE Sample_Veg SET PlotNumber='mix' WHERE PlotNumber='MIX'`); err != nil {
					t.Fatal(err)
				}
				correction = `UPDATE Sample_Veg SET PlotNumber='MIX' WHERE PlotNumber='mix'`
			case "spaced-child":
				if _, err := tx.ExecContext(ctx, `UPDATE Sample_Veg SET PlotNumber='MIX ' WHERE PlotNumber='MIX'`); err != nil {
					t.Fatal(err)
				}
				correction = `UPDATE Sample_Veg SET PlotNumber='MIX' WHERE PlotNumber='MIX '`
			case "numeric-child":
				columns := []string{}
				for index, name := range native.Physical["Sample_Veg"].Columns {
					kind := ""
					if index == 1 {
						kind = " TEXT"
					} else if index > 1 {
						kind = " REAL"
					}
					columns = append(columns, quoteHeaderIdentifier(name)+kind)
				}
				if _, err := tx.ExecContext(ctx, `DROP VIEW USysVeg; ALTER TABLE Sample_Veg RENAME TO OriginalVeg;
					CREATE TABLE Sample_Veg(`+strings.Join(columns, ",")+`);
					INSERT INTO Sample_Veg SELECT * FROM OriginalVeg; UPDATE Sample_Veg SET PlotNumber=7 WHERE PlotNumber='MIX';
					CREATE VIEW USysVeg AS SELECT * FROM Sample_Veg`); err != nil {
					t.Fatal(err)
				}
				var storage string
				if err := tx.QueryRowContext(ctx, `SELECT typeof(PlotNumber) FROM Sample_Veg WHERE typeof(PlotNumber)='integer' LIMIT 1`).Scan(&storage); err != nil || storage != "integer" {
					t.Fatal("numeric-child test did not establish actual numeric storage", storage, err)
				}
				correction = `UPDATE Sample_Veg SET PlotNumber='MIX' WHERE typeof(PlotNumber)='integer'`
			case "case-scope":
				if _, err := tx.ExecContext(ctx, `INSERT INTO Sample_Env VALUES('mix'); INSERT INTO Sample_Admin VALUES('mix')`); err != nil {
					t.Fatal(err)
				}
				correction = `DELETE FROM Sample_Env WHERE PlotNumber='mix'; DELETE FROM Sample_Admin WHERE Plot='mix'`
			case "spaced-scope":
				if _, err := tx.ExecContext(ctx, `INSERT INTO Sample_Env VALUES('MIX '); INSERT INTO Sample_Admin VALUES('MIX ')`); err != nil {
					t.Fatal(err)
				}
				correction = `DELETE FROM Sample_Env WHERE PlotNumber='MIX '; DELETE FROM Sample_Admin WHERE Plot='MIX '`
			case "unicode-scope":
				if _, err := tx.ExecContext(ctx, `INSERT INTO Sample_Env VALUES('é'); INSERT INTO Sample_Admin VALUES('é')`); err != nil {
					t.Fatal(err)
				}
				correction = `DELETE FROM Sample_Env WHERE PlotNumber='é'; DELETE FROM Sample_Admin WHERE Plot='é'`
			}
			plan, err := compilePlotProfilePresence(rules)
			if err != nil {
				t.Fatal(err)
			}
			result, err := executePlotProfilePresence(ctx, tx, plan, lump)
			if err == nil || !reflect.DeepEqual(result, plotProfilePresenceResult{}) || failure == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("failed request returned partial/success-shaped results", result, err)
			}
			if correction != "" {
				if _, err := tx.ExecContext(context.Background(), correction); err != nil {
					t.Fatal(err)
				}
			}
			result, err = executePlotProfilePresence(context.Background(), tx, plan, "Sample_Lump")
			expected := native.VegMatches["TARGET"]
			if failure == "missing-lump" || failure == "bad-boolean" {
				expected = native.LumpMatches["LT"]
			}
			if err != nil || !reflect.DeepEqual(result.plotNumbers, expected) {
				t.Fatal("explicit correction/retry failed or cancellation discarded caller snapshot", result, expected, err)
			}
		})
	}
	if result, err := executePlotProfilePresence(context.Background(), nil, plotProfilePresencePlan{}, ""); err == nil || !reflect.DeepEqual(result, plotProfilePresenceResult{}) {
		t.Fatal("missing snapshot not surfaced", result, err)
	}
}
