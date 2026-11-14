package vprocore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type ReportPreferenceHelper struct {
	Contexts   *ContextService              `json:"-" yaml:"-"`
	Snapshot   PublicationReadSnapshotHooks `json:"-" yaml:"-"`
	CheckScope func(contextID string) error `json:"-" yaml:"-"`
}

type ReportPreferenceOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

func RequireReportPreferenceContext(ctx context.Context, h ReportPreferenceHelper, contextID, label string) error {
	if ctx == nil {
		return fmt.Errorf("%s preferences require a context", label)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if h.Contexts == nil || h.Contexts.Projects == nil || h.Contexts.Projects.Sqlite == nil || h.Contexts.Plots == nil {
		return fmt.Errorf("%s preference context service is unavailable", label)
	}
	if err := ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return err
	}
	if h.CheckScope != nil {
		return h.CheckScope(contextID)
	}
	return nil
}

func WithReportPreferencePlot[T any](
	ctx context.Context,
	h ReportPreferenceHelper,
	contextID, label string,
	fn func(plots *PlotService) (T, error),
) (T, error) {
	if err := RequireReportPreferenceContext(ctx, h, contextID, label); err != nil {
		var zero T
		return zero, err
	}
	return WithContextPlotRequest(ctx, h.Contexts, contextID, fn)
}

func GetOwnedReportPreferences[Review any](
	ctx context.Context,
	h ReportPreferenceHelper,
	contextID, label string,
	read func(plots *PlotService, owner *SqliteContext, tx *sql.Tx) (*Review, error),
) (*Review, error) {
	return WithReportPreferencePlot(ctx, h, contextID, label, func(plots *PlotService) (*Review, error) {
		return WithOwnedPreferenceSnapshot(ctx, plots, h.Snapshot, func(owner *SqliteContext, tx *sql.Tx) (*Review, error) {
			return read(plots, owner, tx)
		})
	})
}

// RunReportPreferenceSave preserves Changed/Committed receipts even when post-commit
// cleanup or context cancellation returns an error.
func RunReportPreferenceSave[Request any](
	ctx context.Context,
	h ReportPreferenceHelper,
	contextID, requestJSON, label string,
	execute func(plots *PlotService, request Request) (ReportPreferenceUpdate, error),
) ReportPreferenceOutcome {
	outcome := ReportPreferenceOutcome{ContextID: contextID}
	err := RequireReportPreferenceContext(ctx, h, contextID, label)
	var request Request
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = WithContextPlotRequest(ctx, h.Contexts, contextID, func(plots *PlotService) (struct{}, error) {
			update, saveErr := execute(plots, request)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}

func SaveOwnedReportPreferences[Request any](
	ctx context.Context,
	h ReportPreferenceHelper,
	contextID, requestJSON, label string,
	validateSnapshot func(owner *SqliteContext, tx *sql.Tx, request Request) error,
	apply func(plots *PlotService, request Request) (ReportPreferenceUpdate, error),
) ReportPreferenceOutcome {
	return RunReportPreferenceSave(ctx, h, contextID, requestJSON, label, func(plots *PlotService, request Request) (ReportPreferenceUpdate, error) {
		_, readErr := WithOwnedPreferenceSnapshot(ctx, plots, h.Snapshot, func(owner *SqliteContext, tx *sql.Tx) (struct{}, error) {
			if validateSnapshot != nil {
				return struct{}{}, validateSnapshot(owner, tx, request)
			}
			return struct{}{}, nil
		})
		if readErr != nil {
			return ReportPreferenceUpdate{}, readErr
		}
		return apply(plots, request)
	})
}
