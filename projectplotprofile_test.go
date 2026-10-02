package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestProjectPlotProfileReviewPreservesOriginalOwnershipOrderAndDescriptions(t *testing.T) {
	service, state := contextServiceFixture(t)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	var columns []string
	for _, column := range review.Rules.Columns {
		columns = append(columns, column.Name)
	}
	if review.Project != "Sample" || review.Table != "Sample_Profile" ||
		!reflect.DeepEqual(columns, []string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount"}) ||
		len(review.Rules.Rows) != 8 {
		t.Fatal("original project profile ownership/schema/rows changed", review)
	}
	var identities []string
	for _, row := range review.Rules.Rows {
		identities = append(identities, row.RowID)
	}
	if !reflect.DeepEqual(identities, []string{"3", "6", "1", "2", "4", "7", "5", "8"}) {
		t.Fatal("source Order was replaced by physical row order", identities)
	}
	if len(review.Descriptions.Rows) != 1 || len(review.Descriptions.Columns) != 2 ||
		review.Descriptions.Rows[0].Cells[1].Text == nil || *review.Descriptions.Rows[0].Cells[1].Text != "VP05-2" {
		t.Fatal("native object description was treated as project compatibility or discarded", review.Descriptions)
	}
	if _, err := json.Marshal(review); err != nil {
		t.Fatal("typed profile transport failed", err)
	}
	for role, original := range files {
		current, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("readonly review changed project/support/scratch/counts/descriptions", role, err)
		}
	}
}

func TestProjectPlotProfileReviewPreservesHistoricalTypesNullsAndDuplicateRules(t *testing.T) {
	service, state := contextServiceFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`DELETE FROM Sample_Profile;
		INSERT INTO Sample_Profile(rowid,"Order","Table",Species,Criteria,Operation,PlotCount)
		VALUES (-2,NULL,'Env',NULL,'',NULL,9007199254740993),
		(-1,4,'Veg','  literal  ',1.25,'Add plots',-1),(2,4,'Lump','',x'00FF','Common plots',NULL);
		ALTER TABLE Sample_Profile ADD COLUMN HistoricalExtra;
		UPDATE Sample_Profile SET HistoricalExtra=1.25 WHERE rowid=-1;
		UPDATE _table_metadata SET description=NULL WHERE table_name='Sample_Profile';`); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Rules.Columns) != 10 || len(review.Rules.Rows) != 3 || len(review.Descriptions.Rows) != 1 ||
		review.Rules.Rows[0].RowID != "-2" || review.Rules.Rows[1].RowID != "-1" || review.Rules.Rows[2].RowID != "2" {
		t.Fatal("historical columns/NULL/duplicate rule order collapsed", review)
	}
	cell := metadataTestCell(t, review.Rules, "-2", "PlotCount")
	if cell.Integer == nil || *cell.Integer != "9007199254740993" {
		t.Fatal("large historical count rounded", cell)
	}
	if metadataTestCell(t, review.Rules, "-2", "Species").Storage != "null" ||
		*metadataTestCell(t, review.Rules, "2", "Species").Text != "" ||
		*metadataTestCell(t, review.Rules, "-1", "Species").Text != "  literal  " ||
		*metadataTestCell(t, review.Rules, "-1", "Criteria").Text != "1.25" ||
		*metadataTestCell(t, review.Rules, "-1", "HistoricalExtra").Real != 1.25 ||
		*metadataTestCell(t, review.Rules, "2", "Criteria").BlobHex != "00ff" ||
		review.Descriptions.Rows[0].Cells[1].Storage != "null" {
		t.Fatal("typed literal values/descriptions were repaired or collapsed")
	}
	if _, err := db.Exec(`UPDATE _table_metadata SET description='' WHERE table_name='Sample_Profile'`); err != nil {
		t.Fatal(err)
	}
	review, err = service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil || review.Descriptions.Rows[0].Cells[1].Text == nil || *review.Descriptions.Rows[0].Cells[1].Text != "" {
		t.Fatal("empty description collapsed into NULL", review, err)
	}
}

func TestProjectPlotProfileReviewLegacyDuplicateDescriptionsRemainPhysical(t *testing.T) {
	service, state := contextServiceFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// Deliberate disposable legacy variation, not the canonical UNIQUE schema.
	if _, err := db.Exec(`CREATE TABLE LegacyDescriptions AS SELECT * FROM _table_metadata;
		DROP TABLE _table_metadata;
		ALTER TABLE LegacyDescriptions RENAME TO _table_metadata;
		INSERT INTO _table_metadata VALUES ('Sample_Profile',NULL),('Sample_Profile','');`); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil || len(review.Descriptions.Rows) != 3 ||
		review.Descriptions.Rows[1].Cells[1].Storage != "null" || *review.Descriptions.Rows[2].Cells[1].Text != "" ||
		review.Descriptions.Rows[0].RowID == review.Descriptions.Rows[1].RowID {
		t.Fatal("legacy physical descriptions collapsed or selected implicitly", review, err)
	}
}

func TestProjectPlotProfileReviewRejectsStaleCancelledMalformedAndIncompleteSources(t *testing.T) {
	service, state := contextServiceFixture(t)
	if _, err := service.ReviewProjectPlotProfile(context.Background(), "stale"); err == nil {
		t.Fatal("stale context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ReviewProjectPlotProfile(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled read returned success", err)
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`UPDATE Sample_Profile SET Species=CAST(x'FF' AS TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID); err == nil {
		t.Fatal("malformed historical Unicode was repaired")
	}
	if _, err := db.Exec(`DROP TABLE Sample_Profile; CREATE TABLE Sample_Profile ("Order" INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID); err == nil {
		t.Fatal("incomplete original profile schema accepted")
	}
}
