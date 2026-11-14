package envreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type siteUnitLifeformCover struct {
	Lifeform      int
	Caption       string
	Value         string
	PhysicalPlots int
	DistinctPlots int
	ValueCount    int
	Minimum       *float64
	Mean          *float64
	Maximum       *float64
}

type siteUnitLifeformCoverUnit struct {
	Code   string
	NPlots int
	Rows   []siteUnitLifeformCover
}

func planSiteUnitLifeformCovers(ctx context.Context, input siteUnitQuickVegetationInput) ([]siteUnitLifeformCoverUnit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.Quick.Project != input.Environment.Report.Project || input.Quick.SU != input.Environment.Report.SU {
		return nil, errors.New("Summary lifeform cover preparation and environment ownership differ")
	}
	columns, err := reportcore.VegetationLifeformReferenceSchema(ctx, input.References.Table)
	if err != nil {
		return nil, err
	}
	references := map[string][]int{}
	for _, row := range input.References.Table.Rows {
		form, err := vprocore.VegetationLifeformInteger(row.Cells[columns["Lifeform"]])
		if err != nil {
			return nil, err
		}
		code := row.Cells[columns["Code"]].Text
		if code != nil && form != nil && *form >= 0 && *form <= 13 {
			references[*code] = append(references[*code], *form)
		}
	}
	result := []siteUnitLifeformCoverUnit{}
	seen := map[string]bool{}
	for _, unit := range input.Environment.Report.Units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[unit.Code] || len(unit.Plots) < 1 {
			return nil, errors.New("Summary lifeform cover units require unique identities and positive owned plot counts")
		}
		seen[unit.Code] = true
		scope := map[string]int{}
		physical := 0
		for _, row := range input.Quick.Memberships {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if row.SiteUnit.Text != nil && *row.SiteUnit.Text == unit.Code && row.PlotNumber.Text != nil {
				scope[*row.PlotNumber.Text]++
				physical++
			}
		}
		type accumulation struct {
			plots map[string]bool
			sum   *big.Rat
			count int
			min   float64
			max   float64
		}
		values := make([]accumulation, 14)
		for form := range values {
			values[form] = accumulation{plots: map[string]bool{}, sum: new(big.Rat)}
		}
		for _, group := range input.Quick.Groups {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if math.IsNaN(group.MyCover) || math.IsInf(group.MyCover, 0) {
				return nil, fmt.Errorf("Summary lifeform cover group %s is not finite", group.PlotNumber)
			}
			if group.Species.Text == nil || scope[group.PlotNumber] == 0 {
				continue
			}
			for _, form := range references[*group.Species.Text] {
				value := &values[form]
				value.plots[group.PlotNumber] = true
				if value.count == 0 || group.MyCover < value.min {
					value.min = group.MyCover
				}
				if value.count == 0 || group.MyCover > value.max {
					value.max = group.MyCover
				}
				weight := scope[group.PlotNumber]
				cover := new(big.Rat).SetFloat64(group.MyCover)
				cover.Mul(cover, new(big.Rat).SetInt64(int64(weight)))
				value.sum.Add(value.sum, cover)
				value.count += weight
			}
		}
		next := siteUnitLifeformCoverUnit{Code: unit.Code, NPlots: len(unit.Plots), Rows: []siteUnitLifeformCover{}}
		for form, value := range values {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row := siteUnitLifeformCover{Lifeform: form, Caption: siteUnitLifeformCaptions[form],
				PhysicalPlots: physical, DistinctPlots: len(value.plots), ValueCount: value.count}
			if value.count > 0 {
				minimum := value.min
				if len(value.plots) < physical {
					minimum = 0
				}
				mean, _ := new(big.Rat).Quo(value.sum, new(big.Rat).SetInt64(int64(len(unit.Plots)))).Float64()
				maximum := value.max
				row.Minimum, row.Mean, row.Maximum = &minimum, &mean, &maximum
			}
			row.Value, err = formatSiteUnitLifeformCover(row)
			if err != nil {
				return nil, err
			}
			next.Rows = append(next.Rows, row)
		}
		result = append(result, next)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

var siteUnitLifeformCaptions = [...]string{
	"Genus-level and mixed", "Coniferous Tree", "Deciduous Tree",
	"Evergreen Shrub", "Deciduous Shrub", "Ferns or Fern-ally",
	"Graminoid", "Forb", "Parasite or Saprophyte", "Moss",
	"Liverwort", "Lichen", "Dwarf woody plant", "Macroalgae",
}

func formatSiteUnitLifeformCover(row siteUnitLifeformCover) (string, error) {
	if row.Lifeform < 0 || row.Lifeform >= len(siteUnitLifeformCaptions) {
		return "", errors.New("Summary lifeform caption is outside the source 0..13 range")
	}
	if row.Minimum == nil && row.Mean == nil && row.Maximum == nil && row.ValueCount == 0 {
		return "", nil
	}
	if row.Minimum == nil || row.Mean == nil || row.Maximum == nil ||
		row.ValueCount < 1 || row.PhysicalPlots < 1 || row.DistinctPlots < 1 ||
		row.DistinctPlots > row.PhysicalPlots || row.ValueCount < row.DistinctPlots {
		return "", errors.New("Summary lifeform formatting requires complete owned scalar statistics")
	}
	for _, value := range []*float64{row.Minimum, row.Mean, row.Maximum} {
		if math.IsNaN(*value) || math.IsInf(*value, 0) {
			return "", errors.New("Summary lifeform formatting requires finite scalar values")
		}
	}
	minimum := reportcore.SiteUnitFixedNumber(*row.Minimum, 1, 1)
	if row.DistinctPlots < row.PhysicalPlots {
		if *row.Minimum != 0 {
			return "", errors.New("Summary lifeform incomplete-plot minimum must retain the source zero guard")
		}
		minimum = "0"
	}
	return minimum + "---" + reportcore.SiteUnitFixedNumber(*row.Mean, 1, 1) + "---" +
		reportcore.SiteUnitFixedNumber(*row.Maximum, 1, 1), nil
}

func PreviewSiteUnitSummaryLifeforms(s *vprocore.ContextService, ctx context.Context, contextID string, request SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	if ctx == nil {
		return SiteUnitSummaryPreview{}, errors.New("Summary lifeform preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSummaryPreview{}, err
	}
	if s == nil {
		return SiteUnitSummaryPreview{}, errors.New("Summary lifeform preview requires owned contexts")
	}
	input, err := ReadSiteUnitQuickVegetationInput(s, ctx, contextID, request.Method, vprocore.PublicationReadSnapshotHooks{})
	if err != nil {
		return SiteUnitSummaryPreview{}, err
	}
	return siteUnitSummaryLifeformPreview(ctx, input)
}

func siteUnitSummaryLifeformPreview(ctx context.Context, input siteUnitQuickVegetationInput) (SiteUnitSummaryPreview, error) {
	fail := func(err error) (SiteUnitSummaryPreview, error) { return SiteUnitSummaryPreview{}, err }
	result := input.Environment
	fields := []SiteUnitSummaryField{}
	indices := []int{}
	for i, field := range result.Report.Fields {
		switch field.Key {
		case "StrataCoverTree":
			for form, caption := range siteUnitLifeformCaptions {
				fields = append(fields, SiteUnitSummaryField{"Lifeform", fmt.Sprintf("Lifeform%d", form),
					caption, "VEGETATION", "lifeform-cover"})
				indices = append(indices, -1)
			}
		case "StrataCoverShrub", "StrataCoverHerb", "StrataCoverMoss":
		default:
			fields = append(fields, field)
			indices = append(indices, i)
		}
	}
	if len(fields) != 49 || len(input.Lifeform) != len(result.Report.Units) {
		return fail(errors.New("Summary lifeform preview requires the complete owned source field registry"))
	}
	result.Report.Fields = fields
	result.Report.Units = append([]SiteUnitSummaryUnit{}, result.Report.Units...)
	result.Report.QuerySource = "selected-su-filtered-env-admin-quickveg-lifeform"
	for i, unit := range result.Report.Units {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		lifeform := input.Lifeform[i]
		if lifeform.Code != unit.Code || lifeform.NPlots != len(unit.Plots) || len(lifeform.Rows) != 14 || len(unit.Values) != 39 {
			return fail(errors.New("Summary lifeform unit/count differs from owned Environment provenance"))
		}
		values := []string{}
		form := 0
		for _, index := range indices {
			if index >= 0 {
				values = append(values, unit.Values[index])
				continue
			}
			row := lifeform.Rows[form]
			if row.Lifeform != form || row.Caption != siteUnitLifeformCaptions[form] {
				return fail(errors.New("Summary lifeform row identity/caption differs from the source"))
			}
			values = append(values, row.Value)
			form++
		}
		unit.Values = values
		result.Report.Units[i] = unit
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

type siteUnitQuickVegetationInput struct {
	Environment SiteUnitSummaryPreview
	Quick       reportcore.SiteUnitQuickVegetation
	References  reportcore.VegetationLifeformReferences
	Lifeform    []siteUnitLifeformCoverUnit
	Tables      []vprocore.ProjectMetadataTable
}

func ReadSiteUnitQuickVegetationInput(s *vprocore.ContextService, ctx context.Context, contextID string, method int,
	hooks vprocore.PublicationReadSnapshotHooks) (siteUnitQuickVegetationInput, error) {
	if ctx == nil {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg requires a context")
	}
	if err := ctx.Err(); err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	if s == nil || s.Projects == nil || s.Projects.Sqlite == nil || s.Plots == nil {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg context service is unavailable")
	}
	if method != 1 && method != 2 {
		return siteUnitQuickVegetationInput{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (siteUnitQuickVegetationInput, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (siteUnitQuickVegetationInput, error) {
			return readSiteUnitQuickVegetationSource(ctx, owner, tx, contextID, method)
		})
	})
}

func readSiteUnitQuickVegetationSource(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx,
	contextID string, method int) (siteUnitQuickVegetationInput, error) {
	if owner.Selection.SU == "USysSuTableDynamic" {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg hierarchy/dynamic-break scope is unavailable")
	}
	environment, err := readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, method)
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	tables := append([]vprocore.ProjectMetadataTable{}, environment.Tables...)
	for _, source := range []struct{ role, table string }{
		{"project", owner.Selection.Project + "_Veg"},
		{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"},
	} {
		table, err := vprocore.ReadPhysicalLocationTable(ctx, tx, source.role, source.table)
		if err != nil {
			return siteUnitQuickVegetationInput{}, err
		}
		tables = append(tables, table)
	}
	quick, err := reportcore.PrepareSiteUnitQuickVegetation(ctx, owner.Selection.Project, owner.Selection.SU, 2, tables[4], tables[2])
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	references, err := reportcore.PrepareVegetationLifeformReferences(ctx, tables[5], tables[6])
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	input := siteUnitQuickVegetationInput{
		Environment: environment.Preview, Quick: quick, References: references, Tables: tables,
	}
	input.Lifeform, err = planSiteUnitLifeformCovers(ctx, input)
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	return input, nil
}
