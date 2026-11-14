package vegreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type SummaryVegetationWorkbookAncillaryOptions struct {
	CreateSummary bool `json:"createSummary"`
}

func (options *SummaryVegetationReferenceOptions) UnmarshalJSON(data []byte) error {
	var raw struct {
		ShowCommon    *bool `json:"showCommon"`
		UseGuideNames *bool `json:"useGuideNames"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(data, &raw, "Summary effective reference options", "showCommon", "useGuideNames"); err != nil {
		return err
	}
	if raw.ShowCommon == nil || raw.UseGuideNames == nil {
		return errors.New("Summary effective references require explicit non-NULL common/guide selections")
	}
	*options = SummaryVegetationReferenceOptions{ShowCommon: *raw.ShowCommon, UseGuideNames: *raw.UseGuideNames}
	return nil
}

func (options *SummaryVegetationWorkbookAncillaryOptions) UnmarshalJSON(data []byte) error {
	var raw struct {
		CreateSummary *bool `json:"createSummary"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(data, &raw, "Summary ancillary workbook options", "createSummary"); err != nil {
		return err
	}
	if raw.CreateSummary == nil {
		return errors.New("Summary ancillary workbook requires an explicit non-NULL summary selection")
	}
	*options = SummaryVegetationWorkbookAncillaryOptions{CreateSummary: *raw.CreateSummary}
	return nil
}

type SummaryVegetationWorkbookOptions struct {
	Title           string                                     `json:"title"`
	Average         int                                        `json:"average"`
	Display         int                                        `json:"display"`
	ShowClass       int                                        `json:"showClass"`
	IncludeEnv      bool                                       `json:"includeEnv"`
	Ancillary       *SummaryVegetationWorkbookAncillaryOptions `json:"ancillary,omitempty"`
	References      *SummaryVegetationReferenceOptions         `json:"references,omitempty"`
	GroupAndOrder   *int                                       `json:"groupAndOrder,omitempty"`
	Constancy       bool                                       `json:"constancy,omitempty"`
	UnanimousCommon bool                                       `json:"unanimousCommon,omitempty"`
}

func (options *SummaryVegetationWorkbookOptions) UnmarshalJSON(data []byte) error {
	var raw struct {
		Title           *string                                    `json:"title"`
		Average         *int                                       `json:"average"`
		Display         *int                                       `json:"display"`
		ShowClass       *int                                       `json:"showClass"`
		IncludeEnv      *bool                                      `json:"includeEnv"`
		Ancillary       *SummaryVegetationWorkbookAncillaryOptions `json:"ancillary"`
		References      *SummaryVegetationReferenceOptions         `json:"references"`
		GroupAndOrder   *int                                       `json:"groupAndOrder"`
		Constancy       *bool                                      `json:"constancy"`
		UnanimousCommon *bool                                      `json:"unanimousCommon"`
	}
	properties, err := sourceform.SourceChildJSONObject(data, "Summary draft workbook options",
		[]string{"title", "average", "display", "showClass", "includeEnv"}, []string{"ancillary", "references", "groupAndOrder", "constancy", "unanimousCommon"})
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, "Summary draft workbook options", "ancillary", "references", "groupAndOrder", "constancy", "unanimousCommon"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Title == nil || raw.Average == nil || raw.Display == nil || raw.ShowClass == nil || raw.IncludeEnv == nil {
		return errors.New("Summary draft workbook requires explicit title, calculation and environmental options; NULL is not a value")
	}
	if raw.Constancy != nil && !*raw.Constancy {
		return errors.New("Summary Constancy selection must be true when present; omit it for ordinary inspection")
	}
	if raw.UnanimousCommon != nil && !*raw.UnanimousCommon {
		return errors.New("Summary unanimous common-name selection must be true when present; omit it for ordinary inspection")
	}
	parsed := SummaryVegetationWorkbookOptions{Title: *raw.Title, Average: *raw.Average, Display: *raw.Display,
		ShowClass: *raw.ShowClass, IncludeEnv: *raw.IncludeEnv, Ancillary: raw.Ancillary, References: raw.References,
		GroupAndOrder: raw.GroupAndOrder, Constancy: raw.Constancy != nil, UnanimousCommon: raw.UnanimousCommon != nil}
	if err := validateSummaryWorkbookInspectionOptions(parsed); err != nil {
		return err
	}
	*options = parsed
	return nil
}

func validateSummaryWorkbookInspectionOptions(options SummaryVegetationWorkbookOptions) error {
	if options.Constancy && options.UnanimousCommon {
		return errors.New("Summary Constancy and unanimous common-name workbook selections are mutually exclusive")
	}
	if options.Constancy && (options.GroupAndOrder == nil || *options.GroupAndOrder != 13 ||
		options.Display != 1 || options.References == nil || options.References.ShowCommon || options.References.UseGuideNames) {
		return errors.New("Summary Constancy workbook requires explicit Group13/Display1 and effective references without common/guide names")
	}
	if options.UnanimousCommon && (options.GroupAndOrder == nil || options.References == nil ||
		!summaryVegetationUnanimousCommonAvailable(*options.GroupAndOrder, options.Display,
			options.References.ShowCommon || options.References.UseGuideNames)) {
		return errors.New("Summary unanimous common-name workbook requires explicit Group13/14 Display6 and common/guide references")
	}
	return nil
}

type summaryVegetationWorkbookExportRequest struct {
	Options      SummaryVegetationWorkbookOptions `json:"options"`
	ApprovalHash string                           `json:"approvalHash"`
	Destination  string                           `json:"destination"`
	CreatedAt    string                           `json:"createdAt,omitempty"`
}

func (request *summaryVegetationWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Options      *SummaryVegetationWorkbookOptions `json:"options"`
		ApprovalHash *string                           `json:"approvalHash"`
		Destination  *string                           `json:"destination"`
		CreatedAt    *string                           `json:"createdAt"`
	}
	properties, err := sourceform.SourceChildJSONObject(data, "Summary draft workbook publication",
		[]string{"options", "approvalHash", "destination"}, []string{"createdAt"})
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, "Summary draft workbook publication", "createdAt"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Options == nil || raw.ApprovalHash == nil || raw.Destination == nil {
		return errors.New("Summary draft workbook publication requires explicit options, approval and destination")
	}
	createdAt := ""
	if raw.Options.Ancillary != nil {
		if raw.CreatedAt == nil {
			return errors.New("Summary ancillary workbook publication requires its reviewed creation time")
		}
		if err := validateSummaryWorkbookCreatedAt(*raw.CreatedAt); err != nil {
			return err
		}
		createdAt = *raw.CreatedAt
	} else if raw.CreatedAt != nil {
		return errors.New("Summary draft workbook creation time requires explicit ancillary mode")
	}
	*request = summaryVegetationWorkbookExportRequest{Options: *raw.Options, ApprovalHash: *raw.ApprovalHash,
		Destination: *raw.Destination, CreatedAt: createdAt}
	return nil
}

type summaryVegetationWorkbookOptions = SummaryVegetationWorkbookOptions

type summaryVegetationWorkbookSourcePath struct {
	Role string
	Path string
}

type summaryVegetationWorkbookSource struct {
	Input       summaryVegetationWorkbookInput
	Raw         summaryVegetationSnapshotPreview
	Environment summaryEnvironmentalPhysicalInput
	Paths       []summaryVegetationWorkbookSourcePath
	Ancillary   *summaryVegetationWorkbookAncillarySource `json:"Ancillary,omitempty"`
}

type summaryVegetationWorkbookAncillarySource struct {
	CreatedAt string
	Tables    summaryVegetationAncillaryTables
}

func readSummaryVegetationWorkbookSource(ctx context.Context, service *SummaryVegetationEnvironmentalLifeformService,
	plots *vprocore.PlotService, owner *vprocore.SqliteContext, tx *sql.Tx, contextID string,
	options summaryVegetationWorkbookOptions) (summaryVegetationWorkbookSource, error) {
	fail := func(err error) (summaryVegetationWorkbookSource, error) {
		return summaryVegetationWorkbookSource{}, err
	}
	if ctx == nil {
		return fail(errors.New("Summary workbook source requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := validateSummaryWorkbookInspectionOptions(options); err != nil {
		return fail(err)
	}
	if service == nil || service.environment == nil || service.environment.report == nil {
		return fail(errors.New("Summary workbook source requires Summary Vegetation"))
	}
	environment := service.environment
	if plots == nil || plots.Projects == nil || owner == nil || plots.Projects.Sqlite != owner || tx == nil {
		return fail(errors.New("Summary workbook source requires the owned plot context and read transaction"))
	}
	contexts := environment.report.contexts
	if contexts == nil || contexts.Projects == nil || contexts.Projects.Sqlite != owner ||
		!plots.ContextScoped || contextID == "" || plots.Projects.ContextID != contextID {
		return fail(errors.New("Summary workbook source requires its current scoped report context"))
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, options.Title); err != nil {
		return fail(err)
	}
	if options.ShowClass != 0 && options.ShowClass != 1 && options.ShowClass != 10 && options.ShowClass != 20 {
		return fail(errors.New("Summary workbook requires supported retained ShowClass0,1,10 or20"))
	}
	calculationClass := options.ShowClass
	if options.Display != 4 && options.Display != 5 {
		// These source displays retain Show but do not consume it; keep the legacy calculator policy unchanged.
		calculationClass = 0
	}
	if err := validateSummaryVegetationDisplayOptions(ctx, options.Average, options.Display, calculationClass); err != nil {
		return fail(err)
	}
	grouping := 11
	if options.GroupAndOrder != nil {
		if !options.UnanimousCommon && options.References != nil && !summaryVegetationReferenceGroupingAvailable(
			*options.GroupAndOrder, options.References.ShowCommon || options.References.UseGuideNames) {
			return fail(errors.New("Summary Lifeform/None common/guide names are not qualified; attribute references without names remain supported"))
		}
		grouping = *options.GroupAndOrder
		if err := validateSummaryVegetationGroupedOptions(ctx, options.Average, grouping, options.Display, calculationClass); err != nil {
			return fail(err)
		}
	}
	mode := summaryVegetationGroupedOrdinary
	if options.Constancy {
		mode = summaryVegetationGroupedConstancy
	} else if options.UnanimousCommon {
		mode = summaryVegetationGroupedUnanimousCommon
	}
	raw, err := environment.report.readSnapshotWithReferenceMode(ctx, contextID, plots, owner, tx,
		options.Average, options.Display, calculationClass, grouping, options.References, mode)
	if err != nil {
		return fail(err)
	}
	result := summaryVegetationWorkbookSource{Raw: raw,
		Input: summaryVegetationWorkbookInput{Title: options.Title, Vegetation: raw.Preview},
		Paths: []summaryVegetationWorkbookSourcePath{}}
	for _, role := range []string{"project", "su", "VLists", "VUser", "VPro64"} {
		path, ok := owner.Attachments[role]
		if !ok || path == "" {
			return fail(errors.New("Summary workbook source is missing a required owned attachment path"))
		}
		result.Paths = append(result.Paths, summaryVegetationWorkbookSourcePath{role, path})
	}
	if options.IncludeEnv {
		result.Environment, err = readSummaryEnvironmentalTables(ctx, owner, tx, validateSummaryEnvironmentalLifeformSelectedFunction)
		if err != nil {
			return fail(err)
		}
		rows, err := planSummaryEnvironmentalSnapshotRows(ctx, raw, result.Environment.FieldList, result.Environment.Env)
		if err != nil {
			return fail(err)
		}
		result.Input.Environment = &summaryEnvironmentalWorkbookInput{
			FirstRow: 5, FieldList: result.Environment.FieldList,
			Units: summaryEnvironmentalPreviewUnits(raw.Preview), Rows: rows,
			English: raw.Preview.References != nil && raw.Preview.References.ShowCommon,
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
