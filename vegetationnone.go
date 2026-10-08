package main

import "errors"

const longVegetationNoneFeatureEnvironment = "VPRO_LONG_VEGETATION_NONE"

func (s *ContextService) checkLongVegetationGrouping(options longVegetationOptions) error {
	if options.ShowEnglishName && options.ShowSpeciesCode {
		return errors.New("Long Vegetation supplementary fields cannot combine English name and Code")
	}
	if options.ShowSpeciesCode && !s.longVegetationCodeEnabled {
		return errors.New("Long Vegetation species Code display is disabled in this session; retained LVShowEnglishName=2 was not reset")
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
