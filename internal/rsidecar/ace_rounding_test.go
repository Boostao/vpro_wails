package rsidecar

import (
	"context"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestQuartoMeanCoverACE15SigDigitDecimalRounding(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	vprotest.MutateContextFixture(t, state.ProjectPath, `
		INSERT INTO "Sample_Env" ("PlotNumber") VALUES ('ACEQ1');
		INSERT INTO "Sample_Admin" ("Plot") VALUES ('ACEQ1');
		INSERT INTO "Sample_Veg" ("PlotNumber", "ID", "Species", "Cover6") VALUES
			('ACEQ1', 9901, 'PSEUMEN', 1.005),
			('ACEQ1', 9902, 'TSUGHET', 35.855);
	`)

	rows, err := buildQuartoVegSummary(ctx, svc.Projects.Sqlite, []string{"ACEQ1"}, false, 0.0)
	if err != nil {
		t.Fatalf("buildQuartoVegSummary failed: %v", err)
	}
	gotBySpp := map[string]float64{}
	for _, r := range rows {
		gotBySpp[r.Species] = r.MeanCover
	}
	if gotBySpp["PSEUMEN"] != 1.01 {
		t.Fatalf("expected MeanCover(1.005) == 1.01, got %v", gotBySpp["PSEUMEN"])
	}
	if gotBySpp["TSUGHET"] != 35.86 {
		t.Fatalf("expected MeanCover(35.855) == 35.86, got %v", gotBySpp["TSUGHET"])
	}
}
