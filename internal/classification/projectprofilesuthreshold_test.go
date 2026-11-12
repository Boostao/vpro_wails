package classification

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestProfileSUEmptyAndOverNavigationLimitAreExplicitExactFiles(t *testing.T) {
	for _, count := range []int{0, 200, 201} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			service, state, _ := profileRunFixture(t)
			db, _, release, err := service.Plots.GetActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin;
				DELETE FROM Sample_Profile WHERE rowid<>3;
				UPDATE Sample_Profile SET "Order"=1,"Table"='Env',"Field"='PlotNumber',"Operator"='Like',
				Layer=NULL,Species=NULL,Criteria='*',Operation='Add plots' WHERE rowid=3`); err != nil {
				t.Fatal(err)
			}
			for index := 0; index < count; index++ {
				plot := fmt.Sprintf("P%03d", index)
				if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES(?);
					INSERT INTO Sample_Admin(Plot) VALUES(?)`, plot, plot); err != nil {
					t.Fatal(err)
				}
			}
			release()
			ctx := context.Background()
			rules, err := ReviewProjectPlotProfile(service, ctx, state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			input := ProjectPlotProfileRunRequest{OriginalRules: rules.Rules}
			result, err := RunProjectPlotProfile(service, ctx, state.ContextID, input)
			if err != nil || len(result.PlotNumbers) != count {
				t.Fatal("required measurable threshold not actually exercised", result, err)
			}
			review, err := ReviewProjectPlotProfileSU(service, ctx, state.ContextID, ProjectPlotProfileFilterRequest{input, result})
			if err != nil || len(review.Plots) != count {
				t.Fatal("empty/large review became an unavailable navigation proxy", review, err)
			}
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			request := ProfileSUCreation{review, "Threshold", filepath.Join(t.TempDir(), "threshold.db"), true}
			created, err := SaveProjectPlotProfileSU(service, ctx, state.ContextID, request)
			if err != nil || created.PlotCount != count {
				t.Fatal("explicit empty/large SU creation failed", created, err)
			}
			output, err := sqlutil.OpenReadOnlyContext(ctx, created.Path)
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			stored, err := vprocore.ReadSQLiteStorageRows(ctx, output, "main", "Threshold_SU", "", nil, "PlotNumber")
			if err != nil || len(stored.Rows) != count {
				t.Fatal("required exact persisted row count failed", stored, err)
			}
			for index, row := range stored.Rows {
				if row.Cells[0].Text == nil || *row.Cells[0].Text != fmt.Sprintf("P%03d", index) || row.Cells[1].Storage != "null" {
					t.Fatal("large SU literal identity or NULL assignment changed", row)
				}
			}
			vprotest.AssertProfileSUFiles(t, service, before)
		})
	}
}
