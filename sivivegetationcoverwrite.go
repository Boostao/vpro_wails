package main

import "context"

const siviCoverHistoryTable = "__VPRO_SIVICoverHistory"
const siviCoverHistorySQL = `CREATE TABLE "__VPRO_SIVICoverHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviCoverWritePolicy() siviVegetationWritePolicy {
	return siviVegetationWritePolicy{"cover", siviCoverHistoryTable, siviCoverHistorySQL,
		[]string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB",
			"Cover6", "Cover7", "Cover8", "Cover9"}, planSIVICoverEdits}
}

func (s *ContextService) writeSIVICovers(ctx context.Context, contextID, plot string, extended bool, original []siviVegetationProjection, edits []siviHeightEdit) (*siviHeightWriteResult, error) {
	return s.writeSIVIVegetationCells(ctx, contextID, plot, extended, original, edits, siviCoverWritePolicy())
}

func (s *ContextService) restoreSIVICovers(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	return s.restoreSIVIVegetationCells(ctx, contextID, plot, historyID, action, siviCoverWritePolicy())
}
