package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
)

type ProjectPlotProfileFilterRequest struct {
	Input   ProjectPlotProfileRunRequest `json:"input"`
	Preview ProjectPlotProfileResult     `json:"preview"`
}

type ProjectPlotProfileNavigation struct {
	ContextID string                   `json:"contextId"`
	Result    ProjectPlotProfileResult `json:"result"`
	Plots     []PlotSummary            `json:"plots"`
}

func (request *ProjectPlotProfileFilterRequest) UnmarshalJSON(data []byte) error {
	type plain ProjectPlotProfileFilterRequest
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "input", "preview"); err != nil {
		return err
	}
	var raw struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := decodeProfileLifecycleJSON(raw.Input, &decoded.Input, "originalRules", "subvarieties"); err != nil {
		return err
	}
	*request = ProjectPlotProfileFilterRequest(decoded)
	return nil
}

func (s *ContextService) ResolveProjectPlotProfileNavigation(ctx context.Context, contextID string, request ProjectPlotProfileFilterRequest) (ProjectPlotProfileNavigation, error) {
	plots := []PlotSummary{}
	result, err := s.runProjectPlotProfile(ctx, contextID, request.Input, func(tx *sql.Tx, current ProjectPlotProfileResult) (resultErr error) {
		if !reflect.DeepEqual(current, request.Preview) {
			return errors.New("profile execution changed since preview; rerun before applying navigation")
		}
		if len(current.PlotNumbers) == 0 {
			return errors.New("profile resulted in zero plots; current navigation filter remains unchanged")
		}
		if len(current.PlotNumbers) > 200 {
			return errors.New("profile exceeds 200 plots; Save as SU is unavailable and current navigation filter remains unchanged")
		}
		rows, err := tx.QueryContext(ctx, `SELECT env.PlotNumber,env.FieldNumber,env.PlotRepresenting,env.Zone,env.SubZone,env.SiteSeries
			FROM USysEnv AS env JOIN VProProfileRunPlots AS selected
			ON env.PlotNumber COLLATE BINARY=selected.PlotNumber
			ORDER BY env.PlotNumber COLLATE BINARY`)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
		for rows.Next() {
			var plot PlotSummary
			if err := rows.Scan(&plot.PlotNumber, &plot.FieldNumber, &plot.PlotRepresenting, &plot.Zone, &plot.SubZone, &plot.SiteSeries); err != nil {
				return err
			}
			plots = append(plots, plot)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(plots) != len(current.PlotNumbers) {
			return errors.New("profile navigation does not have exactly one scoped record per reviewed plot")
		}
		for index, plot := range plots {
			if plot.PlotNumber != current.PlotNumbers[index] {
				return errors.New("profile navigation differs from exact reviewed plot membership")
			}
		}
		return nil
	})
	if err != nil {
		return ProjectPlotProfileNavigation{}, err
	}
	return ProjectPlotProfileNavigation{contextID, result, plots}, nil
}
