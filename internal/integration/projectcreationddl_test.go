package integration

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNewProjectDDLExactEightTemplateMappingAndDetachedEvidence(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	review, err := service.ReadNewProjectTemplates(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	plan, err := vprocore.PlanNewProjectDDL(context.Background(), "Prepared", review)
	if err != nil || plan == nil || len(plan.Tables) != 8 || plan.ContextID != state.ContextID ||
		plan.ProjectPath != state.ProjectPath || plan.TemplatePath != review.TemplatePath {
		t.Fatal("exact template mapping unavailable", err, plan)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("ATTACH DATABASE ? AS original", sqlutil.SqliteFileURI(review.TemplatePath, "ro")); err != nil {
		t.Fatal(err)
	}
	for i, table := range plan.Tables {
		template := review.Templates[i]
		if table.SourceTable != template.SourceTable || table.TargetTable != "Prepared_"+template.Suffix ||
			table.Schema[0].Source.Type != "table" {
			t.Fatal("source table order or create-before-index order differs", table)
		}
		for _, mapping := range table.Schema {
			if mapping.SQL != nil {
				if _, err := db.Exec(*mapping.SQL); err != nil {
					t.Fatal("mapped retained SQLite schema cannot prepare in disposable memory", err)
				}
			}
		}
		sourceColumns := ddlQueryRows(t, db, `SELECT cid,name,type,"notnull",dflt_value,pk FROM pragma_table_info(?,?) ORDER BY cid`,
			table.SourceTable, "original")
		targetColumns := ddlQueryRows(t, db, `SELECT cid,name,type,"notnull",dflt_value,pk FROM pragma_table_info(?,?) ORDER BY cid`,
			table.TargetTable, "main")
		if !reflect.DeepEqual(sourceColumns, targetColumns) {
			t.Fatal("column/default/NOT NULL/primary key changed", table.SourceTable)
		}
		indexSQL := `SELECT i."unique",i.origin,i.partial,x.seqno,x.cid,x.name,x."desc",x.coll,x."key"
			FROM pragma_index_list(?,?) i JOIN pragma_index_xinfo(i.name,?) x
			ORDER BY i."unique",i.origin,i.partial,x.name,x.seqno`
		if !reflect.DeepEqual(ddlQueryRows(t, db, indexSQL, table.SourceTable, "original", "original"),
			ddlQueryRows(t, db, indexSQL, table.TargetTable, "main", "main")) {
			t.Fatal("index uniqueness/key/collation properties changed", table.SourceTable)
		}
		var count int
		if err := db.QueryRow("SELECT count(*) FROM " + vprocore.QuoteHeaderIdentifier(table.TargetTable)).Scan(&count); err != nil || count != 0 {
			t.Fatal("mapped schema copied data", err, count)
		}
	}
	original, err := vprocore.PlanNewProjectDDL(context.Background(), "Prepared", review)
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range review.Templates[0].Schema {
		if object.SQL != nil {
			*object.SQL = "caller mutation"
		}
	}
	if !reflect.DeepEqual(original, plan) {
		t.Fatal("plan aliases caller-owned original SQL")
	}
	*plan.Tables[0].Schema[0].SQL = "caller mutation"
	if *original.Tables[0].Schema[0].SQL == "caller mutation" ||
		*plan.Tables[0].Schema[0].Source.SQL == "caller mutation" {
		t.Fatal("mapped/original SQL share mutable pointers")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func ddlQueryRows(t *testing.T, db *sql.DB, query string, args ...any) [][]any {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	columns, err := rows.Columns()
	if err != nil {
		rows.Close()
		t.Fatal(err)
	}
	result := [][]any{}
	for rows.Next() {
		values, pointers := make([]any, len(columns)), make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		result = append(result, values)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestNewProjectDDLRewritesOnlyObservedHeaderNotBodyLiterals(t *testing.T) {
	table := `CREATE TABLE "USysEnvTable" ("Value" TEXT DEFAULT 'USysEnvTable', "USysEnvTable" TEXT) -- "USysEnvTable"`
	mapping, err := vprocore.MapNewProjectSchemaObject(vprocore.SqliteSchemaObject{Type: "table", Name: "USysEnvTable", Table: "USysEnvTable", SQL: &table},
		"USysEnvTable", "MixedCase_Env")
	if err != nil || *mapping.SQL != strings.Replace(table, `CREATE TABLE "USysEnvTable"`, `CREATE TABLE "MixedCase_Env"`, 1) {
		t.Fatal("table body/literal/comments were rewritten", err)
	}
	index := `CREATE UNIQUE INDEX "Quoted""Index" ON "USysEnvTable" ("USysEnvTable" COLLATE BINARY DESC) WHERE "Value"='USysEnvTable'`
	mapping, err = vprocore.MapNewProjectSchemaObject(vprocore.SqliteSchemaObject{Type: "index", Name: `Quoted"Index`, Table: "USysEnvTable", SQL: &index},
		"USysEnvTable", "MixedCase_Env")
	if err != nil || !strings.HasPrefix(*mapping.SQL, `CREATE UNIQUE INDEX "MixedCase_Env__Quoted""Index" ON "MixedCase_Env"`) ||
		!strings.HasSuffix(*mapping.SQL, `("USysEnvTable" COLLATE BINARY DESC) WHERE "Value"='USysEnvTable'`) {
		t.Fatal("index quoting/body/literal changed", err)
	}
	implicit, err := vprocore.MapNewProjectSchemaObject(vprocore.SqliteSchemaObject{Type: "index", Name: "sqlite_autoindex_USysEnvTable_1", Table: "USysEnvTable", SQL: nil},
		"USysEnvTable", "Prepared_Env")
	if err != nil || implicit.SQL != nil || implicit.TargetName != "" || implicit.Source.SQL != nil {
		t.Fatal("implicit index acquired inferred executable SQL", err)
	}
}

func TestNewProjectDDLRejectsIncompleteForeignUnsupportedAndCancelledPlans(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	mutations := []func(*vprocore.NewProjectTemplateReview){
		func(v *vprocore.NewProjectTemplateReview) { v.ContextID = "" },
		func(v *vprocore.NewProjectTemplateReview) { v.ProjectPath = "" },
		func(v *vprocore.NewProjectTemplateReview) { v.TemplatePath = "" },
		func(v *vprocore.NewProjectTemplateReview) { v.Templates = v.Templates[:7] },
		func(v *vprocore.NewProjectTemplateReview) { v.Templates[7].Suffix = "Wrong" },
		func(v *vprocore.NewProjectTemplateReview) { v.Templates[7].Original.Rows = nil },
		func(v *vprocore.NewProjectTemplateReview) {
			v.Templates[7].Original.Columns = v.Templates[7].Original.Columns[:19]
		},
		func(v *vprocore.NewProjectTemplateReview) { v.Templates[7].Schema = v.Templates[7].Schema[:1] },
		func(v *vprocore.NewProjectTemplateReview) {
			v.Templates[7].Schema = append(v.Templates[7].Schema, v.Templates[7].Schema[0])
		},
		func(v *vprocore.NewProjectTemplateReview) { v.Templates[7].Schema[0].Table = "foreign" },
		func(v *vprocore.NewProjectTemplateReview) { v.Templates[7].Schema[0].SQL = nil },
		func(v *vprocore.NewProjectTemplateReview) { v.Templates[7].Schema[0].Type = "trigger" },
		func(v *vprocore.NewProjectTemplateReview) {
			bad := "CREATE INDEX wrong ON wrong(Value)"
			v.Templates[7].Schema[0].SQL = &bad
		},
	}
	for i, mutate := range mutations {
		review, err := service.ReadNewProjectTemplates(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		mutate(review)
		if got, err := vprocore.PlanNewProjectDDL(context.Background(), "Prepared", review); err == nil || got != nil {
			t.Fatal("incomplete/foreign/unsupported input returned partial plan", i, err, got)
		}
	}
	review, err := service.ReadNewProjectTemplates(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", " leading", "Trailing ", "Bad-Name", "1Name", strings.Repeat("A", 32), "Bad\x00Name"} {
		if got, err := vprocore.PlanNewProjectDDL(context.Background(), name, review); err == nil || got != nil {
			t.Fatal("literal name policy bypassed", name, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := vprocore.PlanNewProjectDDL(ctx, "Prepared", review); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled plan accepted", err)
	}
}
