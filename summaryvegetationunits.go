package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

type summaryVegetationUnit struct {
	Code          string
	NumPlots      int
	MembershipIDs []string
	RawVegRowIDs  []string
}

// Literal read-model identities are not an Access database-collation claim.
// Admission uses raw Veg; SV1 counts SU PlotNumber, not converted cover rows.
func prepareSummaryVegetationUnits(ctx context.Context, veg, su ProjectMetadataTable) ([]summaryVegetationUnit, error) {
	if ctx == nil {
		return nil, errors.New("summary vegetation unit preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vc, err := siteUnitTransferColumns(veg, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("summary vegetation raw Veg schema: %w", err)
	}
	sc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("summary vegetation SU schema: %w", err)
	}
	raw := map[string][]string{}
	for _, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot, err := vegetationReportIdentity(row.Cells[vc["PlotNumber"]])
		if err != nil {
			return nil, fmt.Errorf("summary vegetation raw Veg row %s PlotNumber: %w", row.RowID, err)
		}
		if plot != nil {
			raw[*plot] = append(raw[*plot], row.RowID)
		}
	}
	type unitState struct {
		unit summaryVegetationUnit
		raw  map[string]bool
	}
	units := map[string]*unitState{}
	groups := map[string]bool{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot, err := vegetationReportIdentity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return nil, fmt.Errorf("summary vegetation SU row %s PlotNumber: %w", row.RowID, err)
		}
		code, err := vegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return nil, fmt.Errorf("summary vegetation SU row %s SiteUnit: %w", row.RowID, err)
		}
		groups[vegetationTextKey(row.Cells[sc["SiteUnit"]])] = true
		if len(groups) > 250 {
			return nil, errors.New("summary vegetation has too many original SU groups (>250); limit units before output")
		}
		if code == nil {
			continue
		}
		state := units[*code]
		if state == nil {
			state = &unitState{unit: summaryVegetationUnit{Code: *code, MembershipIDs: []string{}, RawVegRowIDs: []string{}},
				raw: map[string]bool{}}
			units[*code] = state
		}
		state.unit.MembershipIDs = append(state.unit.MembershipIDs, row.RowID)
		if plot == nil {
			continue
		}
		state.unit.NumPlots++
		for _, id := range raw[*plot] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !state.raw[id] {
				state.raw[id] = true
				state.unit.RawVegRowIDs = append(state.unit.RawVegRowIDs, id)
			}
		}
	}
	result := []summaryVegetationUnit{}
	for _, state := range units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(state.unit.RawVegRowIDs) == 0 {
			continue
		}
		sort.Strings(state.unit.MembershipIDs)
		sort.Strings(state.unit.RawVegRowIDs)
		result = append(result, state.unit)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
