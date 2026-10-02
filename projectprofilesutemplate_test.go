package main

import (
	"context"
	"database/sql"
	"testing"
)

func TestProfileSURejectsChangedPhysicalTemplateWithoutRepair(t *testing.T) {
	for _, change := range []string{
		`DROP INDEX uidx_USysSuTable_PlotNumber`,
		`ALTER TABLE USysSuTable ADD COLUMN Extra TEXT`,
		`INSERT INTO USysSuTable(PlotNumber) VALUES('A')`,
		`DROP TABLE USysSuTable; CREATE TABLE USysSuTable(PlotNumber VARCHAR DEFAULT 'A',SiteUnit VARCHAR);
		 CREATE UNIQUE INDEX uidx_USysSuTable_PlotNumber ON USysSuTable(PlotNumber);
		 CREATE INDEX idx_USysSuTable_SiteUnit ON USysSuTable(SiteUnit)`,
		`DROP TABLE USysSuTable; CREATE VIEW USysSuTable AS SELECT PlotNumber,SiteUnit FROM Sample_SU`,
		`DROP TABLE USysSuTable;
		 CREATE TABLE "USysSuTable" ("PlotNumber" VARCHAR CHECK(length(PlotNumber)<3),"SiteUnit" VARCHAR);
		 CREATE UNIQUE INDEX uidx_USysSuTable_PlotNumber ON USysSuTable(PlotNumber);
		 CREATE INDEX idx_USysSuTable_SiteUnit ON USysSuTable(SiteUnit)`,
	} {
		t.Run(change, func(t *testing.T) {
			service, state, request := profileSUFixture(t)
			db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VPro64"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(change); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if _, err := service.ReviewProjectPlotProfileSU(context.Background(), state.ContextID, request.Review.Filter); err == nil {
				t.Fatal("altered/unavailable template silently repaired or offered")
			}
			if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, request); err == nil {
				t.Fatal("old template snapshot authorized destination creation")
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}
