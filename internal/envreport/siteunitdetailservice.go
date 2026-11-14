package envreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
)

type SiteUnitSummaryRequest struct {
	Method int `json:"method"`
}

func (request *SiteUnitSummaryRequest) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryRequest
	var value plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &value, "Summary Environment preview", "method"); err != nil {
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

type SiteUnitSummaryOptions struct {
	ContextID    string `json:"contextId"`
	Project      string `json:"project"`
	ProjectPath  string `json:"projectPath"`
	SU           string `json:"su"`
	SUPath       string `json:"suPath"`
	Method       int    `json:"method"`
	SiteUnitType int    `json:"siteUnitType"`
}

func GetSiteUnitSummaryOptions(s *vprocore.ContextService, ctx context.Context, contextID string) (SiteUnitSummaryOptions, error) {
	if ctx == nil {
		return SiteUnitSummaryOptions{}, errors.New("Summary Environment options require a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSummaryOptions{}, err
	}
	if s == nil {
		return SiteUnitSummaryOptions{}, errors.New("Summary Environment options require owned contexts")
	}
	return ReadSiteUnitSummaryOptions(s, ctx, contextID, vprocore.PublicationReadSnapshotHooks{})
}

func ReadSiteUnitSummaryOptions(s *vprocore.ContextService, ctx context.Context, contextID string,
	hooks vprocore.PublicationReadSnapshotHooks) (SiteUnitSummaryOptions, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (SiteUnitSummaryOptions, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (SiteUnitSummaryOptions, error) {
			if owner.Selection.SU == "None" {
				return SiteUnitSummaryOptions{}, errors.New("Summary Environment requires an explicitly selected normal SU")
			}
			config := plots.Projects.Preferences
			if config == nil {
				return SiteUnitSummaryOptions{}, errors.New("Summary Environment option configuration is unavailable")
			}
			if err := sqlutil.AcquireMutexLease(ctx, &config.Mu); err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			defer config.Mu.Unlock()
			values, err := config.ReadLocked()
			if err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			method, err := vprocore.ConfigInt(values, "ReportOptions", "SEOptValueMethod", 1, 2)
			if err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			unitType, err := vprocore.ConfigInt(values, "ReportOptions", "SESuType", 1, 3)
			if err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			if err := ctx.Err(); err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			return SiteUnitSummaryOptions{contextID, owner.Selection.Project, owner.Selection.ProjectPath,
				owner.Selection.SU, owner.Selection.SUPath, method, unitType}, nil
		})
	})
}

func PreviewSiteUnitSummary(s *vprocore.ContextService, ctx context.Context, contextID string, request SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	if ctx == nil {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSummaryPreview{}, err
	}
	if s == nil {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment preview requires owned contexts")
	}
	return ReadSiteUnitSummary(s, ctx, contextID, request, vprocore.PublicationReadSnapshotHooks{})
}

func ReadSiteUnitSummary(s *vprocore.ContextService, ctx context.Context, contextID string, request SiteUnitSummaryRequest,
	hooks vprocore.PublicationReadSnapshotHooks) (SiteUnitSummaryPreview, error) {
	if request.Method != 1 && request.Method != 2 {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (SiteUnitSummaryPreview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (SiteUnitSummaryPreview, error) {
			input, err := readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, request.Method)
			if err != nil {
				return SiteUnitSummaryPreview{}, err
			}
			return input.Preview, nil
		})
	})
}
