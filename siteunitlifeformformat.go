package main

import (
	"errors"
	"math"
)

var siteUnitLifeformCaptions = [...]string{
	"Genus-level and mixed", "Coniferous Tree", "Deciduous Tree",
	"Evergreen Shrub", "Deciduous Shrub", "Ferns or Fern-ally",
	"Graminoid", "Forb", "Parasite or Saprophyte", "Moss",
	"Liverwort", "Lichen", "Dwarf woody plant", "Macroalgae",
}

func formatSiteUnitLifeformCover(row siteUnitLifeformCover) (string, error) {
	if row.Lifeform < 0 || row.Lifeform >= len(siteUnitLifeformCaptions) {
		return "", errors.New("Summary lifeform caption is outside the source 0..13 range")
	}
	if row.Minimum == nil && row.Mean == nil && row.Maximum == nil && row.ValueCount == 0 {
		return "", nil
	}
	if row.Minimum == nil || row.Mean == nil || row.Maximum == nil ||
		row.ValueCount < 1 || row.PhysicalPlots < 1 || row.DistinctPlots < 1 ||
		row.DistinctPlots > row.PhysicalPlots || row.ValueCount < row.DistinctPlots {
		return "", errors.New("Summary lifeform formatting requires complete owned scalar statistics")
	}
	for _, value := range []*float64{row.Minimum, row.Mean, row.Maximum} {
		if math.IsNaN(*value) || math.IsInf(*value, 0) {
			return "", errors.New("Summary lifeform formatting requires finite scalar values")
		}
	}
	minimum := siteUnitFixedNumber(*row.Minimum, 1, 1)
	if row.DistinctPlots < row.PhysicalPlots {
		if *row.Minimum != 0 {
			return "", errors.New("Summary lifeform incomplete-plot minimum must retain the source zero guard")
		}
		minimum = "0"
	}
	return minimum + "---" + siteUnitFixedNumber(*row.Mean, 1, 1) + "---" +
		siteUnitFixedNumber(*row.Maximum, 1, 1), nil
}
