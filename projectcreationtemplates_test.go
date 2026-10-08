package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewProjectTemplatesOwnedOriginalEmptySchemaAndIndependence(t *testing.T) {
	service, state := contextServiceFixture(t)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
	if err != nil || review == nil {
		t.Fatal("original owned templates unavailable", err)
	}
	if review.ContextID != state.ContextID || review.ProjectPath != state.ProjectPath ||
		review.TemplatePath != owner.attachments["VPro64"] || review.DescriptionMetadataPresent ||
		len(review.Templates) != 8 {
		t.Fatal("template provenance/scope changed", review)
	}
	db, err := openReadOnly(review.TemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for index, source := range newProjectTemplateSources {
		got := review.Templates[index]
		if got.Suffix != source.suffix || got.SourceTable != source.table || len(got.Original.Columns) != source.columns ||
			len(got.Original.Rows) != 0 || got.Original.Rows == nil ||
			got.Descriptions.Columns == nil || got.Descriptions.Rows == nil ||
			len(got.Descriptions.Columns) != 0 || len(got.Descriptions.Rows) != 0 {
			t.Fatal("source template order/empty/description shape differs", source, got)
		}
		rows, err := db.Query(`SELECT type,name,tbl_name,sql FROM sqlite_master
			WHERE tbl_name COLLATE BINARY=? ORDER BY type COLLATE BINARY,name COLLATE BINARY`, source.table)
		if err != nil {
			t.Fatal(err)
		}
		expected := []sqliteSchemaObject{}
		for rows.Next() {
			var object sqliteSchemaObject
			if err := rows.Scan(&object.Type, &object.Name, &object.Table, &object.SQL); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			expected = append(expected, object)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Schema, expected) {
			t.Fatal("full table/index SQL, NULL implicit index definitions or constraints changed", source.table)
		}
	}
	original, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	review.Templates[0].Original.Columns[0].Name = "caller"
	for _, object := range review.Templates[0].Schema {
		if object.SQL != nil {
			*object.SQL = "caller"
		}
	}
	again, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(again, original) {
		t.Fatal("templates share caller-owned slices/SQL", err)
	}
	assertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(after, config) || service.projects.contextID != state.ContextID {
		t.Fatal("template review changed preferences/context", err)
	}
}

func TestNewProjectTemplatesPresentEmptyAndNullableDuplicateDescriptions(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(map[bool]string{false: "present-empty", true: "physical-duplicates"}[populated], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			path := service.projects.sqlite.attachments["VPro64"]
			statements := `CREATE TABLE _table_metadata(table_name TEXT,description TEXT,extra BLOB)`
			if populated {
				statements += `; INSERT INTO _table_metadata(rowid,table_name,description,extra) VALUES
					(-9007199254740993,'USysEnvTable',NULL,X'00'),
					(2,'USysEnvTable','',X''),
					(3,'USysEnvTable','  duplicate  ',NULL),
					(4,'USysEnvTable','  duplicate  ',NULL),
					(5,'unrelated','excluded',NULL)`
			}
			mutateContextFixture(t, path, statements)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			review, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
			if err != nil || review == nil || !review.DescriptionMetadataPresent {
				t.Fatal("present descriptions lost", err)
			}
			table := review.Templates[0].Descriptions
			if len(table.Columns) != 3 || table.Rows == nil {
				t.Fatal("metadata original shape lost", table)
			}
			if !populated {
				if len(table.Rows) != 0 {
					t.Fatal("empty descriptions inferred", table)
				}
			} else {
				if len(table.Rows) != 4 || table.Rows[0].RowID != "-9007199254740993" ||
					table.Rows[0].Cells[1].Storage != "null" || table.Rows[1].Cells[1].Storage != "text" ||
					table.Rows[1].Cells[1].Text == nil || *table.Rows[1].Cells[1].Text != "" ||
					*table.Rows[2].Cells[1].Text != "  duplicate  " || *table.Rows[3].Cells[1].Text != "  duplicate  " ||
					table.Rows[0].Cells[2].BlobHex == nil || *table.Rows[0].Cells[2].BlobHex != "00" ||
					table.Rows[1].Cells[2].BlobHex == nil || *table.Rows[1].Cells[2].BlobHex != "" {
					t.Fatal("NULL/empty/duplicate/extra metadata normalized", table)
				}
				*table.Rows[2].Cells[1].Text = "caller"
				again, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
				if err != nil || *again.Templates[0].Descriptions.Rows[2].Cells[1].Text != "  duplicate  " {
					t.Fatal("physical descriptions share caller ownership", err)
				}
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}

func TestNewProjectTemplatesUnavailableDefinitionsReturnNoPartialReview(t *testing.T) {
	for _, test := range []struct{ name, query, message string }{
		{"missing late template", `DROP TABLE USysAdminTable`, "table/index set"},
		{"late view impostor", `DROP TABLE USysAdminTable; CREATE VIEW USysAdminTable AS SELECT * FROM USysOtherTable`, "physical table"},
		{"late populated template", `INSERT INTO USysAdminTable(Plot) VALUES('SHOULD_NOT_COPY')`, "be empty"},
		{"late extra column", `ALTER TABLE USysAdminTable ADD COLUMN Unexpected TEXT`, "20 columns"},
		{"malformed metadata", `CREATE TABLE _table_metadata(table_name TEXT,wrong TEXT)`, "descriptions unavailable"},
		{"metadata view", `CREATE VIEW _table_metadata AS SELECT 'USysEnvTable' AS table_name,NULL AS description`, "not a view"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, state := contextServiceFixture(t)
			mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], test.query)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			got, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
			if got != nil || err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatal("unavailable source yielded partial/inferred review", got, err)
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}

func TestNewProjectTemplatesIndexEvidenceAndSchemaDigestRemainExact(t *testing.T) {
	service, state := contextServiceFixture(t)
	path := service.projects.sqlite.attachments["VPro64"]
	db, err := openReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='USysEnvTable' AND sql IS NOT NULL ORDER BY name LIMIT 1`).Scan(&name)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, path, "DROP INDEX "+quoteHeaderIdentifier(name))
	got, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
	if got != nil || err == nil || !strings.Contains(err.Error(), "table/index set") {
		t.Fatal("columns-only template accepted", got, err)
	}
	ddl := `CREATE TABLE "Fixture"(ID INTEGER)`
	schema := []sqliteSchemaObject{{"index", "implicit", "Fixture", nil}, {"table", "Fixture", "Fixture", &ddl}}
	columns := []ProjectMetadataColumn{{Name: "ID", DeclaredType: "INTEGER"}}
	actual, err := siviProjectAssignmentSchemaDigest(columns, schema)
	if err != nil {
		t.Fatal(err)
	}
	oldJSON := `{"Columns":[{"name":"ID","declaredType":"INTEGER"}],"Schema":[{"Type":"index","Name":"implicit","Table":"Fixture","SQL":null},{"Type":"table","Name":"Fixture","Table":"Fixture","SQL":"CREATE TABLE \"Fixture\"(ID INTEGER)"}]}`
	hash := sha256.Sum256([]byte(oldJSON))
	if actual != hex.EncodeToString(hash[:]) {
		t.Fatal("shared schema extraction changed existing SIVI history digest")
	}
}

type newProjectCancelQuery struct {
	projectMetadataQueryer
	cancel context.CancelFunc
}

func (query newProjectCancelQuery) QueryContext(ctx context.Context, statement string, args ...any) (*sql.Rows, error) {
	if strings.Contains(statement, "sqlite_master WHERE tbl_name") && len(args) == 1 && args[0] == "USysVegTable" {
		query.cancel()
	}
	return query.projectMetadataQueryer.QueryContext(ctx, statement, args...)
}

func TestNewProjectTemplatesStaleOwnershipAndCancellationRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	if got, err := service.readNewProjectTemplates(context.Background(), "stale"); got != nil || err == nil {
		t.Fatal("stale context yielded source templates", got, err)
	}
	prior := owner.attachmentInfo["VPro64"]
	owner.attachmentInfo["VPro64"] = owner.attachmentInfo["project"]
	got, err := service.readNewProjectTemplates(context.Background(), state.ContextID)
	owner.attachmentInfo["VPro64"] = prior
	if got != nil || err == nil {
		t.Fatal("substituted template file ownership accepted", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.readNewProjectTemplates(ctx, state.ContextID); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("pre-request cancellation lost", got, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	templates, present, readErr := readNewProjectTemplateSnapshot(ctx, newProjectCancelQuery{tx, cancel})
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Fatal(err)
	}
	if templates != nil || present || !errors.Is(readErr, context.Canceled) {
		t.Fatal("late cancellation returned first observed template", templates, present, readErr)
	}
	if got, err := service.readNewProjectTemplates(context.Background(), state.ContextID); got == nil || err != nil {
		t.Fatal("late cancellation discarded pinned context", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	queued := &tableCSVLeaseContext{Context: ctx, queued: make(chan struct{})}
	owner.mu.Lock()
	locked := true
	defer func() {
		if locked {
			owner.mu.Unlock()
		}
	}()
	done := make(chan struct {
		review *newProjectTemplateReview
		err    error
	}, 1)
	go func() {
		review, err := service.readNewProjectTemplates(queued, state.ContextID)
		done <- struct {
			review *newProjectTemplateReview
			err    error
		}{review, err}
	}()
	select {
	case <-queued.queued:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach held snapshot mutex")
	}
	cancel()
	select {
	case result := <-done:
		if result.review != nil || !errors.Is(result.err, context.Canceled) {
			t.Fatal("queued cancellation yielded partial review", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued cancellation did not release read leases")
	}
	owner.mu.Unlock()
	locked = false
	if got, err := service.readNewProjectTemplates(context.Background(), state.ContextID); got == nil || err != nil {
		t.Fatal("queued cancellation retry failed", err)
	}
	assertProfileSUFiles(t, service, before)
}
