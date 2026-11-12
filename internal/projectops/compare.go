package projectops

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type CompareProjectsRequest struct {
	SourcePath    string `json:"sourcePath,omitempty"`
	SourceProject string `json:"sourceProject,omitempty"`
	TargetPath    string `json:"targetPath"`
	TargetProject string `json:"targetProject,omitempty"`
}

type FieldDiff struct {
	Table      string  `json:"table"`
	PlotNumber string  `json:"plotNumber"`
	Key        string  `json:"key"`
	Field      string  `json:"field"`
	SourceVal  *string `json:"sourceVal"`
	TargetVal  *string `json:"targetVal"`
}

type PlotDiff struct {
	PlotNumber string      `json:"plotNumber"`
	Status     string      `json:"status"` // "Added" | "Removed" | "Identical" | "Modified"
	DiffCount  int         `json:"diffCount"`
	FieldDiffs []FieldDiff `json:"fieldDiffs"`
}

type CompareProjectsResult struct {
	SourcePath     string      `json:"sourcePath"`
	SourceProject  string      `json:"sourceProject"`
	TargetPath     string      `json:"targetPath"`
	TargetProject  string      `json:"targetProject"`
	TotalPlots     int         `json:"totalPlots"`
	AddedCount     int         `json:"addedCount"`
	RemovedCount   int         `json:"removedCount"`
	IdenticalCount int         `json:"identicalCount"`
	ModifiedCount  int         `json:"modifiedCount"`
	Plots          []PlotDiff  `json:"plots"`
	FieldDiffs     []FieldDiff `json:"fieldDiffs"`
}

var compareTableSuffixes = []string{"Env", "Admin", "Veg", "Humus", "Mineral", "Other"}

func resolveProjectInDatabase(ctx context.Context, path, requestedProject string) (string, string, error) {
	resolvedPath, err := vprocore.ExistingDatabasePath(path)
	if err != nil {
		return "", "", err
	}
	projects, err := vprocore.InspectFileContext(ctx, resolvedPath)
	if err != nil {
		return "", "", err
	}
	trimmed := strings.TrimSpace(requestedProject)
	if trimmed != "" {
		for _, p := range projects {
			if p.Name == trimmed && p.Compatible {
				return resolvedPath, p.Name, nil
			}
		}
		return "", "", fmt.Errorf("compatible VP08 project %q not found in %s", trimmed, resolvedPath)
	}
	var compatible []string
	for _, p := range projects {
		if p.Compatible {
			compatible = append(compatible, p.Name)
		}
	}
	if len(compatible) == 0 {
		return "", "", fmt.Errorf("no compatible VP08 project found in %s", resolvedPath)
	}
	if len(compatible) > 1 {
		return "", "", fmt.Errorf("multiple projects (%s) found in %s; specify project name explicitly", strings.Join(compatible, ", "), resolvedPath)
	}
	return resolvedPath, compatible[0], nil
}

func formatComparableCell(cell rawSQLiteCell, declaredBool bool) *string {
	switch cell.storage {
	case "null":
		return nil
	case "integer":
		v := cell.intVal
		if declaredBool && v != 0 {
			v = -1
		}
		s := fmt.Sprintf("%d", v)
		return &s
	case "real":
		s := vprocore.SourceDoubleDecimalString(cell.realVal)
		return &s
	case "text":
		s := string(cell.bytes)
		return &s
	case "blob":
		s := fmt.Sprintf("0x%x", cell.bytes)
		return &s
	default:
		return nil
	}
}

func sameNullableString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

type loadedProjectSnapshot struct {
	project string
	path    string
	plots   map[string]bool
	// tableSuffix -> plotNumber -> logicalRowKey -> colName -> *string
	cells   map[string]map[string]map[string]map[string]*string
	columns map[string][]string
	// raw typed cells for 3-way merge execution: tableSuffix -> plotNumber -> logicalRowKey -> colName -> rawSQLiteCell
	rawCells map[string]map[string]map[string]map[string]rawSQLiteCell
}

func loadProjectSnapshotForDiff(ctx context.Context, dbPath, project string) (*loadedProjectSnapshot, error) {
	db, err := sqlutil.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	snap := &loadedProjectSnapshot{
		project:  project,
		path:     dbPath,
		plots:    map[string]bool{},
		cells:    map[string]map[string]map[string]map[string]*string{},
		columns:  map[string][]string{},
		rawCells: map[string]map[string]map[string]map[string]rawSQLiteCell{},
	}

	for _, suffix := range compareTableSuffixes {
		tblName := project + "_" + suffix
		quotedTbl := vprocore.QuoteHeaderIdentifier(tblName)

		// Inspect declared boolean columns.
		boolCols := map[string]bool{}
		pRows, pErr := tx.QueryContext(ctx, "PRAGMA table_info("+quotedTbl+")")
		if pErr != nil {
			return nil, pErr
		}
		for pRows.Next() {
			var cid, notNull, pk int
			var cName, cType string
			var defVal sql.NullString
			if err := pRows.Scan(&cid, &cName, &cType, &notNull, &defVal, &pk); err != nil {
				pRows.Close()
				return nil, err
			}
			upper := strings.ToUpper(cType)
			if strings.Contains(upper, "BOOL") || strings.Contains(upper, "BIT") || cName == "Flag" {
				boolCols[cName] = true
			}
		}
		if err := errors.Join(pRows.Err(), pRows.Close()); err != nil {
			return nil, err
		}

		cols, rows, rErr := readExactTableRows(ctx, tx, quotedTbl, "")
		if rErr != nil {
			return nil, rErr
		}
		snap.columns[suffix] = cols
		snap.cells[suffix] = map[string]map[string]map[string]*string{}
		snap.rawCells[suffix] = map[string]map[string]map[string]rawSQLiteCell{}

		colIndex := map[string]int{}
		for i, c := range cols {
			colIndex[c] = i
		}

		plotCol := "PlotNumber"
		if suffix == "Admin" {
			plotCol = "Plot"
		}
		pIdx, ok := colIndex[plotCol]
		if !ok {
			return nil, fmt.Errorf("table %s is missing plot column %s", tblName, plotCol)
		}

		// Group rows by plot first so multi-row child tables can receive deterministic logical keys.
		byPlot := map[string][][]rawSQLiteCell{}
		for _, r := range rows {
			pCell := r[pIdx]
			if pCell.storage != "text" {
				continue
			}
			plot := string(pCell.bytes)
			snap.plots[plot] = true
			byPlot[plot] = append(byPlot[plot], r)
		}

		for plot, plotRows := range byPlot {
			snap.cells[suffix][plot] = map[string]map[string]*string{}
			snap.rawCells[suffix][plot] = map[string]map[string]rawSQLiteCell{}

			baseKeyFn := func(r []rawSQLiteCell) string {
				switch suffix {
				case "Env", "Admin":
					return suffix
				case "Veg":
					spp := ""
					if idx, ok := colIndex["Species"]; ok && r[idx].storage == "text" {
						spp = string(r[idx].bytes)
					}
					layer := ""
					if idx, ok := colIndex["Layer"]; ok && r[idx].storage == "text" {
						layer = string(r[idx].bytes)
					}
					if layer != "" {
						return fmt.Sprintf("Species=%s|Layer=%s", spp, layer)
					}
					return fmt.Sprintf("Species=%s", spp)
				case "Humus", "Mineral":
					hz := ""
					if idx, ok := colIndex["Horizon"]; ok && r[idx].storage == "text" {
						hz = string(r[idx].bytes)
					}
					return fmt.Sprintf("Horizon=%s", hz)
				case "Other":
					dn := ""
					if idx, ok := colIndex["DataName"]; ok && r[idx].storage == "text" {
						dn = string(r[idx].bytes)
					}
					return fmt.Sprintf("DataName=%s", dn)
				default:
					return suffix
				}
			}

			// Sort child rows deterministically ignoring physical ID so identical content in different IDs matches!
			sortSig := func(r []rawSQLiteCell) string {
				var parts []string
				parts = append(parts, baseKeyFn(r))
				for i, col := range cols {
					if col == "ID" || col == plotCol {
						continue
					}
					s := formatComparableCell(r[i], boolCols[col])
					if s == nil {
						parts = append(parts, "\x00")
					} else {
						parts = append(parts, *s)
					}
				}
				return strings.Join(parts, "\x1f")
			}
			slices.SortFunc(plotRows, func(a, b []rawSQLiteCell) int {
				return strings.Compare(sortSig(a), sortSig(b))
			})

			baseGroupCounts := map[string]int{}
			for _, r := range plotRows {
				baseGroupCounts[baseKeyFn(r)]++
			}
			idIdx, hasIDCol := colIndex["ID"]
			keyCounts := map[string]int{}
			for _, r := range plotRows {
				baseKey := baseKeyFn(r)
				keyCounts[baseKey]++
				rowKey := baseKey
				if suffix != "Env" && suffix != "Admin" && baseGroupCounts[baseKey] > 1 {
					if hasIDCol && r[idIdx].storage == "integer" {
						rowKey = fmt.Sprintf("%s[ID=%d]", baseKey, r[idIdx].intVal)
					} else {
						rowKey = fmt.Sprintf("%s#%d", baseKey, keyCounts[baseKey])
					}
				}
				colMap := make(map[string]*string, len(cols))
				rawMap := make(map[string]rawSQLiteCell, len(cols))
				for i, col := range cols {
					rawMap[col] = r[i]
					if col == "ID" || col == plotCol {
						continue
					}
					colMap[col] = formatComparableCell(r[i], boolCols[col])
				}
				snap.cells[suffix][plot][rowKey] = colMap
				snap.rawCells[suffix][plot][rowKey] = rawMap
			}
		}
	}
	return snap, nil
}

func summarizeRowMap(colOrder []string, m map[string]*string) string {
	var parts []string
	for _, col := range colOrder {
		val, exists := m[col]
		if !exists || val == nil || *val == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", col, *val))
	}
	if len(parts) == 0 {
		return "(present)"
	}
	return strings.Join(parts, ", ")
}

// CompareProjects performs a deterministic plot-by-plot and cell-by-cell comparison
// between the active project (or explicit SourcePath) and TargetPath across
// _Env, _Admin, _Veg, _Humus, _Mineral, and _Other.
func CompareProjects(s *vprocore.ContextService, ctx context.Context, contextID string, req CompareProjectsRequest) (CompareProjectsResult, error) {
	if strings.TrimSpace(req.TargetPath) == "" {
		return CompareProjectsResult{}, errors.New("targetPath is required for project comparison")
	}
	var srcPath, srcProject string
	if strings.TrimSpace(req.SourcePath) == "" {
		snap, err := inspectContextSnapshot(ctx, s, contextID)
		if err != nil {
			return CompareProjectsResult{}, err
		}
		srcPath = snap.ProjectPath
		srcProject = snap.Project
		if strings.TrimSpace(req.SourceProject) != "" {
			var rErr error
			srcPath, srcProject, rErr = resolveProjectInDatabase(ctx, srcPath, req.SourceProject)
			if rErr != nil {
				return CompareProjectsResult{}, rErr
			}
		}
	} else {
		// Still verify contextID is live when ContextService is provided.
		if s != nil && contextID != "" {
			if _, err := inspectContextSnapshot(ctx, s, contextID); err != nil {
				return CompareProjectsResult{}, err
			}
		}
		var err error
		srcPath, srcProject, err = resolveProjectInDatabase(ctx, req.SourcePath, req.SourceProject)
		if err != nil {
			return CompareProjectsResult{}, err
		}
	}

	tgtPath, tgtProject, err := resolveProjectInDatabase(ctx, req.TargetPath, req.TargetProject)
	if err != nil {
		return CompareProjectsResult{}, err
	}

	srcSnap, err := loadProjectSnapshotForDiff(ctx, srcPath, srcProject)
	if err != nil {
		return CompareProjectsResult{}, fmt.Errorf("read source project %s: %w", srcProject, err)
	}
	tgtSnap, err := loadProjectSnapshotForDiff(ctx, tgtPath, tgtProject)
	if err != nil {
		return CompareProjectsResult{}, fmt.Errorf("read target project %s: %w", tgtProject, err)
	}

	return diffLoadedSnapshots(srcSnap, tgtSnap), nil
}

func diffLoadedSnapshots(srcSnap, tgtSnap *loadedProjectSnapshot) CompareProjectsResult {
	allPlotsMap := map[string]bool{}
	for p := range srcSnap.plots {
		allPlotsMap[p] = true
	}
	for p := range tgtSnap.plots {
		allPlotsMap[p] = true
	}
	allPlots := make([]string, 0, len(allPlotsMap))
	for p := range allPlotsMap {
		allPlots = append(allPlots, p)
	}
	slices.Sort(allPlots)

	out := CompareProjectsResult{
		SourcePath:    srcSnap.path,
		SourceProject: srcSnap.project,
		TargetPath:    tgtSnap.path,
		TargetProject: tgtSnap.project,
		TotalPlots:    len(allPlots),
		Plots:         make([]PlotDiff, 0, len(allPlots)),
		FieldDiffs:    []FieldDiff{},
	}

	for _, plot := range allPlots {
		inSrc := srcSnap.plots[plot]
		inTgt := tgtSnap.plots[plot]

		if inSrc && !inTgt {
			out.AddedCount++
			srcVal := "Present"
			diff := FieldDiff{
				Table:      "Env",
				PlotNumber: plot,
				Key:        "Plot",
				Field:      "PlotNumber",
				SourceVal:  &srcVal,
				TargetVal:  nil,
			}
			out.Plots = append(out.Plots, PlotDiff{
				PlotNumber: plot,
				Status:     "Added",
				DiffCount:  1,
				FieldDiffs: []FieldDiff{diff},
			})
			out.FieldDiffs = append(out.FieldDiffs, diff)
			continue
		}
		if !inSrc && inTgt {
			out.RemovedCount++
			tgtVal := "Present"
			diff := FieldDiff{
				Table:      "Env",
				PlotNumber: plot,
				Key:        "Plot",
				Field:      "PlotNumber",
				SourceVal:  nil,
				TargetVal:  &tgtVal,
			}
			out.Plots = append(out.Plots, PlotDiff{
				PlotNumber: plot,
				Status:     "Removed",
				DiffCount:  1,
				FieldDiffs: []FieldDiff{diff},
			})
			out.FieldDiffs = append(out.FieldDiffs, diff)
			continue
		}

		// Present in both Source and Target: compare across all 6 tables.
		var plotDiffs []FieldDiff
		for _, suffix := range compareTableSuffixes {
			srcRows := srcSnap.cells[suffix][plot]
			tgtRows := tgtSnap.cells[suffix][plot]
			rowKeysMap := map[string]bool{}
			for k := range srcRows {
				rowKeysMap[k] = true
			}
			for k := range tgtRows {
				rowKeysMap[k] = true
			}
			rowKeys := make([]string, 0, len(rowKeysMap))
			for k := range rowKeysMap {
				rowKeys = append(rowKeys, k)
			}
			slices.Sort(rowKeys)

			colOrder := srcSnap.columns[suffix]
			for _, c := range tgtSnap.columns[suffix] {
				if !slices.Contains(colOrder, c) {
					colOrder = append(colOrder, c)
				}
			}

			for _, rk := range rowKeys {
				sRow, sHas := srcRows[rk]
				tRow, tHas := tgtRows[rk]
				if sHas && !tHas {
					summary := summarizeRowMap(colOrder, sRow)
					plotDiffs = append(plotDiffs, FieldDiff{
						Table:      suffix,
						PlotNumber: plot,
						Key:        rk,
						Field:      "*",
						SourceVal:  &summary,
						TargetVal:  nil,
					})
					continue
				}
				if !sHas && tHas {
					summary := summarizeRowMap(colOrder, tRow)
					plotDiffs = append(plotDiffs, FieldDiff{
						Table:      suffix,
						PlotNumber: plot,
						Key:        rk,
						Field:      "*",
						SourceVal:  nil,
						TargetVal:  &summary,
					})
					continue
				}
				for _, col := range colOrder {
					if col == "ID" || col == "PlotNumber" || col == "Plot" {
						continue
					}
					sVal := sRow[col]
					tVal := tRow[col]
					if !sameNullableString(sVal, tVal) {
						cellKey := col
						if suffix != "Env" && suffix != "Admin" {
							cellKey = rk + "." + col
						}
						var sCopy, tCopy *string
						if sVal != nil {
							v := *sVal
							sCopy = &v
						}
						if tVal != nil {
							v := *tVal
							tCopy = &v
						}
						plotDiffs = append(plotDiffs, FieldDiff{
							Table:      suffix,
							PlotNumber: plot,
							Key:        cellKey,
							Field:      col,
							SourceVal:  sCopy,
							TargetVal:  tCopy,
						})
					}
				}
			}
		}

		if len(plotDiffs) == 0 {
			out.IdenticalCount++
			out.Plots = append(out.Plots, PlotDiff{
				PlotNumber: plot,
				Status:     "Identical",
				DiffCount:  0,
				FieldDiffs: []FieldDiff{},
			})
		} else {
			out.ModifiedCount++
			out.Plots = append(out.Plots, PlotDiff{
				PlotNumber: plot,
				Status:     "Modified",
				DiffCount:  len(plotDiffs),
				FieldDiffs: plotDiffs,
			})
			out.FieldDiffs = append(out.FieldDiffs, plotDiffs...)
		}
	}

	return out
}
