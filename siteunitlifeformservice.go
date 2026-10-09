package main

import (
	"context"
	"errors"
	"fmt"
)

const siteUnitSummaryLifeformFeatureEnvironment = "VPRO_SITE_UNIT_SUMMARY_LIFEFORMS"

func (s *ContextService) PreviewSiteUnitSummaryLifeforms(ctx context.Context, contextID string, request SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	if ctx == nil {
		return SiteUnitSummaryPreview{}, errors.New("Summary lifeform preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSummaryPreview{}, err
	}
	if s == nil || !s.siteUnitSummaryLifeformsEnabled {
		return SiteUnitSummaryPreview{}, errors.New("Summary lifeform preview is disabled in this session")
	}
	input, err := s.readSiteUnitQuickVegetationInput(ctx, contextID, request.Method, publicationReadSnapshotHooks{})
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
