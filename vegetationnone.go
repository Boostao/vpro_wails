package main

import "errors"

const longVegetationNoneFeatureEnvironment = "VPRO_LONG_VEGETATION_NONE"

func (s *ContextService) checkLongVegetationGrouping(options longVegetationOptions) error {
	if options.SelectedUnitFields != nil && !s.longVegetationSelectedUnitsEnabled {
		return errors.New("Long Vegetation selected report units are disabled in this session; retained LVUnitGroups=2 was not reset")
	}
	if options.NoneUnits && !s.longVegetationNoneUnitsEnabled {
		return errors.New("Long Vegetation None report units are disabled in this session; retained LVUnitGroups=3 was not reset")
	}
	if options.ShowEnglishName && options.ShowSpeciesCode || options.SelectedField != "" && (options.ShowEnglishName || options.ShowSpeciesCode) {
		return errors.New("Long Vegetation supplementary fields cannot combine English name, Code or a selected field")
	}
	if options.ShowSpeciesCode && !s.longVegetationCodeEnabled {
		return errors.New("Long Vegetation species Code display is disabled in this session; retained LVShowEnglishName=2 was not reset")
	}
	if options.SelectedField != "" && !s.longVegetationSelectedFieldEnabled {
		return errors.New("Long Vegetation selected field display is disabled in this session; retained LVShowEnglishName=4 was not reset")
	}
	if options.NoneGrouping && (options.LifeformGrouping || options.StrataGrouping) ||
		options.LifeformGrouping && options.StrataGrouping {
		return errors.New("Long Vegetation grouping cannot combine None, Lifeform or Strata")
	}
	if options.NoneGrouping && !s.longVegetationNoneEnabled {
		return errors.New("Long Vegetation None ordering is disabled in this session; retained LVGroupBy=4 was not reset")
	}
	if options.LifeformGrouping && !s.longVegetationLifeformEnabled {
		return errors.New("Long Vegetation Lifeform grouping is disabled in this session; retained LVGroupBy=3 was not reset")
	}
	if options.StrataGrouping && !s.longVegetationStrataEnabled {
		return errors.New("Long Vegetation Strata grouping is disabled in this session; retained LVGroupBy=2 was not reset")
	}
	return nil
}
