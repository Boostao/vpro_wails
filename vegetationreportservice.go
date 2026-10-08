package main

import (
	"context"
	"errors"
)

type LongVegetationSettings struct {
	Title                string                         `json:"title"`
	Grouping             string                         `json:"grouping"`
	Average              string                         `json:"average"`
	ConstantSpeciesList  bool                           `json:"constantSpeciesList"`
	PresenceGreaterThan  float64                        `json:"presenceGreaterThan"`
	MeanCoverGreaterThan float64                        `json:"meanCoverGreaterThan"`
	Order                string                         `json:"order"`
	ShowEnglishName      bool                           `json:"showEnglishName"`
	ShowSpeciesCode      bool                           `json:"showSpeciesCode"`
	Quality              *LongVegetationQualitySettings `json:"quality"`
}

type LongVegetationQualityCriterion struct {
	Minimum     string `json:"minimum"`
	IncludeNull bool   `json:"includeNull"`
}

type LongVegetationQualitySettings struct {
	Site LongVegetationQualityCriterion `json:"site"`
	Veg  LongVegetationQualityCriterion `json:"veg"`
	Soil LongVegetationQualityCriterion `json:"soil"`
}

type LongVegetationQualityOccurrence struct {
	MembershipID string                 `json:"membershipId"`
	PlotNumber   ProjectMetadataCell    `json:"plotNumber"`
	SiteUnit     ProjectMetadataCell    `json:"siteUnit"`
	EnvRowID     string                 `json:"envRowId"`
	AdminRowID   string                 `json:"adminRowId"`
	ListRowIDs   [3]*string             `json:"listRowIds"`
	Values       [3]ProjectMetadataCell `json:"values"`
}

type LongVegetationQualityReference struct {
	RowID     string              `json:"rowId"`
	Item      ProjectMetadataCell `json:"item"`
	ListName  ProjectMetadataCell `json:"listName"`
	ItemOrder ProjectMetadataCell `json:"itemOrder"`
}

type LongVegetationQualitySelection struct {
	ThresholdRowIDs       [3][]string                       `json:"thresholdRowIds"`
	Occurrences           []LongVegetationQualityOccurrence `json:"occurrences"`
	ExcludedMembershipIDs []string                          `json:"excludedMembershipIds"`
	References            []LongVegetationQualityReference  `json:"references"`
}

type LongVegetationOptions struct {
	ContextID   string                 `json:"contextId"`
	Project     string                 `json:"project"`
	ProjectPath string                 `json:"projectPath"`
	SU          string                 `json:"su"`
	SUPath      string                 `json:"suPath"`
	Settings    LongVegetationSettings `json:"settings"`
}

type LongVegetationPlot struct {
	PlotNumber string   `json:"plotNumber"`
	Cover      *float64 `json:"cover"`
}

type LongVegetationRow struct {
	Layer       ProjectMetadataCell  `json:"layer"`
	Species     ProjectMetadataCell  `json:"species"`
	EnglishName ProjectMetadataCell  `json:"englishName"`
	MatchedName ProjectMetadataCell  `json:"matchedName"`
	Presence    *float64             `json:"presence"`
	MeanCover   *float64             `json:"meanCover"`
	Plots       []LongVegetationPlot `json:"plots"`
}

type LongVegetationUnit struct {
	Code           ProjectMetadataCell     `json:"code"`
	LongName       *string                 `json:"longName"`
	NameStatus     string                  `json:"nameStatus"`
	NameCandidates []EnvironmentReportName `json:"nameCandidates"`
	NumPlots       int                     `json:"numPlots"`
	MembershipIDs  []string                `json:"membershipIds"`
	Rows           []LongVegetationRow     `json:"rows"`
}

type LongVegetationDiagnostic struct {
	Code     string `json:"code"`
	Identity string `json:"identity"`
	Count    int    `json:"count"`
}

type LongVegetationReport struct {
	Project     string                          `json:"project"`
	SU          string                          `json:"su"`
	Title       string                          `json:"title"`
	Units       []LongVegetationUnit            `json:"units"`
	Diagnostics []LongVegetationDiagnostic      `json:"diagnostics"`
	Quality     *LongVegetationQualitySelection `json:"quality"`
}

type LongVegetationPreview struct {
	ContextID   string                 `json:"contextId"`
	ProjectPath string                 `json:"projectPath"`
	SUPath      string                 `json:"suPath"`
	Settings    LongVegetationSettings `json:"settings"`
	Report      LongVegetationReport   `json:"report"`
}

func loadLongVegetationOptions(plots *PlotService) (longVegetationOptions, error) {
	values, err := plots.projects.preferences.snapshot()
	if err != nil {
		return longVegetationOptions{}, err
	}
	return decodeLongVegetationOptions(values)
}

func vegetationSettings(options longVegetationOptions) LongVegetationSettings {
	settings := LongVegetationSettings{
		Title: options.Title, Average: options.Average, ConstantSpeciesList: options.ConstantSpeciesList,
		PresenceGreaterThan: options.PresenceGreaterThan, MeanCoverGreaterThan: options.MeanCoverGreaterThan,
		Order: options.Order, ShowEnglishName: options.ShowEnglishName, ShowSpeciesCode: options.ShowSpeciesCode,
	}
	settings.Grouping = "layer"
	if options.NoneGrouping {
		settings.Grouping = "none"
	} else if options.LifeformGrouping {
		settings.Grouping = "lifeform"
	} else if options.StrataGrouping {
		settings.Grouping = "strata"
	}
	if options.Quality != nil {
		criteria := *options.Quality
		settings.Quality = &LongVegetationQualitySettings{
			Site: LongVegetationQualityCriterion{criteria[0].Minimum, criteria[0].IncludeNull},
			Veg:  LongVegetationQualityCriterion{criteria[1].Minimum, criteria[1].IncludeNull},
			Soil: LongVegetationQualityCriterion{criteria[2].Minimum, criteria[2].IncludeNull},
		}
	}
	return settings
}

func (s *ContextService) GetLongVegetationOptions(ctx context.Context, contextID string) (LongVegetationOptions, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (LongVegetationOptions, error) {
		owner := plots.projects.sqlite
		if owner.selection.SU == "None" {
			return LongVegetationOptions{}, errors.New("Long Vegetation requires an explicitly selected SU, not an all-project fallback")
		}
		options, err := loadLongVegetationOptions(plots)
		if err != nil {
			return LongVegetationOptions{}, err
		}
		if err := s.checkLongVegetationGrouping(options); err != nil {
			return LongVegetationOptions{}, err
		}
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return LongVegetationOptions{}, err
		}
		defer owner.mu.Unlock()
		if err := profileOwnedFiles(owner); err != nil {
			return LongVegetationOptions{}, err
		}
		if err := ctx.Err(); err != nil {
			return LongVegetationOptions{}, err
		}
		return LongVegetationOptions{
			ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
			SU: owner.selection.SU, SUPath: owner.selection.SUPath, Settings: vegetationSettings(options),
		}, nil
	})
}

func (s *ContextService) PreviewLongVegetation(ctx context.Context, contextID string) (LongVegetationPreview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (LongVegetationPreview, error) {
		options, err := loadLongVegetationOptions(plots)
		if err != nil {
			return LongVegetationPreview{}, err
		}
		if err := s.checkLongVegetationGrouping(options); err != nil {
			return LongVegetationPreview{}, err
		}
		report, err := readLongVegetationLayers(ctx, plots, options)
		if err != nil {
			return LongVegetationPreview{}, err
		}
		transport, err := longVegetationTransport(ctx, report)
		if err != nil {
			return LongVegetationPreview{}, err
		}
		owner := plots.projects.sqlite
		return LongVegetationPreview{
			ContextID: contextID, ProjectPath: owner.selection.ProjectPath, SUPath: owner.selection.SUPath,
			Settings: vegetationSettings(options), Report: transport,
		}, nil
	})
}

func longVegetationTransport(ctx context.Context, source vegetationLayerReport) (LongVegetationReport, error) {
	result := LongVegetationReport{
		Project: source.Project, SU: source.SU, Title: source.Title,
		Units:       make([]LongVegetationUnit, 0, len(source.Units)),
		Diagnostics: make([]LongVegetationDiagnostic, 0, len(source.Diagnostics)),
	}
	for _, unit := range source.Units {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		next := LongVegetationUnit{
			Code: cloneSiteUnitCell(unit.Code), NumPlots: unit.NumPlots,
			LongName: cloneCatalogueValue(unit.LongName), NameStatus: unit.NameStatus,
			NameCandidates: make([]EnvironmentReportName, 0, len(unit.NameCandidates)),
			MembershipIDs:  append([]string{}, unit.MembershipIDs...),
			Rows:           make([]LongVegetationRow, 0, len(unit.Rows)),
		}
		for _, candidate := range unit.NameCandidates {
			if err := ctx.Err(); err != nil {
				return LongVegetationReport{}, err
			}
			next.NameCandidates = append(next.NameCandidates, EnvironmentReportName{candidate.RowID, cloneSiteUnitCell(candidate.Value)})
		}
		for _, row := range unit.Rows {
			if err := ctx.Err(); err != nil {
				return LongVegetationReport{}, err
			}
			item := LongVegetationRow{
				Layer: cloneSiteUnitCell(row.Layer), Species: cloneSiteUnitCell(row.Species),
				EnglishName: cloneSiteUnitCell(row.EnglishName), MatchedName: cloneSiteUnitCell(row.MatchedName),
				Presence: cloneCatalogueValue(row.Presence), MeanCover: cloneCatalogueValue(row.MeanCover),
				Plots: make([]LongVegetationPlot, 0, len(row.Plots)),
			}
			for _, plot := range row.Plots {
				if err := ctx.Err(); err != nil {
					return LongVegetationReport{}, err
				}
				item.Plots = append(item.Plots, LongVegetationPlot{plot.PlotNumber, cloneCatalogueValue(plot.Cover)})
			}
			next.Rows = append(next.Rows, item)
		}
		result.Units = append(result.Units, next)
	}
	for _, diagnostic := range source.Diagnostics {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		result.Diagnostics = append(result.Diagnostics, LongVegetationDiagnostic{diagnostic.Code, diagnostic.Identity, diagnostic.Count})
	}
	if source.Quality != nil {
		quality := &LongVegetationQualitySelection{
			Occurrences:           make([]LongVegetationQualityOccurrence, 0, len(source.Quality.Occurrences)),
			ExcludedMembershipIDs: append([]string{}, source.Quality.ExcludedMembershipIDs...),
			References:            make([]LongVegetationQualityReference, 0, len(source.Quality.References)),
		}
		for i, ids := range source.Quality.ThresholdRowIDs {
			quality.ThresholdRowIDs[i] = append([]string{}, ids...)
		}
		for _, occurrence := range source.Quality.Occurrences {
			if err := ctx.Err(); err != nil {
				return LongVegetationReport{}, err
			}
			next := LongVegetationQualityOccurrence{
				MembershipID: occurrence.Membership.RowID, PlotNumber: cloneSiteUnitCell(occurrence.Membership.PlotNumber),
				SiteUnit: cloneSiteUnitCell(occurrence.Membership.SiteUnit), EnvRowID: occurrence.EnvRowID, AdminRowID: occurrence.AdminRowID,
			}
			for i, id := range occurrence.ListRowIDs {
				next.ListRowIDs[i] = cloneCatalogueValue(id)
				next.Values[i] = cloneSiteUnitCell(occurrence.Values[i])
			}
			quality.Occurrences = append(quality.Occurrences, next)
		}
		for _, reference := range source.Quality.References {
			if err := ctx.Err(); err != nil {
				return LongVegetationReport{}, err
			}
			quality.References = append(quality.References, LongVegetationQualityReference{
				reference.RowID, cloneSiteUnitCell(reference.Item), cloneSiteUnitCell(reference.ListName), cloneSiteUnitCell(reference.ItemOrder)})
		}
		result.Quality = quality
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationReport{}, err
	}
	return result, nil
}
