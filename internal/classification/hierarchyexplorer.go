package classification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	hierarchyIdentityTable = "__VPRO_HierarchyIdentity"
	hierarchyHistoryTable  = "__VPRO_HierarchyEditHistory"
	hierarchyHistorySQL    = `CREATE TABLE "__VPRO_HierarchyEditHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`
)

// Canonical wide phytosociological rank columns (1..11 VPro levels + WorkingUnit).
var PhytosociologicalRanks = []struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Level int64  `json:"level"`
}{
	{Key: "Class", Label: "Class", Level: 2},
	{Key: "Order", Label: "Order", Level: 4},
	{Key: "SubOrder", Label: "SubOrder", Level: 5},
	{Key: "Alliance", Label: "Alliance", Level: 6},
	{Key: "SubAlliance", Label: "SubAlliance", Level: 7},
	{Key: "Association", Label: "Association", Level: 8},
	{Key: "SubAssociation", Label: "SubAssociation", Level: 9},
	{Key: "Facies", Label: "Facies", Level: 10},
	{Key: "WorkingUnit", Label: "WorkingUnit", Level: 10},
	{Key: "SiteSeries", Label: "SiteSeries", Level: 11},
}

type HierarchyExplorerNode struct {
	ID           int64                   `json:"id"`
	Name         string                  `json:"name"`
	CleanName    string                  `json:"cleanName"`
	Parent       *int64                  `json:"parent"`
	Level        int64                   `json:"level"`
	RankName     string                  `json:"rankName"`
	Tag          string                  `json:"tag,omitempty"`
	IsBreakpoint bool                    `json:"isBreakpoint"`
	Depth        int                     `json:"depth"`
	Children     []HierarchyExplorerNode `json:"children"`
}

// FlatHierarchyRow represents one root-to-leaf classification chain in the editable wide rank table:
// Class | Order | SubOrder | Alliance | SubAlliance | Association | SubAssociation | Facies | WorkingUnit | SiteSeries
type FlatHierarchyRow struct {
	LeafID         int64  `json:"leafId"`
	Superclass     string `json:"superclass,omitempty"`
	Class          string `json:"class"`
	Subclass       string `json:"subclass,omitempty"`
	Order          string `json:"order"`
	SubOrder       string `json:"subOrder"`
	Alliance       string `json:"alliance"`
	SubAlliance    string `json:"subAlliance"`
	Association    string `json:"association"`
	SubAssociation string `json:"subAssociation"`
	Facies         string `json:"facies"`
	WorkingUnit    string `json:"workingUnit"`
	SiteSeries     string `json:"siteSeries"`
	BreakpointRank string `json:"breakpointRank,omitempty"`
}

type HierarchyDiagnostics struct {
	TotalNodes      int     `json:"totalNodes"`
	RootNodes       int     `json:"rootNodes"`
	BreakpointNodes int     `json:"breakpointNodes"`
	BlankNameNodes  int     `json:"blankNameNodes"`
	OrphanNodes     []int64 `json:"orphanNodes"`
	CycleNodeIDs    []int64 `json:"cycleNodeIds"`
}

type HierarchyExplorerView struct {
	ContextID     string                  `json:"contextId"`
	Project       string                  `json:"project"`
	Hierarchy     string                  `json:"hierarchy"`
	HierarchyPath string                  `json:"hierarchyPath"`
	TableName     string                  `json:"tableName"`
	Roots         []HierarchyExplorerNode `json:"roots"`
	FlatNodes     []HierarchyExplorerNode `json:"flatNodes"`
	FlatRankRows  []FlatHierarchyRow      `json:"flatRankRows"`
	Diagnostics   HierarchyDiagnostics    `json:"diagnostics"`
}

type CreateHierarchyTableRequest struct {
	Name        string `json:"name"`
	Destination string `json:"destination"` // "project" (default) | "new_file"
	FilePath    string `json:"filePath"`
	Activate    bool   `json:"activate"`
}

type HierarchyTableLifecycleResult struct {
	ContextID string                 `json:"contextId"`
	Name      string                 `json:"name"`
	TableName string                 `json:"tableName"`
	FilePath  string                 `json:"filePath"`
	Activated bool                   `json:"activated"`
	NewState  *vprocore.ProjectState `json:"newState,omitempty"`
}

type HierarchyAddNodeRequest struct {
	Name         string `json:"name"`
	ParentID     *int64 `json:"parentId"`
	Level        int64  `json:"level"`
	Tag          string `json:"tag,omitempty"`
	IsBreakpoint bool   `json:"isBreakpoint"`
}

type HierarchyRenameNodeRequest struct {
	NodeID  int64  `json:"nodeId"`
	NewName string `json:"newName"`
}

type HierarchyReparentNodeRequest struct {
	NodeID      int64  `json:"nodeId"`
	NewParentID *int64 `json:"newParentId"`
	NewLevel    *int64 `json:"newLevel,omitempty"`
}

type HierarchyDeleteNodeRequest struct {
	NodeID          int64 `json:"nodeId"`
	CascadeChildren bool  `json:"cascadeChildren"`
}

type HierarchyMutationResult struct {
	ContextID    string `json:"contextId"`
	Hierarchy    string `json:"hierarchy"`
	TableName    string `json:"tableName"`
	NodeID       int64  `json:"nodeId,omitempty"`
	NodeName     string `json:"nodeName,omitempty"`
	AffectedRows int    `json:"affectedRows"`
}

type SaveFlatHierarchyRequest struct {
	Rows []FlatHierarchyRow `json:"rows"`
}

func levelToRankName(level int64, tag string) string {
	switch level {
	case 1:
		return "Superclass"
	case 2:
		return "Class"
	case 3:
		return "Subclass"
	case 4:
		return "Order"
	case 5:
		return "SubOrder"
	case 6:
		return "Alliance"
	case 7:
		return "SubAlliance"
	case 8:
		return "Association"
	case 9:
		return "SubAssociation"
	case 10:
		if strings.EqualFold(strings.TrimSpace(tag), "working") {
			return "WorkingUnit"
		}
		return "Facies"
	case 11:
		return "SiteSeries"
	default:
		return "SiteSeries"
	}
}

func SplitBreakpointPrefix(name string) (clean string, isBreak bool) {
	trimmed := strings.TrimSpace(name)
	if strings.HasPrefix(trimmed, "~") {
		return strings.TrimPrefix(trimmed, "~"), true
	}
	return trimmed, false
}

type rawHierarchyRecord struct {
	id     int64
	name   string
	parent *int64
	level  int64
	tag    string
}

func DetectHierarchyCyclesAndOrphans(records []rawHierarchyRecord) (cycleIDs []int64, orphanIDs []int64, blankCount int) {
	byID := make(map[int64]rawHierarchyRecord, len(records))
	for _, r := range records {
		byID[r.id] = r
		clean, _ := SplitBreakpointPrefix(r.name)
		if strings.TrimSpace(clean) == "" {
			blankCount++
		}
	}
	cycleSet := map[int64]bool{}
	orphanSet := map[int64]bool{}

	for _, r := range records {
		if r.parent != nil {
			if _, ok := byID[*r.parent]; !ok {
				orphanSet[r.id] = true
			}
		}
		visited := []int64{r.id}
		visitedSet := map[int64]int{r.id: 0}
		curr := r.parent
		for curr != nil {
			if idx, seen := visitedSet[*curr]; seen {
				for _, cid := range visited[idx:] {
					cycleSet[cid] = true
				}
				break
			}
			nextNode, exists := byID[*curr]
			if !exists {
				break
			}
			visitedSet[*curr] = len(visited)
			visited = append(visited, *curr)
			curr = nextNode.parent
		}
	}

	for id := range cycleSet {
		cycleIDs = append(cycleIDs, id)
	}
	slices.Sort(cycleIDs)
	for id := range orphanSet {
		orphanIDs = append(orphanIDs, id)
	}
	slices.Sort(orphanIDs)
	return cycleIDs, orphanIDs, blankCount
}

func buildHierarchyTreeAndFlatRows(records []rawHierarchyRecord) ([]HierarchyExplorerNode, []HierarchyExplorerNode, []FlatHierarchyRow, HierarchyDiagnostics) {
	cycles, orphans, blanks := DetectHierarchyCyclesAndOrphans(records)
	cycleSet := map[int64]bool{}
	for _, id := range cycles {
		cycleSet[id] = true
	}
	byID := make(map[int64]rawHierarchyRecord, len(records))
	childrenByParent := map[int64][]rawHierarchyRecord{}
	var rootsRaw []rawHierarchyRecord
	breakCount := 0

	for _, r := range records {
		byID[r.id] = r
		if _, isBrk := SplitBreakpointPrefix(r.name); isBrk {
			breakCount++
		}
	}

	sortedRecords := slices.Clone(records)
	slices.SortFunc(sortedRecords, func(a, b rawHierarchyRecord) int {
		ca, _ := SplitBreakpointPrefix(a.name)
		cb, _ := SplitBreakpointPrefix(b.name)
		if cmp := strings.Compare(strings.ToLower(ca), strings.ToLower(cb)); cmp != 0 {
			return cmp
		}
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})

	for _, r := range sortedRecords {
		if cycleSet[r.id] {
			continue
		}
		if r.parent == nil {
			rootsRaw = append(rootsRaw, r)
		} else if _, exists := byID[*r.parent]; !exists {
			rootsRaw = append(rootsRaw, r) // surface orphan at root so it remains inspectable/repairable
		} else {
			childrenByParent[*r.parent] = append(childrenByParent[*r.parent], r)
		}
	}

	flatNodes := []HierarchyExplorerNode{}
	flatRows := []FlatHierarchyRow{}

	var buildSubtree func(rec rawHierarchyRecord, depth int, chain []rawHierarchyRecord) HierarchyExplorerNode
	buildSubtree = func(rec rawHierarchyRecord, depth int, chain []rawHierarchyRecord) HierarchyExplorerNode {
		clean, isBrk := SplitBreakpointPrefix(rec.name)
		lvl := rec.level
		if lvl < 1 || lvl > 11 {
			lvl = min(11, int64(depth+2))
		}
		rank := levelToRankName(lvl, rec.tag)
		nextChain := append(slices.Clone(chain), rec)

		kidsRaw := childrenByParent[rec.id]
		node := HierarchyExplorerNode{
			ID:           rec.id,
			Name:         rec.name,
			CleanName:    clean,
			Parent:       rec.parent,
			Level:        lvl,
			RankName:     rank,
			Tag:          rec.tag,
			IsBreakpoint: isBrk,
			Depth:        depth,
			Children:     make([]HierarchyExplorerNode, 0, len(kidsRaw)),
		}
		flatNodes = append(flatNodes, HierarchyExplorerNode{
			ID:           node.ID,
			Name:         node.Name,
			CleanName:    node.CleanName,
			Parent:       node.Parent,
			Level:        node.Level,
			RankName:     node.RankName,
			Tag:          node.Tag,
			IsBreakpoint: node.IsBreakpoint,
			Depth:        node.Depth,
			Children:     []HierarchyExplorerNode{},
		})

		if len(kidsRaw) == 0 {
			flatRows = append(flatRows, chainToFlatHierarchyRow(nextChain))
		} else {
			for _, child := range kidsRaw {
				node.Children = append(node.Children, buildSubtree(child, depth+1, nextChain))
			}
		}
		return node
	}

	roots := make([]HierarchyExplorerNode, 0, len(rootsRaw))
	for _, r := range rootsRaw {
		roots = append(roots, buildSubtree(r, 0, nil))
	}

	diag := HierarchyDiagnostics{
		TotalNodes:      len(records),
		RootNodes:       len(rootsRaw),
		BreakpointNodes: breakCount,
		BlankNameNodes:  blanks,
		OrphanNodes:     orphans,
		CycleNodeIDs:    cycles,
	}
	if diag.OrphanNodes == nil {
		diag.OrphanNodes = []int64{}
	}
	if diag.CycleNodeIDs == nil {
		diag.CycleNodeIDs = []int64{}
	}
	return roots, flatNodes, flatRows, diag
}

func chainToFlatHierarchyRow(chain []rawHierarchyRecord) FlatHierarchyRow {
	var row FlatHierarchyRow
	if len(chain) > 0 {
		row.LeafID = chain[len(chain)-1].id
	}
	for idx, rec := range chain {
		lvl := rec.level
		if lvl < 1 || lvl > 11 {
			if idx == len(chain)-1 {
				lvl = 11
			} else {
				lvl = min(10, int64(idx+2))
			}
		}
		_, isBrk := SplitBreakpointPrefix(rec.name)
		rank := levelToRankName(lvl, rec.tag)
		if isBrk {
			row.BreakpointRank = rank
		}
		// Keep full Name (including '~' prefix when marked as breakpoint) so flat table displays & round-trips breakpoints losslessly.
		switch rank {
		case "Superclass":
			row.Superclass = rec.name
		case "Class":
			row.Class = rec.name
		case "Subclass":
			row.Subclass = rec.name
		case "Order":
			row.Order = rec.name
		case "SubOrder":
			row.SubOrder = rec.name
		case "Alliance":
			row.Alliance = rec.name
		case "SubAlliance":
			row.SubAlliance = rec.name
		case "Association":
			row.Association = rec.name
		case "SubAssociation":
			row.SubAssociation = rec.name
		case "WorkingUnit":
			row.WorkingUnit = rec.name
		case "Facies":
			if row.Facies == "" {
				row.Facies = rec.name
			} else if row.WorkingUnit == "" {
				row.WorkingUnit = rec.name
			}
		default:
			row.SiteSeries = rec.name
		}
	}
	return row
}

func GetHierarchyExplorer(ctx context.Context, contexts *vprocore.ContextService, contextID string) (*HierarchyExplorerView, error) {
	if ctx == nil {
		return nil, errors.New("hierarchy explorer requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ *HierarchyExplorerView, resultErr error) {
		owner := plots.Projects.Sqlite
		if owner.Selection.Hierarchy == "" || owner.Selection.Hierarchy == "None" {
			return &HierarchyExplorerView{
				ContextID:    contextID,
				Project:      owner.Selection.Project,
				Hierarchy:    "None",
				Roots:        []HierarchyExplorerNode{},
				FlatNodes:    []HierarchyExplorerNode{},
				FlatRankRows: []FlatHierarchyRow{},
				Diagnostics:  HierarchyDiagnostics{OrphanNodes: []int64{}, CycleNodeIDs: []int64{}},
			}, nil
		}

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, rb)
			}
		}()

		tableName := owner.Selection.Hierarchy + "_Hierarchy"
		hasTag := false
		cols, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, "hierarchy", tableName)
		if err != nil {
			return nil, err
		}
		for _, c := range cols {
			if strings.EqualFold(c.Name, "Tag") {
				hasTag = true
			}
		}

		tagExpr := `''`
		if hasTag {
			tagExpr = `COALESCE(CAST("Tag" AS TEXT), '')`
		}
		rows, err := tx.QueryContext(ctx,
			`SELECT "ID", COALESCE(CAST("Name" AS TEXT), ''), "Parent", COALESCE("Level", 11), `+tagExpr+
				` FROM "hierarchy".`+vprocore.QuoteHeaderIdentifier(tableName)+` ORDER BY "ID" ASC`)
		if err != nil {
			return nil, err
		}
		var records []rawHierarchyRecord
		for rows.Next() {
			var r rawHierarchyRecord
			var parent sql.NullInt64
			if err := rows.Scan(&r.id, &r.name, &parent, &r.level, &r.tag); err != nil {
				rows.Close()
				return nil, err
			}
			if parent.Valid {
				p := parent.Int64
				r.parent = &p
			}
			records = append(records, r)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}

		roots, flatNodes, flatRows, diag := buildHierarchyTreeAndFlatRows(records)

		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &HierarchyExplorerView{
			ContextID:     contextID,
			Project:       owner.Selection.Project,
			Hierarchy:     owner.Selection.Hierarchy,
			HierarchyPath: owner.Selection.HierarchyPath,
			TableName:     tableName,
			Roots:         roots,
			FlatNodes:     flatNodes,
			FlatRankRows:  flatRows,
			Diagnostics:   diag,
		}, nil
	})
}

func createHierarchyTableObjectsInTx(ctx context.Context, tx *sql.Tx, hierName string) error {
	tableName := hierName + "_Hierarchy"
	var collision int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE name = ? COLLATE NOCASE`, tableName).Scan(&collision); err != nil {
		return err
	}
	if collision != 0 {
		return fmt.Errorf("hierarchy table %s already exists in target database", tableName)
	}
	qTable := vprocore.QuoteHeaderIdentifier(tableName)
	if _, err := tx.ExecContext(ctx, `CREATE TABLE main.`+qTable+` (
		"ID" INTEGER PRIMARY KEY,
		"Name" VARCHAR NOT NULL,
		"Parent" INTEGER,
		"Level" SMALLINT,
		"Tag" VARCHAR,
		"MyOrder" VARCHAR,
		"ChildID" SMALLINT,
		"StartChild" SMALLINT,
		"LastChild" SMALLINT,
		"Flag" BOOLEAN
	)`); err != nil {
		return err
	}
	for _, idx := range []struct {
		unique bool
		name   string
		col    string
	}{
		{true, "uidx_" + tableName + "_Name", "Name"},
		{false, "idx_" + tableName + "_Parent", "Parent"},
		{false, "idx_" + tableName + "_Level", "Level"},
		{false, "idx_" + tableName + "_ChildID", "ChildID"},
	} {
		kw := "CREATE INDEX "
		if idx.unique {
			kw = "CREATE UNIQUE INDEX "
		}
		if _, err := tx.ExecContext(ctx, kw+"main."+vprocore.QuoteHeaderIdentifier(idx.name)+" ON "+qTable+" ("+vprocore.QuoteHeaderIdentifier(idx.col)+")"); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main._table_metadata (table_name TEXT PRIMARY KEY, description TEXT)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO main._table_metadata (table_name, description) VALUES (?, 'VP04')`, tableName); err != nil {
		return err
	}
	return nil
}

func CreateHierarchyTable(ctx context.Context, contexts *vprocore.ContextService, contextID string, request CreateHierarchyTableRequest) (*HierarchyTableLifecycleResult, error) {
	if ctx == nil {
		return nil, errors.New("hierarchy creation requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	hierName, err := validateClassificationObjectName(request.Name, "Hierarchy")
	if err != nil {
		return nil, err
	}
	dest := strings.ToLower(strings.TrimSpace(request.Destination))
	if dest == "" {
		if strings.TrimSpace(request.FilePath) != "" {
			dest = "new_file"
		} else {
			dest = "project"
		}
	}
	if dest != "project" && dest != "new_file" {
		return nil, fmt.Errorf("unsupported hierarchy destination %q", request.Destination)
	}

	var out *HierarchyTableLifecycleResult
	var targetPath string
	var currentSelection vprocore.DesktopSelection

	_, err = vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		owner := plots.Projects.Sqlite
		currentSelection = owner.Selection
		tableName := hierName + "_Hierarchy"
		proposalBytes, err := json.Marshal(request)
		if err != nil {
			return struct{}{}, err
		}

		if dest == "project" {
			targetPath = owner.Selection.ProjectPath
			err = owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
				tx, bErr := conn.BeginTx(ctx, nil)
				if bErr != nil {
					return bErr
				}
				defer func() {
					if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
						txErr = errors.Join(txErr, rb)
					}
				}()
				if err := createHierarchyTableObjectsInTx(ctx, tx, hierName); err != nil {
					return err
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, hierarchyHistoryTable, hierarchyHistorySQL, string(proposalBytes), "hierarchy creation"); err != nil {
					return err
				}
				return tx.Commit()
			})
			if err != nil {
				return struct{}{}, err
			}
		} else {
			freshPath, _, fErr := vprocore.FreshSQLiteDestination(request.FilePath, "Hierarchy")
			if fErr != nil {
				return struct{}{}, fErr
			}
			targetPath = freshPath
			committed := false
			db, oErr := sql.Open("sqlite3", sqlutil.SqliteFileURI(freshPath, "rwc")+"&_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_txlock=immediate")
			if oErr != nil {
				return struct{}{}, oErr
			}
			defer func() {
				_ = db.Close()
				if !committed {
					_ = os.Remove(freshPath)
				}
			}()
			db.SetMaxOpenConns(1)
			tx, bErr := db.BeginTx(ctx, nil)
			if bErr != nil {
				return struct{}{}, bErr
			}
			if err := createHierarchyTableObjectsInTx(ctx, tx, hierName); err != nil {
				_ = tx.Rollback()
				return struct{}{}, err
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, hierarchyHistoryTable, hierarchyHistorySQL, string(proposalBytes), "hierarchy file creation"); err != nil {
				_ = tx.Rollback()
				return struct{}{}, err
			}
			if err := tx.Commit(); err != nil {
				return struct{}{}, err
			}
			committed = true
		}

		out = &HierarchyTableLifecycleResult{
			ContextID: contextID,
			Name:      hierName,
			TableName: tableName,
			FilePath:  targetPath,
		}
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}

	if request.Activate {
		nextState, swErr := contexts.SwitchContext(contextID, vprocore.ContextSelection{
			Project:       currentSelection.Project,
			ProjectPath:   currentSelection.ProjectPath,
			SU:            currentSelection.SU,
			SUPath:        currentSelection.SUPath,
			Hierarchy:     hierName,
			HierarchyPath: filepath.Clean(targetPath),
		})
		if swErr != nil {
			return nil, swErr
		}
		out.Activated = true
		out.ContextID = nextState.ContextID
		out.NewState = &nextState
	}
	return out, nil
}

func allocateHierarchyNodeID(ctx context.Context, tx *sql.Tx, tableName string) (int64, error) {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main.`+vprocore.QuoteHeaderIdentifier(hierarchyIdentityTable)+` (
		"HierarchyTable" TEXT NOT NULL,
		"ID" INTEGER NOT NULL,
		PRIMARY KEY ("HierarchyTable", "ID")
	)`); err != nil {
		return 0, err
	}
	qTable := vprocore.QuoteHeaderIdentifier(tableName)
	var maxID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id_val), 0) FROM (
			SELECT "ID" AS id_val FROM main.`+qTable+` WHERE "ID" > 0
			UNION
			SELECT "ID" AS id_val FROM main.`+vprocore.QuoteHeaderIdentifier(hierarchyIdentityTable)+` WHERE "HierarchyTable" = ? AND "ID" > 0
		)`,
		tableName,
	).Scan(&maxID); err != nil {
		return 0, err
	}
	if maxID >= math.MaxInt32 {
		return 0, errors.New("hierarchy node ID space exhausted")
	}
	nextID := maxID + 1
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO main.`+vprocore.QuoteHeaderIdentifier(hierarchyIdentityTable)+` ("HierarchyTable", "ID") VALUES (?, ?)`,
		tableName, nextID,
	); err != nil {
		return 0, err
	}
	return nextID, nil
}

func validateHierarchyNodeName(rawName string, isBreak bool) (string, error) {
	trimmed := strings.TrimSpace(rawName)
	if !utf8.ValidString(trimmed) || strings.ContainsRune(trimmed, 0) {
		return "", errors.New("hierarchy node name must be valid UTF-8 text without NUL")
	}
	clean := strings.TrimPrefix(trimmed, "~")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "", errors.New("hierarchy node name cannot be blank")
	}
	final := clean
	if isBreak || strings.HasPrefix(trimmed, "~") {
		final = "~" + clean
	}
	if len(utf16.Encode([]rune(final))) > 255 {
		return "", errors.New("hierarchy node name exceeds 255 UTF-16 code units")
	}
	return final, nil
}

func withHierarchyWriter(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	opName string,
	mutate func(owner *vprocore.SqliteContext, tx *sql.Tx, hierName, tableName string) (*HierarchyMutationResult, any, error),
) (*HierarchyMutationResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%s requires a request context", opName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*HierarchyMutationResult, error) {
		owner := plots.Projects.Sqlite
		if owner.Selection.Hierarchy == "" || owner.Selection.Hierarchy == "None" {
			return nil, errors.New("an active Hierarchy table must be selected")
		}
		hierName := owner.Selection.Hierarchy
		tableName := hierName + "_Hierarchy"

		validateFiles := func() error {
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return err
			}
			hInfo := owner.AttachmentInfo["hierarchy"]
			if hInfo == nil {
				return errors.New("active Hierarchy file is not attached")
			}
			for role, info := range owner.AttachmentInfo {
				if strings.HasPrefix(role, "V") && os.SameFile(hInfo, info) {
					return fmt.Errorf("hierarchy database shares a file with support role %s; writes are prohibited", role)
				}
			}
			return nil
		}

		var out *HierarchyMutationResult
		err := owner.WithSelectedFileWriter(ctx, "hierarchy", validateFiles, func(conn *sql.Conn) (txErr error) {
			tx, bErr := conn.BeginTx(ctx, nil) // BEGIN IMMEDIATE
			if bErr != nil {
				return bErr
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			res, proposal, mErr := mutate(owner, tx, hierName, tableName)
			if mErr != nil {
				return mErr
			}
			if err := ensureHierarchyTableAcyclicAndValid(ctx, tx, tableName); err != nil {
				return err
			}
			rawProp, jErr := json.Marshal(struct {
				Op       string `json:"op"`
				Proposal any    `json:"proposal"`
			}{opName, proposal})
			if jErr != nil {
				return jErr
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, hierarchyHistoryTable, hierarchyHistorySQL, string(rawProp), opName); err != nil {
				return err
			}
			if err := validateFiles(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			res.ContextID = contextID
			res.Hierarchy = hierName
			res.TableName = tableName
			out = res
			return nil
		})
		if err != nil {
			return nil, err
		}
		return out, nil
	})
}

func ensureHierarchyTableAcyclicAndValid(ctx context.Context, tx *sql.Tx, tableName string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT "ID", COALESCE(CAST("Name" AS TEXT), ''), "Parent", COALESCE("Level", 11)
		 FROM main.`+vprocore.QuoteHeaderIdentifier(tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	var records []rawHierarchyRecord
	seenNames := map[string]int64{}
	for rows.Next() {
		var r rawHierarchyRecord
		var parent sql.NullInt64
		if err := rows.Scan(&r.id, &r.name, &parent, &r.level); err != nil {
			return err
		}
		if parent.Valid {
			p := parent.Int64
			r.parent = &p
		}
		lowerName := strings.ToLower(strings.TrimSpace(r.name))
		if existingID, dup := seenNames[lowerName]; dup {
			return fmt.Errorf("duplicate hierarchy node name %q (nodes %d and %d)", r.name, existingID, r.id)
		}
		seenNames[lowerName] = r.id
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	cycles, orphans, blanks := DetectHierarchyCyclesAndOrphans(records)
	if blanks > 0 {
		return errors.New("hierarchy cannot contain blank node names")
	}
	if len(orphans) > 0 {
		return fmt.Errorf("hierarchy contains orphan parent reference on node(s) %v", orphans)
	}
	if len(cycles) > 0 {
		return fmt.Errorf("hierarchy mutation rejected: cycle detected involving node(s) %v", cycles)
	}
	return nil
}

func hierarchyTableHasTagColumn(ctx context.Context, tx *sql.Tx, tableName string) bool {
	cols, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, "main", tableName)
	if err != nil {
		return false
	}
	for _, c := range cols {
		if strings.EqualFold(c.Name, "Tag") {
			return true
		}
	}
	return false
}

func AddHierarchyNode(ctx context.Context, contexts *vprocore.ContextService, contextID string, request HierarchyAddNodeRequest) (*HierarchyMutationResult, error) {
	finalName, err := validateHierarchyNodeName(request.Name, request.IsBreakpoint)
	if err != nil {
		return nil, err
	}
	lvl := request.Level
	if lvl == 0 {
		lvl = 11
	}
	if lvl < 1 || lvl > 11 {
		return nil, fmt.Errorf("hierarchy Level must be between 1 and 11, got %d", lvl)
	}

	return withHierarchyWriter(ctx, contexts, contextID, "hierarchy_add_node", func(_ *vprocore.SqliteContext, tx *sql.Tx, _, tableName string) (*HierarchyMutationResult, any, error) {
		qTable := vprocore.QuoteHeaderIdentifier(tableName)
		if request.ParentID != nil {
			var pExists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.`+qTable+` WHERE "ID" = ?)`, *request.ParentID).Scan(&pExists); err != nil {
				return nil, nil, err
			}
			if !pExists {
				return nil, nil, fmt.Errorf("parent hierarchy node %d does not exist", *request.ParentID)
			}
		}
		newID, err := allocateHierarchyNodeID(ctx, tx, tableName)
		if err != nil {
			return nil, nil, err
		}
		if hierarchyTableHasTagColumn(ctx, tx, tableName) && strings.TrimSpace(request.Tag) != "" {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO main.`+qTable+` ("ID","Name","Parent","Level","Tag") VALUES (?,?,?,?,?)`,
				newID, finalName, request.ParentID, lvl, strings.TrimSpace(request.Tag),
			); err != nil {
				return nil, nil, err
			}
		} else {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO main.`+qTable+` ("ID","Name","Parent","Level") VALUES (?,?,?,?)`,
				newID, finalName, request.ParentID, lvl,
			); err != nil {
				return nil, nil, err
			}
		}
		return &HierarchyMutationResult{
			NodeID:       newID,
			NodeName:     finalName,
			AffectedRows: 1,
		}, request, nil
	})
}

func RenameHierarchyNode(ctx context.Context, contexts *vprocore.ContextService, contextID string, request HierarchyRenameNodeRequest) (*HierarchyMutationResult, error) {
	finalName, err := validateHierarchyNodeName(request.NewName, false)
	if err != nil {
		return nil, err
	}
	return withHierarchyWriter(ctx, contexts, contextID, "hierarchy_rename_node", func(_ *vprocore.SqliteContext, tx *sql.Tx, _, tableName string) (*HierarchyMutationResult, any, error) {
		qTable := vprocore.QuoteHeaderIdentifier(tableName)
		res, err := tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "Name" = ? WHERE "ID" = ?`, finalName, request.NodeID)
		if err != nil {
			return nil, nil, err
		}
		n, err := res.RowsAffected()
		if err != nil || n != 1 {
			return nil, nil, fmt.Errorf("hierarchy node %d does not exist", request.NodeID)
		}
		return &HierarchyMutationResult{
			NodeID:       request.NodeID,
			NodeName:     finalName,
			AffectedRows: 1,
		}, request, nil
	})
}

func ReparentHierarchyNode(ctx context.Context, contexts *vprocore.ContextService, contextID string, request HierarchyReparentNodeRequest) (*HierarchyMutationResult, error) {
	if request.NewParentID != nil && *request.NewParentID == request.NodeID {
		return nil, fmt.Errorf("hierarchy cycle detected: node %d cannot be its own parent", request.NodeID)
	}
	if request.NewLevel != nil && (*request.NewLevel < 1 || *request.NewLevel > 11) {
		return nil, fmt.Errorf("hierarchy Level must be between 1 and 11, got %d", *request.NewLevel)
	}

	return withHierarchyWriter(ctx, contexts, contextID, "hierarchy_reparent_node", func(_ *vprocore.SqliteContext, tx *sql.Tx, _, tableName string) (*HierarchyMutationResult, any, error) {
		qTable := vprocore.QuoteHeaderIdentifier(tableName)
		if request.NewParentID != nil {
			var pExists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.`+qTable+` WHERE "ID" = ?)`, *request.NewParentID).Scan(&pExists); err != nil {
				return nil, nil, err
			}
			if !pExists {
				return nil, nil, fmt.Errorf("target parent node %d does not exist", *request.NewParentID)
			}
		}

		var res sql.Result
		var err error
		if request.NewLevel != nil {
			res, err = tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "Parent" = ?, "Level" = ? WHERE "ID" = ?`, request.NewParentID, *request.NewLevel, request.NodeID)
		} else {
			res, err = tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "Parent" = ? WHERE "ID" = ?`, request.NewParentID, request.NodeID)
		}
		if err != nil {
			return nil, nil, err
		}
		n, err := res.RowsAffected()
		if err != nil || n != 1 {
			return nil, nil, fmt.Errorf("hierarchy node %d does not exist", request.NodeID)
		}
		return &HierarchyMutationResult{
			NodeID:       request.NodeID,
			AffectedRows: 1,
		}, request, nil
	})
}

func ToggleHierarchyBreakpoint(ctx context.Context, contexts *vprocore.ContextService, contextID string, nodeID int64) (*HierarchyMutationResult, error) {
	return withHierarchyWriter(ctx, contexts, contextID, "hierarchy_toggle_breakpoint", func(_ *vprocore.SqliteContext, tx *sql.Tx, _, tableName string) (*HierarchyMutationResult, any, error) {
		qTable := vprocore.QuoteHeaderIdentifier(tableName)
		var currentName string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(CAST("Name" AS TEXT), '') FROM main.`+qTable+` WHERE "ID" = ?`, nodeID).Scan(&currentName); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, fmt.Errorf("hierarchy node %d does not exist", nodeID)
			}
			return nil, nil, err
		}
		clean, isBrk := SplitBreakpointPrefix(currentName)
		nextName, err := validateHierarchyNodeName(clean, !isBrk)
		if err != nil {
			return nil, nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "Name" = ? WHERE "ID" = ?`, nextName, nodeID); err != nil {
			return nil, nil, err
		}
		return &HierarchyMutationResult{
			NodeID:       nodeID,
			NodeName:     nextName,
			AffectedRows: 1,
		}, map[string]any{"nodeId": nodeID, "before": currentName, "after": nextName}, nil
	})
}

func DeleteHierarchyNode(ctx context.Context, contexts *vprocore.ContextService, contextID string, request HierarchyDeleteNodeRequest) (*HierarchyMutationResult, error) {
	return withHierarchyWriter(ctx, contexts, contextID, "hierarchy_delete_node", func(_ *vprocore.SqliteContext, tx *sql.Tx, _, tableName string) (*HierarchyMutationResult, any, error) {
		qTable := vprocore.QuoteHeaderIdentifier(tableName)
		var parent sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT "Parent" FROM main.`+qTable+` WHERE "ID" = ?`, request.NodeID).Scan(&parent); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, fmt.Errorf("hierarchy node %d does not exist", request.NodeID)
			}
			return nil, nil, err
		}

		// Reserve all existing positive IDs in __VPRO_HierarchyIdentity before deleting so deleted IDs are never reused!
		if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main.`+vprocore.QuoteHeaderIdentifier(hierarchyIdentityTable)+` (
			"HierarchyTable" TEXT NOT NULL,
			"ID" INTEGER NOT NULL,
			PRIMARY KEY ("HierarchyTable", "ID")
		)`); err != nil {
			return nil, nil, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO main.`+vprocore.QuoteHeaderIdentifier(hierarchyIdentityTable)+` ("HierarchyTable","ID")
			 SELECT ?, "ID" FROM main.`+qTable+` WHERE "ID" > 0`,
			tableName,
		); err != nil {
			return nil, nil, err
		}

		affected := 0
		if request.CascadeChildren {
			// Collect full descendant subtree iteratively
			toDelete := []int64{request.NodeID}
			seen := map[int64]bool{request.NodeID: true}
			for idx := 0; idx < len(toDelete); idx++ {
				curr := toDelete[idx]
				rows, err := tx.QueryContext(ctx, `SELECT "ID" FROM main.`+qTable+` WHERE "Parent" = ?`, curr)
				if err != nil {
					return nil, nil, err
				}
				for rows.Next() {
					var cid int64
					if err := rows.Scan(&cid); err != nil {
						rows.Close()
						return nil, nil, err
					}
					if !seen[cid] {
						seen[cid] = true
						toDelete = append(toDelete, cid)
					}
				}
				rows.Close()
			}
			for i := len(toDelete) - 1; i >= 0; i-- {
				res, err := tx.ExecContext(ctx, `DELETE FROM main.`+qTable+` WHERE "ID" = ?`, toDelete[i])
				if err != nil {
					return nil, nil, err
				}
				n, _ := res.RowsAffected()
				affected += int(n)
			}
		} else {
			var parentPtr *int64
			if parent.Valid {
				p := parent.Int64
				parentPtr = &p
			}
			if _, err := tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "Parent" = ? WHERE "Parent" = ?`, parentPtr, request.NodeID); err != nil {
				return nil, nil, err
			}
			res, err := tx.ExecContext(ctx, `DELETE FROM main.`+qTable+` WHERE "ID" = ?`, request.NodeID)
			if err != nil {
				return nil, nil, err
			}
			n, _ := res.RowsAffected()
			affected = int(n)
		}

		return &HierarchyMutationResult{
			NodeID:       request.NodeID,
			AffectedRows: affected,
		}, request, nil
	})
}

// SaveFlatHierarchyRows synchronizes edited wide phytosociological rank rows
// (Class | Order | SubOrder | Alliance | SubAlliance | Association | SubAssociation | Facies | WorkingUnit | SiteSeries)
// back into the <Name>_Hierarchy adjacency list inside an atomic BEGIN IMMEDIATE transaction.
func SaveFlatHierarchyRows(ctx context.Context, contexts *vprocore.ContextService, contextID string, request SaveFlatHierarchyRequest) (*HierarchyMutationResult, error) {
	return withHierarchyWriter(ctx, contexts, contextID, "hierarchy_save_flat_rows", func(_ *vprocore.SqliteContext, tx *sql.Tx, _, tableName string) (*HierarchyMutationResult, any, error) {
		qTable := vprocore.QuoteHeaderIdentifier(tableName)
		hasTag := hierarchyTableHasTagColumn(ctx, tx, tableName)

		// Preserve existing node IDs by clean/raw name for stability, and ensure deleted IDs stay reserved.
		if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main.`+vprocore.QuoteHeaderIdentifier(hierarchyIdentityTable)+` (
			"HierarchyTable" TEXT NOT NULL,
			"ID" INTEGER NOT NULL,
			PRIMARY KEY ("HierarchyTable", "ID")
		)`); err != nil {
			return nil, nil, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO main.`+vprocore.QuoteHeaderIdentifier(hierarchyIdentityTable)+` ("HierarchyTable","ID")
			 SELECT ?, "ID" FROM main.`+qTable+` WHERE "ID" > 0`,
			tableName,
		); err != nil {
			return nil, nil, err
		}

		existingIDByClean := map[string]int64{}
		rows, err := tx.QueryContext(ctx, `SELECT "ID", COALESCE(CAST("Name" AS TEXT), '') FROM main.`+qTable)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var id int64
			var nm string
			if err := rows.Scan(&id, &nm); err != nil {
				rows.Close()
				return nil, nil, err
			}
			clean, _ := SplitBreakpointPrefix(nm)
			if clean != "" {
				existingIDByClean[strings.ToLower(clean)] = id
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, nil, err
		}

		type desiredNode struct {
			id     int64
			name   string
			parent *int64
			level  int64
			tag    string
		}
		desiredByClean := map[string]*desiredNode{}
		desiredOrder := []string{}

		for rowIdx, flat := range request.Rows {
			steps := []struct {
				raw   string
				level int64
				tag   string
			}{
				{flat.Superclass, 1, ""},
				{flat.Class, 2, ""},
				{flat.Subclass, 3, ""},
				{flat.Order, 4, ""},
				{flat.SubOrder, 5, ""},
				{flat.Alliance, 6, ""},
				{flat.SubAlliance, 7, ""},
				{flat.Association, 8, ""},
				{flat.SubAssociation, 9, ""},
				{flat.Facies, 10, ""},
				{flat.WorkingUnit, 10, "working"},
				{flat.SiteSeries, 11, ""},
			}

			var parentID *int64
			for _, step := range steps {
				if strings.TrimSpace(step.raw) == "" {
					continue
				}
				validatedName, vErr := validateHierarchyNodeName(step.raw, false)
				if vErr != nil {
					return nil, nil, fmt.Errorf("flat row %d: %w", rowIdx+1, vErr)
				}
				clean, _ := SplitBreakpointPrefix(validatedName)
				key := strings.ToLower(clean)

				existing := desiredByClean[key]
				if existing != nil {
					// Verify consistent parent across flat rows!
					sameParent := (existing.parent == nil && parentID == nil) ||
						(existing.parent != nil && parentID != nil && *existing.parent == *parentID)
					if !sameParent {
						return nil, nil, fmt.Errorf("flat row %d places node %q under conflicting parents in the same hierarchy", rowIdx+1, clean)
					}
					if strings.HasPrefix(validatedName, "~") {
						existing.name = validatedName
					}
					idCopy := existing.id
					parentID = &idCopy
					continue
				}

				id, ok := existingIDByClean[key]
				if !ok {
					allocated, aErr := allocateHierarchyNodeID(ctx, tx, tableName)
					if aErr != nil {
						return nil, nil, aErr
					}
					id = allocated
				}
				var pCopy *int64
				if parentID != nil {
					v := *parentID
					pCopy = &v
				}
				node := &desiredNode{
					id:     id,
					name:   validatedName,
					parent: pCopy,
					level:  step.level,
					tag:    step.tag,
				}
				desiredByClean[key] = node
				desiredOrder = append(desiredOrder, key)
				idCopy := id
				parentID = &idCopy
			}
		}

		// Rewrite hierarchy table contents atomically inside tx
		if _, err := tx.ExecContext(ctx, `DELETE FROM main.`+qTable); err != nil {
			return nil, nil, err
		}
		for _, key := range desiredOrder {
			n := desiredByClean[key]
			if hasTag {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO main.`+qTable+` ("ID","Name","Parent","Level","Tag") VALUES (?,?,?,?,?)`,
					n.id, n.name, n.parent, n.level, nullStringIfEmpty(n.tag),
				); err != nil {
					return nil, nil, err
				}
			} else {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO main.`+qTable+` ("ID","Name","Parent","Level") VALUES (?,?,?,?)`,
					n.id, n.name, n.parent, n.level,
				); err != nil {
					return nil, nil, err
				}
			}
		}

		return &HierarchyMutationResult{
			AffectedRows: len(desiredOrder),
		}, request, nil
	})
}

func nullStringIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
