package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

const siviCollectedHistoryTable = "__VPRO_SIVICollectedHistory"
const siviCollectedHistorySQL = `CREATE TABLE "__VPRO_SIVICollectedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviCollectedWritePolicy() siviVegetationWritePolicy {
	return siviVegetationWritePolicy{"collected", siviCollectedHistoryTable, siviCollectedHistorySQL,
		[]string{"Collected"}, planSIVICollectedCells}
}

func collectedSIVIDrafts(ctx context.Context, edits []SIVICollectedEdit) ([]siviHeightEdit, error) {
	if ctx == nil {
		return nil, errors.New("SIVI Collected planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.New("SIVI Collected planning requires explicit clicks")
	}
	drafts := make([]siviHeightEdit, len(edits))
	for i, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := strconv.ParseInt(edit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != edit.RowID {
			return nil, errors.New("SIVI Collected requires the original signed64 physical row identity")
		}
		if edit.Clicks < 1 || edit.Clicks > 3 {
			return nil, errors.New("SIVI Collected requires exactly one to three clicks")
		}
		if _, err := metadataCellValue(edit.Expected); err != nil {
			return nil, fmt.Errorf("SIVI Collected expected value: %w", err)
		}
		if edit.Expected.Storage != "null" && edit.Expected.Storage != "text" {
			return nil, errors.New("historical non-text Collected storage is not cyclable")
		}
		next := collectedAfterClicks(edit.Expected.Text, edit.Clicks)
		value := ProjectMetadataCell{Storage: "null"}
		if next != nil {
			value, err = projectMetadataCell("text", []byte(*next))
			if err != nil {
				return nil, fmt.Errorf("SIVI Collected derived value: %w", err)
			}
		}
		drafts[i] = siviHeightEdit{edit.RowID, edit.Form, "Collected", cloneSiteUnitCell(edit.Expected), value}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return drafts, nil
}

func planSIVICollectedEdits(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable, edits []SIVICollectedEdit) ([]siviHeightAssignment, error) {
	drafts, err := collectedSIVIDrafts(ctx, edits)
	if err != nil {
		return nil, err
	}
	return planSIVICollectedCells(ctx, plot, extended, veg, drafts)
}

func planSIVICollectedCells(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable, edits []siviHeightEdit) ([]siviHeightAssignment, error) {
	if ctx == nil {
		return nil, errors.New("SIVI Collected planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return planSIVIChildEdits(ctx, plot, extended, veg, edits, "Collected",
		func(group int, column string) bool { return group >= 0 && group < 3 && column == "Collected" },
		func(_ string, cell ProjectMetadataCell) error {
			if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil || (*cell.Text != "C" && *cell.Text != "V")) {
				return errors.New("new Collected values must be NULL, C or V")
			}
			value, err := metadataCellValue(cell)
			if err != nil {
				return err
			}
			return validateChildPhysicalText("Veg.Collected", value, 1)
		})
}
