package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

const summaryVegetationFeatureEnvironment = "VPRO_SUMMARY_VEGETATION"

type SummaryVegetationReferenceOrigin struct {
	Table string `json:"table"`
	RowID string `json:"rowId"`
}

type SummaryVegetationWitness struct {
	StageRowID        string                             `json:"stageRowId"`
	ConversionLayerID string                             `json:"conversionLayerId"`
	MembershipID      string                             `json:"membershipId"`
	Unit              string                             `json:"unit"`
	LayerMapID        string                             `json:"layerMapId"`
	ReferenceOrigins  []SummaryVegetationReferenceOrigin `json:"referenceOrigins"`
	RawVegRowIDs      []string                           `json:"rawVegRowIds"`
	Cover             ProjectMetadataCell                `json:"cover"`
}

type SummaryVegetationRow struct {
	Layer      ProjectMetadataCell        `json:"layer"`
	Species    ProjectMetadataCell        `json:"species"`
	CommonName *ProjectMetadataCell       `json:"commonName,omitempty"`
	RawSpecies *ProjectMetadataCell       `json:"rawSpecies,omitempty"`
	Order      ProjectMetadataCell        `json:"order"`
	Covers     []ProjectMetadataCell      `json:"covers"`
	Witnesses  []SummaryVegetationWitness `json:"witnesses"`
}

type SummaryVegetationUnit struct {
	Code          string   `json:"code"`
	NumPlots      int      `json:"numPlots"`
	MembershipIDs []string `json:"membershipIds"`
	RawVegRowIDs  []string `json:"rawVegRowIds"`
}

type SummaryVegetationField struct {
	Name    string `json:"name"`
	DAOType int    `json:"daoType"`
	Size    int    `json:"size"`
}

type SummaryVegetationPreview struct {
	ContextID     string                            `json:"contextId"`
	Project       string                            `json:"project"`
	ProjectPath   string                            `json:"projectPath"`
	SU            string                            `json:"su"`
	SUPath        string                            `json:"suPath"`
	Average       int                               `json:"average"`
	Display       int                               `json:"display"`
	ShowClass     int                               `json:"showClass,omitempty"`
	GroupAndOrder int                               `json:"groupAndOrder,omitempty"`
	GroupField    *SummaryVegetationField           `json:"groupField,omitempty"`
	OrderField    *SummaryVegetationField           `json:"orderField,omitempty"`
	Source        string                            `json:"source"`
	Ordering      string                            `json:"ordering"`
	Memberships   []VegetationReportMembership      `json:"memberships"`
	Units         []SummaryVegetationUnit           `json:"units"`
	Columns       []string                          `json:"columns"`
	Rows          []SummaryVegetationRow            `json:"rows"`
	References    *SummaryVegetationReferenceReview `json:"references,omitempty"`
}

type SummaryVegetationService struct {
	contexts           *ContextService
	enabled            bool
	snapshot           publicationReadSnapshotHooks
	qualityPreferences bool
}

func NewSummaryVegetationService(contexts *ContextService, lookup func(string) (string, bool)) (*SummaryVegetationService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(summaryVegetationFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SummaryVegetationService{contexts: contexts, enabled: enabled}, nil
}

// Explicit read-only draft options do not invoke the mutating Access Form_Load
// or its saved setters, and never borrow Long Vegetation quality/preferences.
func (s *SummaryVegetationService) Preview(ctx context.Context, contextID string, average, display int) (SummaryVegetationPreview, error) {
	return s.preview(ctx, contextID, average, display, 0, 11)
}

func (s *SummaryVegetationService) PreviewClass(ctx context.Context, contextID string, average, display, showClass int) (SummaryVegetationPreview, error) {
	if showClass == 0 {
		return SummaryVegetationPreview{}, errors.New("Summary Vegetation class preview requires explicit ShowClass1,10 or20")
	}
	return s.preview(ctx, contextID, average, display, showClass, 11)
}

func (s *SummaryVegetationService) PreviewGrouped(ctx context.Context, contextID string, average, groupAndOrder int) (SummaryVegetationPreview, error) {
	if groupAndOrder != 12 && groupAndOrder != 13 && groupAndOrder != 14 {
		return SummaryVegetationPreview{}, errors.New("Summary Vegetation grouped preview requires GroupAndOrder12,13 or14")
	}
	return s.preview(ctx, contextID, average, 6, 0, groupAndOrder)
}

func (s *SummaryVegetationService) PreviewGroupedDisplay(ctx context.Context, contextID string, average, groupAndOrder, display, showClass int) (SummaryVegetationPreview, error) {
	if err := validateSummaryVegetationGroupedOptions(ctx, average, groupAndOrder, display, showClass); err != nil {
		return SummaryVegetationPreview{}, err
	}
	return s.preview(ctx, contextID, average, display, showClass, groupAndOrder)
}

func (s *SummaryVegetationService) preview(ctx context.Context, contextID string, average, display, showClass, groupAndOrder int) (SummaryVegetationPreview, error) {
	fail := func(err error) (SummaryVegetationPreview, error) { return SummaryVegetationPreview{}, err }
	if ctx == nil {
		return fail(errors.New("Summary Vegetation requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if s == nil || !s.enabled {
		return fail(errors.New("Summary Vegetation is disabled in this session"))
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return fail(errors.New("Summary Vegetation context service is unavailable"))
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return fail(err)
	}
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return fail(err)
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (SummaryVegetationPreview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (SummaryVegetationPreview, error) {
			source, err := s.readSnapshot(ctx, contextID, plots, owner, tx, average, display, showClass, groupAndOrder)
			if err != nil {
				return fail(err)
			}
			return source.Preview, nil
		})
	})
}

type summaryVegetationSnapshotPreview struct {
	Preview      SummaryVegetationPreview
	SU           ProjectMetadataTable
	Veg          ProjectMetadataTable
	Master       ProjectMetadataTable
	Personal     ProjectMetadataTable
	Layers       ProjectMetadataTable
	Descriptions *ProjectMetadataTable `json:"Descriptions,omitempty"`
}

func (s *SummaryVegetationService) readSnapshot(ctx context.Context, contextID string, plots *PlotService,
	owner *sqliteContext, tx *sql.Tx, average, display, showClass, groupAndOrder int) (summaryVegetationSnapshotPreview, error) {
	return s.readSnapshotWithReferences(ctx, contextID, plots, owner, tx, average, display, showClass, groupAndOrder, nil)
}

func (s *SummaryVegetationService) readSnapshotWithReferences(ctx context.Context, contextID string, plots *PlotService,
	owner *sqliteContext, tx *sql.Tx, average, display, showClass, groupAndOrder int,
	options *SummaryVegetationReferenceOptions) (summaryVegetationSnapshotPreview, error) {
	return s.readSnapshotWithReferenceMode(ctx, contextID, plots, owner, tx,
		average, display, showClass, groupAndOrder, options, summaryVegetationGroupedOrdinary)
}

const summaryVegetationConstancySource = "literal-normal-su-constancy"
const summaryVegetationConstancyOrdering = "literal-null-first-numeric-lifeform-species-raw-code"
const summaryVegetationUnanimousCommonSource = "literal-normal-su-unanimous-common"
const summaryVegetationUnanimousCommonOrdering = "literal-null-first-group-species-unanimous-common"

func (s *SummaryVegetationService) readSnapshotWithReferenceMode(ctx context.Context, contextID string, plots *PlotService,
	owner *sqliteContext, tx *sql.Tx, average, display, showClass, groupAndOrder int,
	options *SummaryVegetationReferenceOptions, mode summaryVegetationGroupedMode) (summaryVegetationSnapshotPreview, error) {
	fail := func(err error) (summaryVegetationSnapshotPreview, error) {
		return summaryVegetationSnapshotPreview{}, err
	}
	if mode != summaryVegetationGroupedOrdinary && mode != summaryVegetationGroupedConstancy &&
		mode != summaryVegetationGroupedUnanimousCommon {
		return fail(errors.New("Summary snapshot requires a supported explicit grouping mode"))
	}
	constancy := mode == summaryVegetationGroupedConstancy
	unanimousCommon := mode == summaryVegetationGroupedUnanimousCommon
	if constancy && (groupAndOrder != 13 || display != 1 || showClass != 0 || options == nil ||
		options.ShowCommon || options.UseGuideNames) {
		return fail(errors.New("Summary Constancy snapshot requires Group13/Display1 and effective references without common/guide names"))
	}
	if unanimousCommon && (showClass != 0 || options == nil ||
		!summaryVegetationUnanimousCommonAvailable(groupAndOrder, display, options.ShowCommon || options.UseGuideNames)) {
		return fail(errors.New("Summary unanimous common-name snapshot requires Group13/14 Display6 and effective common/guide references"))
	}
	if !unanimousCommon && options != nil && !summaryVegetationReferenceGroupingAvailable(groupAndOrder, options.ShowCommon || options.UseGuideNames) {
		return fail(errors.New("Summary effective common/guide references require Layer11 or Strata12 inspection; First(common name) for13/14 remains unavailable"))
	}

	if s.qualityPreferences {
		quality, err := plots.projects.preferences.readSummaryVegetationQualityPreferences(ctx)
		if err != nil {
			return fail(err)
		}
		if quality.Enforce {
			return fail(errors.New("Summary Vegetation quality enforcement is saved but qualified preview is unavailable; no unfiltered report returned"))
		}
	}
	if owner.selection.SU == "None" || owner.selection.SU == "USysSuTableDynamic" {
		return fail(errors.New("Summary Vegetation requires a selected normal SU; hierarchy/dynamic-break scope is unavailable"))
	}
	tables := make([]ProjectMetadataTable, 5)
	for i, source := range []struct{ role, table string }{
		{"project", owner.selection.Project + "_Veg"},
		{"su", owner.selection.SU + "_SU"},
		{"VLists", "USysAllSpecs"},
		{"VUser", "USysUserSpp"},
		{"VPro64", "LayerCode"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return fail(err)
		}
		if count != 1 {
			return fail(fmt.Errorf("Summary Vegetation requires original physical table %s.%s", source.role, source.table))
		}
		var err error
		tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return fail(err)
		}
	}
	var report summaryVegetationLayerReport
	var grouped summaryVegetationGroupedReport
	var err error
	var descriptions *ProjectMetadataTable
	var references *summaryVegetationReferencePlan
	if options != nil {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "VLists".sqlite_master
			WHERE type='table' AND name COLLATE BINARY='USysSppAttributeDescription'`).Scan(&count); err != nil {
			return fail(err)
		}
		if count != 1 {
			return fail(errors.New("Summary effective references require original physical VLists.USysSppAttributeDescription"))
		}
		table, err := readSQLiteStorageRows(ctx, tx, "VLists", "USysSppAttributeDescription", "", nil, "")
		if err != nil {
			return fail(err)
		}
		plan, err := prepareSummaryVegetationReferences(ctx, tables[2], tables[3], table, *options)
		if err != nil {
			return fail(err)
		}
		descriptions, references = &table, &plan
	}
	if groupAndOrder == 11 {
		report, err = prepareSummaryVegetationLayerReferences(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[2], tables[3], tables[4], average, display, showClass, references)
	} else if constancy {
		grouped, err = prepareSummaryVegetationConstancyReferences(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[2], tables[3], tables[4], average, references)
		report = grouped.Report
	} else if unanimousCommon {
		grouped, err = prepareSummaryVegetationUnanimousCommonReferences(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[2], tables[3], tables[4], average, groupAndOrder, references)
		report = grouped.Report
	} else {
		grouped, err = prepareSummaryVegetationGroupedReferences(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[2], tables[3], tables[4], average, groupAndOrder, display, showClass, references)
		report = grouped.Report
	}
	if err != nil {
		return fail(err)
	}
	result := SummaryVegetationPreview{
		ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
		SU: owner.selection.SU, SUPath: owner.selection.SUPath, Average: average, Display: display, ShowClass: showClass,
		Source: "literal-normal-su-layer", Ordering: "literal-null-first-layer-species",
		Memberships: []VegetationReportMembership{}, Units: []SummaryVegetationUnit{},
		Columns: append([]string{}, report.Columns...), Rows: []SummaryVegetationRow{},
	}
	if references != nil {
		result.Source = "literal-normal-su-effective-references"
		result.References = &references.Review
		if references.Review.ShowCommon {
			result.Ordering = "literal-null-first-layer-species-common"
		}
	}
	if groupAndOrder != 11 {
		result.GroupAndOrder = groupAndOrder
		result.GroupField = &SummaryVegetationField{grouped.GroupField.Name, grouped.GroupField.DAOType, grouped.GroupField.Size}
		result.OrderField = &SummaryVegetationField{grouped.OrderField.Name, grouped.OrderField.DAOType, grouped.OrderField.Size}
		result.Source = "literal-normal-su-grouped"
		result.Ordering = "literal-null-first-group-species"
		if references != nil {
			result.Source = "literal-normal-su-effective-references"
			if references.Review.ShowCommon {
				result.Ordering = "literal-null-first-layer-species-common"
			}
		}
	}
	if constancy {
		result.Source, result.Ordering = summaryVegetationConstancySource, summaryVegetationConstancyOrdering
	}
	if unanimousCommon {
		result.Source, result.Ordering = summaryVegetationUnanimousCommonSource, summaryVegetationUnanimousCommonOrdering
	}
	uc, err := siteUnitTransferColumns(tables[1], "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	for _, row := range tables[1].Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		result.Memberships = append(result.Memberships, VegetationReportMembership{
			RowID: row.RowID, PlotNumber: cloneSiteUnitCell(row.Cells[uc["PlotNumber"]]),
			SiteUnit: cloneSiteUnitCell(row.Cells[uc["SiteUnit"]]),
		})
	}
	for _, unit := range report.Units {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		result.Units = append(result.Units, SummaryVegetationUnit{Code: unit.Code, NumPlots: unit.NumPlots,
			MembershipIDs: append([]string{}, unit.MembershipIDs...), RawVegRowIDs: append([]string{}, unit.RawVegRowIDs...)})
	}
	for _, row := range report.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		transport := SummaryVegetationRow{Layer: cloneSiteUnitCell(row.Layer), Species: cloneSiteUnitCell(row.Species),
			Order: cloneSiteUnitCell(row.Order), Covers: []ProjectMetadataCell{}, Witnesses: []SummaryVegetationWitness{}}
		if row.CommonName != nil {
			value := cloneSiteUnitCell(*row.CommonName)
			transport.CommonName = &value
		}
		if row.RawSpecies != nil {
			value := cloneSiteUnitCell(*row.RawSpecies)
			transport.RawSpecies = &value
		}
		for _, cell := range row.Covers {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			transport.Covers = append(transport.Covers, cloneSiteUnitCell(cell))
		}
		for _, witness := range row.Witnesses {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			origins := []SummaryVegetationReferenceOrigin{}
			for _, origin := range witness.ReferenceOrigins {
				origins = append(origins, SummaryVegetationReferenceOrigin{Table: origin.Table, RowID: origin.RowID})
			}
			transport.Witnesses = append(transport.Witnesses, SummaryVegetationWitness{
				StageRowID: witness.StageRowID, ConversionLayerID: witness.ConversionLayerID,
				MembershipID: witness.MembershipID, Unit: witness.Unit, LayerMapID: witness.LayerMapID,
				ReferenceOrigins: origins, RawVegRowIDs: append([]string{}, witness.RawVegRowIDs...),
				Cover: cloneSiteUnitCell(witness.Cover),
			})
		}
		result.Rows = append(result.Rows, transport)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return summaryVegetationSnapshotPreview{Preview: result, SU: tables[1], Veg: tables[0], Master: tables[2],
		Personal: tables[3], Layers: tables[4], Descriptions: descriptions}, nil

}
