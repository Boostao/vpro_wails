package main

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

func TestProfileSUProjectExactZero200201AndNullableSelectedSiteUnits(t *testing.T) {
	for _, count := range []int{0, 3, 200, 201} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			service, state, _ := profileRunFixture(t)
			ctx := context.Background()
			db, _, release, err := service.plots.getActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin; DELETE FROM Sample_Profile WHERE rowid<>3;
				UPDATE Sample_Profile SET "Order"=1,"Table"='Env',"Field"='PlotNumber',"Operator"='Like',
				Layer=NULL,Species=NULL,Criteria='*',Operation='Add plots' WHERE rowid=3;
				CREATE TABLE Picked_SU(PlotNumber TEXT,SiteUnit TEXT)`); err != nil {
				t.Fatal(err)
			}
			expected := []ProfileSUPlot{}
			for index := 0; index < count; index++ {
				plot := fmt.Sprintf("P%03d", index)
				var site *string
				if count == 3 && index != 0 {
					value := ""
					if index == 2 {
						value = " U' # "
					}
					site = &value
				}
				if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES(?);
					INSERT INTO Sample_Admin(Plot) VALUES(?); INSERT INTO Picked_SU VALUES(?,?)`, plot, plot, plot, site); err != nil {
					t.Fatal(err)
				}
				expected = append(expected, ProfileSUPlot{plot, site})
			}
			release()
			if count == 3 {
				state, err = service.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
					SU: "Picked", SUPath: state.ProjectPath, Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath})
				if err != nil {
					t.Fatal(err)
				}
			}
			rules, err := service.ReviewProjectPlotProfile(ctx, state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			input := ProjectPlotProfileRunRequest{OriginalRules: rules.Rules}
			result, err := service.RunProjectPlotProfile(ctx, state.ContextID, input)
			if err != nil || len(result.PlotNumbers) != count {
				t.Fatal("required measurable threshold not exercised", result, err)
			}
			review, err := service.ReviewProjectPlotProfileSUInProject(ctx, state.ContextID, ProjectPlotProfileFilterRequest{input, result})
			if err != nil || !reflect.DeepEqual(review.SU.Plots, expected) {
				t.Fatal("literal/NULL/source SiteUnit membership changed", review.SU.Plots, err)
			}
			created, err := service.SaveProjectPlotProfileSUInProject(ctx, state.ContextID, ProfileSUProjectCreation{review, "Threshold", true})
			if err != nil || created.PlotCount != count {
				t.Fatal("exact empty/large SU insertion failed", created, err)
			}
			stored, err := readSQLiteStorageRows(ctx, service.projects.sqlite.conn, "project", "Threshold_SU", "", nil, "PlotNumber")
			if err != nil || len(stored.Rows) != count {
				t.Fatal("persisted measurable count differs", stored, err)
			}
			for index, row := range stored.Rows {
				plot, site := row.Cells[0], row.Cells[1]
				if plot.Text == nil || *plot.Text != expected[index].PlotNumber ||
					expected[index].SiteUnit == nil && site.Storage != "null" ||
					expected[index].SiteUnit != nil && (site.Text == nil || *site.Text != *expected[index].SiteUnit) {
					t.Fatal("stored nullable SiteUnit or literal identity changed", row)
				}
			}
		})
	}
}

func TestProfileSUProjectSupportAliasAndPresentTemplateMetadataStayUnavailable(t *testing.T) {
	for _, alias := range []bool{true, false} {
		t.Run(fmt.Sprint(alias), func(t *testing.T) {
			service, state, request := profileSUProjectFixture(t)
			c := service.projects.sqlite
			if alias {
				path, info := c.attachments["VUser"], c.attachmentInfo["VUser"]
				c.attachments["VUser"], c.attachmentInfo["VUser"] = c.attachments["project"], c.attachmentInfo["project"]
				defer func() { c.attachments["VUser"], c.attachmentInfo["VUser"] = path, info }()
			} else {
				db, err := sql.Open("sqlite3", sqliteFileURI(c.attachments["VPro64"], "rw"))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TABLE _table_metadata(table_name TEXT,description TEXT)`); err != nil {
					db.Close()
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			before := databaseBytes(t, c.attachments)
			if _, err := service.ReviewProjectPlotProfileSUInProject(context.Background(), state.ContextID, request.Review.SU.Filter); err == nil {
				t.Fatal("support alias/present source metadata offered an unsupported write")
			}
			if _, err := service.SaveProjectPlotProfileSUInProject(context.Background(), state.ContextID, request); err == nil {
				t.Fatal("support alias/present source metadata accepted old review")
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}
