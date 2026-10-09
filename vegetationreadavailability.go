package main

import (
	"context"
	"errors"
	"fmt"
)

type childNullIdentityError struct {
	kind   string
	plot   string
	detail string
}

func (err *childNullIdentityError) Error() string {
	return fmt.Sprintf("%s row for plot %q%s has unsupported NULL ID; repair its identity before editing", err.kind, err.plot, err.detail)
}

type VegetationReadAvailability struct {
	Available bool        `json:"available"`
	Reason    string      `json:"reason"`
	Records   []VegRecord `json:"records"`
}

func vegetationReadAvailability(plots *PlotService, plot string) (*VegetationReadAvailability, error) {
	records, err := plots.ListVegRecords(plot)
	if err != nil {
		var identity *childNullIdentityError
		if !errors.As(err, &identity) || identity.kind != "Veg" {
			return nil, err
		}
		return &VegetationReadAvailability{Reason: identity.Error()}, nil
	}
	seen := make(map[int64]bool)
	for _, record := range records {
		if record.ID < -9007199254740991 || record.ID > 9007199254740991 {
			return &VegetationReadAvailability{Reason: "Ordinary vegetation ID exceeds exact desktop numeric transport; use physical-row source views."}, nil
		}
		if seen[record.ID] {
			return &VegetationReadAvailability{Reason: "Ordinary vegetation ID is duplicated; use physical-row source views without changing historical duplicates."}, nil
		}
		seen[record.ID] = true
	}
	return &VegetationReadAvailability{Available: true, Records: records}, nil
}

func (s *ContextService) GetVegetationReadAvailability(ctx context.Context, contextID, plot string) (*VegetationReadAvailability, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*VegetationReadAvailability, error) {
		return vegetationReadAvailability(plots, plot)
	})
}
