package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type vegetationProjectPreparation struct {
	Project       string
	SourceSU      string
	Entries       vegetationReportEntries
	SelectedUnits []vegetationProjectSelectedUnitOccurrence
	NoneUnits     []vegetationProjectNoneUnitOccurrence
}

func prepareLongVegetationProjectSelectedUnits(ctx context.Context, project, sourceSU string,
	veg, layers, env, admin, su ProjectMetadataTable, names []string,
	quality *vegetationQualitySelection) (vegetationProjectPreparation, error) {
	if ctx == nil {
		return vegetationProjectPreparation{}, errors.New("project vegetation preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return vegetationProjectPreparation{}, err
	}
	if sourceSU == "" || !utf8.ValidString(sourceSU) || strings.ContainsRune(sourceSU, 0) {
		return vegetationProjectPreparation{}, errors.New("project MAX preparation requires an explicit original SU selection or None")
	}
	if sourceSU == "None" {
		if quality != nil {
			return vegetationProjectPreparation{}, errors.New("project quality requires an actual original selected SU; no Env-only qualification was inferred")
		}
		if len(su.Columns) != 0 || len(su.Rows) != 0 {
			return vegetationProjectPreparation{}, errors.New("project None selection cannot carry an invented physical SU")
		}
	} else {
		if _, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit"); err != nil {
			return vegetationProjectPreparation{}, fmt.Errorf("project MAX preparation original selected SU: %w", err)
		}
	}
	var rows []vegetationProjectSelectedUnitOccurrence
	var err error
	if quality == nil {
		rows, err = planLongVegetationProjectSelectedUnits(ctx, env, admin, names)
	} else {
		rows, err = planLongVegetationQualifiedProjectSelectedUnits(ctx, env, admin, su, names, *quality)
	}
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	scope := map[string]bool{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return vegetationProjectPreparation{}, err
		}
		scope[*row.PlotNumber.Text] = true
	}
	entries, err := prepareLongVegetationProjectEntries(ctx, project, veg, layers, scope)
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	return vegetationProjectPreparation{Project: project, SourceSU: sourceSU, Entries: entries, SelectedUnits: rows}, nil
}

func prepareLongVegetationProjectNoneUnits(ctx context.Context, project string,
	veg, layers, env ProjectMetadataTable) (vegetationProjectPreparation, error) {
	rows, err := planLongVegetationProjectNoneUnits(ctx, env)
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	scope := map[string]bool{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return vegetationProjectPreparation{}, err
		}
		if row.PlotNumber.Text != nil {
			scope[*row.PlotNumber.Text] = true
		}
	}
	entries, err := prepareLongVegetationProjectEntries(ctx, project, veg, layers, scope)
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	return vegetationProjectPreparation{Project: project, SourceSU: "None", Entries: entries, NoneUnits: rows}, nil
}

func prepareLongVegetationProjectEntries(ctx context.Context, project string,
	veg, layers ProjectMetadataTable, scope map[string]bool) (vegetationReportEntries, error) {
	if err := ctx.Err(); err != nil {
		return vegetationReportEntries{}, err
	}
	if project == "" || !utf8.ValidString(project) || strings.ContainsRune(project, 0) {
		return vegetationReportEntries{}, errors.New("project vegetation preparation requires an explicit valid project identity")
	}
	vegColumns, err := siteUnitTransferColumns(veg, append([]string{"PlotNumber", "Species"}, longVegetationCoverColumns()...)...)
	if err != nil {
		return vegetationReportEntries{}, fmt.Errorf("Long Vegetation physical Veg schema: %w", err)
	}
	layerColumns, err := siteUnitTransferColumns(layers, "LayerText")
	if err != nil {
		return vegetationReportEntries{}, fmt.Errorf("Long Vegetation LayerCode schema: %w", err)
	}
	return reduceLongVegetationEntries(ctx, veg, layers, vegColumns, layerColumns, scope)
}
