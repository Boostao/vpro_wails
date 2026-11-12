package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"strings"
	"testing"
)

func TestTwoPageEntryFieldPoliciesExactPlannerDomains(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		_, _, _, original, _ := twoPageWriteFixture(t, form, false)
		policies, err := twopage.TwoPageEntryFieldPolicies(form)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, policy := range policies {
			if seen[policy.Column] || policy.Column == "PlotNumber" || policy.Owner != "Env" && policy.Owner != "Admin" {
				t.Fatal("ambiguous, identity or unknown policy owner", policy)
			}
			seen[policy.Column] = true
			edit := sftest.SiviParentEdit(t, original, policy.Column, vprocore.ProjectMetadataCell{Storage: "null"})
			if edit.Table != original.Project+"_"+policy.Owner {
				t.Fatal("policy advertised a different physical owner", policy, edit)
			}
			var value vprocore.ProjectMetadataCell
			switch policy.Kind {
			case "text", "categorical", "plot-type":
				value = vprotest.MetadataText("x")
			case "date":
				value = vprotest.MetadataText("2020-01-02 00:00:00")
			case "integer", "boolean", "option":
				value = vprotest.MetadataInteger("0")
				if policy.Kind == "option" {
					value = vprotest.MetadataText("1")
				}
			case "single", "latitude", "longitude":
				value = vprotest.SiviReal(0.25)
			default:
				t.Fatal("unreviewed published parser domain", policy)
			}
			edit.Value = value
			if _, err := twopage.PlanTwoPageEntryProjection(context.Background(), original, []sourceform.SiviParentScalarEdit{edit}, true); err != nil {
				t.Fatal("advertised parser domain disagrees with actual planner", policy, err)
			}
			if policy.Maximum > 0 {
				edit.Value = vprotest.MetadataText(strings.Repeat("x", policy.Maximum))
				if _, err := twopage.PlanTwoPageEntryProjection(context.Background(), original, []sourceform.SiviParentScalarEdit{edit}, true); err != nil {
					t.Fatal("advertised exact UTF-16 bound rejected by planner", policy, err)
				}
				edit.Value = vprotest.MetadataText(strings.Repeat("x", policy.Maximum+1))
				if _, err := twopage.PlanTwoPageEntryProjection(context.Background(), original, []sourceform.SiviParentScalarEdit{edit}, true); err == nil {
					t.Fatal("advertised overlength threshold accepted by planner", policy)
				}
			}
		}
		if seen["BEC_Use"] != (form == "FS882-8x6XL") || !seen["ProjectID"] || !seen["SV_FloodPlain"] {
			t.Fatal("policy cohort inherited wrong form or omitted required field", seen)
		}
	}
	if _, err := twopage.TwoPageEntryFieldPolicies("frmSIVIsite"); err == nil {
		t.Fatal("another form inherited complete-entry policies")
	}
}
