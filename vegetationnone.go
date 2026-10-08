package main

import "errors"

const longVegetationNoneFeatureEnvironment = "VPRO_LONG_VEGETATION_NONE"

func (s *ContextService) checkLongVegetationGrouping(options longVegetationOptions) error {
	if options.NoneGrouping && !s.longVegetationNoneEnabled {
		return errors.New("Long Vegetation None ordering is disabled in this session; retained LVGroupBy=4 was not reset")
	}
	return nil
}
