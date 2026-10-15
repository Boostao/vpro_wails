package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type twoPageVegetationSpeciesListMatch struct {
	Role       string
	Definition ProjectMetadataRow
}

// Membership alone does not authorize mutation; the caller must independently own the row and snapshot.
func qualifyTwoPageVegetationSpeciesListSelection(ctx context.Context, references *twoPageVegetationSpeciesReferences,
	code string) ([]twoPageVegetationSpeciesListMatch, error) {
	if ctx == nil || references == nil {
		return nil, errors.New("two-page Species list selection requires owned physical references and a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, value := range []string{references.ContextID, references.Project, references.Plot} {
		if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
			return nil, errors.New("two-page Species list references require a valid literal owner")
		}
	}
	if references.Form != "SubVegAXL_BC" && references.Form != "SubVegAXL" {
		return nil, errors.New("two-page Species list selection is qualified only for existing A/BC and CHARS A rows")
	}
	if err := validateVegetationSpeciesLookup(code); err != nil {
		return nil, err
	}
	matches := []twoPageVegetationSpeciesListMatch{}
	for _, source := range []struct {
		role     string
		table    ProjectMetadataTable
		personal bool
	}{
		{"master", references.Master, false},
		{"personal", references.Personal, true},
	} {
		table, err := projectTwoPageVegetationSpeciesDefinitions(ctx, source.table, source.personal)
		if err != nil {
			return nil, fmt.Errorf("two-page Species list %s definitions: %w", source.role, err)
		}
		for _, row := range table.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			literal, lifeform, codetype := row.Cells[0], row.Cells[2], row.Cells[4]
			if literal.Text == nil || *literal.Text != code || lifeform.Integer == nil || codetype.Text == nil {
				continue
			}
			if *codetype.Text != "u" && *codetype.Text != "U" && *codetype.Text != "x" && *codetype.Text != "X" {
				continue
			}
			number, err := strconv.ParseInt(*lifeform.Integer, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("two-page Species list lifeform: %w", err)
			}
			if number >= 1 && number <= 4 {
				matches = append(matches, twoPageVegetationSpeciesListMatch{source.role, row})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, errors.New("two-page Species selection is not an exact qualified existing A/CHARS list code; manual and NotInList routes remain unavailable")
	}
	return matches, nil
}

func planTwoPageVegetationSpeciesListEdits(ctx context.Context, original *twoPageVegetationOriginal,
	edits []twoPageVegetationEdit, references *twoPageVegetationSpeciesReferences) ([]twoPageVegetationAssignment, error) {
	if references == nil {
		return nil, errors.New("two-page Species list planning requires current physical reference definitions")
	}
	return planTwoPageVegetationQualifiedEdits(ctx, original, edits, true, references)
}

func requireTwoPageVegetationSpeciesListCovers(original *twoPageVegetationOriginal, assignments []twoPageVegetationAssignment) error {
	covers := []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"}
	before, err := twoPageVegetationSourceRows(original.Records, original.RecordSource, covers)
	if err != nil {
		return fmt.Errorf("two-page Species list common-cover schema: %w", err)
	}
	after, err := twoPageVegetationSourceRows(twoPageVegetationPlannedTable(original.Records, assignments), original.RecordSource, covers)
	if err != nil {
		return fmt.Errorf("two-page Species list planned common-cover schema: %w", err)
	}
	for _, assignment := range assignments {
		if assignment.Column != "Species" {
			continue
		}
		if !slices.Contains(before, assignment.RowID) || !slices.Contains(after, assignment.RowID) {
			return errors.New("two-page Species list assignment with all seven common covers NULL requires the unavailable source warning/advance route")
		}
	}
	return nil
}
