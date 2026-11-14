package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "errors"

func CheckLongVegetationGrouping(_ *vprocore.ContextService, options longVegetationOptions) error {
	if options.ShowEnglishName && options.ShowSpeciesCode || options.SelectedField != "" && (options.ShowEnglishName || options.ShowSpeciesCode) {
		return errors.New("Long Vegetation supplementary fields cannot combine English name, Code or a selected field")
	}
	if options.NoneGrouping && (options.LifeformGrouping || options.StrataGrouping) ||
		options.LifeformGrouping && options.StrataGrouping {
		return errors.New("Long Vegetation grouping cannot combine None, Lifeform or Strata")
	}
	return nil
}
