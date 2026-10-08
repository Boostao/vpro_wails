package main

import (
	"context"
	"errors"
	"fmt"
)

type SpeciesAttributeDefinition struct {
	Field      string   `json:"field"`
	Label      string   `json:"label"`
	Categories []string `json:"categories"`
}

type SpeciesAttributeMatch struct {
	SURowID        string                `json:"suRowId"`
	VegRowID       string                `json:"vegRowId"`
	AttributeRowID string                `json:"attributeRowId"`
	PlotNumber     string                `json:"plotNumber"`
	Species        string                `json:"species"`
	Values         []ProjectMetadataCell `json:"values"`
}

type SpeciesAttributeCount struct {
	Field           string `json:"field"`
	Count           *int   `json:"count"`
	PlotOccurrences int    `json:"plotOccurrences"`
	Categories      []*int `json:"categories"`
}

type SpeciesAttributeUnit struct {
	Code     ProjectMetadataCell     `json:"code"`
	NPlots   int                     `json:"nPlots"`
	SURowIDs []string                `json:"suRowIds"`
	Rows     []SpeciesAttributeCount `json:"rows"`
}

type SpeciesAttributeSummaryReport struct {
	Project     string                       `json:"project"`
	SU          string                       `json:"su"`
	QuerySource string                       `json:"querySource"`
	Definitions []SpeciesAttributeDefinition `json:"definitions"`
	Memberships []VegetationReportMembership `json:"memberships"`
	Matches     []SpeciesAttributeMatch      `json:"matches"`
	Units       []SpeciesAttributeUnit       `json:"units"`
}

func speciesAttributeDefinitions() []SpeciesAttributeDefinition {
	return []SpeciesAttributeDefinition{
		{"SRank", "SRank Detail", []string{"S1", "S2", "S2S3", "S3", "S3S4", "S4", "S4S5", "S5", "SE1", "SE1SE2", "SE2", "SE3", "SE3SE4", "SE4", "SE5", "SEH", "SEX", "SH", "SU", "SX"}},
		{"Wetland_Ind", "Wetland Indicator", []string{"1", "2", "3", "4"}},
		{"WeedStatus", "Weed Status", []string{"I", "P", "R"}},
		{"RedBlueList", "Red Blue List", []string{"R", "B"}},
		{"Est_ASMR", "Est. ASMR", []string{"0", "1", "2", "3", "4", "5", "6"}},
		{"Climate", "Climate", []string{"0", "1", "2", "3", "4", "5", "6"}},
	}
}

func planSpeciesAttributeSummary(ctx context.Context, project, suName string, veg, su, attributes ProjectMetadataTable) (SpeciesAttributeSummaryReport, error) {
	fail := func(err error) (SpeciesAttributeSummaryReport, error) { return SpeciesAttributeSummaryReport{}, err }
	if ctx == nil {
		return fail(errors.New("species attribute summary requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if suName == "None" || suName == "USysSuTableDynamic" {
		return fail(errors.New("species attribute summary requires a selected normal SU; hierarchy/dynamic-break scope is unavailable"))
	}
	vc, err := siteUnitTransferColumns(veg, "PlotNumber", "Species")
	if err != nil {
		return fail(err)
	}
	sc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	definitions := speciesAttributeDefinitions()
	names := []string{"Code", "Codetype"}
	for _, definition := range definitions {
		names = append(names, definition.Field)
	}
	ac, err := siteUnitTransferColumns(attributes, names...)
	if err != nil {
		return fail(err)
	}
	result := SpeciesAttributeSummaryReport{Project: project, SU: suName,
		QuerySource: "normal-su-raw-veg-attribute-join", Definitions: definitions,
		Memberships: []VegetationReportMembership{}, Matches: []SpeciesAttributeMatch{},
		Units: []SpeciesAttributeUnit{}}
	byCode := map[string][]ProjectMetadataRow{}
	for _, row := range attributes.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		for _, name := range names {
			if _, err := vegetationReportIdentity(row.Cells[ac[name]]); err != nil {
				return fail(fmt.Errorf("USysSppAttributes row %s %s: %w", row.RowID, name, err))
			}
		}
		code, kind := row.Cells[ac["Code"]].Text, row.Cells[ac["Codetype"]].Text
		if code != nil && kind != nil && *kind != "S" && *kind != "s" {
			byCode[*code] = append(byCode[*code], row)
		}
	}
	byPlot := map[string][]ProjectMetadataRow{}
	for _, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vegetationReportIdentity(row.Cells[vc["PlotNumber"]])
		if err != nil {
			return fail(err)
		}
		if _, err := vegetationReportIdentity(row.Cells[vc["Species"]]); err != nil {
			return fail(err)
		}
		if plot != nil {
			byPlot[*plot] = append(byPlot[*plot], row)
		}
	}
	type unitKey struct {
		null bool
		text string
	}
	unitIndices := map[unitKey]int{}
	for _, member := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vegetationReportIdentity(member.Cells[sc["PlotNumber"]])
		if err != nil {
			return fail(err)
		}
		code, err := vegetationReportIdentity(member.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(err)
		}
		membership := VegetationReportMembership{RowID: member.RowID,
			PlotNumber: cloneSiteUnitCell(member.Cells[sc["PlotNumber"]]),
			SiteUnit:   cloneSiteUnitCell(member.Cells[sc["SiteUnit"]])}
		result.Memberships = append(result.Memberships, membership)
		key := unitKey{null: code == nil}
		if code != nil {
			key.text = *code
		}
		index, found := unitIndices[key]
		if !found {
			index = len(result.Units)
			unitIndices[key] = index
			unit := SpeciesAttributeUnit{Code: cloneSiteUnitCell(membership.SiteUnit), SURowIDs: []string{}, Rows: []SpeciesAttributeCount{}}
			for _, definition := range definitions {
				unit.Rows = append(unit.Rows, SpeciesAttributeCount{Field: definition.Field, Categories: make([]*int, len(definition.Categories))})
			}
			result.Units = append(result.Units, unit)
		}
		unit := &result.Units[index]
		unit.SURowIDs = append(unit.SURowIDs, member.RowID)
		if plot != nil {
			unit.NPlots++
		}
		// Source per-unit predicates do not match NULL SiteUnit. Preserve its
		// membership evidence without inventing a string identity for it.
		if code == nil || plot == nil {
			continue
		}
		for _, vegetation := range byPlot[*plot] {
			species := vegetation.Cells[vc["Species"]].Text
			if species == nil {
				continue
			}
			for _, attribute := range byCode[*species] {
				if err := ctx.Err(); err != nil {
					return fail(err)
				}
				match := SpeciesAttributeMatch{SURowID: member.RowID, VegRowID: vegetation.RowID,
					AttributeRowID: attribute.RowID, PlotNumber: *plot, Species: *species, Values: []ProjectMetadataCell{}}
				for _, definition := range definitions {
					match.Values = append(match.Values, cloneSiteUnitCell(attribute.Cells[ac[definition.Field]]))
				}
				result.Matches = append(result.Matches, match)
			}
		}
	}
	byMember := map[string][]SpeciesAttributeMatch{}
	for _, match := range result.Matches {
		byMember[match.SURowID] = append(byMember[match.SURowID], match)
	}
	for i := range result.Units {
		unit := &result.Units[i]
		for j, definition := range definitions {
			plots := map[string]bool{}
			for _, rowID := range unit.SURowIDs {
				for _, match := range byMember[rowID] {
					if err := ctx.Err(); err != nil {
						return fail(err)
					}
					value := match.Values[j].Text
					if value == nil {
						continue
					}
					row := &unit.Rows[j]
					if row.Count == nil {
						row.Count = new(int)
					}
					*row.Count++
					plots[match.PlotNumber] = true
					for category, literal := range definition.Categories {
						if *value == literal {
							if row.Categories[category] == nil {
								row.Categories[category] = new(int)
							}
							*row.Categories[category]++
						}
					}
				}
			}
			unit.Rows[j].PlotOccurrences = len(plots)
		}
	}
	return result, nil
}
