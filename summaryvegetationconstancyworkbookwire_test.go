package main

import (
	"fmt"
	"testing"
)

func summaryConstancyDesktopFixture(t *testing.T, external bool) (*SummaryVegetationWorkbookService, ProjectState) {
	t.Helper()
	service, state := summaryStrataReferenceWorkbookFixture(t, external)
	service.presentationEnabled = true
	owner := service.source.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `
		INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype)
			VALUES('TWIN','Lifeform 2',2,'Twin','U'),('UNUSED','Unused wide master',2147483647,'Unused','U');`)
	mutateContextFixture(t, owner.attachments["project"], `INSERT INTO `+
		quoteHeaderIdentifier(state.ActiveProject+"_Veg")+`(PlotNumber,Species,Cover1)
		VALUES('P1','TWIN',1.25),('P2','TWIN',3.75);`)
	return service, state
}

func TestSummaryConstancyWorkbookClientWire(t *testing.T) {
	service, state := summaryConstancyDesktopFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, average := range []int{1, 2} {
		for _, mode := range []string{"vegetation", "environment", "memberships", "combinedSummary"} {
			options := summaryConstancyWorkbookOptions()
			options.Average = average
			options.IncludeEnv = mode == "environment" || mode == "combinedSummary"
			if mode == "memberships" || mode == "combinedSummary" {
				options.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: mode == "combinedSummary"}
			}
			review := portableSummaryWorkbookReview(t, service, state, options)
			twins := []SummaryVegetationRow{}
			for _, row := range review.Preview.Rows {
				if row.Species.Text != nil && *row.Species.Text == "Lifeform 2" {
					twins = append(twins, row)
				}
			}
			want := "0.75 - 1.56"
			if average == 2 {
				want = "0.75 - 2.08"
			}
			if len(twins) != 2 {
				t.Fatal("visibly duplicate raw-code groups collapsed", twins)
			}
			for i, row := range twins {
				code := []string{"LF2", "TWIN"}[i]
				if row.Layer.Integer == nil || *row.Layer.Integer != "2" ||
					row.RawSpecies == nil || row.RawSpecies.Text == nil || *row.RawSpecies.Text != code ||
					row.Order.Text == nil || *row.Order.Text != "0.75" || len(row.Covers) != 2 ||
					row.Covers[0].Text == nil || *row.Covers[0].Text != want || row.Covers[1].Storage != "null" {
					t.Fatal("independent MAX/fanout/denominator/raw-identity expectation differs", average, mode, workbookRequest(t, row), want)
				}
			}
			wire[fmt.Sprintf("%sAverage%d", mode, average)] = review
		}
	}
	assertSummaryWorkbookClientWire(t, "summaryVegetationWorkbookConstancyWire.json",
		"VPRO_UPDATE_SUMMARY_CONSTANCY_WORKBOOK_WIRE", wire)
}
