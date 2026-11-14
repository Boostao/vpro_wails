package vegreport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

type summaryVegetationEnvironmentalCategory struct {
	label string
	count int64
}

func formatSummaryVegetationEnvironmentalCategories(ctx context.Context, function string, siteUnit *string,
	categories []summaryVegetationEnvironmentalCategory, nullCount int64) (vprocore.ProjectMetadataCell, error) {
	fail := func(err error) (vprocore.ProjectMetadataCell, error) {
		return vprocore.ProjectMetadataCell{}, fmt.Errorf("environmental categorical summary %s: %w", function, err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if _, supported := summaryVegetationEnvironmentalCategoryField(function); !supported {
		return fail(errors.New("function has no calibrated TEXT category renderer"))
	}
	if siteUnit == nil {
		return fail(errors.New("requires a non-NULL Site Unit argument (source error94)"))
	}
	if !utf8.ValidString(*siteUnit) || strings.ContainsRune(*siteUnit, 0) {
		return fail(errors.New("prepared Site Unit contains malformed text"))
	}
	if nullCount < 0 || nullCount > 2147483647 {
		return fail(errors.New("prepared NULL count is outside the nonnegative source Long range"))
	}
	var result strings.Builder
	for _, category := range categories {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if !utf8.ValidString(category.label) || strings.ContainsRune(category.label, 0) {
			return fail(errors.New("prepared category label contains malformed text"))
		}
		if category.count <= 0 || category.count > 2147483647 {
			return fail(errors.New("prepared category count is outside the positive source Long range"))
		}
		label := category.label
		if function == "SVSmr" {
			label = summaryVegetationSMRClassCode(label)
		}
		result.WriteString(label)
		result.WriteByte('(')
		result.WriteString(strconv.FormatInt(category.count, 10))
		result.WriteString(")  ")
	}
	if nullCount > 0 {
		result.WriteString("(Null ")
		result.WriteString(strconv.FormatInt(nullCount, 10))
		result.WriteByte(')')
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	text := result.String()
	return vprocore.ProjectMetadataCell{Storage: "text", Text: &text}, nil
}

func summaryVegetationEnvironmentalCategoryField(function string) (string, bool) {
	switch function {
	case "SVStructureStage":
		return "StructuralStage", true
	case "SVSuccessional":
		return "SuccessionalStatus", true
	case "SVSurfaceTopographyType":
		return "SurfaceTopographyType", true
	case "SVSnr":
		return "NutrientRegime", true
	case "SVMesoSlopePosition":
		return "MesoSlopePosition", true
	case "SVHydroGeoSubSystem":
		return "HydroGeoSubSystem", true
	case "SVHydroGeoSystem":
		return "HydroGeoSystem", true
	case "SVSoilDrainage":
		return "SoilDrainage", true
	case "SVHumusForm":
		return "HumusForm", true
	case "SVRootRestrictingType":
		return "RootRestrictingType", true
	case "SVSoilClassSubGroup":
		return "SoilClassSubGroup", true
	case "SVSoilClassGroup":
		return "SoilClassGroup", true
	case "SVRootZonePartSize":
		return "RootZoneParticleSize", true
	case "SVSurficialMaterialSurf":
		return "SurficialMaterialSurf", true
	case "SVExposure":
		return "Exposure1", true
	case "SVBioGeoClimaticZone":
		return "Zone", true
	case "SVBioGeoClimaticUnit":
		return "Zone", true
	case "SVAspect":
		return "Aspect", true
	case "SVSmr":
		return "MoistureRegime", true
	default:
		return "", false
	}
}

func summaryVegetationEnvironmentalTextField(function string) (string, bool) {
	if function == "SVSmr" || function == "SVBioGeoClimaticUnit" || function == "SVAspect" {
		return "", false
	}
	return summaryVegetationEnvironmentalCategoryField(function)
}

func prepareSummaryVegetationEnvironmentalCategories(ctx context.Context, function string, siteUnit *string,
	env, su vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataCell, error) {
	fail := func(err error) (vprocore.ProjectMetadataCell, error) {
		return vprocore.ProjectMetadataCell{}, fmt.Errorf("environmental categorical %s literal source preparation: %w", function, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if function == "SVBioGeoClimaticUnit" {
		return prepareSummaryVegetationBGCUnit(ctx, siteUnit, env, su)
	}
	if function == "SVAspect" {
		projected, err := projectSummaryVegetationAspect(ctx, env)
		if err != nil {
			return fail(err)
		}
		env = projected
	}
	field, supported := summaryVegetationEnvironmentalCategoryField(function)
	if !supported {
		return fail(errors.New("function has no calibrated TEXT category preparation"))
	}
	if siteUnit == nil {
		return formatSummaryVegetationEnvironmentalCategories(ctx, function, nil, nil, 0)
	}
	if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	categories, nulls, err := prepareSummaryEnvironmentalLiteralCategories(ctx, field, *siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	return formatSummaryVegetationEnvironmentalCategories(ctx, function, siteUnit, categories, nulls)
}

func prepareSummaryEnvironmentalLiteralCategories(ctx context.Context, field, siteUnit string,
	env, su vprocore.ProjectMetadataTable) ([]summaryVegetationEnvironmentalCategory, int64, error) {
	fail := func(err error) ([]summaryVegetationEnvironmentalCategory, int64, error) {
		return nil, 0, err
	}
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	ec, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber", field)
	if err != nil {
		return fail(err)
	}
	memberships := map[string]int64{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vprocore.VegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s PlotNumber: %w", row.RowID, err))
		}
		if unit != nil && *unit == siteUnit && plot != nil {
			memberships[*plot]++
		}
	}
	counts := map[string]int64{}
	var nulls int64
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[ec["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Env row %s PlotNumber: %w", row.RowID, err))
		}
		label, err := vprocore.VegetationReportIdentity(row.Cells[ec[field]])
		if err != nil {
			return fail(fmt.Errorf("Env row %s %s: %w", row.RowID, field, err))
		}
		if plot == nil || memberships[*plot] == 0 {
			continue
		}
		weight := memberships[*plot]
		if label == nil {
			if weight > 2147483647-nulls {
				return fail(errors.New("joined NULL count exceeds source Long range"))
			}
			nulls += weight
		} else {
			if weight > 2147483647-counts[*label] {
				return fail(errors.New("joined category count exceeds source Long range"))
			}
			counts[*label] += weight
		}
	}
	if len(counts) > 32766 {
		return fail(errors.New("original large Integer category-loop boundary is unavailable"))
	}
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	categories := make([]summaryVegetationEnvironmentalCategory, 0, len(labels))
	for _, label := range labels {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		categories = append(categories, summaryVegetationEnvironmentalCategory{label: label, count: counts[label]})
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return categories, nulls, nil
}

func prepareSummaryVegetationEnvironmentalDynamicCategory(ctx context.Context, function string, argument int,
	siteUnit *string, env, su vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataCell, error) {
	fail := func(err error) (vprocore.ProjectMetadataCell, error) {
		return vprocore.ProjectMetadataCell{}, fmt.Errorf("environmental dynamic category %s argument%d: %w", function, argument, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	var field string
	switch function {
	case "SVBedrockType":
		if argument < 1 || argument > 3 {
			return fail(errors.New("only original physical bedrock layers1/2/3 are calibrated"))
		}
		field = "BedrockGeology" + strconv.Itoa(argument)
	case "SVSiteDisturbance":
		if argument < 1 || argument > 2 {
			return fail(errors.New("only disturbance fields1/2 are calibrated; field3 active source loop reads unprojected field2"))
		}
		field = "SiteDisturbance" + strconv.Itoa(argument)
	default:
		return fail(errors.New("function has no calibrated dynamic TEXT category primitive"))
	}
	if siteUnit == nil {
		return fail(errors.New("original String argument rejects NULL (94)"))
	}
	if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	categories, nulls, err := prepareSummaryEnvironmentalLiteralCategories(ctx, field, *siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	var text strings.Builder
	for _, category := range categories {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		text.WriteString(category.label)
		text.WriteString("(" + strconv.FormatInt(category.count, 10) + ")  ")
	}
	if nulls != 0 {
		text.WriteString("(Null " + strconv.FormatInt(nulls, 10) + ")")
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	value := text.String()
	return vprocore.ProjectMetadataCell{Storage: "text", Text: &value}, nil
}

func summaryEnvironmentalSelectedFunction(function string, argument vprocore.ProjectMetadataCell) (bool, int, error) {
	if function == "SVBedrockType" || function == "SVSiteDisturbance" {
		text, err := vprocore.VegetationReportIdentity(argument)
		if err != nil || text == nil || (*text != "1" && *text != "2" && *text != "3") ||
			function == "SVSiteDisturbance" && *text == "3" {
			return false, 0, fmt.Errorf("function %q requires calibrated literal TEXT FieldNumber: bedrock1/2/3 or disturbance1/2; disturbance3 unavailable", function)
		}
		number, err := strconv.Atoi(*text)
		if err != nil {
			return false, 0, err
		}
		return true, number, nil
	}
	if function != "SVnPlots" {
		if _, supported := summaryVegetationEnvironmentalCategoryField(function); !supported {
			return false, 0, fmt.Errorf("function %q is unavailable for environmental consumption", function)
		}
	}
	if argument.Storage != "null" {
		return false, 0, fmt.Errorf("function %q has an unavailable FieldNumber argument", function)
	}
	if _, err := vprocore.MetadataCellValue(argument); err != nil {
		return false, 0, errors.New("FieldNumber NULL carries contradictory payload")
	}
	return false, 0, nil
}

type SummaryVegetationEnvironmentalRow struct {
	Field       vprocore.ProjectMetadataRow    `json:"field"`
	LabelColumn int                            `json:"labelColumn"`
	Values      []vprocore.ProjectMetadataCell `json:"values"`
}

func planSummaryVegetationEnvironmentalRows(ctx context.Context, fieldList vprocore.ProjectMetadataTable,
	units []summaryVegetationUnit, env, su vprocore.ProjectMetadataTable) ([]SummaryVegetationEnvironmentalRow, error) {
	return planSummaryEnvironmentalSelectedRows(ctx, fieldList, units, env, su,
		func(function string, argument vprocore.ProjectMetadataCell) error {
			_, _, err := summaryEnvironmentalSelectedFunction(function, argument)
			return err
		},
		func(function string, argument vprocore.ProjectMetadataCell, unit string) (vprocore.ProjectMetadataCell, error) {
			return prepareSummaryEnvironmentalSelectedCell(ctx, function, argument, &unit, env, su)
		},
		func(field vprocore.ProjectMetadataRow, label int, values []vprocore.ProjectMetadataCell) SummaryVegetationEnvironmentalRow {
			return SummaryVegetationEnvironmentalRow{Field: field, LabelColumn: label, Values: values}
		})
}

func prepareSummaryEnvironmentalSelectedCell(ctx context.Context, function string, argument vprocore.ProjectMetadataCell,
	unit *string, env, su vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataCell, error) {
	dynamic, number, err := summaryEnvironmentalSelectedFunction(function, argument)
	if err != nil {
		return vprocore.ProjectMetadataCell{}, err
	}
	if function == "SVnPlots" {
		return prepareSummaryVegetationEnvironmentalPlotCount(ctx, unit, su)
	}
	if dynamic {
		return prepareSummaryVegetationEnvironmentalDynamicCategory(ctx, function, number, unit, env, su)
	}
	return prepareSummaryVegetationEnvironmentalCategories(ctx, function, unit, env, su)
}

func planSummaryEnvironmentalSelectedRows[V, R any](ctx context.Context, fieldList vprocore.ProjectMetadataTable,
	units []summaryVegetationUnit, env, su vprocore.ProjectMetadataTable,
	validate func(string, vprocore.ProjectMetadataCell) error,
	prepare func(string, vprocore.ProjectMetadataCell, string) (V, error),
	build func(vprocore.ProjectMetadataRow, int, []V) R) ([]R, error) {
	fail := func(err error) ([]R, error) {
		return nil, fmt.Errorf("Summary environmental selected-field matrix: %w", err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(units) == 0 || len(units) > 250 {
		return fail(errors.New("requires an available nonempty prepared SV1 unit frame within source250-group limit"))
	}
	seen := map[string]bool{}
	for _, unit := range units {
		if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: &unit.Code}); err != nil {
			return fail(err)
		}
		if seen[unit.Code] {
			return fail(errors.New("prepared SV1 unit frame has duplicate literal unit identities"))
		}
		seen[unit.Code] = true
	}
	if err := validateSummaryVegetationFieldListWitness(ctx, fieldList); err != nil {
		return fail(err)
	}
	index, err := indexSummaryVegetationFieldList(ctx, fieldList)
	if err != nil {
		return fail(err)
	}
	rows := make(map[string]vprocore.ProjectMetadataRow, len(fieldList.Rows))
	for _, row := range fieldList.Rows {
		rows[row.RowID] = row
	}
	result := make([]R, 0, len(index.Selected))
	for _, id := range index.Selected {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		row := rows[id]
		function, err := vprocore.VegetationReportIdentity(row.Cells[5])
		if err != nil || function == nil {
			return fail(fmt.Errorf("field-list row %s requires an original TEXT FunctionName", id))
		}
		if err := validate(*function, row.Cells[6]); err != nil {
			return fail(fmt.Errorf("field-list row %s: %w", id, err))
		}
		for _, position := range []int{1, 2} {
			if _, err := vprocore.VegetationReportIdentity(row.Cells[position]); err != nil {
				return fail(fmt.Errorf("field-list row %s caption: %w", id, err))
			}
		}
		label := 2
		values := make([]V, 0, len(units))
		if row.Cells[2].Text != nil && *row.Cells[2].Text == "n Plots" {
			label = 3
		}
		for _, unit := range units {
			value, err := prepare(*function, row.Cells[6], unit.Code)
			if err != nil {
				return fail(fmt.Errorf("field-list row %s unit %q: %w", id, unit.Code, err))
			}
			values = append(values, value)
		}
		result = append(result, build(cloneSummaryVegetationFieldListRow(row), label, values))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
