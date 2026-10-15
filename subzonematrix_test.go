package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/reports/subzonematrix"
	"github.com/mattn/go-sqlite3"
)

func subzoneMatrixFixture(t *testing.T) (*ContextService, ProjectState) {
	t.Helper()
	service, state := contextServiceFixture(t)
	mutateContextFixture(t, state.HierarchyPath, `
		DELETE FROM Sample_Hierarchy;
		INSERT INTO Sample_Hierarchy(ID,Name,Parent,Level,Tag,Flag) VALUES
		(1,'Association Alpha',NULL,10,X'00FF',-1),(2,'Association Beta',NULL,10,NULL,0),
		(11,'AAAAAAAAA101',1,11,NULL,0),(12,'BBBBBBBBB102',1,11,NULL,0),(13,'AAAAAAAAA103',2,11,NULL,0);`)
	mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], `
		DELETE FROM USysSubzoneMatrixLabels;
		INSERT INTO USysSubzoneMatrixLabels("BGC unit",Len,MyUnit) VALUES
		(NULL,1.125,'AAAAAAAAA1'),('',NULL,'BBBBBBBBB1'),(NULL,NULL,'ZZZZZZZZZ1');`)
	return service, state
}

func TestSubzoneMatrixOwnedOriginalsAndSourceProjection(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	expected := [][]*string{
		{nil, nil, nil},
		{metadataText("01").Text, metadataText("02").Text, nil},
		{metadataText("03").Text, nil, nil},
	}
	if result.ContextID != state.ContextID || result.Hierarchy != state.ActiveHierarchy ||
		result.HierarchyPath != state.HierarchyPath || result.LabelsPath != service.projects.sqlite.attachments["VPro64"] ||
		!reflect.DeepEqual(result.Matrix.Columns, []string{"AAAAAAAAA", "BBBBBBBBB", "ZZZZZZZZZ"}) ||
		len(result.Matrix.Rows) != 3 || len(result.HierarchyOriginal.Rows) != 5 || len(result.LabelsOriginal.Rows) != 3 {
		t.Fatal("owned source provenance/complete originals differ", result)
	}
	for i, row := range result.Matrix.Rows {
		if !reflect.DeepEqual(row.Values, expected[i]) {
			t.Fatal("source matrix values differ", row, expected[i])
		}
	}
	indices, err := siteUnitTransferColumns(result.HierarchyOriginal, "Tag", "Flag")
	if err != nil {
		t.Fatal(err)
	}
	if *result.HierarchyOriginal.Rows[0].Cells[indices["Tag"]].BlobHex != "00ff" ||
		*result.HierarchyOriginal.Rows[0].Cells[indices["Flag"]].Integer != "-1" ||
		result.LabelsOriginal.Rows[0].Cells[0].Storage != "null" ||
		*result.LabelsOriginal.Rows[1].Cells[0].Text != "" ||
		*result.LabelsOriginal.Rows[0].Cells[1].Real != 1.125 {
		t.Fatal("unused complete original storage was repaired", result)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("readonly matrix changed the family")
	}
}

func TestSubzoneMatrixAllControlledNativeCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("internal", "reports", "subzonematrix", "testdata", "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name      string
			Hierarchy []subzonematrix.HierarchyRow
			Labels    []*string
			Columns   []string
			Rows      [][]*string
			Refusal   string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 11 {
		t.Fatal("missing actual source cases")
	}
	service, state := subzoneMatrixFixture(t)
	literal := func(value *string) string {
		if value == nil {
			return "NULL"
		}
		return "'" + strings.ReplaceAll(*value, "'", "''") + "'"
	}
	number := func(value *int64) string {
		if value == nil {
			return "NULL"
		}
		return fmt.Sprint(*value)
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			sql := "DELETE FROM Sample_Hierarchy;"
			for _, row := range test.Hierarchy {
				sql += fmt.Sprintf("INSERT INTO Sample_Hierarchy(ID,Name,Parent,Level) VALUES(%d,%s,%s,%s);",
					row.ID, literal(row.Name), number(row.Parent), number(row.Level))
			}
			mutateContextFixture(t, state.HierarchyPath, sql)
			sql = "DELETE FROM USysSubzoneMatrixLabels;"
			for _, label := range test.Labels {
				sql += "INSERT INTO USysSubzoneMatrixLabels(MyUnit) VALUES(" + literal(label) + ");"
			}
			mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], sql)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
			if test.Refusal != "" {
				if err == nil || !strings.Contains(err.Error(), test.Refusal) || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
					t.Fatal("source ambiguity produced a successful/partial review", result, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				rows := [][]*string{}
				for _, row := range result.Matrix.Rows {
					rows = append(rows, append([]*string{row.Name}, row.Values...))
				}
				if !reflect.DeepEqual(append([]string{"Name"}, result.Matrix.Columns...), test.Columns) ||
					!reflect.DeepEqual(rows, test.Rows) {
					t.Fatal("actual owned reader differs from native expected matrix", result.Matrix, test.Rows)
				}
			}
			if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
				t.Fatal("source read/refusal wrote family data")
			}
		})
	}
}

func TestSubzoneMatrixReadFailuresCorrectionAndRetry(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	for _, test := range []struct{ mutation, correction, want string }{
		{`UPDATE Sample_Hierarchy SET Parent=2147483648 WHERE ID=11`, `UPDATE Sample_Hierarchy SET Parent=1 WHERE ID=11`, "source-width"},
		{`UPDATE Sample_Hierarchy SET Level=32768 WHERE ID=11`, `UPDATE Sample_Hierarchy SET Level=11 WHERE ID=11`, "source-width"},
		{`UPDATE Sample_Hierarchy SET Level=11.5 WHERE ID=11`, `UPDATE Sample_Hierarchy SET Level=11 WHERE ID=11`, "INTEGER"},
		{`UPDATE Sample_Hierarchy SET Name=CAST(X'FF' AS TEXT) WHERE ID=11`, `UPDATE Sample_Hierarchy SET Name='AAAAAAAAA101' WHERE ID=11`, "malformed Unicode"},
		{`UPDATE Sample_Hierarchy SET Tag=CAST(X'FF' AS TEXT) WHERE ID=11`, `UPDATE Sample_Hierarchy SET Tag=NULL WHERE ID=11`, "malformed Unicode"},
		{`UPDATE Sample_Hierarchy SET Name=X'4141' WHERE ID=11`, `UPDATE Sample_Hierarchy SET Name='AAAAAAAAA101' WHERE ID=11`, "TEXT/NULL"},
	} {
		mutateContextFixture(t, state.HierarchyPath, test.mutation)
		result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
		if err == nil || !strings.Contains(err.Error(), test.want) || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
			t.Fatal("invalid historical storage repaired or partially returned", result, err)
		}
		mutateContextFixture(t, state.HierarchyPath, test.correction)
		if _, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err != nil {
			t.Fatal("correction could not retry", err)
		}
	}
	path := service.projects.sqlite.attachments["VPro64"]
	for _, test := range []struct{ mutation, correction, want string }{
		{`ALTER TABLE USysSubzoneMatrixLabels RENAME COLUMN MyUnit TO myunit`,
			`ALTER TABLE USysSubzoneMatrixLabels RENAME COLUMN myunit TO MyUnit`, "MyUnit"},
		{`UPDATE USysSubzoneMatrixLabels SET MyUnit=X'4141' WHERE rowid=1`,
			`UPDATE USysSubzoneMatrixLabels SET MyUnit='AAAAAAAAA1' WHERE rowid=1`, "TEXT/NULL"},
		{`ALTER TABLE USysSubzoneMatrixLabels RENAME TO SavedLabels; CREATE VIEW USysSubzoneMatrixLabels AS SELECT * FROM SavedLabels`,
			`DROP VIEW USysSubzoneMatrixLabels; ALTER TABLE SavedLabels RENAME TO USysSubzoneMatrixLabels`, "physical table"},
	} {
		mutateContextFixture(t, path, test.mutation)
		result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
		if err == nil || !strings.Contains(err.Error(), test.want) || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
			t.Fatal("missing/invalid source was substituted", result, err)
		}
		mutateContextFixture(t, path, test.correction)
		if _, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err != nil {
			t.Fatal("source correction could not retry", err)
		}
	}
}

func TestSubzoneMatrixOwnershipAndNoneRefusal(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	for _, id := range []string{"", "foreign", state.ContextID + " "} {
		if result, err := service.readSubzoneMatrix(context.Background(), id); err == nil || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
			t.Fatal("unowned request returned saved data", result, err)
		}
	}
	if result, err := service.readSubzoneMatrix(nil, state.ContextID); err == nil || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("nil request context accepted")
	}
	var absent *ContextService
	if _, err := absent.readSubzoneMatrix(context.Background(), state.ContextID); err == nil {
		t.Fatal("absent owned service accepted")
	}
	current, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err == nil {
		t.Fatal("retired context still owns hierarchy")
	}
	selection := contextSelection(current)
	selection.Hierarchy, selection.HierarchyPath = "None", ""
	current, err = service.SwitchContext(current.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.readSubzoneMatrix(context.Background(), current.ContextID); err == nil ||
		!strings.Contains(err.Error(), "selected physical hierarchy") || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("None silently used project/master hierarchy", result, err)
	}
}

func TestSubzoneMatrixPhysicalRowAndOriginalCellBounds(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	path := service.projects.sqlite.attachments["VPro64"]
	mutateContextFixture(t, path, `DELETE FROM USysSubzoneMatrixLabels;
		WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<100000)
		INSERT INTO USysSubzoneMatrixLabels(MyUnit) SELECT NULL FROM n;`)
	result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
	if err != nil || len(result.LabelsOriginal.Rows) != 100000 || len(result.Matrix.Rows) != 1 ||
		!reflect.DeepEqual(result.Matrix.Columns, []string{"<>"}) {
		t.Fatal("exact physical label bound unavailable", err)
	}
	mutateContextFixture(t, path, `INSERT INTO USysSubzoneMatrixLabels(MyUnit) VALUES(NULL)`)
	if result, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err == nil ||
		!strings.Contains(err.Error(), "physical row") || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("max+1 labels returned partial data", err)
	}
	mutateContextFixture(t, path, `DELETE FROM USysSubzoneMatrixLabels`)
	ddl := `DROP TABLE Sample_Hierarchy; CREATE TABLE Sample_Hierarchy(ID INTEGER,Name TEXT,Parent INTEGER,Level INTEGER`
	for i := range 16 {
		ddl += fmt.Sprintf(",Extra%d BLOB", i)
	}
	ddl += `); WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<100000)
		INSERT INTO Sample_Hierarchy(ID) SELECT x FROM n;`
	mutateContextFixture(t, state.HierarchyPath, ddl)
	result, err = service.readSubzoneMatrix(context.Background(), state.ContextID)
	if err != nil || len(result.HierarchyOriginal.Rows) != 100000 || len(result.HierarchyOriginal.Columns) != 20 ||
		len(result.Matrix.Rows) != 0 {
		t.Fatal("exact 2000000 original-cell bound unavailable", err)
	}
	mutateContextFixture(t, state.HierarchyPath, `ALTER TABLE Sample_Hierarchy ADD COLUMN Overbound BLOB`)
	if result, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err == nil ||
		!strings.Contains(err.Error(), "cell bounds") || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("overbound original cells returned a partial original", err)
	}
}

func TestSubzoneMatrixCompleteOriginalByteBound(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], `DELETE FROM USysSubzoneMatrixLabels`)
	mutateContextFixture(t, state.HierarchyPath, `DROP TABLE Sample_Hierarchy;
		CREATE TABLE Sample_Hierarchy(ID INTEGER,Name TEXT,Parent INTEGER,Level INTEGER,Payload BLOB);
		INSERT INTO Sample_Hierarchy(ID,Payload) VALUES(1,zeroblob(67108863));`)
	result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
	if err != nil || len(result.HierarchyOriginal.Rows) != 1 ||
		len(*result.HierarchyOriginal.Rows[0].Cells[4].BlobHex) != 2*(subzonematrix.MaxInputBytes-1) {
		t.Fatal("exact 64MiB complete-original bound unavailable", err)
	}
	mutateContextFixture(t, state.HierarchyPath, `UPDATE Sample_Hierarchy SET Payload=zeroblob(67108864)`)
	if result, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err == nil ||
		!strings.Contains(err.Error(), "64MiB") || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("unused overbound BLOB bypassed original byte guard", err)
	}
}

func TestSubzoneMatrixOriginalColumnBoundsAndScalarWidths(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], `DELETE FROM USysSubzoneMatrixLabels`)
	ddl := `DROP TABLE Sample_Hierarchy; CREATE TABLE Sample_Hierarchy(ID INTEGER,Name TEXT,Parent INTEGER,Level INTEGER`
	for i := range 60 {
		ddl += fmt.Sprintf(",Extra%d BLOB", i)
	}
	mutateContextFixture(t, state.HierarchyPath, ddl+`); INSERT INTO Sample_Hierarchy(ID) VALUES(1)`)
	result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
	if err != nil || len(result.HierarchyOriginal.Columns) != 64 || len(result.HierarchyOriginal.Rows) != 1 {
		t.Fatal("exact physical column bound unavailable", err)
	}
	mutateContextFixture(t, state.HierarchyPath, `ALTER TABLE Sample_Hierarchy ADD COLUMN Overbound BLOB`)
	if result, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err == nil ||
		!strings.Contains(err.Error(), "1..64") || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("overbound columns produced partial data", err)
	}
	hierarchy := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{"ID", "INTEGER"}, {"Name", "TEXT"}, {"Parent", "INTEGER"}, {"Level", "INTEGER"}},
		Rows: []ProjectMetadataRow{{"1", []ProjectMetadataCell{
			metadataInteger("-2147483648"), metadataText("literal"), metadataInteger("2147483647"), metadataInteger("-32768"),
		}}},
	}
	labels := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{"MyUnit", "TEXT"}},
		Rows: []ProjectMetadataRow{{"1", []ProjectMetadataCell{{Storage: "null"}}}}}
	rows, units, err := subzoneMatrixInputs(context.Background(), hierarchy, labels)
	if err != nil || rows[0].ID != -2147483648 || *rows[0].Parent != 2147483647 || *rows[0].Level != -32768 ||
		len(units) != 1 || units[0] != nil {
		t.Fatal("source signed scalar boundaries/NULL differ", rows, units, err)
	}
	for _, test := range []struct {
		column int
		value  ProjectMetadataCell
	}{
		{0, metadataInteger("2147483648")},
		{0, ProjectMetadataCell{Storage: "null"}},
		{0, metadataInteger("01")},
		{1, metadataInteger("1")},
		{2, metadataInteger("-2147483649")},
		{3, metadataInteger("32768")},
	} {
		original := hierarchy.Rows[0].Cells[test.column]
		hierarchy.Rows[0].Cells[test.column] = test.value
		rows, units, err := subzoneMatrixInputs(context.Background(), hierarchy, labels)
		hierarchy.Rows[0].Cells[test.column] = original
		if err == nil || rows != nil || units != nil {
			t.Fatal("storage/width violation returned partial typed input", rows, units, err)
		}
	}
}

func TestSubzoneMatrixGeneratedAndShadowedSchemaRefusal(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	for _, column := range []string{
		`GeneratedPayload BLOB GENERATED ALWAYS AS (zeroblob(67108865)) STORED`,
		`GeneratedPayload BLOB GENERATED ALWAYS AS (zeroblob(67108865)) VIRTUAL`,
		`rowid INTEGER`, `_rowid_ INTEGER`, `oid INTEGER`,
	} {
		mutateContextFixture(t, state.HierarchyPath, `DROP TABLE Sample_Hierarchy;
			CREATE TABLE Sample_Hierarchy(ID INTEGER,Name TEXT,Parent INTEGER,Level INTEGER,`+column+`);
			INSERT INTO Sample_Hierarchy(ID,Name,Level) VALUES(1,'Parent',10);`)
		result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
		if err == nil || !strings.Contains(err.Error(), "ordinary columns") || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
			t.Fatal("generated/hidden/shadowed schema bypassed complete-original guards", column, result, err)
		}
	}
	mutateContextFixture(t, state.HierarchyPath, `DROP TABLE Sample_Hierarchy;
		CREATE TABLE Sample_Hierarchy(ID INTEGER,Name TEXT,Parent INTEGER,Level INTEGER);
		INSERT INTO Sample_Hierarchy(ID,Name,Level) VALUES(1,'Parent',10);`)
	if _, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err != nil {
		t.Fatal("corrected ordinary schema could not retry", err)
	}
	path := service.projects.sqlite.attachments["VPro64"]
	mutateContextFixture(t, path, `DROP TABLE USysSubzoneMatrixLabels;
		CREATE TABLE USysSubzoneMatrixLabels(MyUnit TEXT,GeneratedPayload BLOB GENERATED ALWAYS AS (zeroblob(67108865)) STORED);
		INSERT INTO USysSubzoneMatrixLabels(MyUnit) VALUES('AAAAAAAAA1');`)
	if result, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err == nil ||
		!strings.Contains(err.Error(), "ordinary columns") || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("generated label payload bypassed all-column guards", result, err)
	}
}

func TestSubzoneMatrixInFlightCancellationAndPinnedRetry(t *testing.T) {
	service, state := subzoneMatrixFixture(t)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	observed := false
	setAuthorizer := func(authorize func(int, string, string, string) int) {
		t.Helper()
		if err := owner.conn.Raw(func(raw any) error {
			driver, valid := raw.(*sqlite3.SQLiteConn)
			if !valid {
				return errors.New("Subzone Matrix requires actual SQLite cancellation witness")
			}
			driver.RegisterAuthorizer(authorize)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	setAuthorizer(func(action int, table, column, database string) int {
		if !observed && action == sqlite3.SQLITE_READ && table == "Sample_Hierarchy" && database == "hierarchy" {
			observed = true
			close(entered)
			<-release
		}
		return sqlite3.SQLITE_OK
	})
	defer setAuthorizer(nil)
	go func() {
		defer close(finished)
		select {
		case <-entered:
		case <-time.After(10 * time.Second):
		}
		cancel()
		close(release)
	}()
	result, err := service.readSubzoneMatrix(ctx, state.ContextID)
	<-finished
	select {
	case <-entered:
	default:
		t.Fatal("actual hierarchy query was not reached")
	}
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, subzoneMatrixReview{}) {
		t.Fatal("cancelled query returned successful/partial data", result, err)
	}
	setAuthorizer(nil)
	if _, err := service.readSubzoneMatrix(context.Background(), state.ContextID); err != nil {
		t.Fatal("cancellation lost pinned attachments/TEMP views", err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("cancellation/retry changed saved family")
	}
}
