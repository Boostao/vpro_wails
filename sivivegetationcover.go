package main

import (
	"context"
	"fmt"
)

// This private plan does not authorize writes or widen the height service.
func planSIVICoverEdits(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable, edits []siviHeightEdit) ([]siviHeightAssignment, error) {
	return planSIVIChildEdits(ctx, plot, extended, veg, edits, "cover",
		func(group int, column string) bool {
			switch group {
			case 0:
				switch column {
				case "Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB":
					return true
				case "Cover5a", "Cover5b", "Cover5c":
					return extended
				}
			case 1:
				return column == "Cover6"
			case 2:
				return column == "Cover7" || column == "Cover8" || column == "Cover9"
			}
			return false
		}, validateSIVICoverCell)
}

func validateSIVICoverCell(column string, cell ProjectMetadataCell) error {
	if err := validateSIVIChildSingle(column, cell); err != nil {
		return err
	}
	if cell.Real != nil && *cell.Real >= 100 {
		return fmt.Errorf("Veg.%s requires a value less than 100 or NULL, matching the SIVI source control", column)
	}
	return nil
}
