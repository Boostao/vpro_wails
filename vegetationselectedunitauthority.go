package main

import (
	"context"
	"database/sql"
	"errors"
	"maps"
)

func selectedUnitConfigForChoice(values configValues, choice int) (configValues, error) {
	if choice < 1 || choice > 3 {
		return nil, errors.New("selected report unit approval requires source unit choice 1, 2 or 3")
	}
	if _, err := configInt(values, "ReportOptions", "LVUnitGroups", 1, 3); err != nil {
		return nil, err
	}
	section, err := configSection(values, "ReportOptions")
	if err != nil {
		return nil, err
	}
	result := maps.Clone(values)
	options := maps.Clone(section)
	options["LVUnitGroups"] = choice
	result["ReportOptions"] = options
	return result, nil
}

func (s *ContextService) selectedUnitAvailability(ctx context.Context, plots *PlotService, tx *sql.Tx) (bool, error) {
	if err := s.requireSelectedListReportScope(plots.projects.contextID); err != nil {
		return false, err
	}
	if !s.longVegetationSelectedUnitsEnabled {
		return false, nil
	}
	entry := s.selectedUnitSelections.read(plots.projects.contextID)
	if entry.build == nil {
		return false, nil
	}
	tables, err := readSelectedUnitSelectionTables(ctx, plots.projects.sqlite, tx)
	if err != nil {
		return false, err
	}
	values, err := plots.projects.preferences.snapshot()
	if err != nil {
		return false, err
	}
	hash, err := selectedUnitSelectionHash(plots.projects.contextID, plots.projects.sqlite, tables, entry.fields, values)
	if err != nil {
		return false, err
	}
	return hash == entry.build.ApprovalHash, nil
}

func (s *ContextService) decodeOwnedLongVegetationOptions(ctx context.Context, plots *PlotService,
	tx *sql.Tx, values configValues) (longVegetationOptions, error) {
	if err := s.requireSelectedListReportScope(plots.projects.contextID); err != nil {
		return longVegetationOptions{}, err
	}
	group, err := configInt(values, "ReportOptions", "LVUnitGroups", 1, 3)
	if err != nil {
		return longVegetationOptions{}, err
	}

	if group != 2 {
		return decodeLongVegetationOptions(values)
	}
	if !s.longVegetationSelectedUnitsEnabled {
		return longVegetationOptions{}, errors.New("Long Vegetation selected report units are disabled in this session; retained LVUnitGroups=2 was not reset")
	}
	fields, err := s.approvedSelectedUnitFields(ctx, plots.projects.contextID, plots.projects.sqlite, tx, values)
	if err != nil {
		return longVegetationOptions{}, err
	}
	ordinary, err := selectedUnitConfigForChoice(values, 1)
	if err != nil {
		return longVegetationOptions{}, err
	}
	options, err := decodeLongVegetationOptions(ordinary)
	if err != nil {
		return longVegetationOptions{}, err
	}
	options.SelectedUnitFields = fields
	return options, nil
}
