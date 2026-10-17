package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

type summaryVegetationGroupField struct {
	Name    string
	DAOType int
	Size    int
}

type summaryVegetationGroupedReport struct {
	GroupField summaryVegetationGroupField
	OrderField summaryVegetationGroupField
	Report     summaryVegetationLayerReport
}

func prepareSummaryVegetationGrouped(ctx context.Context, project, suName string,
	veg, su, master, personal, layers ProjectMetadataTable, average, grouping int) (summaryVegetationGroupedReport, error) {
	return prepareSummaryVegetationGroupedOptions(ctx, project, suName, veg, su, master, personal, layers, average, grouping, 6, 0)
}

func validateSummaryVegetationGroupedOptions(ctx context.Context, average, grouping, display, showClass int) error {
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return err
	}
	if grouping != 12 && grouping != 13 && grouping != 14 {
		return errors.New("summary vegetation grouped source requires GroupAndOrder12,13 or14")
	}
	if display == 2 && grouping != 14 {
		return fmt.Errorf("summary vegetation group%d display2 has original CDbl composite criteria type mismatch; projection unavailable", grouping)
	}
	return nil
}

func prepareSummaryVegetationGroupedOptions(ctx context.Context, project, suName string,
	veg, su, master, personal, layers ProjectMetadataTable, average, grouping, display, showClass int) (summaryVegetationGroupedReport, error) {
	return prepareSummaryVegetationGroupedReferences(ctx, project, suName, veg, su, master, personal, layers,
		average, grouping, display, showClass, nil)
}

func prepareSummaryVegetationGroupedReferences(ctx context.Context, project, suName string,
	veg, su, master, personal, layers ProjectMetadataTable, average, grouping, display, showClass int,
	references *summaryVegetationReferencePlan) (summaryVegetationGroupedReport, error) {
	fail := func(err error) (summaryVegetationGroupedReport, error) { return summaryVegetationGroupedReport{}, err }
	if err := validateSummaryVegetationGroupedOptions(ctx, average, grouping, display, showClass); err != nil {
		return fail(err)
	}
	if references != nil && grouping != 12 {
		return fail(errors.New("summary vegetation effective grouped references require GroupAndOrder12; First(common name) for13/14 is unqualified"))
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, veg, su, layers)
	if err != nil {
		return fail(fmt.Errorf("summary vegetation grouped source preparation: %w", err))
	}
	var union ProjectMetadataTable
	if references != nil {
		union = references.References.Table
	} else {
		legacy, err := prepareVegetationLifeformReferences(ctx, master, personal)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation grouped source references: %w", err))
		}
		union = legacy.Table
	}
	stage, err := prepareSummaryVegetationGroupedEntries(ctx, prepared, union, grouping)
	if err != nil {
		return fail(err)
	}
	report, err := planSummaryVegetationGroupedStageReferences(ctx, veg, su, stage, master, personal, layers,
		average, grouping, display, showClass, references)
	if err != nil {
		return fail(err)
	}
	origins := map[string][]string{}
	for _, row := range prepared.ReducedRows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		key := strconv.Quote(row.PlotNumber) + "/" + vegetationTextKey(row.Species)
		origins[key] = row.SourceRowIDs
	}
	stageOrigins := map[string][]string{}
	for _, row := range stage.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		key := strconv.Quote(*row.Cells[0].Text) + "/" + vegetationTextKey(row.Cells[2])
		stageOrigins[row.RowID] = origins[key]
	}
	for i := range report.Report.Rows {
		for j := range report.Report.Rows[i].Witnesses {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			witness := &report.Report.Rows[i].Witnesses[j]
			witness.RawVegRowIDs = append([]string{}, stageOrigins[witness.StageRowID]...)
			// These builders do not expand through LayerCode conversion rows.
			witness.ConversionLayerID = ""
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return report, nil
}

func planSummaryVegetationGroupedStage(ctx context.Context,
	raw, su, stage, master, personal, layers ProjectMetadataTable, average, grouping int) (summaryVegetationGroupedReport, error) {
	return planSummaryVegetationGroupedStageOptions(ctx, raw, su, stage, master, personal, layers, average, grouping, 6, 0)
}

func planSummaryVegetationGroupedStageOptions(ctx context.Context,
	raw, su, stage, master, personal, layers ProjectMetadataTable, average, grouping, display, showClass int) (summaryVegetationGroupedReport, error) {
	return planSummaryVegetationGroupedStageReferences(ctx, raw, su, stage, master, personal, layers,
		average, grouping, display, showClass, nil)
}

func planSummaryVegetationGroupedStageReferences(ctx context.Context,
	raw, su, stage, master, personal, layers ProjectMetadataTable, average, grouping, display, showClass int,
	references *summaryVegetationReferencePlan) (summaryVegetationGroupedReport, error) {
	fail := func(err error) (summaryVegetationGroupedReport, error) { return summaryVegetationGroupedReport{}, err }
	if err := validateSummaryVegetationGroupedOptions(ctx, average, grouping, display, showClass); err != nil {
		return fail(err)
	}
	if references != nil && grouping != 12 {
		return fail(errors.New("summary vegetation effective grouped references require GroupAndOrder12; First(common name) for13/14 is unqualified"))
	}
	// Class4/5 override the earlier numeric grouped Order in the source.
	projection := summaryVegetationStageProjection{joinField: "Layer1234567", joinLimit: 2, labelField: "Strata", labelLimit: 1,
		numericOrder: display == 6, references: references}
	orderField := summaryVegetationGroupField{Name: "Order", DAOType: 10}
	if display == 6 {
		orderField.DAOType, orderField.Size = 7, 8
	}
	field := summaryVegetationGroupField{Name: "Layer", DAOType: 10, Size: 1}
	switch grouping {
	case 12:
	case 13:
		projection.joinField, projection.labelField = "LayerCode", "LayerCode"
		projection.joinLimit, projection.labelLimit = 3, 3
		field = summaryVegetationGroupField{Name: "LifeForm", DAOType: 10, Size: 3}
	case 14:
		projection.labelField, projection.labelLimit, projection.nullGroup = "Layer", 3, true
		field = summaryVegetationGroupField{Name: "Group", DAOType: 9}
	default:
		return fail(errors.New("summary vegetation grouped stage requires original GroupAndOrder12,13 or14"))
	}
	report, err := planSummaryVegetationStage(ctx, raw, su, stage, master, personal, layers, average, display, showClass, projection)
	if err != nil {
		return fail(err)
	}
	return summaryVegetationGroupedReport{
		GroupField: field, OrderField: orderField, Report: report,
	}, nil
}
