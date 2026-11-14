package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"maps"
)

func selectedUnitConfigForChoice(values vprocore.ConfigValues, choice int) (vprocore.ConfigValues, error) {
	if choice < 1 || choice > 3 {
		return nil, errors.New("selected report unit approval requires source unit choice 1, 2 or 3")
	}
	if _, err := vprocore.ConfigInt(values, "ReportOptions", "LVUnitGroups", 1, 3); err != nil {
		return nil, err
	}
	section, err := vprocore.ConfigSection(values, "ReportOptions")
	if err != nil {
		return nil, err
	}
	result := maps.Clone(values)
	options := maps.Clone(section)
	options["LVUnitGroups"] = choice
	result["ReportOptions"] = options
	return result, nil
}

func SelectedUnitAvailability(s *vprocore.ContextService, ctx context.Context, plots *vprocore.PlotService, tx *sql.Tx) (bool, error) {
	if err := s.RequireSelectedListReportScope(plots.Projects.ContextID); err != nil {
		return false, err
	}
	entry := s.SelectedUnitSelections.Read(plots.Projects.ContextID)
	if entry.Build == nil {
		return false, nil
	}
	tables, err := readSelectedUnitSelectionTables(ctx, plots.Projects.Sqlite, tx)
	if err != nil {
		return false, err
	}
	values, err := plots.Projects.Preferences.Snapshot()
	if err != nil {
		return false, err
	}
	hash, err := selectedUnitSelectionHash(plots.Projects.ContextID, plots.Projects.Sqlite, tables, entry.Fields, values)
	if err != nil {
		return false, err
	}
	return hash == entry.Build.ApprovalHash, nil
}

func DecodeOwnedLongVegetationOptions(s *vprocore.ContextService, ctx context.Context, plots *vprocore.PlotService,
	tx *sql.Tx, values vprocore.ConfigValues) (longVegetationOptions, error) {
	if err := s.RequireSelectedListReportScope(plots.Projects.ContextID); err != nil {
		return longVegetationOptions{}, err
	}
	group, err := vprocore.ConfigInt(values, "ReportOptions", "LVUnitGroups", 1, 3)
	if err != nil {
		return longVegetationOptions{}, err
	}

	if group != 2 {
		return decodeLongVegetationOptions(values)
	}
	fields, err := ApprovedSelectedUnitFields(s, ctx, plots.Projects.ContextID, plots.Projects.Sqlite, tx, values)
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
