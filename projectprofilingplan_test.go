package main

import (
	"context"
	"reflect"
	"testing"
)

func TestSourceProfileCombinedLumpUsesOriginalUnionAndNormalizedBoolean(t *testing.T) {
	conn := profileSQLFixture(t)
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, `ATTACH ':memory:' AS vlists;
		CREATE TABLE vlists.USysAllSpecs(Code TEXT);
		INSERT INTO vlists.USysAllSpecs VALUES('ABCD0'),('ABCD1'),('ABCD8'),('ABCD9'),('EFGH0'),('IJKL1');
		DELETE FROM Lumps;
		INSERT INTO Lumps VALUES('CUSTOM','ABCD1',1),('CUSTOM','ABCD8',-1),('IGNORED','EFGH0',0),
		('NULLUSE','IJKL1',NULL),('CUSTOM','ABCD1',1)`); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	name, err := createProfileCombinedLump(ctx, tx, "Lumps")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT LumpCode,SppCode FROM `+name+` ORDER BY SppCode,LumpCode`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := [][2]string{}
	for rows.Next() {
		var pair [2]string
		if err := rows.Scan(&pair[0], &pair[1]); err != nil {
			t.Fatal(err)
		}
		got = append(got, pair)
	}
	want := [][2]string{{"ABCD", "ABCD0"}, {"CUSTOM", "ABCD1"}, {"CUSTOM", "ABCD8"}, {"EFGH", "EFGH0"}, {"IJKL", "IJKL1"}}
	if err := rows.Err(); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("source explicit Use override, last-character bound, NULL or UNION DISTINCT changed", got, err)
	}
}

func TestSourceProfileStepCountsDescribeUniqueSetChanges(t *testing.T) {
	conn := profileSQLFixture(t)
	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE VProProfileRunPlots(PlotNumber TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		operation string
		matches   []string
		count     int
		remaining int
	}{
		{"Add plots", []string{"P", "Z"}, 2, 2},
		{"Add plots", []string{"P", "M"}, 1, 3},
		{"Common plots", []string{"M", "N"}, 1, 1},
		{"Subtract plots", []string{"P"}, 0, 1},
		{"Subtract plots", []string{"M"}, 1, 0},
		{"Add plots", nil, 0, 0},
	} {
		if err := applyProfileStep(ctx, tx, plotProfileRule{operation: step.operation}, step.matches); err != nil {
			t.Fatal(err)
		}
		var count, remaining int
		if err := tx.QueryRowContext(ctx, `SELECT changes(),(SELECT COUNT(*) FROM VProProfileRunPlots)`).Scan(&count, &remaining); err != nil {
			t.Fatal(err)
		}
		if step.operation == "Common plots" {
			count = remaining
		}
		if count != step.count || remaining != step.remaining {
			t.Fatal("run-only source set counts changed", step, count, remaining)
		}
	}
}
