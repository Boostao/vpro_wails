package projectops

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type OptimizeVegOptions struct {
	PlotNumbers []string `json:"plotNumbers,omitempty"`
}

type OptimizeVegGroup struct {
	PlotNumber                string             `json:"plotNumber"`
	Species                   string             `json:"species"`
	SourceRowCount            int                `json:"sourceRowCount"`
	SurvivorID                int64              `json:"survivorId"`
	RetiredIDs                []int64            `json:"retiredIds"`
	HasOverlappingLayerCovers bool               `json:"hasOverlappingLayerCovers"`
	CappedCoverColumns        []string           `json:"cappedCoverColumns"`
	MergedCovers              map[string]float64 `json:"mergedCovers"`
	MergedHeights             map[string]float64 `json:"mergedHeights"`
	MergedHeightB             *string            `json:"mergedHeightB,omitempty"`
}

type OptimizeVegPreview struct {
	Project               string             `json:"project"`
	BeforeRowCount        int                `json:"beforeRowCount"`
	AfterRowCount         int                `json:"afterRowCount"`
	RowsToConsolidate     int                `json:"rowsToConsolidate"`
	DuplicateSpeciesCount int                `json:"duplicateSpeciesCount"`
	OverlappingGroupCount int                `json:"overlappingGroupCount"`
	CappedCoverCount      int                `json:"cappedCoverCount"`
	PreviewDigest         string             `json:"previewDigest"`
	Groups                []OptimizeVegGroup `json:"groups"`
}

type ApplyOptimizeVegRequest struct {
	PlotNumbers    []string `json:"plotNumbers,omitempty"`
	ExpectedDigest string   `json:"expectedDigest,omitempty"`
}

type OptimizeVegResult struct {
	Project            string             `json:"project"`
	BeforeRowCount     int                `json:"beforeRowCount"`
	AfterRowCount      int                `json:"afterRowCount"`
	ConsolidatedGroups int                `json:"consolidatedGroups"`
	DeletedRows        int                `json:"deletedRows"`
	ReservedDeletedIDs []int64            `json:"reservedDeletedIds"`
	AuditRowsWritten   int                `json:"auditRowsWritten"`
	Groups             []OptimizeVegGroup `json:"groups"`
}

var vegCoverCols = []string{
	"Cover1", "Cover2", "Cover3", "TotalA",
	"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB",
	"Cover6", "Cover7", "Cover8", "Cover9", "Cover10",
}

var vegNumericHeightCols = []string{
	"Height1", "Height2", "Height3", "HeightA",
	"Height4", "Height5", "Height5a", "Height5b", "Height5c",
	"Height6",
}

var vegFirstIntCols = []string{
	"LL", "AF", "DC", "UT", "VI", "PV", "PG", "FFA",
	"Cultural1", "Cultural2", "Other1", "Other2",
}

type plannedVegConsolidation struct {
	group          OptimizeVegGroup
	survivorRowID  int64
	deletedRowIDs  []int64
	deletedPhysIDs []int64
	survivorBefore map[string]rawSQLiteCell
	survivorAfter  map[string]rawSQLiteCell
	deletedBefore  []map[string]rawSQLiteCell
}

type vegRowWithRowID struct {
	rowID int64
	cells map[string]rawSQLiteCell
}

func buildOptimizeVegPlan(ctx context.Context, db sqliteQueryer, project string, filterPlots []string) (OptimizeVegPreview, []plannedVegConsolidation, []string, error) {
	vegTable := project + "_Veg"
	quotedVeg := vprocore.QuoteHeaderIdentifier(vegTable)

	colRows, err := db.QueryContext(ctx, "SELECT * FROM "+quotedVeg+" LIMIT 0")
	if err != nil {
		return OptimizeVegPreview{}, nil, nil, err
	}
	cols, colErr := colRows.Columns()
	_ = colRows.Close()
	if colErr != nil {
		return OptimizeVegPreview{}, nil, nil, colErr
	}

	cntRows, err := db.QueryContext(ctx, "SELECT COUNT(*) FROM "+quotedVeg)
	if err != nil {
		return OptimizeVegPreview{}, nil, nil, err
	}
	var beforeRowCount int
	if cntRows.Next() {
		if err := cntRows.Scan(&beforeRowCount); err != nil {
			_ = cntRows.Close()
			return OptimizeVegPreview{}, nil, nil, err
		}
	}
	if err := errors.Join(cntRows.Err(), cntRows.Close()); err != nil {
		return OptimizeVegPreview{}, nil, nil, err
	}

	plotFilter := map[string]bool{}
	var sortedFilterPlots []string
	for _, p := range filterPlots {
		if strings.TrimSpace(p) != "" && !plotFilter[p] {
			plotFilter[p] = true
			sortedFilterPlots = append(sortedFilterPlots, p)
		}
	}
	slices.Sort(sortedFilterPlots)

	type groupKey struct {
		plot    string
		species string
	}
	dupKeys := map[groupKey]bool{}
	dupPlotsSet := map[string]bool{}
	var dupPlots []string
	var groupOrder []groupKey

	dupKeyQuery := `SELECT CAST("PlotNumber" AS BLOB), CAST("Species" AS BLOB) FROM ` + quotedVeg + `
		WHERE typeof("PlotNumber")='text' AND typeof("Species")='text'
		  AND "PlotNumber" <> '' AND "Species" <> ''
		GROUP BY "PlotNumber" COLLATE BINARY, "Species" COLLATE BINARY
		HAVING COUNT(*) > 1`
	dupKeyRows, err := db.QueryContext(ctx, dupKeyQuery)
	if err != nil {
		return OptimizeVegPreview{}, nil, nil, err
	}
	for dupKeyRows.Next() {
		if err := ctx.Err(); err != nil {
			_ = dupKeyRows.Close()
			return OptimizeVegPreview{}, nil, nil, err
		}
		var plotBytes, sppBytes []byte
		if err := dupKeyRows.Scan(&plotBytes, &sppBytes); err != nil {
			_ = dupKeyRows.Close()
			return OptimizeVegPreview{}, nil, nil, err
		}
		plot := string(plotBytes)
		spp := string(sppBytes)
		if len(plotFilter) > 0 && !plotFilter[plot] {
			continue
		}
		gk := groupKey{plot: plot, species: spp}
		if !dupKeys[gk] {
			dupKeys[gk] = true
			groupOrder = append(groupOrder, gk)
		}
		if !dupPlotsSet[plot] {
			dupPlotsSet[plot] = true
			dupPlots = append(dupPlots, plot)
		}
	}
	if err := errors.Join(dupKeyRows.Err(), dupKeyRows.Close()); err != nil {
		return OptimizeVegPreview{}, nil, nil, err
	}

	slices.Sort(dupPlots)
	slices.SortFunc(groupOrder, func(a, b groupKey) int {
		if c := strings.Compare(a.plot, b.plot); c != 0 {
			return c
		}
		return strings.Compare(a.species, b.species)
	})

	plotColIdx, sppColIdx := -1, -1
	selectParts := []string{"rowid"}
	for i, c := range cols {
		if c == "PlotNumber" {
			plotColIdx = i
		} else if c == "Species" {
			sppColIdx = i
		}
		q := vprocore.QuoteHeaderIdentifier(c)
		selectParts = append(selectParts,
			"typeof("+q+")",
			"CASE typeof("+q+") WHEN 'integer' THEN CAST("+q+" AS INTEGER) WHEN 'real' THEN CAST("+q+" AS REAL) ELSE CAST("+q+" AS BLOB) END",
		)
	}

	var dupRows []vegRowWithRowID
	if len(dupPlots) > 0 && plotColIdx >= 0 && sppColIdx >= 0 {
		const batchSize = 400
		storages := make([]string, len(cols))
		vals := make([]any, len(cols))
		var rid int64
		dest := make([]any, 0, 1+len(cols)*2)
		dest = append(dest, &rid)
		for i := range cols {
			dest = append(dest, &storages[i], &vals[i])
		}

		for start := 0; start < len(dupPlots); start += batchSize {
			end := min(start+batchSize, len(dupPlots))
			chunk := dupPlots[start:end]
			marks := make([]string, len(chunk))
			args := make([]any, len(chunk))
			for i, p := range chunk {
				marks[i] = "?"
				args[i] = p
			}
			q := "SELECT " + strings.Join(selectParts, ", ") + " FROM " + quotedVeg +
				` WHERE typeof("PlotNumber")='text' AND typeof("Species")='text' AND "PlotNumber" IN (` + strings.Join(marks, ",") + `) ORDER BY rowid`
			rows, qErr := db.QueryContext(ctx, q, args...)
			if qErr != nil {
				return OptimizeVegPreview{}, nil, nil, qErr
			}
			for rows.Next() {
				if err := ctx.Err(); err != nil {
					_ = rows.Close()
					return OptimizeVegPreview{}, nil, nil, err
				}
				if err := rows.Scan(dest...); err != nil {
					_ = rows.Close()
					return OptimizeVegPreview{}, nil, nil, err
				}
				if storages[plotColIdx] != "text" || storages[sppColIdx] != "text" {
					continue
				}
				var plot, spp string
				switch b := vals[plotColIdx].(type) {
				case []byte:
					plot = string(b)
				case string:
					plot = b
				}
				switch b := vals[sppColIdx].(type) {
				case []byte:
					spp = string(b)
				case string:
					spp = b
				}
				if !dupKeys[groupKey{plot: plot, species: spp}] {
					continue
				}

				cellMap := make(map[string]rawSQLiteCell, len(cols))
				for i, col := range cols {
					c := rawSQLiteCell{storage: storages[i]}
					switch storages[i] {
					case "integer":
						if v, ok := vals[i].(int64); ok {
							c.intVal = v
						}
					case "real":
						if v, ok := vals[i].(float64); ok {
							c.realVal = v
						}
					case "text", "blob":
						switch b := vals[i].(type) {
						case []byte:
							c.bytes = append([]byte(nil), b...)
						case string:
							c.bytes = []byte(b)
						}
					}
					cellMap[col] = c
				}
				dupRows = append(dupRows, vegRowWithRowID{rowID: rid, cells: cellMap})
			}
			if err := errors.Join(rows.Err(), rows.Close()); err != nil {
				return OptimizeVegPreview{}, nil, nil, err
			}
		}
	}

	slices.SortFunc(dupRows, func(a, b vegRowWithRowID) int {
		if a.rowID < b.rowID {
			return -1
		}
		if a.rowID > b.rowID {
			return 1
		}
		return 0
	})

	digestHasher := sha256.New()
	_, _ = fmt.Fprintf(digestHasher, "count:%d|filters:%s|", beforeRowCount, strings.Join(sortedFilterPlots, ","))
	grouped := make(map[groupKey][]vegRowWithRowID, len(groupOrder))
	for _, r := range dupRows {
		_, _ = fmt.Fprintf(digestHasher, "row:%d|", r.rowID)
		for _, col := range cols {
			c := r.cells[col]
			switch c.storage {
			case "integer":
				_, _ = fmt.Fprintf(digestHasher, "%s=i:%d;", col, c.intVal)
			case "real":
				_, _ = fmt.Fprintf(digestHasher, "%s=r:%g;", col, c.realVal)
			case "text", "blob":
				_, _ = fmt.Fprintf(digestHasher, "%s=%s:%x;", col, c.storage, c.bytes)
			default:
				_, _ = fmt.Fprintf(digestHasher, "%s=null;", col)
			}
		}
		gk := groupKey{plot: string(r.cells["PlotNumber"].bytes), species: string(r.cells["Species"].bytes)}
		grouped[gk] = append(grouped[gk], r)
	}

	preview := OptimizeVegPreview{
		Project:        project,
		BeforeRowCount: beforeRowCount,
		AfterRowCount:  beforeRowCount,
		PreviewDigest:  hex.EncodeToString(digestHasher.Sum(nil)),
		Groups:         []OptimizeVegGroup{},
	}
	var plans []plannedVegConsolidation

	for _, gk := range groupOrder {
		members := grouped[gk]
		if len(members) <= 1 {
			continue
		}
		survivor := members[0]
		var survivorLogicalID int64
		if idCell := survivor.cells["ID"]; idCell.storage == "integer" && idCell.intVal > 0 {
			survivorLogicalID = idCell.intVal
		}

		afterCells := make(map[string]rawSQLiteCell, len(cols))
		for k, v := range survivor.cells {
			afterCells[k] = v
		}

		mergedCovers := map[string]float64{}
		mergedHeights := map[string]float64{}
		var cappedCols []string
		overlapping := false

		// Combine covers: sum non-null values in float32; cap >=100 at 99.0!
		for _, col := range vegCoverCols {
			if _, hasCol := survivor.cells[col]; !hasCol {
				continue
			}
			countNonNull := 0
			var sumF32 float32
			var singleVal float64
			for _, m := range members {
				c := m.cells[col]
				if c.storage == "null" {
					continue
				}
				var val float64
				switch c.storage {
				case "real":
					val = c.realVal
				case "integer":
					val = float64(c.intVal)
				default:
					continue
				}
				if math.IsNaN(val) || math.IsInf(val, 0) {
					continue
				}
				if val < 0 {
					val = 0
				}
				countNonNull++
				singleVal = val
				sumF32 += float32(val)
			}
			if countNonNull == 0 {
				afterCells[col] = rawSQLiteCell{storage: "null"}
				continue
			}
			if countNonNull > 1 {
				overlapping = true
			}
			var finalCover float64
			if countNonNull == 1 {
				if _, ok := NarrowSingleCover(singleVal); ok {
					finalCover = singleVal
				} else {
					finalCover = 99.0
					cappedCols = append(cappedCols, col)
				}
			} else {
				if sumF32 >= 100 {
					finalCover = 99.0
					cappedCols = append(cappedCols, col)
				} else {
					finalCover = math.Round(float64(sumF32)*1e6) / 1e6
					if _, ok := NarrowSingleCover(finalCover); !ok {
						finalCover = 99.0
						cappedCols = append(cappedCols, col)
					}
				}
			}
			afterCells[col] = rawSQLiteCell{storage: "real", realVal: finalCover}
			mergedCovers[col] = finalCover
		}

		// Combine numeric heights: STRICTLY MAX(Height*) (fixing V7mdlSpellCheck2020 Height6 SUM bug!).
		for _, col := range vegNumericHeightCols {
			if _, hasCol := survivor.cells[col]; !hasCol {
				continue
			}
			hasVal := false
			var maxVal float64
			for _, m := range members {
				c := m.cells[col]
				if c.storage == "null" {
					continue
				}
				var val float64
				switch c.storage {
				case "real":
					val = c.realVal
				case "integer":
					val = float64(c.intVal)
				default:
					continue
				}
				if math.IsNaN(val) || math.IsInf(val, 0) {
					continue
				}
				if !hasVal || val > maxVal {
					maxVal = val
					hasVal = true
				}
			}
			if !hasVal {
				afterCells[col] = rawSQLiteCell{storage: "null"}
			} else {
				afterCells[col] = rawSQLiteCell{storage: "real", realVal: maxVal}
				mergedHeights[col] = maxVal
			}
		}

		// Combine HeightB (VARCHAR in VP08) using MAX numeric value when numeric, else lexical MAX.
		var mergedHeightBPtr *string
		if _, hasHB := survivor.cells["HeightB"]; hasHB {
			hasNum := false
			var maxNum float64
			var maxNumStr string
			var maxLex string
			hasAny := false
			for _, m := range members {
				c := m.cells["HeightB"]
				if c.storage != "text" {
					continue
				}
				txt := strings.TrimSpace(string(c.bytes))
				if txt == "" {
					continue
				}
				hasAny = true
				if num, err := strconv.ParseFloat(txt, 64); err == nil && !math.IsNaN(num) && !math.IsInf(num, 0) {
					if !hasNum || num > maxNum {
						maxNum = num
						maxNumStr = string(c.bytes)
						hasNum = true
					}
				} else if txt > maxLex {
					maxLex = string(c.bytes)
				}
			}
			if !hasAny {
				afterCells["HeightB"] = rawSQLiteCell{storage: "null"}
			} else {
				chosen := maxLex
				if hasNum {
					chosen = maxNumStr
				}
				afterCells["HeightB"] = rawSQLiteCell{storage: "text", bytes: []byte(chosen)}
				chosenCopy := chosen
				mergedHeightBPtr = &chosenCopy
			}
		}

		// Layer: preserve if all non-empty Layer values match and only one cover stratum, else NULL.
		if _, hasLayer := survivor.cells["Layer"]; hasLayer {
			distinctLayers := map[string]bool{}
			var singleLayer string
			for _, m := range members {
				c := m.cells["Layer"]
				if c.storage == "text" && strings.TrimSpace(string(c.bytes)) != "" {
					singleLayer = string(c.bytes)
					distinctLayers[singleLayer] = true
				}
			}
			if len(distinctLayers) == 1 && len(mergedCovers) <= 1 {
				afterCells["Layer"] = rawSQLiteCell{storage: "text", bytes: []byte(singleLayer)}
			} else {
				afterCells["Layer"] = rawSQLiteCell{storage: "null"}
			}
		}

		// Collected: first non-null/non-empty text.
		if _, hasColl := survivor.cells["Collected"]; hasColl {
			chosen := rawSQLiteCell{storage: "null"}
			for _, m := range members {
				c := m.cells["Collected"]
				if c.storage == "text" && len(c.bytes) > 0 {
					chosen = rawSQLiteCell{storage: "text", bytes: append([]byte(nil), c.bytes...)}
					break
				}
			}
			afterCells["Collected"] = chosen
		}

		// Flag: Access BOOLEAN (-1 if any row has non-zero Flag, else 0).
		if _, hasFlag := survivor.cells["Flag"]; hasFlag {
			flagVal := int64(0)
			for _, m := range members {
				c := m.cells["Flag"]
				if c.storage == "integer" && c.intVal != 0 {
					flagVal = -1
					break
				}
			}
			afterCells["Flag"] = rawSQLiteCell{storage: "integer", intVal: flagVal}
		}

		// First non-null integer attributes LL..Other2.
		for _, col := range vegFirstIntCols {
			if _, hasCol := survivor.cells[col]; !hasCol {
				continue
			}
			chosen := rawSQLiteCell{storage: "null"}
			for _, m := range members {
				c := m.cells[col]
				if c.storage == "integer" {
					chosen = rawSQLiteCell{storage: "integer", intVal: c.intVal}
					break
				}
			}
			afterCells[col] = chosen
		}

		var retiredIDs []int64
		var deletedRowIDs []int64
		var deletedBefore []map[string]rawSQLiteCell
		for _, dup := range members[1:] {
			deletedRowIDs = append(deletedRowIDs, dup.rowID)
			deletedBefore = append(deletedBefore, dup.cells)
			if idCell := dup.cells["ID"]; idCell.storage == "integer" && idCell.intVal != 0 && idCell.intVal != survivorLogicalID {
				retiredIDs = append(retiredIDs, idCell.intVal)
			}
		}

		grp := OptimizeVegGroup{
			PlotNumber:                gk.plot,
			Species:                   gk.species,
			SourceRowCount:            len(members),
			SurvivorID:                survivorLogicalID,
			RetiredIDs:                retiredIDs,
			HasOverlappingLayerCovers: overlapping,
			CappedCoverColumns:        cappedCols,
			MergedCovers:              mergedCovers,
			MergedHeights:             mergedHeights,
			MergedHeightB:             mergedHeightBPtr,
		}
		preview.Groups = append(preview.Groups, grp)
		preview.DuplicateSpeciesCount++
		preview.RowsToConsolidate += len(members) - 1
		preview.AfterRowCount -= len(members) - 1
		if overlapping {
			preview.OverlappingGroupCount++
		}
		preview.CappedCoverCount += len(cappedCols)

		plans = append(plans, plannedVegConsolidation{
			group:          grp,
			survivorRowID:  survivor.rowID,
			deletedRowIDs:  deletedRowIDs,
			deletedPhysIDs: retiredIDs,
			survivorBefore: survivor.cells,
			survivorAfter:  afterCells,
			deletedBefore:  deletedBefore,
		})
	}

	return preview, plans, cols, nil
}

// PreviewOptimizeVeg computes a non-destructive preview of consolidating split/duplicate
// (PlotNumber, Species) rows in <Project>_Veg.
func PreviewOptimizeVeg(s *vprocore.ContextService, ctx context.Context, contextID string, opts OptimizeVegOptions) (OptimizeVegPreview, error) {
	snap, err := inspectContextSnapshot(ctx, s, contextID)
	if err != nil {
		return OptimizeVegPreview{}, err
	}
	db, err := sqlutil.OpenReadOnlyContext(ctx, snap.ProjectPath)
	if err != nil {
		return OptimizeVegPreview{}, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return OptimizeVegPreview{}, err
	}
	defer func() { _ = tx.Rollback() }()

	preview, _, _, err := buildOptimizeVegPlan(ctx, tx, snap.Project, opts.PlotNumbers)
	return preview, err
}

func rawCellsEqual(a, b rawSQLiteCell) bool {
	if a.storage != b.storage {
		return false
	}
	switch a.storage {
	case "null":
		return true
	case "integer":
		return a.intVal == b.intVal
	case "real":
		return math.Float64bits(a.realVal) == math.Float64bits(b.realVal)
	case "text", "blob":
		return string(a.bytes) == string(b.bytes)
	default:
		return false
	}
}

func auditTextForRawCell(c rawSQLiteCell) any {
	switch c.storage {
	case "null":
		return nil
	case "integer":
		return strconv.FormatInt(c.intVal, 10)
	case "real":
		return vprocore.SourceDoubleDecimalString(c.realVal)
	case "text":
		return string(c.bytes)
	case "blob":
		return hex.EncodeToString(c.bytes)
	default:
		return nil
	}
}

// ApplyOptimizeVeg executes multi-row (PlotNumber, Species) consolidation inside a single
// BEGIN IMMEDIATE transaction, reserving all retired child IDs in __VPRO_ChildIdentity
// and writing full <Project>_Audit records.
func ApplyOptimizeVeg(s *vprocore.ContextService, ctx context.Context, contextID string, req ApplyOptimizeVegRequest) (OptimizeVegResult, error) {
	if s == nil {
		return OptimizeVegResult{}, errors.New("project context service is unavailable")
	}
	var out OptimizeVegResult
	err := s.Edit(contextID, func(plots *vprocore.PlotService) error {
		owner := plots.Projects.Sqlite
		user := strings.TrimSpace(plots.CurrentUser)
		if user == "" {
			user = "VProUser"
		}
		return owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			project := owner.Selection.Project
			preview, plans, cols, err := buildOptimizeVegPlan(ctx, tx, project, req.PlotNumbers)
			if err != nil {
				return err
			}
			if expected := strings.TrimSpace(req.ExpectedDigest); expected != "" && expected != preview.PreviewDigest {
				return errors.New("vegetation table changed since preview; re-run PreviewOptimizeVeg before applying")
			}
			if len(plans) == 0 {
				out = OptimizeVegResult{
					Project:            project,
					BeforeRowCount:     preview.BeforeRowCount,
					AfterRowCount:      preview.AfterRowCount,
					ReservedDeletedIDs: []int64{},
					Groups:             []OptimizeVegGroup{},
				}
				return nil
			}

			if err := ensureChildIdentityTable(ctx, tx); err != nil {
				return err
			}

			vegTable := project + "_Veg"
			quotedVeg := vprocore.QuoteHeaderIdentifier(vegTable)
			quotedAudit := vprocore.QuoteHeaderIdentifier(project + "_Audit")
			now := time.Now().Format("2006-01-02 15:04:05")

			// Seed all existing positive IDs first so any survivor missing a positive ID gets a monotonic ID.
			existingRows, err := tx.QueryContext(ctx, `SELECT "ID" FROM `+quotedVeg+` WHERE typeof("ID")='integer' AND "ID" > 0`)
			if err != nil {
				return err
			}
			var existingIDs []int64
			for existingRows.Next() {
				var id int64
				if err := existingRows.Scan(&id); err != nil {
					existingRows.Close()
					return err
				}
				existingIDs = append(existingIDs, id)
			}
			if err := errors.Join(existingRows.Err(), existingRows.Close()); err != nil {
				return err
			}
			for _, id := range existingIDs {
				if err := reserveChildID(ctx, tx, vegTable, id); err != nil {
					return err
				}
			}

			auditStmt, err := tx.PrepareContext(ctx, `INSERT INTO `+quotedAudit+` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`)
			if err != nil {
				return err
			}
			defer auditStmt.Close()

			var allReserved []int64
			deletedCount := 0
			auditCount := 0

			for i := range plans {
				if err := ctx.Err(); err != nil {
					return err
				}
				plan := &plans[i]
				survivorID := plan.group.SurvivorID
				if survivorID <= 0 {
					allocID, aErr := allocateSafeChildID(ctx, tx, project, "Veg")
					if aErr != nil {
						return aErr
					}
					survivorID = allocID
					plan.group.SurvivorID = survivorID
					plan.survivorAfter["ID"] = rawSQLiteCell{storage: "integer", intVal: survivorID}
				} else {
					if err := reserveChildID(ctx, tx, vegTable, survivorID); err != nil {
						return err
					}
				}

				// Update changed columns on survivor row.
				var setParts []string
				var updateArgs []any
				for _, col := range cols {
					if col == "PlotNumber" || col == "Species" {
						continue
					}
					beforeCell := plan.survivorBefore[col]
					afterCell := plan.survivorAfter[col]
					if rawCellsEqual(beforeCell, afterCell) {
						continue
					}
					setParts = append(setParts, vprocore.QuoteHeaderIdentifier(col)+" = ?")
					updateArgs = append(updateArgs, afterCell.sqlParam())
					if col != "ID" {
						if _, err := auditStmt.ExecContext(ctx, project, user, plan.group.PlotNumber, "_Veg", col, now,
							auditTextForRawCell(beforeCell), auditTextForRawCell(afterCell), survivorID); err != nil {
							return err
						}
						auditCount++
					}
				}
				if len(setParts) > 0 {
					updateArgs = append(updateArgs, plan.survivorRowID)
					uRes, uErr := tx.ExecContext(ctx, "UPDATE "+quotedVeg+" SET "+strings.Join(setParts, ", ")+" WHERE rowid = ?", updateArgs...)
					if uErr != nil {
						return uErr
					}
					if n, _ := uRes.RowsAffected(); n != 1 {
						return errors.New("OptimizeVeg survivor row update did not match 1 row")
					}
				}

				// Delete duplicate rows and permanently reserve their IDs in __VPRO_ChildIdentity.
				for dIdx, dupRowID := range plan.deletedRowIDs {
					dupCells := plan.deletedBefore[dIdx]
					var dupID any
					if idCell := dupCells["ID"]; idCell.storage == "integer" {
						dupID = idCell.intVal
						if idCell.intVal > 0 && idCell.intVal != survivorID {
							if err := reserveChildID(ctx, tx, vegTable, idCell.intVal); err != nil {
								return err
							}
							allReserved = append(allReserved, idCell.intVal)
						}
					}
					for _, col := range cols {
						if col == "ID" || col == "PlotNumber" {
							continue
						}
						beforeCell := dupCells[col]
						if beforeCell.storage == "null" || (col == "Flag" && beforeCell.storage == "integer" && beforeCell.intVal == 0) {
							continue
						}
						if _, err := auditStmt.ExecContext(ctx, project, user, plan.group.PlotNumber, "_Veg", col, now,
							auditTextForRawCell(beforeCell), nil, dupID); err != nil {
							return err
						}
						auditCount++
					}
					dRes, dErr := tx.ExecContext(ctx, "DELETE FROM "+quotedVeg+" WHERE rowid = ?", dupRowID)
					if dErr != nil {
						return dErr
					}
					if n, _ := dRes.RowsAffected(); n != 1 {
						return errors.New("OptimizeVeg duplicate row delete did not match 1 row")
					}
					deletedCount++
				}
			}

			// Record a project-level OptimizeVeg provenance receipt in Audit.
			summaryJSON, _ := json.Marshal(map[string]any{
				"consolidatedGroups": len(plans),
				"deletedRows":        deletedCount,
			})
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+quotedAudit+` ("Project","User","Table","EditField","EditWhen","AfterEdit","Restore","Flag") VALUES (?,?,?,?,?,?,0,0)`,
				project, user, "_Veg", "OptimizeVeg", now, string(summaryJSON)); err != nil {
				return err
			}
			auditCount++

			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}

			groups := make([]OptimizeVegGroup, len(plans))
			for i, p := range plans {
				groups[i] = p.group
			}
			slices.Sort(allReserved)
			allReserved = slices.Compact(allReserved)
			out = OptimizeVegResult{
				Project:            project,
				BeforeRowCount:     preview.BeforeRowCount,
				AfterRowCount:      preview.AfterRowCount,
				ConsolidatedGroups: len(plans),
				DeletedRows:        deletedCount,
				ReservedDeletedIDs: allReserved,
				AuditRowsWritten:   auditCount,
				Groups:             groups,
			}
			return nil
		})
	})
	if err != nil {
		return OptimizeVegResult{}, err
	}
	return out, nil
}
