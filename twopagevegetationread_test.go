package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func twoPageVegetationReadFixture(t *testing.T, external bool, affinity string) (*ContextService, ProjectState, *sql.DB) {
	t.Helper()
	service, state, db := twoPageSoilReadFixture(t, external, "TEXT")
	if _, err := db.Exec(`DROP TABLE Sample_Veg; CREATE TABLE Sample_Veg(
		ID, PlotNumber ` + affinity + `, Species TEXT, Cover1, Cover2, cover3, TotalA, Cover4, Cover5,
		TotalB, Cover5a, Cover5b, Cover5c, Cover6, Cover7, Cover8, Cover9, Collected TEXT,
		HeightB TEXT, Unmapped)`); err != nil {
		t.Fatal(err)
	}
	return service, state, db
}

func TestTwoPageVegetationOriginalPhysicalStorageAndMembership(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprint("external-", external), func(t *testing.T) {
			service, state, db := twoPageVegetationReadFixture(t, external, "TEXT")
			if _, err := db.Exec(`INSERT INTO Sample_Veg
				(rowid,ID,PlotNumber,Species,Cover5a,Cover6,Cover7,Collected,HeightB,Unmapped) VALUES
				(-7,NULL,'108050','',0,0,NULL,NULL,'',X'FF00'),
				(3,42,'108050','historical overlength',NULL,NULL,0,'',NULL,7),
				(4,42,'108050','duplicate ID',NULL,NULL,NULL,'C','2.5',NULL),
				(5,'01','108050',NULL,NULL,NULL,NULL,'V','raw text',NULL),
				(6,0,'other','outside scope',1,1,1,NULL,NULL,NULL);
				UPDATE Sample_Veg SET Cover1=X'00FF' WHERE rowid=5`); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			for _, test := range []struct {
				form, source string
				rows         []string
			}{
				{"SubVegAXL_BC", "USysVegA", []string{"-7", "5"}},
				{"SubVegAXL", "USysVegA", []string{"-7", "5"}},
				{"SubVegCXL", "USysVegC", []string{"-7"}},
				{"SubVegDXL", "USysVegD", []string{"3"}},
			} {
				got, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", test.form)
				if err != nil {
					t.Fatal(err)
				}
				if got.ContextID != state.ContextID || got.Project != "Sample" || got.Plot != "108050" ||
					got.Form != test.form || got.RecordSource != test.source || got.Table != "Sample_Veg" ||
					!reflect.DeepEqual(got.SourceRowIDs, test.rows) {
					t.Fatalf("source ownership/membership = %+v", got)
				}
				if len(got.Records.Columns) != 20 || len(got.Records.Rows) != 4 ||
					got.Records.Columns[5].Name != "cover3" || got.Records.Columns[18].DeclaredType != "TEXT" ||
					got.Records.Columns[19].Name != "Unmapped" {
					t.Fatalf("complete physical schema/rows = %+v", got.Records)
				}
				if got.Records.Rows[0].RowID != "-7" || got.Records.Rows[0].Cells[0].Storage != "null" ||
					!reflect.DeepEqual(got.Records.Rows[0].Cells[2], metadataText("")) ||
					!reflect.DeepEqual(got.Records.Rows[0].Cells[18], metadataText("")) ||
					*got.Records.Rows[0].Cells[19].BlobHex != "ff00" ||
					*got.Records.Rows[1].Cells[0].Integer != "42" || *got.Records.Rows[2].Cells[0].Integer != "42" ||
					*got.Records.Rows[3].Cells[0].Text != "01" ||
					!reflect.DeepEqual(got.Records.Rows[2].Cells[18], metadataText("2.5")) {
					t.Fatal("duplicate/NULL/text identities or unmapped/empty/text storage were repaired")
				}
			}
			if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
				t.Fatal("vegetation original read mutated an owned database")
			}
		})
	}
}

func TestTwoPageVegetationOriginalExhaustiveSourceMasks(t *testing.T) {
	service, state, db := twoPageVegetationReadFixture(t, false, "TEXT")
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for mask := 0; mask < 1024; mask++ {
		args := []any{mask + 1, "108050"}
		for bit := 0; bit < 10; bit++ {
			var value any
			if mask&(1<<bit) != 0 {
				value = 0
			}
			args = append(args, value)
		}
		if _, err := tx.Exec(`INSERT INTO Sample_Veg
			(rowid,PlotNumber,Cover1,Cover2,cover3,TotalA,Cover4,Cover5,TotalB,Cover5a,Cover5b,Cover5c)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, args...); err != nil {
			t.Fatal(err)
		}
	}
	for mask := 0; mask < 8; mask++ {
		args := []any{2000 + mask, "108050"}
		for bit := 0; bit < 3; bit++ {
			var value any
			if mask&(1<<bit) != 0 {
				value = 0
			}
			args = append(args, value)
		}
		if _, err := tx.Exec(`INSERT INTO Sample_Veg(rowid,PlotNumber,Cover7,Cover8,Cover9) VALUES(?,?,?,?,?)`, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
		got, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{}
		switch form {
		case "SubVegAXL_BC", "SubVegAXL":
			for row := 2; row <= 1024; row++ {
				want = append(want, strconv.Itoa(row))
			}
		case "SubVegDXL":
			for row := 2001; row <= 2007; row++ {
				want = append(want, strconv.Itoa(row))
			}
		}
		if len(got.Records.Rows) != 1032 || !reflect.DeepEqual(got.SourceRowIDs, want) {
			t.Fatalf("%s source mask membership differs: physical=%d source=%v", form, len(got.Records.Rows), got.SourceRowIDs)
		}
	}
}

func TestTwoPageVegetationOriginalLiteralAffinities(t *testing.T) {
	for _, affinity := range []string{"TEXT", "INTEGER", "NUMERIC", "REAL", "BLOB", ""} {
		t.Run(affinity, func(t *testing.T) {
			service, state, db := twoPageVegetationReadFixture(t, false, affinity)
			if _, err := db.Exec(`INSERT INTO Sample_Veg(rowid,PlotNumber,Cover6) VALUES
				(1,'108050',0),(2,108050,0),(3,CAST('108050' AS BLOB),0),
				(4,'0108050',0),(5,'108050 ',0),(6,NULL,0)`); err != nil {
				t.Fatal(err)
			}
			got, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", "SubVegCXL")
			if err != nil {
				t.Fatal(err)
			}
			want := []string{}
			switch affinity {
			case "TEXT":
				want = []string{"1", "2"}
			case "BLOB", "":
				want = []string{"1"}
			}
			if !reflect.DeepEqual(got.SourceRowIDs, want) || len(got.Records.Rows) != len(want) {
				t.Fatalf("literal scoped storage = %+v, want %v", got, want)
			}
		})
	}
}

func TestTwoPageVegetationOriginalScopeSchemaAndRetry(t *testing.T) {
	service, state, db := twoPageVegetationReadFixture(t, false, "TEXT")
	ctx := context.Background()
	for _, test := range []struct{ contextID, plot, form string }{
		{state.ContextID, "", "SubVegCXL"}, {state.ContextID, string([]byte{255}), "SubVegCXL"},
		{state.ContextID, "108050 ", "SubVegCXL"}, {state.ContextID, "absent", "SubVegCXL"},
		{state.ContextID + "-stale", "108050", "SubVegCXL"}, {state.ContextID, "108050", "SubVegCXL "},
	} {
		if got, err := service.readTwoPageVegetationOriginal(ctx, test.contextID, test.plot, test.form); err == nil || got != nil {
			t.Fatalf("invalid source ownership accepted: %+v %v", got, err)
		}
	}
	var absent *ContextService
	if got, err := absent.readTwoPageVegetationOriginal(ctx, state.ContextID, "108050", "SubVegCXL"); err == nil || got != nil {
		t.Fatal("nil owner accepted")
	}
	if got, err := service.readTwoPageVegetationOriginal(nil, state.ContextID, "108050", "SubVegCXL"); err == nil || got != nil {
		t.Fatal("nil context accepted")
	}
	if _, err := service.projects.sqlite.conn.ExecContext(ctx, `CREATE TEMP TABLE Sample_Veg(FalseShadow)`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readTwoPageVegetationOriginal(ctx, state.ContextID, "108050", "SubVegCXL"); err != nil || len(got.Records.Columns) != 20 {
		t.Fatalf("TEMP shadow changed original physical reader: %+v %v", got, err)
	}
	for _, test := range []struct{ breakSQL, fixSQL, diagnostic string }{
		{`ALTER TABLE Sample_Veg RENAME COLUMN Cover6 TO MissingCover`, `ALTER TABLE Sample_Veg RENAME COLUMN MissingCover TO Cover6`, "physical cover column"},
		{`ALTER TABLE Sample_Veg RENAME COLUMN PlotNumber TO WrongParent`, `ALTER TABLE Sample_Veg RENAME COLUMN WrongParent TO PlotNumber`, "physical parent column"},
		{`ALTER TABLE Sample_Veg RENAME TO Saved_Veg; CREATE VIEW Sample_Veg AS SELECT * FROM Saved_Veg`, `DROP VIEW Sample_Veg; ALTER TABLE Saved_Veg RENAME TO Sample_Veg`, "original physical project table"},
		{`INSERT INTO Sample_Veg(rowid,PlotNumber,Unmapped) VALUES(1,'108050',CAST(X'FF' AS TEXT))`, `UPDATE Sample_Veg SET Unmapped='' WHERE rowid=1`, "malformed Unicode"},
	} {
		if _, err := db.Exec(test.breakSQL); err != nil {
			t.Fatal(err)
		}
		if got, err := service.readTwoPageVegetationOriginal(ctx, state.ContextID, "108050", "SubVegCXL"); err == nil || got != nil || !strings.Contains(err.Error(), test.diagnostic) {
			t.Fatalf("source refusal lost %q: %+v %v", test.diagnostic, got, err)
		}
		if _, err := db.Exec(test.fixSQL); err != nil {
			t.Fatal(err)
		}
		if got, err := service.readTwoPageVegetationOriginal(ctx, state.ContextID, "108050", "SubVegCXL"); err != nil || got == nil {
			t.Fatalf("corrected source did not retry: %+v %v", got, err)
		}
	}
}

func TestTwoPageVegetationOriginalPhysicalIdentityRefusals(t *testing.T) {
	for _, column := range []string{`rowid TEXT`, `_rowid_ INTEGER`, `oid TEXT`, `Generated TEXT GENERATED ALWAYS AS ('source drift') VIRTUAL`} {
		t.Run(column, func(t *testing.T) {
			service, state, db := twoPageVegetationReadFixture(t, false, "TEXT")
			if _, err := db.Exec(`ALTER TABLE Sample_Veg ADD COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
			if got, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", "SubVegCXL"); err == nil || got != nil || !strings.Contains(err.Error(), "ordinary columns") {
				t.Fatalf("physical identity drift accepted: %+v %v", got, err)
			}
		})
	}
}

func TestTwoPageVegetationOriginalCancellationAndRetry(t *testing.T) {
	service, state, _ := twoPageVegetationReadFixture(t, false, "TEXT")
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	original, err := service.readTwoPageVegetationOriginal(ctx, state.ContextID, "108050", "SubVegCXL")
	owner.mu.Unlock()
	if original != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued owned read did not cancel: %+v %v", original, err)
	}
	if got, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", "SubVegCXL"); err != nil || got == nil || len(got.SourceRowIDs) != 0 {
		t.Fatalf("cancelled read lost the owned coordinator: %+v %v", got, err)
	}
}

func TestTwoPageVegetationOriginalParentRefusals(t *testing.T) {
	for _, parent := range []struct{ table, column string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		for _, drift := range []string{"duplicate", "missing-column", "view"} {
			t.Run(parent.table+"/"+drift, func(t *testing.T) {
				service, state, db := twoPageVegetationReadFixture(t, false, "TEXT")
				table := quoteHeaderIdentifier("Sample_" + parent.table)
				var query, diagnostic string
				switch drift {
				case "duplicate":
					query = `CREATE TABLE Duplicate_Parent AS SELECT * FROM ` + table +
						`; INSERT INTO Duplicate_Parent SELECT * FROM ` + table + ` WHERE ` + quoteHeaderIdentifier(parent.column) + `='108050';
						ALTER TABLE ` + table + ` RENAME TO Archived_Parent; ALTER TABLE Duplicate_Parent RENAME TO ` + table
					diagnostic = "one literal physical " + parent.table + " parent"
				case "missing-column":
					query = `DROP TABLE ` + table + `; CREATE TABLE ` + table + `(WrongParent TEXT)`
					diagnostic = "missing required physical parent column"
				case "view":
					query = `ALTER TABLE ` + table + ` RENAME TO Archived_Parent; CREATE VIEW ` + table + ` AS SELECT * FROM Archived_Parent`
					diagnostic = "original physical project table"
				}
				if _, err := db.Exec(query); err != nil {
					t.Fatal(err)
				}
				for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
					if got, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form); err == nil || got != nil || !strings.Contains(err.Error(), diagnostic) {
						t.Fatalf("%s parent drift accepted: %+v %v", form, got, err)
					}
				}
			})
		}
	}
}

func TestTwoPageVegetationOriginalAfterActualContextSwitch(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprint("external-", external), func(t *testing.T) {
			service, state, db := twoPageSoilReadFixture(t, external, "TEXT")
			if _, err := db.Exec(`INSERT INTO Sample_Veg(ID,PlotNumber,Species,Cover5c,Cover6,Cover9) VALUES(1299001,'108050','TEST',0,0,0)`); err != nil {
				t.Fatal(err)
			}
			originals := make(map[string]*twoPageVegetationOriginal)
			for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
				original, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form)
				if err != nil || len(original.Records.Rows) == 0 || len(original.SourceRowIDs) == 0 {
					t.Fatalf("pre-switch physical %s original unavailable: %+v %v", form, original, err)
				}
				originals[form] = original
			}
			selection := service.projects.sqlite.selection
			next, err := service.SwitchContext(state.ContextID, ContextSelection{Project: selection.Project, ProjectPath: selection.ProjectPath,
				SU: selection.SU, SUPath: selection.SUPath, Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath})
			if err != nil || next.ContextID == state.ContextID {
				t.Fatalf("actual context did not rotate: %+v %v", next, err)
			}
			for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
				if got, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form); err == nil || got != nil {
					t.Fatalf("stale context inherited %s read authority: %+v %v", form, got, err)
				}
				got, err := service.readTwoPageVegetationOriginal(context.Background(), next.ContextID, "108050", form)
				if err != nil || got.ContextID != next.ContextID || !reflect.DeepEqual(got.Records, originals[form].Records) || !reflect.DeepEqual(got.SourceRowIDs, originals[form].SourceRowIDs) {
					t.Fatalf("new context changed exact physical %s storage: %+v %v", form, got, err)
				}
			}
		})
	}
}
