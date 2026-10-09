package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"unicode/utf16"
)

type LifeformSummaryEntry struct {
	PlotNumber    string              `json:"plotNumber"`
	Species       string              `json:"species"`
	Lifeform      int                 `json:"lifeform"`
	Cover         float64             `json:"cover"`
	InsertSURowID string              `json:"insertSuRowId"`
	SpeciesRowID  string              `json:"speciesRowId"`
	ProjectID     ProjectMetadataCell `json:"projectId"`
}

type LifeformSummaryCatalogueRow struct {
	RowID      string              `json:"rowId"`
	Lifeform   int                 `json:"lifeform"`
	Label      ProjectMetadataCell `json:"label"`
	Definition ProjectMetadataCell `json:"definition"`
	ShortName  ProjectMetadataCell `json:"shortName"`
}

type LifeformSummaryRow struct {
	CatalogueRowID string   `json:"catalogueRowId"`
	Lifeform       int      `json:"lifeform"`
	PlotGroups     int      `json:"plotGroups"`
	CoverCount     int      `json:"coverCount"`
	Presence       *float64 `json:"presence"`
	MeanCover      *float64 `json:"meanCover"`
}

type LifeformSummaryUnit struct {
	Code          ProjectMetadataCell  `json:"code"`
	NPlots        int                  `json:"nPlots"`
	SURowIDs      []string             `json:"suRowIds"`
	UniqueSpecies int                  `json:"uniqueSpecies"`
	Occurrences   int                  `json:"occurrences"`
	Rows          []LifeformSummaryRow `json:"rows"`
}

type LifeformSummaryReport struct {
	Project     string                        `json:"project"`
	SU          string                        `json:"su"`
	QuerySource string                        `json:"querySource"`
	Ordering    string                        `json:"ordering"`
	Memberships []VegetationReportMembership  `json:"memberships"`
	Entries     []LifeformSummaryEntry        `json:"entries"`
	Catalogue   []LifeformSummaryCatalogueRow `json:"catalogue"`
	Units       []LifeformSummaryUnit         `json:"units"`
}

// This preview models the normal-SU SQL, not Excel, attributes or hierarchy
// breaks. Binary identity joins are the shared preparation's explicit policy;
// CodeType uses explicit ASCII S/s exclusion, without asserting locale collation.
func planLifeformSummary(ctx context.Context, project, suName string, veg, su, species, catalogue ProjectMetadataTable) (LifeformSummaryReport, error) {
	fail := func(err error) (LifeformSummaryReport, error) { return LifeformSummaryReport{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if suName == "USysSuTableDynamic" {
		return fail(errors.New("Lifeform Summary hierarchy/dynamic-break scope is unavailable"))
	}
	coverNames := []string{"PlotNumber", "Species"}
	for i := 1; i <= 10; i++ {
		coverNames = append(coverNames, fmt.Sprintf("Cover%d", i))
	}
	columns, err := siteUnitTransferColumns(veg, coverNames...)
	if err != nil {
		return fail(fmt.Errorf("Lifeform Summary physical Veg schema: %w", err))
	}
	// The common MAX kernel also supports layer-only fields. Project only this
	// source's ten covers so unused Cover5a/b/c and totals cannot affect it.
	projected := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}}}
	for _, name := range longVegetationCoverColumns() {
		projected.Columns = append(projected.Columns, ProjectMetadataColumn{Name: name})
	}
	for _, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		cells := []ProjectMetadataCell{row.Cells[columns["PlotNumber"]], row.Cells[columns["Species"]]}
		for _, name := range longVegetationCoverColumns() {
			cell := ProjectMetadataCell{Storage: "null"}
			for _, included := range coverNames[2:] {
				if name == included {
					cell = row.Cells[columns[name]]
					break
				}
			}
			cells = append(cells, cell)
		}
		projected.Rows = append(projected.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, projected, su,
		ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "LayerText"}}})
	if err != nil {
		return fail(err)
	}
	byRowID := make(map[string]VegetationReportMembership, len(prepared.Memberships))
	for _, membership := range prepared.Memberships {
		byRowID[membership.RowID] = membership
	}
	prepared.Memberships = []VegetationReportMembership{}
	for _, row := range su.Rows {
		prepared.Memberships = append(prepared.Memberships, byRowID[row.RowID])
	}
	// CreateSmallVeg copies USysVegTable: each reduced cover is assigned to
	// SINGLE before the species join, including rows with no qualifying Code.
	for i := range prepared.ReducedRows {
		for column, name := range prepared.CoverColumns {
			value, err := vegetationReportNumber(prepared.ReducedRows[i].Covers[column])
			if err != nil {
				return fail(err)
			}
			if value == nil {
				continue
			}
			maximum, err := lifeformSummarySingle(value)
			if err != nil {
				return fail(fmt.Errorf("TempReportVegLF %s: %w", name, err))
			}
			prepared.ReducedRows[i].Covers[column] = ProjectMetadataCell{Storage: "real", Real: &maximum}
		}
	}
	result := LifeformSummaryReport{Project: project, SU: suName,
		QuerySource: "normal-su-global-entrydat-plot-rejoin",
		Ordering:    "physical-catalogue-rowid-and-first-membership",
		Memberships: prepared.Memberships, Entries: []LifeformSummaryEntry{},
		Catalogue: []LifeformSummaryCatalogueRow{}, Units: []LifeformSummaryUnit{}}
	specColumns, err := siteUnitTransferColumns(species, "Code", "Lifeform", "Codetype")
	if err != nil {
		return fail(err)
	}
	catColumns, err := siteUnitTransferColumns(catalogue, "Lifeform", "LifeformTXT", "Definition", "ShortName")
	if err != nil {
		return fail(err)
	}
	seen := map[int]bool{}
	for _, row := range catalogue.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		form, err := lifeformSummaryInteger(row.Cells[catColumns["Lifeform"]])
		if err != nil || form == nil || seen[*form] {
			return fail(fmt.Errorf("LifeformCodes row %s violates its unique non-NULL INTEGER key", row.RowID))
		}
		seen[*form] = true
		if *form == 13 {
			continue
		}
		values := []ProjectMetadataCell{}
		for _, column := range []string{"LifeformTXT", "Definition", "ShortName"} {
			cell := row.Cells[catColumns[column]]
			if err := lifeformSummaryText(cell, 255); err != nil {
				return fail(fmt.Errorf("LifeformCodes row %s %s: %w", row.RowID, column, err))
			}
			values = append(values, cloneSiteUnitCell(cell))
		}
		result.Catalogue = append(result.Catalogue, LifeformSummaryCatalogueRow{row.RowID, *form, values[0], values[1], values[2]})
	}
	type definition struct {
		rowID string
		form  int
	}
	definitions := map[string][]definition{}
	for _, row := range species.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		code, codeType := row.Cells[specColumns["Code"]], row.Cells[specColumns["Codetype"]]
		if _, err := vegetationReportIdentity(code); err != nil {
			return fail(fmt.Errorf("USysAllSpecs row %s Code: %w", row.RowID, err))
		}
		if _, err := vegetationReportIdentity(codeType); err != nil {
			return fail(fmt.Errorf("USysAllSpecs row %s Codetype: %w", row.RowID, err))
		}
		if code.Text == nil || codeType.Text == nil || *codeType.Text == "S" || *codeType.Text == "s" {
			continue
		}
		form, err := lifeformSummaryInteger(row.Cells[specColumns["Lifeform"]])
		if err != nil {
			return fail(fmt.Errorf("USysAllSpecs row %s Lifeform: %w", row.RowID, err))
		}
		if form != nil && *form >= 0 && *form <= 12 {
			definitions[*code.Text] = append(definitions[*code.Text], definition{row.RowID, *form})
		}
	}
	// EntryDat is global across the selected SU. Each insertion retains every
	// physical SU and Code match; its ProjectID is NOT a later unit predicate.
	for form := 0; form <= 12; form++ {
		for _, reduced := range prepared.ReducedRows {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			if reduced.Species.Text == nil {
				continue
			}
			for _, def := range definitions[*reduced.Species.Text] {
				if def.form != form {
					continue
				}
				for _, membership := range prepared.Memberships {
					if membership.PlotNumber.Text == nil || *membership.PlotNumber.Text != reduced.PlotNumber {
						continue
					}
					for _, item := range []struct {
						cell  ProjectMetadataCell
						limit int
						name  string
					}{
						{metadataLifeformText(reduced.PlotNumber), 8, "PlotNumber"},
						{reduced.Species, 8, "Species"}, {membership.SiteUnit, 20, "ProjectID"},
					} {
						if err := lifeformSummaryText(item.cell, item.limit); err != nil {
							return fail(fmt.Errorf("EntryDatVegLF %s: %w", item.name, err))
						}
					}
					total := new(big.Rat)
					for i, name := range prepared.CoverColumns {
						if name == "Cover5a" || name == "Cover5b" || name == "Cover5c" || name == "TotalA" || name == "TotalB" {
							continue
						}
						value, err := vegetationReportNumber(reduced.Covers[i])
						if err != nil {
							return fail(err)
						}
						if value != nil {
							total.Add(total, value)
						}
					}
					// USysV2VegLF.Cover is SINGLE: round once at assignment,
					// before any qryLF1 sums or duplicate-weighted rejoin.
					single, err := lifeformSummarySingle(total)
					if err != nil {
						return fail(fmt.Errorf("EntryDatVegLF Cover: %w", err))
					}
					result.Entries = append(result.Entries, LifeformSummaryEntry{reduced.PlotNumber,
						*reduced.Species.Text, form, single, membership.RowID, def.rowID, cloneSiteUnitCell(membership.SiteUnit)})
				}
			}
		}
	}
	unitIndexes := map[string]int{}
	for _, membership := range prepared.Memberships {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		key := membership.SiteUnit.Storage + "\x00"
		if membership.SiteUnit.Text != nil {
			key += *membership.SiteUnit.Text
		}
		index, found := unitIndexes[key]
		if !found {
			index = len(result.Units)
			unitIndexes[key] = index
			result.Units = append(result.Units, LifeformSummaryUnit{Code: cloneSiteUnitCell(membership.SiteUnit),
				SURowIDs: []string{}, Rows: []LifeformSummaryRow{}})
		}
		unit := &result.Units[index]
		unit.SURowIDs = append(unit.SURowIDs, membership.RowID)
		if membership.PlotNumber.Text != nil {
			unit.NPlots++
		}
	}
	for i := range result.Units {
		unit := &result.Units[i]
		type aggregate struct {
			sum   *big.Rat
			count int
		}
		groups := map[int]map[string]*aggregate{}
		unique := map[string]bool{}
		for _, membership := range prepared.Memberships {
			if membership.SiteUnit.Text == nil || unit.Code.Text == nil || *membership.SiteUnit.Text != *unit.Code.Text || membership.PlotNumber.Text == nil {
				continue
			}
			for _, entry := range result.Entries {
				if err := ctx.Err(); err != nil {
					return fail(err)
				}
				if entry.PlotNumber != *membership.PlotNumber.Text {
					continue
				}
				unit.Occurrences++
				unique[entry.Species] = true
				if groups[entry.Lifeform] == nil {
					groups[entry.Lifeform] = map[string]*aggregate{}
				}
				group := groups[entry.Lifeform][entry.PlotNumber]
				if group == nil {
					group = &aggregate{sum: new(big.Rat)}
					groups[entry.Lifeform][entry.PlotNumber] = group
				}
				group.count++
				group.sum.Add(group.sum, new(big.Rat).SetFloat64(entry.Cover))
			}
		}
		unit.UniqueSpecies = len(unique)
		for _, cat := range result.Catalogue {
			row := LifeformSummaryRow{CatalogueRowID: cat.RowID, Lifeform: cat.Lifeform, PlotGroups: len(groups[cat.Lifeform])}
			total := new(big.Rat)
			for _, group := range groups[cat.Lifeform] {
				row.CoverCount += group.count
				total.Add(total, group.sum)
			}
			if unit.NPlots > 0 {
				presence := float64(row.PlotGroups) / float64(unit.NPlots)
				row.Presence = &presence
				if row.PlotGroups > 0 {
					sum, _ := total.Float64()
					mean := sum / float64(unit.NPlots) / 100
					if math.IsNaN(mean) || math.IsInf(mean, 0) {
						return fail(errors.New("Lifeform Summary aggregate is not finite"))
					}
					row.MeanCover = &mean
				}
			}
			unit.Rows = append(unit.Rows, row)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

func metadataLifeformText(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "text", Text: &value}
}

func lifeformSummarySingle(value *big.Rat) (float64, error) {
	precision := uint(value.Num().BitLen() + value.Denom().BitLen() + 32)
	single, _ := new(big.Float).SetPrec(precision).SetMode(big.ToNearestEven).SetRat(value).Float32()
	result := float64(single)
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return 0, errors.New("cannot assign a finite SINGLE; no clamping applied")
	}
	return result, nil
}

func lifeformSummaryText(cell ProjectMetadataCell, limit int) error {
	value, err := vegetationReportIdentity(cell)
	if err != nil {
		return err
	}
	if value != nil && len(utf16.Encode([]rune(*value))) > limit {
		return fmt.Errorf("historical text exceeds source TEXT(%d) UTF-16 units; no repair or truncation applied", limit)
	}
	return nil
}

func lifeformSummaryInteger(cell ProjectMetadataCell) (*int, error) {
	if cell.Storage == "null" {
		return nil, nil
	}
	number, err := vegetationReportNumber(cell)
	if err != nil || number == nil || !number.IsInt() || !number.Num().IsInt64() {
		return nil, errors.New("expected nullable source INTEGER, not repaired/coerced metadata")
	}
	value := number.Num().Int64()
	if value < -32768 || value > 32767 {
		return nil, fmt.Errorf("source INTEGER out of range: %s", strconv.FormatInt(value, 10))
	}
	result := int(value)
	return &result, nil
}
