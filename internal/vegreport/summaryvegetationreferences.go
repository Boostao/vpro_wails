package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

type SummaryVegetationReferenceOptions struct {
	ShowCommon    bool `json:"showCommon"`
	UseGuideNames bool `json:"useGuideNames"`
}

type SummaryVegetationReferenceReview struct {
	MasterRows        int  `json:"masterRows"`
	PersonalRows      int  `json:"personalRows"`
	DescriptionRows   int  `json:"descriptionRows"`
	RemovedMasterRows int  `json:"removedMasterRows"`
	DerivedRows       int  `json:"derivedRows"`
	UnionRows         int  `json:"unionRows"`
	ShowCommon        bool `json:"showCommon"`
	UseGuideNames     bool `json:"useGuideNames"`
}

type summaryVegetationReferencePlan struct {
	References reportcore.VegetationLifeformReferences
	Review     SummaryVegetationReferenceReview
}

func summaryVegetationReferenceGroupingAvailable(grouping int, common bool) bool {
	return grouping == 11 || grouping == 12 || !common && (grouping == 13 || grouping == 14)
}

func summaryVegetationUnanimousCommonAvailable(grouping, display int, common bool) bool {
	return (grouping == 13 || grouping == 14) && display == 6 && common
}

func summaryVegetationMasterLongInteger(cell vprocore.ProjectMetadataCell) (*int, error) {
	if _, err := vprocore.MetadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage == "null" {
		return nil, nil
	}
	if cell.Storage != "integer" {
		return nil, errors.New("master Lifeform requires exact nullable signed32 LONG storage")
	}
	value, err := strconv.ParseInt(*cell.Integer, 10, 32)
	if err != nil {
		return nil, errors.New("master Lifeform exceeds source signed32 LONG range")
	}
	result := int(value)
	return &result, nil
}

func summaryReferenceAttributeFlag(cell vprocore.ProjectMetadataCell) (bool, error) {
	if _, err := vprocore.MetadataCellValue(cell); err != nil {
		return false, err
	}
	if cell.Storage != "null" && (cell.Storage != "integer" || *cell.Integer != "0" && *cell.Integer != "-1") {
		return false, errors.New("Summary effective master IsAttribute requires nullable Access BOOLEAN0/-1 storage")
	}
	return cell.Integer != nil && *cell.Integer == "-1", nil
}

func summaryEffectiveReferenceEnvironmentMaster(ctx context.Context, master vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataTable, error) {
	if ctx == nil {
		return vprocore.ProjectMetadataTable{}, errors.New("Summary effective environmental master requires a context")
	}
	columns, err := vprocore.SiteUnitTransferColumns(master, "IsAttribute")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	result := vprocore.ProjectMetadataTable{Columns: append([]vprocore.ProjectMetadataColumn{}, master.Columns...), Rows: []vprocore.ProjectMetadataRow{}}
	for _, row := range master.Rows {
		if err := ctx.Err(); err != nil {
			return vprocore.ProjectMetadataTable{}, err
		}
		attribute, err := summaryReferenceAttributeFlag(row.Cells[columns["IsAttribute"]])
		if err != nil {
			return vprocore.ProjectMetadataTable{}, err
		}
		if !attribute {
			result.Rows = append(result.Rows, row)
		}
	}
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	return result, nil
}

func prepareSummaryVegetationReferences(ctx context.Context, master, personal, descriptions vprocore.ProjectMetadataTable,
	options SummaryVegetationReferenceOptions) (summaryVegetationReferencePlan, error) {
	fail := func(err error) (summaryVegetationReferencePlan, error) {
		return summaryVegetationReferencePlan{}, err
	}
	if ctx == nil {
		return fail(errors.New("Summary effective references require a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	result := summaryVegetationReferencePlan{
		References: reportcore.VegetationLifeformReferences{
			Table:   vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
			Origins: map[string][]reportcore.VegetationLifeformReferenceOrigin{},
		},
		Review: SummaryVegetationReferenceReview{
			MasterRows: len(master.Rows), PersonalRows: len(personal.Rows), DescriptionRows: len(descriptions.Rows),
			ShowCommon: options.ShowCommon || options.UseGuideNames, UseGuideNames: options.UseGuideNames,
		},
	}
	for _, name := range reportcore.VegetationLifeformReferenceColumns {
		kind := "TEXT"
		if name == "Lifeform" {
			kind = "INTEGER"
		}
		result.References.Table.Columns = append(result.References.Table.Columns,
			vprocore.ProjectMetadataColumn{Name: name, DeclaredType: kind})
	}
	seen := map[string]string{}
	appendRow := func(cells []vprocore.ProjectMetadataCell, table, id string) error {
		if cells[4].Text == nil || *cells[4].Text == "s" || *cells[4].Text == "S" {
			return nil
		}
		return reportcore.AppendVegetationLifeformUnionRow(&result.References, seen, cells,
			reportcore.VegetationLifeformReferenceOrigin{Table: table, RowID: id})
	}
	for _, source := range []struct {
		name     string
		table    vprocore.ProjectMetadataTable
		personal bool
	}{{"USysAllSpecs", master, false}, {"USysUserSpp", personal, true}} {
		lifeform := "Lifeform"
		if source.personal {
			canonical, original := false, false
			for _, column := range source.table.Columns {
				canonical = canonical || column.Name == "Lifeform"
				original = original || column.Name == "LifeForm"
			}
			if canonical && original {
				return fail(errors.New("Summary effective references have ambiguous personal LifeForm/Lifeform columns"))
			}
			if original {
				lifeform = "LifeForm"
			}
		}
		required := []string{"Code", "ScientificName", lifeform, "EnglishName", "Codetype"}
		if !source.personal {
			required = append(required, "IsAttribute")
			if options.UseGuideNames {
				required = append(required, "CombinedEnglishName")
			}
		}
		columns, err := vprocore.SiteUnitTransferColumns(source.table, required...)
		if err != nil {
			return fail(fmt.Errorf("Summary effective %s schema: %w", source.name, err))
		}
		for _, column := range source.table.Columns {
			if !source.personal && column.Name == "JustEnglishName" {
				return fail(errors.New("Summary effective master has partially renamed guide fields; no physical repair applied"))
			}
		}
		for _, row := range source.table.Rows {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			for _, cell := range row.Cells {
				if _, err := vprocore.MetadataCellValue(cell); err != nil {
					return fail(fmt.Errorf("Summary effective %s row%s: %w", source.name, row.RowID, err))
				}
			}
			codeLimit, typeLimit := 255, 255
			readInteger := summaryVegetationMasterLongInteger
			if source.personal {
				codeLimit, typeLimit, readInteger = 8, 1, vprocore.VegetationLifeformInteger
			}
			for name, limit := range map[string]int{
				"Code": codeLimit, "ScientificName": 255, "EnglishName": 255, "Codetype": typeLimit,
			} {
				if err := reportcore.LifeformSummaryText(row.Cells[columns[name]], limit); err != nil {
					return fail(fmt.Errorf("Summary effective %s row%s %s: %w", source.name, row.RowID, name, err))
				}
			}
			form := row.Cells[columns[lifeform]]
			if _, err := readInteger(form); err != nil {
				return fail(fmt.Errorf("Summary effective %s row%s Lifeform: %w", source.name, row.RowID, err))
			}
			if source.personal && row.Cells[columns["Code"]].Storage == "null" {
				return fail(errors.New("Summary effective personal Code requires its non-NULL source identity"))
			}
			if !source.personal {
				attribute, err := summaryReferenceAttributeFlag(row.Cells[columns["IsAttribute"]])
				if err != nil {
					return fail(err)
				}
				if attribute {
					result.Review.RemovedMasterRows++
					continue
				}
			}
			english := row.Cells[columns["EnglishName"]]
			if !source.personal && options.UseGuideNames {
				english = row.Cells[columns["CombinedEnglishName"]]
				if err := reportcore.LifeformSummaryText(english, 255); err != nil {
					return fail(fmt.Errorf("Summary effective master guide row%s: %w", row.RowID, err))
				}
			}
			cells := []vprocore.ProjectMetadataCell{
				vprocore.CloneSiteUnitCell(row.Cells[columns["Code"]]), vprocore.CloneSiteUnitCell(row.Cells[columns["ScientificName"]]),
				vprocore.CloneSiteUnitCell(form), vprocore.CloneSiteUnitCell(english), vprocore.CloneSiteUnitCell(row.Cells[columns["Codetype"]]),
			}
			if err := appendRow(cells, source.name, row.RowID); err != nil {
				return fail(err)
			}
		}
	}
	columns, err := vprocore.SiteUnitTransferColumns(descriptions, "ShortName", "Definition")
	if err != nil {
		return fail(fmt.Errorf("Summary effective attribute-description schema: %w", err))
	}
	for _, row := range descriptions.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		for _, cell := range row.Cells {
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
				return fail(fmt.Errorf("Summary effective attribute-description row%s: %w", row.RowID, err))
			}
		}
		code, definition := row.Cells[columns["ShortName"]], row.Cells[columns["Definition"]]
		for _, cell := range []vprocore.ProjectMetadataCell{code, definition} {
			if err := reportcore.LifeformSummaryText(cell, 255); err != nil {
				return fail(fmt.Errorf("Summary effective attribute-description row%s: %w", row.RowID, err))
			}
		}
		if code.Storage == "null" {
			continue
		}
		result.Review.DerivedRows++
		if options.UseGuideNames {
			// Guide mode exposes the inserted row's default NULL CombinedEnglishName.
			definition = vprocore.ProjectMetadataCell{Storage: "null"}
		}
		user := "U"
		cells := []vprocore.ProjectMetadataCell{vprocore.CloneSiteUnitCell(code), {Storage: "null"}, {Storage: "null"},
			vprocore.CloneSiteUnitCell(definition), {Storage: "text", Text: &user}}
		if err := appendRow(cells, "USysSppAttributeDescription", row.RowID); err != nil {
			return fail(err)
		}
	}
	result.Review.UnionRows = len(result.References.Table.Rows)
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
