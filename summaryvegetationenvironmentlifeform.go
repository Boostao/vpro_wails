package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
)

func prepareSummaryVegetationEnvironmentalLifeform(ctx context.Context, argument string, siteUnit *string,
	quick, master, su ProjectMetadataTable) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) {
		return ProjectMetadataCell{}, fmt.Errorf("Summary environmental lifeform %q: %w", argument, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	var form int
	switch argument {
	case "1":
		form = 1
	case "2":
		form = 2
	case "3":
		form = 3
	case "4":
		form = 4
	case "5":
		form = 5
	case "6":
		form = 6
	case "7":
		form = 7
	case "8":
		form = 8
	case "9":
		form = 9
	case "10":
		form = 10
	case "11":
		form = 11
	case "12":
		form = 12
	case "13":
		form = 13
	default:
		return fail(errors.New("requires a calibrated literal TEXT lifeform argument1 through13"))
	}
	if siteUnit == nil {
		return fail(errors.New("original String SiteUnit argument rejects NULL (94)"))
	}
	if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	sc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	qc, err := siteUnitTransferColumns(quick, "Plotnumber", "Species", "Cover")
	if err != nil {
		return fail(err)
	}
	mc, err := siteUnitTransferColumns(master, "Code", "Lifeform", "Codetype")
	if err != nil {
		return fail(err)
	}
	memberships := map[string]int64{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vegetationReportIdentity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s PlotNumber: %w", row.RowID, err))
		}
		if plot != nil && unit != nil && *unit == *siteUnit {
			if memberships[*plot] == math.MaxInt32 {
				return fail(errors.New("physical SU multiplicity exceeds source Long range"))
			}
			memberships[*plot]++
		}
	}
	references := map[string]int64{}
	for _, row := range master.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		code, err := vegetationReportIdentity(row.Cells[mc["Code"]])
		if err != nil {
			return fail(fmt.Errorf("master row %s Code: %w", row.RowID, err))
		}
		kind, err := vegetationReportIdentity(row.Cells[mc["Codetype"]])
		if err != nil {
			return fail(fmt.Errorf("master row %s Codetype: %w", row.RowID, err))
		}
		lifeform, err := vegetationLifeformInteger(row.Cells[mc["Lifeform"]])
		if err != nil {
			return fail(fmt.Errorf("master row %s Lifeform: %w", row.RowID, err))
		}
		if code != nil && lifeform != nil && *lifeform == form && kind != nil && *kind != "S" && *kind != "s" {
			if references[*code] == math.MaxInt32 {
				return fail(errors.New("physical reference multiplicity exceeds source Long range"))
			}
			references[*code]++
		}
	}
	type plotSum struct {
		sum   *big.Rat
		count int64
	}
	plots := map[string]*plotSum{}
	var joined int64
	for _, row := range quick.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vegetationReportIdentity(row.Cells[qc["Plotnumber"]])
		if err != nil {
			return fail(fmt.Errorf("QuickVeg row %s Plotnumber: %w", row.RowID, err))
		}
		species, err := vegetationReportIdentity(row.Cells[qc["Species"]])
		if err != nil {
			return fail(fmt.Errorf("QuickVeg row %s Species: %w", row.RowID, err))
		}
		number, err := vegetationReportNumber(row.Cells[qc["Cover"]])
		if err != nil {
			return fail(fmt.Errorf("QuickVeg row %s Cover: %w", row.RowID, err))
		}
		if number != nil {
			single, err := lifeformSummarySingle(number)
			if err != nil {
				return fail(fmt.Errorf("QuickVeg row %s Cover: %w", row.RowID, err))
			}
			if new(big.Rat).SetFloat64(single).Cmp(number) != 0 {
				return fail(fmt.Errorf("QuickVeg row %s Cover requires exactly assigned finite SINGLE", row.RowID))
			}
		}
		if plot == nil || species == nil || memberships[*plot] == 0 || references[*species] == 0 {
			continue
		}
		weight := memberships[*plot] * references[*species]
		if weight > math.MaxInt32-joined {
			return fail(errors.New("joined lifeform multiplicity exceeds source Long range"))
		}
		joined += weight
		value := plots[*plot]
		if value == nil {
			value = &plotSum{sum: new(big.Rat)}
			plots[*plot] = value
		}
		if number != nil {
			value.sum.Add(value.sum, new(big.Rat).Mul(number, new(big.Rat).SetInt64(weight)))
			value.count += weight
		}
	}
	var minimum, maximum float64
	var count int64
	total := new(big.Rat)
	for _, value := range plots {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if value.count == 0 {
			continue
		}
		number, _ := value.sum.Float64()
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return fail(errors.New("per-plot lifeform sum is not finite"))
		}
		if count == 0 || number < minimum {
			minimum = number
		}
		if count == 0 || number > maximum {
			maximum = number
		}
		total.Add(total, new(big.Rat).SetFloat64(number))
		count++
	}
	text := ""
	if count != 0 {
		mean, _ := new(big.Rat).Quo(total, new(big.Rat).SetInt64(count)).Float64()
		text = siteUnitFixedNumber(minimum, 1, 1) + " - " + siteUnitFixedNumber(mean, 1, 1) + " - " + siteUnitFixedNumber(maximum, 1, 1)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return ProjectMetadataCell{Storage: "text", Text: &text}, nil
}
