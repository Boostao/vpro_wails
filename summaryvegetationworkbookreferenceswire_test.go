package main

import (
	"testing"
)

func TestSummaryEffectiveReferenceWorkbookClientWire(t *testing.T) {
	service, state := summaryReferenceWorkbookFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range []struct {
		name       string
		references SummaryVegetationReferenceOptions
		env        bool
		ancillary  *SummaryVegetationWorkbookAncillaryOptions
	}{
		{"attributes", SummaryVegetationReferenceOptions{}, false, nil},
		{"common", SummaryVegetationReferenceOptions{ShowCommon: true}, false, nil},
		{"guide", SummaryVegetationReferenceOptions{UseGuideNames: true}, false, nil},
		{"guideCommon", SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}, false, nil},
		{"environment", SummaryVegetationReferenceOptions{ShowCommon: true}, true, nil},
		{"memberships", SummaryVegetationReferenceOptions{ShowCommon: true}, false, &SummaryVegetationWorkbookAncillaryOptions{}},
		{"summary", SummaryVegetationReferenceOptions{ShowCommon: true}, true, &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}},
		{"guideSummary", SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}, true, &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}},
	} {
		options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: 1, Display: 6, ShowClass: 1,
			IncludeEnv: item.env, References: &item.references, Ancillary: item.ancillary}
		wire[item.name] = portableSummaryWorkbookReview(t, service, state, options)
	}
	assertSummaryWorkbookClientWire(t, "summaryVegetationWorkbookReferencesWire.json",
		"VPRO_UPDATE_SUMMARY_REFERENCE_WORKBOOK_WIRE", wire)
}
