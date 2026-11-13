package projectops

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	MergeResolutionKeepTarget   = "keep_target"
	MergeResolutionAcceptSource = "accept_source"
)

type ThreeWayMergePreviewRequest struct {
	SourcePath    string `json:"sourcePath"`
	SourceProject string `json:"sourceProject,omitempty"`
	TargetPath    string `json:"targetPath,omitempty"`
	TargetProject string `json:"targetProject,omitempty"`
	BasePath      string `json:"basePath,omitempty"`
	BaseProject   string `json:"baseProject,omitempty"`
}

type MergeConflictItem struct {
	ConflictID        string  `json:"conflictId"`
	PlotNumber        string  `json:"plotNumber"`
	Table             string  `json:"table"`
	Key               string  `json:"key"`
	Field             string  `json:"field"`
	BaseVal           *string `json:"baseVal,omitempty"`
	SourceVal         *string `json:"sourceVal"`
	TargetVal         *string `json:"targetVal"`
	Kind              string  `json:"kind"`              // "FastForward" | "Conflict" | "TargetModifiedOnly"
	DefaultResolution string  `json:"defaultResolution"` // "accept_source" | "keep_target"
}

type MergePlotPreview struct {
	PlotNumber string              `json:"plotNumber"`
	Category   string              `json:"category"` // "Added" | "Identical" | "FastForward" | "Conflict" | "TargetOnly"
	Items      []MergeConflictItem `json:"items"`
}

type ThreeWayMergePreview struct {
	SourcePath       string             `json:"sourcePath"`
	SourceProject    string             `json:"sourceProject"`
	TargetPath       string             `json:"targetPath"`
	TargetProject    string             `json:"targetProject"`
	BasePath         string             `json:"basePath,omitempty"`
	BaseProject      string             `json:"baseProject,omitempty"`
	PreviewDigest    string             `json:"previewDigest"`
	AddedCount       int                `json:"addedCount"`
	IdenticalCount   int                `json:"identicalCount"`
	FastForwardCount int                `json:"fastForwardCount"`
	ConflictCount    int                `json:"conflictCount"`
	TargetOnlyCount  int                `json:"targetOnlyCount"`
	Plots            []MergePlotPreview `json:"plots"`
	Conflicts        []MergeConflictItem `json:"conflicts"`
}

type ThreeWayMergeExecuteRequest struct {
	SourcePath                string            `json:"sourcePath"`
	SourceProject             string            `json:"sourceProject,omitempty"`
	TargetPath                string            `json:"targetPath,omitempty"`
	TargetProject             string            `json:"targetProject,omitempty"`
	BasePath                  string            `json:"basePath,omitempty"`
	BaseProject               string            `json:"baseProject,omitempty"`
	ExpectedPreviewDigest     string            `json:"expectedPreviewDigest,omitempty"`
	DefaultConflictResolution string            `json:"defaultConflictResolution,omitempty"`
	PlotResolutions           map[string]string `json:"plotResolutions,omitempty"`
	FieldResolutions          map[string]string `json:"fieldResolutions,omitempty"`
}

type ThreeWayMergeResult struct {
	SourceProject      string  `json:"sourceProject"`
	TargetProject      string  `json:"targetProject"`
	TargetPath         string  `json:"targetPath"`
	AddedPlots         int     `json:"addedPlots"`
	UpdatedPlots       int     `json:"updatedPlots"`
	SkippedPlots       int     `json:"skippedPlots"`
	AllocatedChildIDs  []int64 `json:"allocatedChildIds"`
	ReservedDeletedIDs []int64 `json:"reservedDeletedIds"`
	AuditRowsWritten   int     `json:"auditRowsWritten"`
}

func makeConflictID(plot, table, key string) string {
	return plot + "::" + table + "::" + key
}

func validateMergeResolutionChoice(choice string) error {
	switch choice {
	case MergeResolutionKeepTarget, MergeResolutionAcceptSource:
		return nil
	default:
		return fmt.Errorf("invalid merge resolution %q (expected %q or %q)", choice, MergeResolutionKeepTarget, MergeResolutionAcceptSource)
	}
}

func resolveMergeEndpoints(ctx context.Context, s *vprocore.ContextService, contextID string, srcPathReq, srcProjReq, tgtPathReq, tgtProjReq, basePathReq, baseProjReq string) (srcPath, srcProj, tgtPath, tgtProj, basePath, baseProj, actor string, err error) {
	if strings.TrimSpace(srcPathReq) == "" {
		return "", "", "", "", "", "", "", errors.New("sourcePath (staging .db) is required for 3-way merge")
	}
	actor = "VProUser"
	var activePath, activeProj string
	if s != nil && contextID != "" {
		snap, sErr := inspectContextSnapshot(ctx, s, contextID)
		if sErr != nil {
			return "", "", "", "", "", "", "", sErr
		}
		activePath = snap.ProjectPath
		activeProj = snap.Project
		actor = snap.CurrentUser
	}
	srcPath, srcProj, err = resolveProjectInDatabase(ctx, srcPathReq, srcProjReq)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("source staging database: %w", err)
	}
	if strings.TrimSpace(tgtPathReq) == "" {
		if activePath == "" {
			return "", "", "", "", "", "", "", errors.New("targetPath is required when no active project context is provided")
		}
		tgtPath = activePath
		tgtProj = activeProj
		if strings.TrimSpace(tgtProjReq) != "" {
			tgtPath, tgtProj, err = resolveProjectInDatabase(ctx, tgtPath, tgtProjReq)
			if err != nil {
				return "", "", "", "", "", "", "", err
			}
		}
	} else {
		tgtPath, tgtProj, err = resolveProjectInDatabase(ctx, tgtPathReq, tgtProjReq)
		if err != nil {
			return "", "", "", "", "", "", "", fmt.Errorf("target database: %w", err)
		}
	}
	if vprocore.SameDesktopPath(srcPath, tgtPath) && srcProj == tgtProj {
		return "", "", "", "", "", "", "", errors.New("source and target refer to the same project in the same database file")
	}
	if strings.TrimSpace(basePathReq) != "" {
		basePath, baseProj, err = resolveProjectInDatabase(ctx, basePathReq, baseProjReq)
		if err != nil {
			return "", "", "", "", "", "", "", fmt.Errorf("base ancestor database: %w", err)
		}
	}
	return srcPath, srcProj, tgtPath, tgtProj, basePath, baseProj, actor, nil
}

func writeDigestBytes(h hash.Hash, b []byte) {
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(b)))
	_, _ = h.Write(lenBuf[:])
	_, _ = h.Write(b)
}

func writeDigestString(h hash.Hash, s string) {
	writeDigestBytes(h, []byte(s))
}

func writeDigestUint64(h hash.Hash, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	_, _ = h.Write(buf[:])
}

func hashLoadedProjectSnapshot(snap *loadedProjectSnapshot) string {
	if snap == nil {
		return ""
	}
	h := sha256.New()
	writeDigestString(h, snap.project)
	writeDigestString(h, snap.path)

	for _, suffix := range compareTableSuffixes {
		writeDigestString(h, suffix)
		cols := snap.columns[suffix]
		writeDigestUint64(h, uint64(len(cols)))
		for _, col := range cols {
			writeDigestString(h, col)
		}

		byPlot := snap.rawCells[suffix]
		plots := make([]string, 0, len(byPlot))
		for plot := range byPlot {
			plots = append(plots, plot)
		}
		slices.Sort(plots)
		writeDigestUint64(h, uint64(len(plots)))

		for _, plot := range plots {
			writeDigestString(h, plot)
			byKey := byPlot[plot]
			keys := make([]string, 0, len(byKey))
			for k := range byKey {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			writeDigestUint64(h, uint64(len(keys)))

			for _, k := range keys {
				writeDigestString(h, k)
				rowMap := byKey[k]
				for _, col := range cols {
					cell := rowMap[col]
					writeDigestString(h, cell.storage)
					switch cell.storage {
					case "integer":
						writeDigestUint64(h, uint64(cell.intVal))
					case "real":
						writeDigestUint64(h, math.Float64bits(cell.realVal))
					case "text", "blob":
						writeDigestBytes(h, cell.bytes)
					}
				}
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func computeMergeSnapshotDigest(srcSnap, tgtSnap, baseSnap *loadedProjectSnapshot) string {
	h := sha256.New()
	writeDigestString(h, hashLoadedProjectSnapshot(srcSnap))
	writeDigestString(h, hashLoadedProjectSnapshot(tgtSnap))
	writeDigestString(h, hashLoadedProjectSnapshot(baseSnap))
	return hex.EncodeToString(h.Sum(nil))
}

func buildThreeWayMergePreviewInternal(ctx context.Context, srcPath, srcProj, tgtPath, tgtProj, basePath, baseProj string, tgtQueryer sqliteQueryer) (ThreeWayMergePreview, *loadedProjectSnapshot, *loadedProjectSnapshot, *loadedProjectSnapshot, error) {
	srcSnap, err := loadProjectSnapshotForDiff(ctx, srcPath, srcProj)
	if err != nil {
		return ThreeWayMergePreview{}, nil, nil, nil, err
	}
	var tgtSnap *loadedProjectSnapshot
	if tgtQueryer != nil {
		tgtSnap, err = loadProjectSnapshotFromQueryer(ctx, tgtQueryer, tgtPath, tgtProj)
	} else {
		tgtSnap, err = loadProjectSnapshotForDiff(ctx, tgtPath, tgtProj)
	}
	if err != nil {
		return ThreeWayMergePreview{}, nil, nil, nil, err
	}
	var baseSnap *loadedProjectSnapshot
	if basePath != "" {
		baseSnap, err = loadProjectSnapshotForDiff(ctx, basePath, baseProj)
		if err != nil {
			return ThreeWayMergePreview{}, nil, nil, nil, err
		}
	}
	digest := computeMergeSnapshotDigest(srcSnap, tgtSnap, baseSnap)

	diff := diffLoadedSnapshots(srcSnap, tgtSnap)
	preview := ThreeWayMergePreview{
		SourcePath:    srcPath,
		SourceProject: srcProj,
		TargetPath:    tgtPath,
		TargetProject: tgtProj,
		BasePath:      basePath,
		BaseProject:   baseProj,
		PreviewDigest: digest,
		Plots:         make([]MergePlotPreview, 0, len(diff.Plots)),
		Conflicts:     []MergeConflictItem{},
	}

	lookupBaseVal := func(suffix, plot, rowKey, field string) (*string, bool) {
		if baseSnap == nil || !baseSnap.plots[plot] {
			return nil, false
		}
		rowMap, hasRow := baseSnap.cells[suffix][plot][rowKey]
		if !hasRow {
			return nil, true
		}
		if field == "*" {
			summary := summarizeRowMap(baseSnap.columns[suffix], rowMap)
			return &summary, true
		}
		return rowMap[field], true
	}

	for _, pd := range diff.Plots {
		switch pd.Status {
		case "Added":
			// If Base had this plot and Target deleted it while Source kept it, classify as Conflict!
			if baseSnap != nil && baseSnap.plots[pd.PlotNumber] {
				cid := makeConflictID(pd.PlotNumber, "Env", "Plot")
				present := "Present"
				item := MergeConflictItem{
					ConflictID:        cid,
					PlotNumber:        pd.PlotNumber,
					Table:             "Env",
					Key:               "Plot",
					Field:             "PlotNumber",
					BaseVal:           &present,
					SourceVal:         &present,
					TargetVal:         nil,
					Kind:              "Conflict",
					DefaultResolution: MergeResolutionKeepTarget,
				}
				preview.ConflictCount++
				preview.Plots = append(preview.Plots, MergePlotPreview{
					PlotNumber: pd.PlotNumber,
					Category:   "Conflict",
					Items:      []MergeConflictItem{item},
				})
				preview.Conflicts = append(preview.Conflicts, item)
			} else {
				preview.AddedCount++
				preview.Plots = append(preview.Plots, MergePlotPreview{
					PlotNumber: pd.PlotNumber,
					Category:   "Added",
					Items:      []MergeConflictItem{},
				})
			}
		case "Removed":
			preview.TargetOnlyCount++
			preview.Plots = append(preview.Plots, MergePlotPreview{
				PlotNumber: pd.PlotNumber,
				Category:   "TargetOnly",
				Items:      []MergeConflictItem{},
			})
		case "Identical":
			preview.IdenticalCount++
			preview.Plots = append(preview.Plots, MergePlotPreview{
				PlotNumber: pd.PlotNumber,
				Category:   "Identical",
				Items:      []MergeConflictItem{},
			})
		case "Modified":
			var items []MergeConflictItem
			hasRealConflict := false
			hasFastForward := false
			for _, fd := range pd.FieldDiffs {
				rowKey := fd.Table
				if fd.Field == "*" {
					rowKey = fd.Key
				} else if fd.Table != "Env" && fd.Table != "Admin" {
					if idx := strings.LastIndex(fd.Key, "."); idx > 0 {
						rowKey = fd.Key[:idx]
					}
				}
				bVal, inBase := lookupBaseVal(fd.Table, pd.PlotNumber, rowKey, fd.Field)
				kind := "Conflict"
				defRes := MergeResolutionKeepTarget
				if baseSnap != nil && inBase {
					if sameNullableString(fd.TargetVal, bVal) && !sameNullableString(fd.SourceVal, bVal) {
						kind = "FastForward"
						defRes = MergeResolutionAcceptSource
						hasFastForward = true
					} else if sameNullableString(fd.SourceVal, bVal) && !sameNullableString(fd.TargetVal, bVal) {
						kind = "TargetModifiedOnly"
						defRes = MergeResolutionKeepTarget
					} else {
						hasRealConflict = true
					}
				} else {
					hasRealConflict = true
				}
				item := MergeConflictItem{
					ConflictID:        makeConflictID(pd.PlotNumber, fd.Table, fd.Key),
					PlotNumber:        pd.PlotNumber,
					Table:             fd.Table,
					Key:               fd.Key,
					Field:             fd.Field,
					BaseVal:           bVal,
					SourceVal:         fd.SourceVal,
					TargetVal:         fd.TargetVal,
					Kind:              kind,
					DefaultResolution: defRes,
				}
				items = append(items, item)
				if kind == "Conflict" {
					preview.Conflicts = append(preview.Conflicts, item)
				}
			}
			category := "Conflict"
			if !hasRealConflict && hasFastForward {
				category = "FastForward"
				preview.FastForwardCount++
			} else if !hasRealConflict {
				category = "TargetOnly"
				preview.TargetOnlyCount++
			} else {
				preview.ConflictCount++
			}
			preview.Plots = append(preview.Plots, MergePlotPreview{
				PlotNumber: pd.PlotNumber,
				Category:   category,
				Items:      items,
			})
		}
	}

	return preview, srcSnap, tgtSnap, baseSnap, nil
}

// PreviewThreeWayMerge inspects Source (Staging.db), Target (active or Master.db),
// and optional Base ancestor .db, categorizing every plot and field-level conflict.
func PreviewThreeWayMerge(s *vprocore.ContextService, ctx context.Context, contextID string, req ThreeWayMergePreviewRequest) (ThreeWayMergePreview, error) {
	srcPath, srcProj, tgtPath, tgtProj, basePath, baseProj, _, err := resolveMergeEndpoints(
		ctx, s, contextID,
		req.SourcePath, req.SourceProject,
		req.TargetPath, req.TargetProject,
		req.BasePath, req.BaseProject,
	)
	if err != nil {
		return ThreeWayMergePreview{}, err
	}
	preview, _, _, _, err := buildThreeWayMergePreviewInternal(ctx, srcPath, srcProj, tgtPath, tgtProj, basePath, baseProj, nil)
	return preview, err
}

// ExecuteThreeWayMerge applies Staging.db -> Target.db changes inside a single atomic
// BEGIN IMMEDIATE transaction on Target.db, honoring per-plot and per-field conflict
// resolutions ('keep_target' | 'accept_source'), allocating monotonic child IDs via
// __VPRO_ChildIdentity without reusing deleted IDs, and recording full <Target>_Audit entries.
func ExecuteThreeWayMerge(s *vprocore.ContextService, ctx context.Context, contextID string, req ThreeWayMergeExecuteRequest) (ThreeWayMergeResult, error) {
	defaultRes := strings.TrimSpace(req.DefaultConflictResolution)
	if defaultRes == "" {
		defaultRes = MergeResolutionKeepTarget
	}
	if err := validateMergeResolutionChoice(defaultRes); err != nil {
		return ThreeWayMergeResult{}, err
	}
	for k, v := range req.PlotResolutions {
		if k == "" {
			return ThreeWayMergeResult{}, errors.New("plot resolution key must not be empty")
		}
		if err := validateMergeResolutionChoice(v); err != nil {
			return ThreeWayMergeResult{}, err
		}
	}
	for k, v := range req.FieldResolutions {
		if k == "" {
			return ThreeWayMergeResult{}, errors.New("field resolution key must not be empty")
		}
		if err := validateMergeResolutionChoice(v); err != nil {
			return ThreeWayMergeResult{}, err
		}
	}

	srcPath, srcProj, tgtPath, tgtProj, basePath, baseProj, actor, err := resolveMergeEndpoints(
		ctx, s, contextID,
		req.SourcePath, req.SourceProject,
		req.TargetPath, req.TargetProject,
		req.BasePath, req.BaseProject,
	)
	if err != nil {
		return ThreeWayMergeResult{}, err
	}

	resolveDecision := func(item MergeConflictItem) string {
		if choice, ok := req.FieldResolutions[item.ConflictID]; ok {
			return choice
		}
		if choice, ok := req.PlotResolutions[item.PlotNumber]; ok {
			return choice
		}
		if item.Kind == "FastForward" || item.Kind == "TargetModifiedOnly" {
			return item.DefaultResolution
		}
		return defaultRes
	}

	runMergeOnConn := func(conn *sql.Conn) (ThreeWayMergeResult, error) {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return ThreeWayMergeResult{}, err
		}
		var txErr error
		defer func() {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				txErr = errors.Join(txErr, rb)
			}
		}()

		preview, srcSnap, tgtSnap, _, pErr := buildThreeWayMergePreviewInternal(ctx, srcPath, srcProj, tgtPath, tgtProj, basePath, baseProj, tx)
		if pErr != nil {
			return ThreeWayMergeResult{}, pErr
		}
		if exp := strings.TrimSpace(req.ExpectedPreviewDigest); exp != "" && exp != preview.PreviewDigest {
			return ThreeWayMergeResult{}, errors.New("source or target database changed since preview; re-run PreviewThreeWayMerge before executing")
		}

		if err := ensureChildIdentityTable(ctx, tx); err != nil {
			return ThreeWayMergeResult{}, err
		}

		auditTbl := vprocore.QuoteHeaderIdentifier(tgtProj + "_Audit")
		auditStmt, err := tx.PrepareContext(ctx, `INSERT INTO `+auditTbl+` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`)
		if err != nil {
			return ThreeWayMergeResult{}, err
		}
		defer auditStmt.Close()

		now := time.Now().Format("2006-01-02 15:04:05")
		var allocatedIDs, reservedDeletedIDs []int64
		addedPlots, updatedPlots, skippedPlots, auditRows := 0, 0, 0, 0

		insertPlotChildRow := func(suffix, plot string, srcRaw map[string]rawSQLiteCell) error {
			cols := tgtSnap.columns[suffix]
			newID, aErr := allocateSafeChildID(ctx, tx, tgtProj, suffix)
			if aErr != nil {
				return aErr
			}
			allocatedIDs = append(allocatedIDs, newID)

			quotedCols := make([]string, len(cols))
			marks := make([]string, len(cols))
			args := make([]any, len(cols))
			for i, col := range cols {
				quotedCols[i] = vprocore.QuoteHeaderIdentifier(col)
				marks[i] = "?"
				switch col {
				case "PlotNumber":
					args[i] = plot
				case "ID":
					args[i] = newID
				default:
					cell := srcRaw[col]
					if err := validateMergedCell(suffix, col, cell); err != nil {
						return fmt.Errorf("plot %s %s.%s: %w", plot, suffix, col, err)
					}
					args[i] = normalizeMergedParam(suffix, col, cell)
				}
			}
			insSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				vprocore.QuoteHeaderIdentifier(tgtProj+"_"+suffix),
				strings.Join(quotedCols, ", "),
				strings.Join(marks, ", "),
			)
			if _, err := tx.ExecContext(ctx, insSQL, args...); err != nil {
				return err
			}
			for _, col := range cols {
				if col == "PlotNumber" || col == "ID" {
					continue
				}
				c := srcRaw[col]
				if c.storage == "null" || (col == "Flag" && c.storage == "integer" && c.intVal == 0) {
					continue
				}
				if _, err := auditStmt.ExecContext(ctx, tgtProj, actor, plot, "_"+suffix, col, now, nil, auditTextForRawCell(c), newID); err != nil {
					return err
				}
				auditRows++
			}
			return nil
		}

		insertWholePlotFromSource := func(plot string) error {
			if err := validateUTF16Max("PlotNumber", plot, 7); err != nil {
				return err
			}
			for _, suffix := range []string{"Env", "Admin"} {
				srcRaw, ok := srcSnap.rawCells[suffix][plot][suffix]
				if !ok {
					return fmt.Errorf("source plot %s is missing required %s row", plot, suffix)
				}
				cols := tgtSnap.columns[suffix]
				quotedCols := make([]string, len(cols))
				marks := make([]string, len(cols))
				args := make([]any, len(cols))
				keyCol := "PlotNumber"
				if suffix == "Admin" {
					keyCol = "Plot"
				}
				for i, col := range cols {
					quotedCols[i] = vprocore.QuoteHeaderIdentifier(col)
					marks[i] = "?"
					if col == keyCol {
						args[i] = plot
					} else {
						cell := srcRaw[col]
						if err := validateMergedCell(suffix, col, cell); err != nil {
							return fmt.Errorf("plot %s %s.%s: %w", plot, suffix, col, err)
						}
						args[i] = normalizeMergedParam(suffix, col, cell)
					}
				}
				insSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
					vprocore.QuoteHeaderIdentifier(tgtProj+"_"+suffix),
					strings.Join(quotedCols, ", "),
					strings.Join(marks, ", "),
				)
				if _, err := tx.ExecContext(ctx, insSQL, args...); err != nil {
					return err
				}
				for _, col := range cols {
					c := srcRaw[col]
					if col == keyCol {
						if _, err := auditStmt.ExecContext(ctx, tgtProj, actor, plot, "_"+suffix, col, now, nil, plot, nil); err != nil {
							return err
						}
						auditRows++
						continue
					}
					if c.storage == "null" {
						continue
					}
					if _, err := auditStmt.ExecContext(ctx, tgtProj, actor, plot, "_"+suffix, col, now, nil, auditTextForRawCell(c), nil); err != nil {
						return err
					}
					auditRows++
				}
			}
			for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other"} {
				srcChildRows := srcSnap.rawCells[suffix][plot]
				var keys []string
				for k := range srcChildRows {
					keys = append(keys, k)
				}
				slices.Sort(keys)
				for _, rk := range keys {
					if err := insertPlotChildRow(suffix, plot, srcChildRows[rk]); err != nil {
						return err
					}
				}
			}
			return nil
		}

		for _, plotPlan := range preview.Plots {
			if err := ctx.Err(); err != nil {
				return ThreeWayMergeResult{}, err
			}
			plot := plotPlan.PlotNumber
			switch plotPlan.Category {
			case "Identical", "TargetOnly":
				skippedPlots++
			case "Added":
				if choice, ok := req.PlotResolutions[plot]; ok && choice == MergeResolutionKeepTarget {
					skippedPlots++
					continue
				}
				if err := insertWholePlotFromSource(plot); err != nil {
					return ThreeWayMergeResult{}, err
				}
				addedPlots++
			case "FastForward", "Conflict":
				// Handle special case where Base had plot, Target deleted it, Source kept it
				if len(plotPlan.Items) == 1 && plotPlan.Items[0].Key == "Plot" && !tgtSnap.plots[plot] && srcSnap.plots[plot] {
					if resolveDecision(plotPlan.Items[0]) == MergeResolutionAcceptSource {
						if err := insertWholePlotFromSource(plot); err != nil {
							return ThreeWayMergeResult{}, err
						}
						addedPlots++
					} else {
						skippedPlots++
					}
					continue
				}

				plotModified := false
				for _, item := range plotPlan.Items {
					if resolveDecision(item) != MergeResolutionAcceptSource {
						continue
					}
					suffix := item.Table
					quotedTbl := vprocore.QuoteHeaderIdentifier(tgtProj + "_" + suffix)

					if suffix == "Env" || suffix == "Admin" {
						keyCol := "PlotNumber"
						if suffix == "Admin" {
							keyCol = "Plot"
						}
						srcCell := srcSnap.rawCells[suffix][plot][suffix][item.Field]
						if err := validateMergedCell(suffix, item.Field, srcCell); err != nil {
							return ThreeWayMergeResult{}, fmt.Errorf("plot %s %s.%s: %w", plot, suffix, item.Field, err)
						}
						param := normalizeMergedParam(suffix, item.Field, srcCell)
						uRes, uErr := tx.ExecContext(ctx, `UPDATE `+quotedTbl+` SET `+vprocore.QuoteHeaderIdentifier(item.Field)+
							` = ? WHERE typeof(`+vprocore.QuoteHeaderIdentifier(keyCol)+`)='text' AND CAST(`+vprocore.QuoteHeaderIdentifier(keyCol)+` AS BLOB)=CAST(? AS BLOB)`,
							param, plot)
						if uErr != nil {
							return ThreeWayMergeResult{}, uErr
						}
						if n, _ := uRes.RowsAffected(); n != 1 {
							return ThreeWayMergeResult{}, fmt.Errorf("merge update on %s.%s for plot %s matched %d rows", suffix, item.Field, plot, n)
						}
						var beforeVal, afterVal any
						if item.TargetVal != nil {
							beforeVal = *item.TargetVal
						}
						if item.SourceVal != nil {
							afterVal = *item.SourceVal
						}
						if _, err := auditStmt.ExecContext(ctx, tgtProj, actor, plot, "_"+suffix, item.Field, now, beforeVal, afterVal, nil); err != nil {
							return ThreeWayMergeResult{}, err
						}
						auditRows++
						plotModified = true
						continue
					}

					// Child table item (Veg, Humus, Mineral, Other).
					if item.Field == "*" {
						if item.SourceVal != nil && item.TargetVal == nil {
							// Row added in Source -> allocate monotonic child ID & insert.
							srcRow := srcSnap.rawCells[suffix][plot][item.Key]
							if err := insertPlotChildRow(suffix, plot, srcRow); err != nil {
								return ThreeWayMergeResult{}, err
							}
							plotModified = true
						} else if item.SourceVal == nil && item.TargetVal != nil {
							// Row deleted in Source -> delete from Target and permanently reserve ID!
							tgtRow := tgtSnap.rawCells[suffix][plot][item.Key]
							idCell := tgtRow["ID"]
							if idCell.storage != "integer" {
								return ThreeWayMergeResult{}, fmt.Errorf("cannot delete target %s row in plot %s without integer ID", suffix, plot)
							}
							targetID := idCell.intVal
							if targetID > 0 {
								if err := reserveChildID(ctx, tx, tgtProj+"_"+suffix, targetID); err != nil {
									return ThreeWayMergeResult{}, err
								}
								reservedDeletedIDs = append(reservedDeletedIDs, targetID)
							}
							dRes, dErr := tx.ExecContext(ctx, `DELETE FROM `+quotedTbl+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB) AND "ID" = ?`, plot, targetID)
							if dErr != nil {
								return ThreeWayMergeResult{}, dErr
							}
							if n, _ := dRes.RowsAffected(); n != 1 {
								return ThreeWayMergeResult{}, fmt.Errorf("merge child delete on %s (plot %s, ID %d) matched %d rows", suffix, plot, targetID, n)
							}
							for _, col := range tgtSnap.columns[suffix] {
								if col == "PlotNumber" || col == "ID" {
									continue
								}
								oldCell := tgtRow[col]
								if oldCell.storage == "null" || (col == "Flag" && oldCell.storage == "integer" && oldCell.intVal == 0) {
									continue
								}
								if _, err := auditStmt.ExecContext(ctx, tgtProj, actor, plot, "_"+suffix, col, now, auditTextForRawCell(oldCell), nil, targetID); err != nil {
									return ThreeWayMergeResult{}, err
								}
								auditRows++
							}
							plotModified = true
						}
						continue
					}

					// Field-level update on a matched child row.
					idx := strings.LastIndex(item.Key, ".")
					if idx <= 0 {
						return ThreeWayMergeResult{}, fmt.Errorf("malformed child diff key %q", item.Key)
					}
					rowKey := item.Key[:idx]
					srcCell := srcSnap.rawCells[suffix][plot][rowKey][item.Field]
					tgtRow := tgtSnap.rawCells[suffix][plot][rowKey]
					idCell := tgtRow["ID"]
					if idCell.storage != "integer" {
						return ThreeWayMergeResult{}, fmt.Errorf("target %s row in plot %s lacks integer ID for field merge", suffix, plot)
					}
					if err := validateMergedCell(suffix, item.Field, srcCell); err != nil {
						return ThreeWayMergeResult{}, fmt.Errorf("plot %s %s.%s: %w", plot, suffix, item.Field, err)
					}
					param := normalizeMergedParam(suffix, item.Field, srcCell)
					uRes, uErr := tx.ExecContext(ctx, `UPDATE `+quotedTbl+` SET `+vprocore.QuoteHeaderIdentifier(item.Field)+
						` = ? WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB) AND "ID" = ?`,
						param, plot, idCell.intVal)
					if uErr != nil {
						return ThreeWayMergeResult{}, uErr
					}
					if n, _ := uRes.RowsAffected(); n != 1 {
						return ThreeWayMergeResult{}, fmt.Errorf("merge child update on %s.%s (plot %s, ID %d) matched %d rows", suffix, item.Field, plot, idCell.intVal, n)
					}
					var beforeVal, afterVal any
					if item.TargetVal != nil {
						beforeVal = *item.TargetVal
					}
					if item.SourceVal != nil {
						afterVal = *item.SourceVal
					}
					if _, err := auditStmt.ExecContext(ctx, tgtProj, actor, plot, "_"+suffix, item.Field, now, beforeVal, afterVal, idCell.intVal); err != nil {
						return ThreeWayMergeResult{}, err
					}
					auditRows++
					plotModified = true
				}
				if plotModified {
					updatedPlots++
				} else {
					skippedPlots++
				}
			}
		}

		if err := ctx.Err(); err != nil {
			return ThreeWayMergeResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return ThreeWayMergeResult{}, err
		}
		slices.Sort(allocatedIDs)
		slices.Sort(reservedDeletedIDs)
		reservedDeletedIDs = slices.Compact(reservedDeletedIDs)
		return ThreeWayMergeResult{
			SourceProject:      srcProj,
			TargetProject:      tgtProj,
			TargetPath:         tgtPath,
			AddedPlots:         addedPlots,
			UpdatedPlots:       updatedPlots,
			SkippedPlots:       skippedPlots,
			AllocatedChildIDs:  allocatedIDs,
			ReservedDeletedIDs: reservedDeletedIDs,
			AuditRowsWritten:   auditRows,
		}, txErr
	}

	// If merging into the active context's project database, acquire the ContextService writer lease!
	if s != nil && contextID != "" {
		snap, sErr := inspectContextSnapshot(ctx, s, contextID)
		if sErr == nil && vprocore.SameDesktopPath(tgtPath, snap.ProjectPath) {
			var res ThreeWayMergeResult
			editErr := s.Edit(contextID, func(plots *vprocore.PlotService) error {
				return plots.Projects.Sqlite.WithMetadataWriter(ctx, func(conn *sql.Conn) error {
					var mErr error
					res, mErr = runMergeOnConn(conn)
					return mErr
				})
			})
			return res, editErr
		}
	}

	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tgtPath, "rw")+"&_foreign_keys=on&_busy_timeout=5000&_txlock=immediate&_journal_mode=WAL")
	if err != nil {
		return ThreeWayMergeResult{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return ThreeWayMergeResult{}, err
	}
	defer conn.Close()
	return runMergeOnConn(conn)
}

func validateMergedCell(suffix, col string, cell rawSQLiteCell) error {
	if cell.storage == "null" {
		return nil
	}
	if cell.storage == "text" {
		txt := string(cell.bytes)
		switch suffix {
		case "Veg":
			if col == "Species" {
				return validateUTF16Max("Veg.Species", txt, 8)
			}
		case "Humus":
			if col == "Horizon" {
				return validateUTF16Max("Humus.Horizon", txt, 8)
			}
		case "Mineral":
			switch col {
			case "Horizon":
				return validateUTF16Max("Mineral.Horizon", txt, 8)
			case "Texture":
				return validateUTF16Max("Mineral.Texture", txt, 4)
			case "Colour":
				return validateUTF16Max("Mineral.Colour", txt, 14)
			}
		}
	}
	if suffix == "Veg" && slices.Contains(vegCoverCols, col) && (cell.storage == "real" || cell.storage == "integer") {
		val := cell.realVal
		if cell.storage == "integer" {
			val = float64(cell.intVal)
		}
		if _, ok := NarrowSingleCover(val); !ok {
			return fmt.Errorf("cover %s=%v must be >= 0 and < 100 in IEEE-754 SINGLE", col, val)
		}
	}
	return nil
}

func normalizeMergedParam(suffix, col string, cell rawSQLiteCell) any {
	if cell.storage == "integer" {
		isBool := col == "Flag" ||
			(suffix == "Env" && (col == "SpeciesListComplete" || col == "SV_FloodPlain")) ||
			(suffix == "Admin" && col == "UpdatedFromCards") ||
			(suffix == "Other" && strings.HasPrefix(col, "UserFlag"))
		if isBool {
			if cell.intVal != 0 {
				return int64(-1)
			}
			return int64(0)
		}
	}
	return cell.sqlParam()
}
