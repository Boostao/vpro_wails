package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "context"

const SiviCoverHistoryTable = "__VPRO_SIVICoverHistory"
const siviCoverHistorySQL = `CREATE TABLE "__VPRO_SIVICoverHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func SiviCoverWritePolicy() SiviVegetationWritePolicy {
	return SiviVegetationWritePolicy{"cover", SiviCoverHistoryTable, siviCoverHistorySQL,
		[]string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB",
			"Cover6", "Cover7", "Cover8", "Cover9"}, PlanSIVICoverEdits}
}

func WriteSIVICovers(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool, original []SiviVegetationProjection, edits []SiviHeightEdit) (*siviHeightWriteResult, error) {
	return WriteSIVIVegetationCells(s, ctx, contextID, plot, extended, original, edits, SiviCoverWritePolicy())
}

func RestoreSIVICovers(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	return RestoreSIVIVegetationCells(s, ctx, contextID, plot, historyID, action, SiviCoverWritePolicy())
}
