package main

import (
	"context"
	"database/sql"
	"errors"
)

const siteUnitSummaryFeatureEnvironment = "VPRO_SITE_UNIT_SUMMARY"

type SiteUnitSummaryRequest struct {
	Method int `json:"method"`
}

func (request *SiteUnitSummaryRequest) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryRequest
	var value plain
	if err := decodeStrictRequiredJSON(data, &value, "Summary Environment preview", "method"); err != nil {
		return err
	}
	*request = SiteUnitSummaryRequest(value)
	return nil
}

type SiteUnitSummaryPreview struct {
	ContextID   string                `json:"contextId"`
	ProjectPath string                `json:"projectPath"`
	SUPath      string                `json:"suPath"`
	Report      SiteUnitSummaryReport `json:"report"`
}

func (s *ContextService) PreviewSiteUnitSummary(ctx context.Context, contextID string, request SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	if err := ctx.Err(); err != nil {
		return SiteUnitSummaryPreview{}, err
	}
	if !s.siteUnitSummaryEnabled {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment preview is disabled in this session")
	}
	return s.readSiteUnitSummary(ctx, contextID, request, publicationReadSnapshotHooks{})
}

func (s *ContextService) readSiteUnitSummary(ctx context.Context, contextID string, request SiteUnitSummaryRequest,
	hooks publicationReadSnapshotHooks) (SiteUnitSummaryPreview, error) {
	if request.Method != 1 && request.Method != 2 {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (SiteUnitSummaryPreview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *sqliteContext, tx *sql.Tx) (SiteUnitSummaryPreview, error) {
			if owner.selection.SU == "None" {
				return SiteUnitSummaryPreview{}, errors.New("Summary Environment requires an explicitly selected normal SU")
			}
			tables := make([]ProjectMetadataTable, 4)
			for i, source := range []struct{ role, table string }{
				{"project", owner.selection.Project + "_Env"}, {"project", owner.selection.Project + "_Admin"},
				{"su", owner.selection.SU + "_SU"}, {"VLists", "MasterSiteUnitList"},
			} {
				table, err := readPhysicalLocationTable(ctx, tx, source.role, source.table)
				if err != nil {
					return SiteUnitSummaryPreview{}, err
				}
				tables[i] = table
			}
			report, err := planSiteUnitSummary(ctx, owner.selection.Project, owner.selection.SU, request.Method, 1000000,
				tables[0], tables[1], tables[2], tables[3])
			if err != nil {
				return SiteUnitSummaryPreview{}, err
			}
			return SiteUnitSummaryPreview{contextID, owner.selection.ProjectPath, owner.selection.SUPath, report}, nil
		})
	})
}
