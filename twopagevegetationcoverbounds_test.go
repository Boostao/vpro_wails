package main

import (
	"context"
	"math"
	"strings"
	"testing"
)

func TestTwoPageVegetationSourceCoverControlBounds(t *testing.T) {
	for _, test := range []struct {
		form   string
		fields []string
	}{
		{"SubVegAXL_BC", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"}},
		{"SubVegAXL", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"}},
		{"SubVegCXL", []string{"Cover6"}},
		{"SubVegDXL", []string{"Cover7", "Cover8", "Cover9"}},
	} {
		t.Run(test.form, func(t *testing.T) {
			original := twoPageVegetationPlanFixture(t, test.form)
			for _, column := range test.fields {
				for _, invalid := range []float64{math.Nextafter(100, math.Inf(-1)), 99.999999, 100, math.Nextafter(100, math.Inf(1)), 101} {
					got, err := planTwoPageVegetationEdits(context.Background(), original,
						[]twoPageVegetationEdit{vegetationPlanEdit(original, column, siviReal(invalid))})
					if err == nil || got != nil || !strings.Contains(err.Error(), "below 100 or NULL") {
						t.Fatalf("%s source >=100 control bound missing: %+v %v", column, got, err)
					}
				}

				for _, valid := range []ProjectMetadataCell{
					siviReal(float64(math.Nextafter32(100, float32(math.Inf(-1))))),
					siviReal(-1), siviReal(-math.MaxFloat32), {Storage: "null"},
				} {
					got, err := planTwoPageVegetationEdits(context.Background(), original,
						[]twoPageVegetationEdit{vegetationPlanEdit(original, column, valid)})
					if err != nil || len(got) != 1 {
						t.Fatalf("%s below100/negative/NULL acquired an invented constraint: %+v %v", column, got, err)
					}
				}
				for _, historical := range []float64{100, 101} {
					*vegetationPlanCell(original, "-7", column) = siviReal(historical)
					got, err := planTwoPageVegetationEdits(context.Background(), original,
						[]twoPageVegetationEdit{vegetationPlanEdit(original, column, siviReal(historical))})
					if err != nil || len(got) != 0 {
						t.Fatalf("%s unchanged historical >=100 was implicitly rejected/rewritten: %+v %v", column, got, err)
					}
				}
				*vegetationPlanCell(original, "-7", column) = metadataInteger("0")
			}
		})
	}
}
