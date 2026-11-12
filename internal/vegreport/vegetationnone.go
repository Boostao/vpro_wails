package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "errors"

const LongVegetationNoneFeatureEnvironment = "VPRO_LONG_VEGETATION_NONE"

func CheckLongVegetationGrouping(s *vprocore.ContextService, options longVegetationOptions) error {
	if options.SelectedUnitFields != nil && !s.LongVegetationSelectedUnitsEnabled {
		return errors.New("Long Vegetation selected report units are disabled in this session; retained LVUnitGroups=2 was not reset")
	}
	if options.NoneUnits && !s.LongVegetationNoneUnitsEnabled {
		return errors.New("Long Vegetation None report units are disabled in this session; retained LVUnitGroups=3 was not reset")
	}
	if options.ShowEnglishName && options.ShowSpeciesCode || options.SelectedField != "" && (options.ShowEnglishName || options.ShowSpeciesCode) {
		return errors.New("Long Vegetation supplementary fields cannot combine English name, Code or a selected field")
	}
	if options.ShowSpeciesCode && !s.LongVegetationCodeEnabled {
		return errors.New("Long Vegetation species Code display is disabled in this session; retained LVShowEnglishName=2 was not reset")
	}
	if options.SelectedField != "" && !s.LongVegetationSelectedFieldEnabled {
		return errors.New("Long Vegetation selected field display is disabled in this session; retained LVShowEnglishName=4 was not reset")
	}
	if options.NoneGrouping && (options.LifeformGrouping || options.StrataGrouping) ||
		options.LifeformGrouping && options.StrataGrouping {
		return errors.New("Long Vegetation grouping cannot combine None, Lifeform or Strata")
	}
	if options.NoneGrouping && !s.LongVegetationNoneEnabled {
		return errors.New("Long Vegetation None ordering is disabled in this session; retained LVGroupBy=4 was not reset")
	}
	if options.LifeformGrouping && !s.LongVegetationLifeformEnabled {
		return errors.New("Long Vegetation Lifeform grouping is disabled in this session; retained LVGroupBy=3 was not reset")
	}
	if options.StrataGrouping && !s.LongVegetationStrataEnabled {
		return errors.New("Long Vegetation Strata grouping is disabled in this session; retained LVGroupBy=2 was not reset")
	}
	return nil
}
