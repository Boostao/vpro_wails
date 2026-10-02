package main

import (
	"context"
	"fmt"
)

type SoilSuggestion struct {
	ListName        *string  `json:"listName"`
	ListFilter      *string  `json:"listFilter"`
	ItemOrder       *float64 `json:"itemOrder"`
	Item            *string  `json:"item"`
	ItemDescription *string  `json:"itemDescription"`
	FieldUsedIn     *string  `json:"fieldUsedIn"`
	ValidateLoops   *string  `json:"validateLoops"`
	Validate        *bool    `json:"validate"`
	Note            *string  `json:"note"`
	Flag            *bool    `json:"flag"`
}

var soilSuggestionGroups = []string{
	"HumusHorizon", "HumusStructureKind", "HumusStructureDegree", "vonPost", "MycelAbundance",
	"MinSoilAspect", "SoilTexture", "MineralStructureClass", "MineralStructureKind",
}

func (s *ContextService) ListSoilSuggestions(ctx context.Context, contextID string) ([]SoilSuggestion, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]SoilSuggestion, error) {
		rows, err := plots.projects.sqlite.conn.QueryContext(ctx, `SELECT
			ListName,ListFilter,ItemOrder,Item,ItemDescription,FieldUsedIn,ValidateLoops,
			CASE WHEN Validate IS NULL THEN NULL WHEN CAST(Validate AS INTEGER) != 0 THEN 1 ELSE 0 END,
			Note,CASE WHEN Flag IS NULL THEN NULL WHEN CAST(Flag AS INTEGER) != 0 THEN 1 ELSE 0 END
			FROM USysTableOfLists
			WHERE ListName IN (?,?,?,?,?,?,?,?,?)
			ORDER BY ListName,ItemOrder,Item`,
			soilSuggestionGroups[0], soilSuggestionGroups[1], soilSuggestionGroups[2],
			soilSuggestionGroups[3], soilSuggestionGroups[4], soilSuggestionGroups[5],
			soilSuggestionGroups[6], soilSuggestionGroups[7], soilSuggestionGroups[8])
		if err != nil {
			return nil, fmt.Errorf("soil reference suggestions unavailable: %w", err)
		}
		defer rows.Close()
		result := []SoilSuggestion{}
		counts := map[string]int{}
		for rows.Next() {
			var entry SoilSuggestion
			if err := rows.Scan(&entry.ListName, &entry.ListFilter, &entry.ItemOrder, &entry.Item,
				&entry.ItemDescription, &entry.FieldUsedIn, &entry.ValidateLoops, &entry.Validate, &entry.Note, &entry.Flag); err != nil {
				return nil, fmt.Errorf("soil reference metadata could not be read: %w", err)
			}
			result = append(result, entry)
			if entry.ListName != nil {
				counts[*entry.ListName]++
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, group := range soilSuggestionGroups {
			if counts[group] == 0 {
				return nil, fmt.Errorf("soil reference group %q is unavailable", group)
			}
		}
		return result, nil
	})
}
