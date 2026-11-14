package vegreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

type LongVegetationProjectFieldDefinitions struct {
	ContextID   string                            `json:"contextId"`
	Project     string                            `json:"project"`
	ProjectPath string                            `json:"projectPath"`
	SU          string                            `json:"su"`
	SUPath      string                            `json:"suPath"`
	SourceKind  string                            `json:"sourceKind"`
	Revision    string                            `json:"revision"`
	Definitions []LongVegetationSelectedUnitField `json:"definitions"`
}

func (s *LongVegetationReportScopeService) GetLongVegetationProjectFieldDefinitions(ctx context.Context,
	contextID string) (*LongVegetationProjectFieldDefinitions, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationProjectFieldDefinitions, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationProjectFieldDefinitions, error) {
			var tables [2]vprocore.ProjectMetadataTable
			for i, suffix := range []string{"_Env", "_Admin"} {
				name := owner.Selection.Project + suffix
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master
					WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
					return nil, err
				}
				if count != 1 {
					return nil, fmt.Errorf("project report field catalogue requires original physical table project.%s", name)
				}
				columns, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, "project", name)
				if err != nil {
					return nil, err
				}
				tables[i] = vprocore.ProjectMetadataTable{Columns: columns}
			}
			entry := s.contexts.ProjectReportScopes.Read(contextID)
			return &LongVegetationProjectFieldDefinitions{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, SourceKind: entry.SourceKind,
				Revision: strconv.FormatInt(entry.Revision, 10), Definitions: reportUnitFieldDefinitions(tables[0], tables[1]),
			}, nil
		})
	})
}

// QCLV still owns original selected-SU witnesses. Only qualified plot existence
// prunes a complete project source2 Build; its fanout never weights that Build.
func planLongVegetationQualifiedProjectSelectedUnits(ctx context.Context,
	env, admin, su vprocore.ProjectMetadataTable, names []string,
	quality vegetationQualitySelection) ([]vegetationProjectSelectedUnitOccurrence, error) {
	projectRows, err := planLongVegetationProjectSelectedUnits(ctx, env, admin, names)
	if err != nil {
		return nil, err
	}
	witnesses, err := planLongVegetationSelectedUnits(ctx, env, admin, su, names, &quality)
	if err != nil {
		return nil, err
	}
	qualified := map[string]bool{}
	for _, row := range witnesses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if row.PlotNumber.Text != nil {
			qualified[*row.PlotNumber.Text] = true
		}
	}
	result := []vegetationProjectSelectedUnitOccurrence{}
	for _, row := range projectRows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if qualified[*row.PlotNumber.Text] {
			result = append(result, row)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

type vegetationProjectPreparation struct {
	Project       string
	SourceSU      string
	Entries       vegetationReportEntries
	SelectedUnits []vegetationProjectSelectedUnitOccurrence
	NoneUnits     []vegetationProjectNoneUnitOccurrence
}

func prepareLongVegetationProjectSelectedUnits(ctx context.Context, project, sourceSU string,
	veg, layers, env, admin, su vprocore.ProjectMetadataTable, names []string,
	quality *vegetationQualitySelection) (vegetationProjectPreparation, error) {
	if ctx == nil {
		return vegetationProjectPreparation{}, errors.New("project vegetation preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return vegetationProjectPreparation{}, err
	}
	if sourceSU == "" || !utf8.ValidString(sourceSU) || strings.ContainsRune(sourceSU, 0) {
		return vegetationProjectPreparation{}, errors.New("project MAX preparation requires an explicit original SU selection or None")
	}
	if sourceSU == "None" {
		if quality != nil {
			return vegetationProjectPreparation{}, errors.New("project quality requires an actual original selected SU; no Env-only qualification was inferred")
		}
		if len(su.Columns) != 0 || len(su.Rows) != 0 {
			return vegetationProjectPreparation{}, errors.New("project None selection cannot carry an invented physical SU")
		}
	} else {
		if _, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit"); err != nil {
			return vegetationProjectPreparation{}, fmt.Errorf("project MAX preparation original selected SU: %w", err)
		}
	}
	var rows []vegetationProjectSelectedUnitOccurrence
	var err error
	if quality == nil {
		rows, err = planLongVegetationProjectSelectedUnits(ctx, env, admin, names)
	} else {
		rows, err = planLongVegetationQualifiedProjectSelectedUnits(ctx, env, admin, su, names, *quality)
	}
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	scope := map[string]bool{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return vegetationProjectPreparation{}, err
		}
		scope[*row.PlotNumber.Text] = true
	}
	entries, err := prepareLongVegetationProjectEntries(ctx, project, veg, layers, scope)
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	return vegetationProjectPreparation{Project: project, SourceSU: sourceSU, Entries: entries, SelectedUnits: rows}, nil
}

func prepareLongVegetationProjectNoneUnits(ctx context.Context, project string,
	veg, layers, env vprocore.ProjectMetadataTable) (vegetationProjectPreparation, error) {
	rows, err := planLongVegetationProjectNoneUnits(ctx, env)
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	scope := map[string]bool{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return vegetationProjectPreparation{}, err
		}
		if row.PlotNumber.Text != nil {
			scope[*row.PlotNumber.Text] = true
		}
	}
	entries, err := prepareLongVegetationProjectEntries(ctx, project, veg, layers, scope)
	if err != nil {
		return vegetationProjectPreparation{}, err
	}
	return vegetationProjectPreparation{Project: project, SourceSU: "None", Entries: entries, NoneUnits: rows}, nil
}

func prepareLongVegetationProjectEntries(ctx context.Context, project string,
	veg, layers vprocore.ProjectMetadataTable, scope map[string]bool) (vegetationReportEntries, error) {
	if err := ctx.Err(); err != nil {
		return vegetationReportEntries{}, err
	}
	if project == "" || !utf8.ValidString(project) || strings.ContainsRune(project, 0) {
		return vegetationReportEntries{}, errors.New("project vegetation preparation requires an explicit valid project identity")
	}
	vegColumns, err := vprocore.SiteUnitTransferColumns(veg, append([]string{"PlotNumber", "Species"}, longVegetationCoverColumns()...)...)
	if err != nil {
		return vegetationReportEntries{}, fmt.Errorf("Long Vegetation physical Veg schema: %w", err)
	}
	layerColumns, err := vprocore.SiteUnitTransferColumns(layers, "LayerText")
	if err != nil {
		return vegetationReportEntries{}, fmt.Errorf("Long Vegetation LayerCode schema: %w", err)
	}
	return reduceLongVegetationEntries(ctx, veg, layers, vegColumns, layerColumns, scope)
}

// A nil field list means the original Env-only source3, not an empty source2 draft.
func readLongVegetationProjectSource(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx,
	fields []string, options longVegetationOptions) (longVegetationReportSource, error) {
	if ctx == nil || owner == nil || tx == nil {
		return longVegetationReportSource{}, errors.New("project report source requires an owned physical read snapshot")
	}
	if err := ctx.Err(); err != nil {
		return longVegetationReportSource{}, err
	}
	if options.NoneUnits || options.SelectedUnitFields != nil {
		return longVegetationReportSource{}, errors.New("project report source cannot combine selected-list unit options")
	}
	if fields == nil && owner.Selection.SU != "None" {
		return longVegetationReportSource{}, errors.New("project Env-only source3 requires the explicit original None SU context")
	}
	if options.Quality != nil && (fields == nil || owner.Selection.SU == "None") {
		return longVegetationReportSource{}, errors.New("project quality requires actual original selected-SU authority")
	}
	sources := []struct{ role, table string }{
		{"project", owner.Selection.Project + "_Veg"},
		{"project", owner.Selection.Project + "_Env"},
		{"VLists", "USysAllSpecs"},
		{"VPro64", "LayerCode"},
		{"VLists", "MasterSiteUnitList"},
	}
	adminIndex, suIndex, qualityIndex, personalIndex := -1, -1, -1, -1
	if fields != nil {
		adminIndex = len(sources)
		sources = append(sources, struct{ role, table string }{"project", owner.Selection.Project + "_Admin"})
		if owner.Selection.SU != "None" {
			suIndex = len(sources)
			sources = append(sources, struct{ role, table string }{"su", owner.Selection.SU + "_SU"})
		}
	}
	if options.Quality != nil {
		qualityIndex = len(sources)
		sources = append(sources, struct{ role, table string }{"VLists", "USysTableOfLists"})
	}
	if options.LifeformGrouping {
		personalIndex = len(sources)
		sources = append(sources, struct{ role, table string }{"VUser", "USysUserSpp"})
	}
	tables := make([]vprocore.ProjectMetadataTable, len(sources))
	for i, source := range sources {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return longVegetationReportSource{}, err
		}
		if count != 1 {
			return longVegetationReportSource{}, fmt.Errorf("project report requires original physical table %s.%s", source.role, source.table)
		}
		var err error
		tables[i], err = vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return longVegetationReportSource{}, err
		}
	}
	var project vegetationProjectPreparation
	var quality *vegetationQualitySelection
	var err error
	if fields == nil {
		project, err = prepareLongVegetationProjectNoneUnits(ctx, owner.Selection.Project, tables[0], tables[3], tables[1])
	} else {
		var su vprocore.ProjectMetadataTable
		if suIndex >= 0 {
			su = tables[suIndex]
		}
		if options.Quality != nil {
			qualified, qualityErr := planLongVegetationQuality(ctx, su, tables[1], tables[adminIndex], tables[qualityIndex], *options.Quality)
			if qualityErr != nil {
				return longVegetationReportSource{}, qualityErr
			}
			quality = &qualified
		}
		project, err = prepareLongVegetationProjectSelectedUnits(ctx, owner.Selection.Project, owner.Selection.SU,
			tables[0], tables[3], tables[1], tables[adminIndex], su, fields, quality)
	}
	if err != nil {
		return longVegetationReportSource{}, err
	}
	if quality != nil && len(quality.Occurrences) == 0 {
		return longVegetationReportSource{}, errors.New("Sorry, no plots. Please adjust your plot quality standards or use a different site unit table")
	}
	prepared, err := projectReportPreparation(ctx, project)
	if err != nil {
		return longVegetationReportSource{}, err
	}
	if len(prepared.projectUnitInputs) == 0 {
		return longVegetationReportSource{}, errors.New("project report has no original derived unit tuples; empty-unit output is unavailable")
	}
	var personal vprocore.ProjectMetadataTable
	if personalIndex >= 0 {
		personal = tables[personalIndex]
	}
	report, err := planLongVegetationPreparedSource(ctx, prepared, tables[2], personal, tables[3], options)
	if err != nil {
		return longVegetationReportSource{}, err
	}
	if quality != nil {
		report.Quality = quality
		if err := addLongVegetationQualityMultiplicity(ctx, &report, *quality); err != nil {
			return longVegetationReportSource{}, err
		}
	}
	if err := addLongVegetationUnitNames(ctx, &report, tables[4]); err != nil {
		return longVegetationReportSource{}, err
	}
	return longVegetationReportSource{report, tables}, nil
}
