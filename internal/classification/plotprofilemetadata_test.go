package classification

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func bareProfileFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState, string) {
	t.Helper()
	service, state, _ := profileRunFixture(t)
	path := filepath.Join(t.TempDir(), "original empty profile.db")
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Bare_Profile(
		"Order" INTEGER,"Table" VARCHAR,"Field" VARCHAR,"Operator" VARCHAR,
		"Layer" VARCHAR,"Species" VARCHAR,"Criteria" VARCHAR,"Operation" VARCHAR,"PlotCount" INTEGER)`)
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	return service, state, path
}

func TestProfileMetadataGenuineAbsenceSupportsSelectionReviewAndOwnedBlankRules(t *testing.T) {
	service, state, path := bareProfileFixture(t)
	ctx := context.Background()
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found, err := service.ListPlotProfileSources(ctx, state.ContextID, path)
	if err != nil || len(found) != 1 || !found[0].Available || found[0].Writable {
		t.Fatal("genuine metadata absence made an original empty profile unusable", found, err)
	}
	next, err := service.SelectPlotProfile(state.ContextID, found[0].Source)
	if err != nil {
		t.Fatal(err)
	}
	review, err := ReviewProjectPlotProfile(service, ctx, next.ContextID)
	if err != nil || len(review.Rules.Columns) != 9 || len(review.Rules.Rows) != 0 ||
		review.Descriptions.Columns == nil || review.Descriptions.Rows == nil ||
		len(review.Descriptions.Columns) != 0 || len(review.Descriptions.Rows) != 0 {
		t.Fatal("absent metadata was fabricated, ambiguous or unreadable", review, err)
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, current) {
		t.Fatal("readonly absent-metadata selection created descriptions", err)
	}
	granted := authorizeProfileFixture(t, service, next, true)
	created, err := CreateProjectPlotProfileRule(service, ctx, granted.ContextID, blankProfileCreation(review.Rules))
	if err != nil || created.RowID != "1" {
		t.Fatal("empty original profile cannot create an explicitly owned rule", created, err)
	}
	c := service.Projects.Sqlite
	var exists bool
	if err := c.Conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profile.sqlite_master WHERE name='_table_metadata')`).Scan(&exists); err != nil || exists {
		t.Fatal("profile writer synthesized description metadata", exists, err)
	}
	review, err = ReviewProjectPlotProfile(service, ctx, granted.ContextID)
	if err != nil || len(review.Rules.Rows) != 1 || len(review.Descriptions.Columns) != 0 {
		t.Fatal("owned empty-profile workflow cannot review its saved rule", review, err)
	}
	for role, original := range files {
		current, err := os.ReadFile(c.Attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("bare profile workflow changed independent project/support data", role, err)
		}
	}
}

func TestProfileMetadataPresentMalformedAndViewAreNotAbsence(t *testing.T) {
	for _, definition := range []string{
		`CREATE TABLE _table_metadata(table_name TEXT)`,
		`CREATE TABLE _table_metadata(description TEXT)`,
		`CREATE VIEW _table_metadata AS SELECT 'Bare_Profile' AS table_name,NULL AS description`,
	} {
		t.Run(definition, func(t *testing.T) {
			service, state, path := bareProfileFixture(t)
			db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(definition)
			closeErr := db.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			found, err := service.ListPlotProfileSources(context.Background(), state.ContextID, path)
			if err == nil && (len(found) != 1 || found[0].Available || found[0].Reason == "") {
				t.Fatal("malformed/view metadata became available as genuine absence", found)
			}
			if _, err := service.SelectPlotProfile(state.ContextID, vprocore.PlotProfileSource{Name: "Bare", Path: path}); err == nil {
				t.Fatal("malformed/view metadata selected as absent")
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(original, current) {
				t.Fatal("rejected metadata schema was repaired", err)
			}
		})
	}
}

func TestProfileMetadataAbsenceVersusEmptyPhysicalTableGuardsAuthorization(t *testing.T) {
	service, state, path := bareProfileFixture(t)
	ctx := context.Background()
	next, err := service.SelectPlotProfile(state.ContextID, vprocore.PlotProfileSource{Name: "Bare", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	review, err := ReviewProjectPlotProfile(service, ctx, next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE _table_metadata(table_name TEXT,description TEXT,Extra BLOB)`)
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	if _, err := SetPlotProfileEditing(service, ctx, next.ContextID, PlotProfileWriteRequest{Review: review, Enabled: true, Confirmed: true}); err == nil {
		t.Fatal("absent metadata review accepted a newly present physical metadata table")
	}
	fresh, err := ReviewProjectPlotProfile(service, ctx, next.ContextID)
	if err != nil || len(fresh.Descriptions.Columns) != 3 || len(fresh.Descriptions.Rows) != 0 {
		t.Fatal("empty physical metadata table lost schema/presence identity", fresh, err)
	}
	authorizeProfileFixture(t, service, next, true)
}
