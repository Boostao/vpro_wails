package projectops

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type NewProjectRequest struct {
	ProjectName       string `json:"projectName"`
	ProjectID         string `json:"projectId,omitempty"`
	ProjectTitle      string `json:"projectTitle,omitempty"`
	PresetID          string `json:"presetId,omitempty"`
	DestinationPath   string `json:"destinationPath,omitempty"`
	ActivateOnSuccess bool   `json:"activateOnSuccess"`
}

type NewProjectResult struct {
	ProjectName     string                 `json:"projectName"`
	ProjectID       string                 `json:"projectId"`
	DestinationPath string                 `json:"destinationPath"`
	DestinationSHA  string                 `json:"destinationSha256"`
	Activated       bool                   `json:"activated"`
	State           *vprocore.ProjectState `json:"state,omitempty"`
}

// CreateNewProject creates a canonical VP08 SQLite <Name>.db project from VPro64.USys*Table
// templates, tags all 8 core tables 'VP08' in _table_metadata, inserts initial <Name>_Metadata
// and <Name>_Audit provenance rows, publishes atomically via os.Link without overwriting,
// and optionally switches the active context.
func CreateNewProject(s *vprocore.ContextService, ctx context.Context, contextID string, req NewProjectRequest) (_ NewProjectResult, resultErr error) {
	projectName, err := ValidateProjectName(req.ProjectName)
	if err != nil {
		return NewProjectResult{}, err
	}
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		projectID = projectName
		if utf16Len(projectID) > 20 {
			projectID = string([]rune(projectID)[:20])
		}
	}
	if err := validateUTF16Max("Metadata.ProjectID", projectID, 20); err != nil {
		return NewProjectResult{}, err
	}
	if err := validateUTF16Max("Metadata.ProjectTitle", req.ProjectTitle, 255); err != nil {
		return NewProjectResult{}, err
	}

	var preset ProjectMetadataPreset
	hasPreset := false
	if strings.TrimSpace(req.PresetID) != "" {
		var ok bool
		preset, ok = GetProjectMetadataPreset(req.PresetID)
		if !ok {
			return NewProjectResult{}, fmt.Errorf("unknown metadata preset %q", req.PresetID)
		}
		hasPreset = true
	}

	snap, err := inspectContextSnapshot(ctx, s, contextID)
	if err != nil {
		return NewProjectResult{}, err
	}
	resolvedDest, parentInfo, err := resolveNewProjectDestination(filepath.Join(snap.Root, "projects"), projectName, req.DestinationPath, "New project")
	if err != nil {
		return NewProjectResult{}, err
	}

	review, err := s.ReadNewProjectTemplates(ctx, contextID)
	if err != nil {
		return NewProjectResult{}, err
	}
	ddlPlan, err := vprocore.PlanNewProjectDDL(ctx, projectName, review)
	if err != nil {
		return NewProjectResult{}, err
	}
	allSpecsStamp, listsStamp, _, err := readVListsVersionStamps(ctx, snap.VListsPath)
	if err != nil {
		return NewProjectResult{}, err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(resolvedDest), ".vpro-new-project-*.db")
	if err != nil {
		return NewProjectResult{}, err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return NewProjectResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		return NewProjectResult{}, err
	}

	sqlDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL&_txlock=immediate")
	if err != nil {
		return NewProjectResult{}, err
	}
	sqlDB.SetMaxOpenConns(1)
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		_ = sqlDB.Close()
		return NewProjectResult{}, err
	}

	buildErr := func() error {
		for _, tblPlan := range ddlPlan.Tables {
			for _, mapping := range tblPlan.Schema {
				if mapping.SQL != nil {
					if _, err := tx.ExecContext(ctx, *mapping.SQL); err != nil {
						return fmt.Errorf("create %s: %w", tblPlan.TargetTable, err)
					}
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT)`); err != nil {
			return err
		}
		for _, suffix := range vprocore.CoreTables {
			if _, err := tx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP08')`, projectName+"_"+suffix); err != nil {
				return err
			}
		}
		if err := ensureChildIdentityTable(ctx, tx); err != nil {
			return err
		}
		if err := reserveChildID(ctx, tx, projectName+"_Metadata", 1); err != nil {
			return err
		}

		now := time.Now().Format("2006-01-02 15:04:05")
		metaCols := []string{"ID", "ProjectID", "AllSpecs", "TableOfLists", "DateLastEdited"}
		metaVals := []any{int64(1), projectID, allSpecsStamp, listsStamp, now}
		if strings.TrimSpace(req.ProjectTitle) != "" {
			metaCols = append(metaCols, "ProjectTitle")
			metaVals = append(metaVals, req.ProjectTitle)
		}
		if hasPreset {
			metaCols = append(metaCols, "EcosysCollectionStandard", "VegCoverMethod", "PlotMethod", "Datum")
			metaVals = append(metaVals, preset.EcosysCollectionStandard, preset.VegCoverMethod, preset.PlotMethod, preset.Datum)
			for _, col := range coverDescriptionColumns {
				if val, ok := preset.CoverDescriptions[col]; ok {
					metaCols = append(metaCols, col)
					metaVals = append(metaVals, val)
				}
			}
		}
		quotedMetaCols := make([]string, len(metaCols))
		marks := make([]string, len(metaCols))
		for i, c := range metaCols {
			quotedMetaCols[i] = vprocore.QuoteHeaderIdentifier(c)
			marks[i] = "?"
		}
		metaInsertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			vprocore.QuoteHeaderIdentifier(projectName+"_Metadata"),
			strings.Join(quotedMetaCols, ", "),
			strings.Join(marks, ", "),
		)
		if _, err := tx.ExecContext(ctx, metaInsertSQL, metaVals...); err != nil {
			return fmt.Errorf("insert initial metadata row: %w", err)
		}

		auditTbl := vprocore.QuoteHeaderIdentifier(projectName + "_Audit")
		for _, entry := range []struct {
			table string
			after any
		}{
			{"NewProject", nil},
			{"USysAllSpecs", allSpecsStamp},
			{"USysTableOfLists", listsStamp},
		} {
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+auditTbl+` ("Project","User","Table","EditWhen","AfterEdit","Restore","Flag") VALUES (?,?,?,?,?,0,0)`,
				projectName, snap.CurrentUser, entry.table, now, entry.after); err != nil {
				return err
			}
		}
		return ctx.Err()
	}()
	if buildErr != nil {
		_ = tx.Rollback()
		_ = sqlDB.Close()
		return NewProjectResult{}, buildErr
	}
	if err := tx.Commit(); err != nil {
		_ = sqlDB.Close()
		return NewProjectResult{}, err
	}
	if err := sqlDB.Close(); err != nil {
		return NewProjectResult{}, err
	}

	if err := verifyStagedVP08Project(ctx, tmpPath, projectName); err != nil {
		return NewProjectResult{}, err
	}
	if err := publishStagedDatabase(tmpPath, resolvedDest, parentInfo); err != nil {
		return NewProjectResult{}, err
	}
	published = true
	_ = os.Remove(tmpPath)

	shaHex, err := computeFileSHA256(resolvedDest)
	if err != nil {
		return NewProjectResult{}, err
	}
	res := NewProjectResult{
		ProjectName:     projectName,
		ProjectID:       projectID,
		DestinationPath: resolvedDest,
		DestinationSHA:  shaHex,
	}
	if req.ActivateOnSuccess {
		state, swErr := s.SwitchContext(contextID, vprocore.ContextSelection{
			Project:     projectName,
			ProjectPath: resolvedDest,
			SU:          "None",
			Hierarchy:   "None",
		})
		if swErr != nil {
			return res, fmt.Errorf("new project published at %s, but context switch failed: %w", resolvedDest, swErr)
		}
		res.Activated = true
		res.State = &state
	}
	return res, nil
}

type SaveProjectAsRequest struct {
	NewProjectName    string `json:"newProjectName"`
	DestinationPath   string `json:"destinationPath,omitempty"`
	ActivateOnSuccess bool   `json:"activateOnSuccess"`
}

type SaveProjectAsResult struct {
	SourceProject   string                 `json:"sourceProject"`
	NewProjectName  string                 `json:"newProjectName"`
	DestinationPath string                 `json:"destinationPath"`
	DestinationSHA  string                 `json:"destinationSha256"`
	CopiedTables    []string               `json:"copiedTables"`
	CompanionTables []string               `json:"companionTables"`
	Activated       bool                   `json:"activated"`
	State           *vprocore.ProjectState `json:"state,omitempty"`
}

var companionProjectSuffixes = []string{"SU", "Hierarchy", "Lump", "Profile", "Herbarium", "Theme"}

func rewriteProjectObjectSQL(originalSQL, oldPrefix, newPrefix, oldTable, newTable string) string {
	unquotedCreate := regexp.MustCompile(`(?i)^(\s*CREATE\s+TABLE\s+)` + regexp.QuoteMeta(oldTable) + `(\b)`)
	normalized := unquotedCreate.ReplaceAllString(originalSQL, "${1}"+vprocore.QuoteHeaderIdentifier(oldTable)+"${2}")
	replacer := strings.NewReplacer(
		vprocore.QuoteHeaderIdentifier(oldTable), vprocore.QuoteHeaderIdentifier(newTable),
		vprocore.QuoteHeaderIdentifier(oldPrefix+"_Env"), vprocore.QuoteHeaderIdentifier(newPrefix+"_Env"),
		oldPrefix+"_", newPrefix+"_",
	)
	return replacer.Replace(normalized)
}

func rewriteProjectIndexSQL(originalSQL, idxName, oldPrefix, newPrefix, oldTable, newTable string) string {
	newIdxName := strings.ReplaceAll(idxName, oldPrefix+"_", newPrefix+"_")
	if newIdxName == idxName {
		newIdxName = newTable + "__" + idxName
	}
	unquotedIdx := regexp.MustCompile(`(?i)^(\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?)` + regexp.QuoteMeta(idxName) + `(\b)`)
	normalized := unquotedIdx.ReplaceAllString(originalSQL, "${1}"+vprocore.QuoteHeaderIdentifier(idxName)+"${2}")
	replacer := strings.NewReplacer(
		vprocore.QuoteHeaderIdentifier(idxName), vprocore.QuoteHeaderIdentifier(newIdxName),
		vprocore.QuoteHeaderIdentifier(oldTable), vprocore.QuoteHeaderIdentifier(newTable),
		vprocore.QuoteHeaderIdentifier(oldPrefix+"_Env"), vprocore.QuoteHeaderIdentifier(newPrefix+"_Env"),
		oldPrefix+"_", newPrefix+"_",
	)
	return replacer.Replace(normalized)
}

func rewriteAuditProjectCells(cols []string, rows [][]rawSQLiteCell, targetProject string) {
	projIdx := -1
	for i, c := range cols {
		if strings.EqualFold(c, "Project") {
			projIdx = i
			break
		}
	}
	if projIdx < 0 {
		return
	}
	targetBytes := []byte(targetProject)
	for i := range rows {
		cp := make([]byte, len(targetBytes))
		copy(cp, targetBytes)
		rows[i][projIdx] = rawSQLiteCell{
			storage: "text",
			bytes:   cp,
		}
	}
}

// SaveProjectAs performs a lossless atomic copy of the active <Project>.db renaming
// <OldPrefix>_* tables, indexes, foreign keys, and _table_metadata to <NewPrefix>_*
// while preserving companion _SU, _Hierarchy, _Lump, _Profile tables and VP08 tags.
func SaveProjectAs(s *vprocore.ContextService, ctx context.Context, contextID string, req SaveProjectAsRequest) (_ SaveProjectAsResult, resultErr error) {
	newName, err := ValidateProjectName(req.NewProjectName)
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	snap, err := inspectContextSnapshot(ctx, s, contextID)
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	oldName := snap.Project
	resolvedDest, parentInfo, err := resolveNewProjectDestination(filepath.Join(snap.Root, "projects"), newName, req.DestinationPath, "Save project as")
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	if vprocore.SameDesktopPath(resolvedDest, snap.ProjectPath) {
		return SaveProjectAsResult{}, errors.New("destination database path must differ from the active project database")
	}

	srcDB, err := sqlutil.OpenReadOnlyContext(ctx, snap.ProjectPath)
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	defer srcDB.Close()
	srcTx, err := srcDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	defer func() { _ = srcTx.Rollback() }()

	type tableCopySpec struct {
		suffix     string
		companion  bool
		oldTable   string
		newTable   string
		createSQL  string
		indexSQLs  []string
		columns    []string
		rows       [][]rawSQLiteCell
		metaTag    string
		hasMetaTag bool
	}

	existingTables := map[string]string{}
	masterRows, err := srcTx.QueryContext(ctx, `SELECT name, sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	for masterRows.Next() {
		var name, ddl string
		if err := masterRows.Scan(&name, &ddl); err != nil {
			masterRows.Close()
			return SaveProjectAsResult{}, err
		}
		existingTables[name] = ddl
	}
	if err := errors.Join(masterRows.Err(), masterRows.Close()); err != nil {
		return SaveProjectAsResult{}, err
	}

	metaTags := map[string]string{}
	if _, hasMeta := existingTables["_table_metadata"]; hasMeta {
		mRows, mErr := srcTx.QueryContext(ctx, `SELECT table_name, COALESCE(description, '') FROM "_table_metadata"`)
		if mErr != nil {
			return SaveProjectAsResult{}, mErr
		}
		for mRows.Next() {
			var tName, desc string
			if err := mRows.Scan(&tName, &desc); err != nil {
				mRows.Close()
				return SaveProjectAsResult{}, err
			}
			metaTags[tName] = desc
		}
		if err := errors.Join(mRows.Err(), mRows.Close()); err != nil {
			return SaveProjectAsResult{}, err
		}
	}

	// Order Env first so FK references from Admin/Veg/Humus/Mineral/Other/Audit resolve cleanly!
	orderedSuffixes := []struct {
		suffix    string
		companion bool
	}{
		{"Env", false},
		{"Admin", false},
		{"Veg", false},
		{"Humus", false},
		{"Mineral", false},
		{"Other", false},
		{"Audit", false},
		{"Metadata", false},
	}
	for _, comp := range companionProjectSuffixes {
		orderedSuffixes = append(orderedSuffixes, struct {
			suffix    string
			companion bool
		}{comp, true})
	}

	var specs []tableCopySpec
	var copiedTables, companionTables []string
	hasCompanionSU := false

	for _, item := range orderedSuffixes {
		oldTbl := oldName + "_" + item.suffix
		ddl, present := existingTables[oldTbl]
		if !present {
			if !item.companion {
				return SaveProjectAsResult{}, fmt.Errorf("active project %s is missing core table %s", oldName, oldTbl)
			}
			continue
		}
		newTbl := newName + "_" + item.suffix
		rewrittenTableSQL := rewriteProjectObjectSQL(ddl, oldName, newName, oldTbl, newTbl)

		idxRows, idxErr := srcTx.QueryContext(ctx, `SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql IS NOT NULL ORDER BY name`, oldTbl)
		if idxErr != nil {
			return SaveProjectAsResult{}, idxErr
		}
		var idxSQLs []string
		for idxRows.Next() {
			var idxName, idxSQL string
			if err := idxRows.Scan(&idxName, &idxSQL); err != nil {
				idxRows.Close()
				return SaveProjectAsResult{}, err
			}
			rewrittenIdx := rewriteProjectIndexSQL(idxSQL, idxName, oldName, newName, oldTbl, newTbl)
			idxSQLs = append(idxSQLs, rewrittenIdx)
		}
		if err := errors.Join(idxRows.Err(), idxRows.Close()); err != nil {
			return SaveProjectAsResult{}, err
		}

		cols, tableRows, readErr := readExactTableRows(ctx, srcTx, vprocore.QuoteHeaderIdentifier(oldTbl), "")
		if readErr != nil {
			return SaveProjectAsResult{}, readErr
		}
		if item.suffix == "Audit" {
			rewriteAuditProjectCells(cols, tableRows, newName)
		}
		tag, hasTag := metaTags[oldTbl]
		if !item.companion && (tag == "" || item.suffix == "Env") {
			tag = "VP08"
			hasTag = true
		}

		specs = append(specs, tableCopySpec{
			suffix:     item.suffix,
			companion:  item.companion,
			oldTable:   oldTbl,
			newTable:   newTbl,
			createSQL:  rewrittenTableSQL,
			indexSQLs:  idxSQLs,
			columns:    cols,
			rows:       tableRows,
			metaTag:    tag,
			hasMetaTag: hasTag,
		})
		copiedTables = append(copiedTables, newTbl)
		if item.companion {
			companionTables = append(companionTables, newTbl)
			if item.suffix == "SU" {
				hasCompanionSU = true
			}
		}
	}

	// Capture __VPRO_ChildIdentity reservations if present.
	type childIdEntry struct {
		childTable string
		id         int64
	}
	var childIds []childIdEntry
	if _, ok := existingTables["__VPRO_ChildIdentity"]; ok {
		cRows, cErr := srcTx.QueryContext(ctx, `SELECT "ChildTable", "ID" FROM "__VPRO_ChildIdentity"`)
		if cErr != nil {
			return SaveProjectAsResult{}, cErr
		}
		for cRows.Next() {
			var ct string
			var cid int64
			if err := cRows.Scan(&ct, &cid); err != nil {
				cRows.Close()
				return SaveProjectAsResult{}, err
			}
			unquoted := strings.Trim(ct, `"`)
			if after, matched := strings.CutPrefix(unquoted, oldName+"_"); matched {
				ct = vprocore.QuoteHeaderIdentifier(newName + "_" + after)
			}
			childIds = append(childIds, childIdEntry{ct, cid})
		}
		if err := errors.Join(cRows.Err(), cRows.Close()); err != nil {
			return SaveProjectAsResult{}, err
		}
	}

	// Capture _vpro_su_policy for <Old>_SU if present.
	var suPolicyKind string
	if _, ok := existingTables["_vpro_su_policy"]; ok && hasCompanionSU {
		_ = srcTx.QueryRowContext(ctx, `SELECT kind FROM "_vpro_su_policy" WHERE table_name = ?`, oldName+"_SU").Scan(&suPolicyKind)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(resolvedDest), ".vpro-saveas-*.db")
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return SaveProjectAsResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		return SaveProjectAsResult{}, err
	}

	dstDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL&_txlock=immediate")
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	dstDB.SetMaxOpenConns(1)
	dstTx, err := dstDB.BeginTx(ctx, nil)
	if err != nil {
		_ = dstDB.Close()
		return SaveProjectAsResult{}, err
	}

	writeErr := func() error {
		// Temporarily defer FK enforcement during bulk row load, then verify foreign_key_check.
		for _, spec := range specs {
			if _, err := dstTx.ExecContext(ctx, spec.createSQL); err != nil {
				return fmt.Errorf("create table %s: %w", spec.newTable, err)
			}
			if err := insertExactTableRows(ctx, dstTx, spec.newTable, spec.columns, spec.rows); err != nil {
				return fmt.Errorf("copy rows into %s: %w", spec.newTable, err)
			}
			for _, idxSQL := range spec.indexSQLs {
				if _, err := dstTx.ExecContext(ctx, idxSQL); err != nil {
					return fmt.Errorf("create index on %s: %w", spec.newTable, err)
				}
			}
		}

		if _, err := dstTx.ExecContext(ctx, `CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT)`); err != nil {
			return err
		}
		for _, spec := range specs {
			if spec.hasMetaTag {
				if _, err := dstTx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, ?)`, spec.newTable, spec.metaTag); err != nil {
					return err
				}
			}
		}
		if len(childIds) > 0 {
			if err := ensureChildIdentityTable(ctx, dstTx); err != nil {
				return err
			}
			for _, entry := range childIds {
				if _, err := dstTx.ExecContext(ctx, `INSERT INTO "__VPRO_ChildIdentity" ("ChildTable", "ID") VALUES (?, ?) ON CONFLICT DO NOTHING`, entry.childTable, entry.id); err != nil {
					return err
				}
			}
		}
		if suPolicyKind != "" {
			if _, err := dstTx.ExecContext(ctx, `CREATE TABLE "_vpro_su_policy" ("table_name" TEXT PRIMARY KEY, "kind" TEXT NOT NULL)`); err != nil {
				return err
			}
			if _, err := dstTx.ExecContext(ctx, `INSERT INTO "_vpro_su_policy" ("table_name", "kind") VALUES (?, ?)`, newName+"_SU", suPolicyKind); err != nil {
				return err
			}
		}
		return ctx.Err()
	}()
	if writeErr != nil {
		_ = dstTx.Rollback()
		_ = dstDB.Close()
		return SaveProjectAsResult{}, writeErr
	}
	if err := dstTx.Commit(); err != nil {
		_ = dstDB.Close()
		return SaveProjectAsResult{}, err
	}
	if err := dstDB.Close(); err != nil {
		return SaveProjectAsResult{}, err
	}

	if err := verifyStagedVP08Project(ctx, tmpPath, newName); err != nil {
		return SaveProjectAsResult{}, err
	}
	if err := publishStagedDatabase(tmpPath, resolvedDest, parentInfo); err != nil {
		return SaveProjectAsResult{}, err
	}
	published = true
	_ = os.Remove(tmpPath)

	shaHex, err := computeFileSHA256(resolvedDest)
	if err != nil {
		return SaveProjectAsResult{}, err
	}
	res := SaveProjectAsResult{
		SourceProject:   oldName,
		NewProjectName:  newName,
		DestinationPath: resolvedDest,
		DestinationSHA:  shaHex,
		CopiedTables:    copiedTables,
		CompanionTables: companionTables,
	}
	if req.ActivateOnSuccess {
		nextSU := "None"
		nextSUPath := ""
		if hasCompanionSU && snap.SU == oldName && vprocore.SameDesktopPath(snap.SUPath, snap.ProjectPath) {
			nextSU = newName
			nextSUPath = resolvedDest
		}
		state, swErr := s.SwitchContext(contextID, vprocore.ContextSelection{
			Project:     newName,
			ProjectPath: resolvedDest,
			SU:          nextSU,
			SUPath:      nextSUPath,
			Hierarchy:   "None",
		})
		if swErr != nil {
			return res, fmt.Errorf("project saved as %s, but context switch failed: %w", resolvedDest, swErr)
		}
		res.Activated = true
		res.State = &state
	}
	return res, nil
}

type ExportSplinterRequest struct {
	SplinterProject   string   `json:"splinterProject"`
	DestinationPath   string   `json:"destinationPath,omitempty"`
	UseActiveSU       bool     `json:"useActiveSu"`
	SiteUnits         []string `json:"siteUnits,omitempty"`
	PlotNumbers       []string `json:"plotNumbers,omitempty"`
	ActivateOnSuccess bool     `json:"activateOnSuccess"`
}

type ExportSplinterResult struct {
	SourceProject     string                 `json:"sourceProject"`
	SplinterProject   string                 `json:"splinterProject"`
	DestinationPath   string                 `json:"destinationPath"`
	DestinationSHA    string                 `json:"destinationSha256"`
	SourcePlotCount   int                    `json:"sourcePlotCount"`
	ExportedPlotCount int                    `json:"exportedPlotCount"`
	CreatedSUTable    bool                   `json:"createdSuTable"`
	Activated         bool                   `json:"activated"`
	State             *vprocore.ProjectState `json:"state,omitempty"`
}

// ExportSplinterProject extracts plots matching the active *_SU, explicit SiteUnits filter,
// and/or explicit PlotNumbers subset into a standalone VP08 <Splinter>.db carrying all matching
// _Env, _Admin, _Veg, _Humus, _Mineral, _Other, _Audit, matching _Metadata rows, and filtered <Splinter>_SU.
func ExportSplinterProject(s *vprocore.ContextService, ctx context.Context, contextID string, req ExportSplinterRequest) (_ ExportSplinterResult, resultErr error) {
	splinterName, err := ValidateProjectName(req.SplinterProject)
	if err != nil {
		return ExportSplinterResult{}, err
	}
	for _, p := range req.PlotNumbers {
		if p == "" || !utf8.ValidString(p) || strings.ContainsRune(p, 0) {
			return ExportSplinterResult{}, errors.New("explicit plot numbers must be non-empty valid UTF-8")
		}
	}
	for _, u := range req.SiteUnits {
		if !utf8.ValidString(u) || strings.ContainsRune(u, 0) {
			return ExportSplinterResult{}, errors.New("site unit filters must be valid UTF-8")
		}
	}

	snap, err := inspectContextSnapshot(ctx, s, contextID)
	if err != nil {
		return ExportSplinterResult{}, err
	}
	useSU := req.UseActiveSU || (len(req.PlotNumbers) == 0 && len(req.SiteUnits) == 0 && snap.SU != "None")
	if useSU && snap.SU == "None" {
		return ExportSplinterResult{}, errors.New("splinter export requires an active SU table, site unit filter, or explicit plot subset")
	}
	if !useSU && len(req.PlotNumbers) == 0 && len(req.SiteUnits) == 0 {
		return ExportSplinterResult{}, errors.New("splinter export requires an active SU table, site unit filter, or explicit plot subset")
	}

	resolvedDest, parentInfo, err := resolveNewProjectDestination(filepath.Join(snap.Root, "projects"), splinterName, req.DestinationPath, "Splinter project")
	if err != nil {
		return ExportSplinterResult{}, err
	}

	review, err := s.ReadNewProjectTemplates(ctx, contextID)
	if err != nil {
		return ExportSplinterResult{}, err
	}
	ddlPlan, err := vprocore.PlanNewProjectDDL(ctx, splinterName, review)
	if err != nil {
		return ExportSplinterResult{}, err
	}

	srcDB, err := sqlutil.OpenReadOnlyContext(ctx, snap.ProjectPath)
	if err != nil {
		return ExportSplinterResult{}, err
	}
	defer srcDB.Close()

	// Load SU map if active SU is available.
	suMap := map[string]*string{}
	hasSUSource := snap.SU != "None" && snap.SUPath != ""
	if hasSUSource {
		suDB := srcDB
		if !vprocore.SameDesktopPath(snap.SUPath, snap.ProjectPath) {
			extDB, oErr := sqlutil.OpenReadOnlyContext(ctx, snap.SUPath)
			if oErr != nil {
				return ExportSplinterResult{}, oErr
			}
			defer extDB.Close()
			suDB = extDB
		}
		suTbl := vprocore.QuoteHeaderIdentifier(snap.SU + "_SU")
		sRows, sErr := suDB.QueryContext(ctx, `SELECT CAST("PlotNumber" AS BLOB), typeof("PlotNumber"), CAST("SiteUnit" AS BLOB), typeof("SiteUnit") FROM `+suTbl)
		if sErr != nil {
			return ExportSplinterResult{}, sErr
		}
		for sRows.Next() {
			var pBytes, uBytes []byte
			var pType, uType string
			if err := sRows.Scan(&pBytes, &pType, &uBytes, &uType); err != nil {
				sRows.Close()
				return ExportSplinterResult{}, err
			}
			if pType != "text" {
				continue
			}
			pKey := string(pBytes)
			if uType == "text" {
				uStr := string(uBytes)
				suMap[pKey] = &uStr
			} else {
				suMap[pKey] = nil
			}
		}
		if err := errors.Join(sRows.Err(), sRows.Close()); err != nil {
			return ExportSplinterResult{}, err
		}
	}

	srcTx, err := srcDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ExportSplinterResult{}, err
	}
	defer func() { _ = srcTx.Rollback() }()

	envTbl := vprocore.QuoteHeaderIdentifier(snap.Project + "_Env")
	adminTbl := vprocore.QuoteHeaderIdentifier(snap.Project + "_Admin")

	// Determine total source plot count and matching plot set using exact literal PlotNumber bytes.
	plotFilter := map[string]bool{}
	for _, p := range req.PlotNumbers {
		plotFilter[p] = true
	}
	unitFilter := map[string]bool{}
	for _, u := range req.SiteUnits {
		unitFilter[u] = true
	}

	pRows, err := srcTx.QueryContext(ctx, `SELECT CAST(e."PlotNumber" AS BLOB), typeof(e."PlotNumber"),
		COALESCE(CAST(e."ProjectID" AS TEXT), ''),
		COALESCE(CAST(a."UserSiteUnit" AS TEXT), ''),
		COALESCE(CAST(e."SiteSeries" AS TEXT), '')
		FROM `+envTbl+` e
		LEFT JOIN `+adminTbl+` a ON typeof(a."Plot")='text' AND CAST(a."Plot" AS BLOB)=CAST(e."PlotNumber" AS BLOB)
		ORDER BY e."PlotNumber"`)
	if err != nil {
		return ExportSplinterResult{}, err
	}
	totalSourcePlots := 0
	selectedPlots := map[string]bool{}
	var orderedSelectedPlots []string
	selectedProjectIDs := map[string]bool{}
	fallbackSUMap := map[string]*string{}

	for pRows.Next() {
		var pBytes []byte
		var pType, pid, userSU, siteSeries string
		if err := pRows.Scan(&pBytes, &pType, &pid, &userSU, &siteSeries); err != nil {
			pRows.Close()
			return ExportSplinterResult{}, err
		}
		if pType != "text" {
			continue
		}
		totalSourcePlots++
		plot := string(pBytes)

		if useSU {
			if _, inSU := suMap[plot]; !inSU {
				continue
			}
		}
		if len(plotFilter) > 0 && !plotFilter[plot] {
			continue
		}
		if len(unitFilter) > 0 {
			if hasSUSource {
				uPtr, inSU := suMap[plot]
				uVal := ""
				if uPtr != nil {
					uVal = *uPtr
				}
				if !inSU || !unitFilter[uVal] {
					continue
				}
			} else {
				candidateUnit := strings.TrimSpace(userSU)
				if candidateUnit == "" {
					candidateUnit = strings.TrimSpace(siteSeries)
				}
				if !unitFilter[candidateUnit] {
					continue
				}
				uCopy := candidateUnit
				fallbackSUMap[plot] = &uCopy
			}
		}

		if !selectedPlots[plot] {
			selectedPlots[plot] = true
			orderedSelectedPlots = append(orderedSelectedPlots, plot)
			if pid != "" {
				selectedProjectIDs[pid] = true
			}
		}
	}
	if err := errors.Join(pRows.Err(), pRows.Close()); err != nil {
		return ExportSplinterResult{}, err
	}

	filterRowsByColumn := func(cols []string, allRows [][]rawSQLiteCell, colName string, allowed map[string]bool) [][]rawSQLiteCell {
		colIdx := -1
		for i, c := range cols {
			if strings.EqualFold(c, colName) {
				colIdx = i
				break
			}
		}
		if colIdx < 0 {
			return nil
		}
		var filtered [][]rawSQLiteCell
		for _, r := range allRows {
			cell := r[colIdx]
			if cell.storage == "text" && allowed[string(cell.bytes)] {
				filtered = append(filtered, r)
			}
		}
		return filtered
	}

	type tableData struct {
		cols []string
		rows [][]rawSQLiteCell
	}
	extracted := map[string]tableData{}
	for _, spec := range []struct {
		suffix string
		keyCol string
	}{
		{"Env", "PlotNumber"},
		{"Admin", "Plot"},
		{"Veg", "PlotNumber"},
		{"Humus", "PlotNumber"},
		{"Mineral", "PlotNumber"},
		{"Other", "PlotNumber"},
		{"Audit", "PlotNumber"},
	} {
		cols, allRows, rErr := readExactTableRows(ctx, srcTx, vprocore.QuoteHeaderIdentifier(snap.Project+"_"+spec.suffix), "")
		if rErr != nil {
			return ExportSplinterResult{}, rErr
		}
		filteredRows := filterRowsByColumn(cols, allRows, spec.keyCol, selectedPlots)
		if spec.suffix == "Audit" {
			rewriteAuditProjectCells(cols, filteredRows, splinterName)
		}
		extracted[spec.suffix] = tableData{
			cols: cols,
			rows: filteredRows,
		}
	}

	metaCols, metaAllRows, err := readExactTableRows(ctx, srcTx, vprocore.QuoteHeaderIdentifier(snap.Project+"_Metadata"), "")
	if err != nil {
		return ExportSplinterResult{}, err
	}
	extracted["Metadata"] = tableData{
		cols: metaCols,
		rows: filterRowsByColumn(metaCols, metaAllRows, "ProjectID", selectedProjectIDs),
	}

	var sourceIdentityReservations []struct {
		suffix string
		id     int64
	}
	var hasSourceIdentity bool
	if err := srcTx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='__VPRO_ChildIdentity')`).Scan(&hasSourceIdentity); err != nil {
		return ExportSplinterResult{}, err
	}
	if hasSourceIdentity {
		idRows, qErr := srcTx.QueryContext(ctx, `SELECT ChildTable, ID FROM "__VPRO_ChildIdentity"`)
		if qErr != nil {
			return ExportSplinterResult{}, qErr
		}
		srcPrefix := strings.Trim(vprocore.QuoteHeaderIdentifier(snap.Project+"_"), `"`)
		for idRows.Next() {
			var childTbl string
			var reservedID int64
			if sErr := idRows.Scan(&childTbl, &reservedID); sErr != nil {
				_ = idRows.Close()
				return ExportSplinterResult{}, sErr
			}
			unquoted := strings.Trim(childTbl, `"`)
			if strings.HasPrefix(unquoted, srcPrefix) {
				sourceIdentityReservations = append(sourceIdentityReservations, struct {
					suffix string
					id     int64
				}{suffix: strings.TrimPrefix(unquoted, srcPrefix), id: reservedID})
			}
		}
		if err := errors.Join(idRows.Err(), idRows.Close()); err != nil {
			return ExportSplinterResult{}, err
		}
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(resolvedDest), ".vpro-splinter-*.db")
	if err != nil {
		return ExportSplinterResult{}, err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return ExportSplinterResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		return ExportSplinterResult{}, err
	}

	dstDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL&_txlock=immediate")
	if err != nil {
		return ExportSplinterResult{}, err
	}
	dstDB.SetMaxOpenConns(1)
	dstTx, err := dstDB.BeginTx(ctx, nil)
	if err != nil {
		_ = dstDB.Close()
		return ExportSplinterResult{}, err
	}

	createdSU := hasSUSource || len(fallbackSUMap) > 0
	writeErr := func() error {
		for _, tblPlan := range ddlPlan.Tables {
			for _, mapping := range tblPlan.Schema {
				if mapping.SQL != nil {
					if _, err := dstTx.ExecContext(ctx, *mapping.SQL); err != nil {
						return err
					}
				}
			}
		}
		if _, err := dstTx.ExecContext(ctx, `CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT)`); err != nil {
			return err
		}
		for _, suffix := range vprocore.CoreTables {
			if _, err := dstTx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP08')`, splinterName+"_"+suffix); err != nil {
				return err
			}
		}
		if err := ensureChildIdentityTable(ctx, dstTx); err != nil {
			return err
		}
		for _, res := range sourceIdentityReservations {
			if err := reserveChildID(ctx, dstTx, splinterName+"_"+res.suffix, res.id); err != nil {
				return err
			}
		}

		for _, suffix := range []string{"Env", "Admin", "Veg", "Humus", "Mineral", "Other", "Audit", "Metadata"} {
			td := extracted[suffix]
			targetTbl := splinterName + "_" + suffix
			if err := insertExactTableRows(ctx, dstTx, targetTbl, td.cols, td.rows); err != nil {
				return fmt.Errorf("insert splinter %s: %w", targetTbl, err)
			}
			// Reserve copied positive child IDs so monotonic child ID allocation never collides.
			if suffix == "Veg" || suffix == "Humus" || suffix == "Mineral" || suffix == "Other" || suffix == "Metadata" {
				idIdx := -1
				for i, c := range td.cols {
					if c == "ID" {
						idIdx = i
						break
					}
				}
				if idIdx >= 0 {
					for _, r := range td.rows {
						if r[idIdx].storage == "integer" && r[idIdx].intVal > 0 {
							if err := reserveChildID(ctx, dstTx, targetTbl, r[idIdx].intVal); err != nil {
								return err
							}
						}
					}
				}
			}
		}

		if createdSU {
			suTableName := splinterName + "_SU"
			quotedSU := vprocore.QuoteHeaderIdentifier(suTableName)
			if _, err := dstTx.ExecContext(ctx, `CREATE TABLE `+quotedSU+` ("PlotNumber" VARCHAR, "SiteUnit" VARCHAR);
				CREATE UNIQUE INDEX `+vprocore.QuoteHeaderIdentifier("uidx_"+suTableName+"_PlotNumber")+` ON `+quotedSU+` ("PlotNumber");
				CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+suTableName+"_SiteUnit")+` ON `+quotedSU+` ("SiteUnit");`); err != nil {
				return err
			}
			stmt, err := dstTx.PrepareContext(ctx, `INSERT INTO `+quotedSU+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`)
			if err != nil {
				return err
			}
			defer stmt.Close()
			for _, plot := range orderedSelectedPlots {
				var unitVal any
				if hasSUSource {
					if uPtr := suMap[plot]; uPtr != nil {
						unitVal = *uPtr
					}
				} else if uPtr := fallbackSUMap[plot]; uPtr != nil {
					unitVal = *uPtr
				}
				if _, err := stmt.ExecContext(ctx, plot, unitVal); err != nil {
					return err
				}
			}
			if _, err := dstTx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP04')`, suTableName); err != nil {
				return err
			}
		}
		return ctx.Err()
	}()
	if writeErr != nil {
		_ = dstTx.Rollback()
		_ = dstDB.Close()
		return ExportSplinterResult{}, writeErr
	}
	if err := dstTx.Commit(); err != nil {
		_ = dstDB.Close()
		return ExportSplinterResult{}, err
	}
	if err := dstDB.Close(); err != nil {
		return ExportSplinterResult{}, err
	}

	if err := verifyStagedVP08Project(ctx, tmpPath, splinterName); err != nil {
		return ExportSplinterResult{}, err
	}
	if err := publishStagedDatabase(tmpPath, resolvedDest, parentInfo); err != nil {
		return ExportSplinterResult{}, err
	}
	published = true
	_ = os.Remove(tmpPath)

	shaHex, err := computeFileSHA256(resolvedDest)
	if err != nil {
		return ExportSplinterResult{}, err
	}

	res := ExportSplinterResult{
		SourceProject:     snap.Project,
		SplinterProject:   splinterName,
		DestinationPath:   resolvedDest,
		DestinationSHA:    shaHex,
		SourcePlotCount:   totalSourcePlots,
		ExportedPlotCount: len(orderedSelectedPlots),
		CreatedSUTable:    createdSU,
	}
	if req.ActivateOnSuccess {
		nextSU := "None"
		nextSUPath := ""
		if createdSU {
			nextSU = splinterName
			nextSUPath = resolvedDest
		}
		state, swErr := s.SwitchContext(contextID, vprocore.ContextSelection{
			Project:     splinterName,
			ProjectPath: resolvedDest,
			SU:          nextSU,
			SUPath:      nextSUPath,
			Hierarchy:   "None",
		})
		if swErr != nil {
			return res, fmt.Errorf("splinter project exported to %s, but context switch failed: %w", resolvedDest, swErr)
		}
		res.Activated = true
		res.State = &state
	}
	return res, nil
}

func verifyStagedVP08Project(ctx context.Context, dbPath, projectName string) error {
	inspected, err := vprocore.InspectFileContext(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("verify staged VP08 SQLite project: %w", err)
	}
	for _, info := range inspected {
		if info.Name == projectName && info.Compatible {
			return nil
		}
	}
	return fmt.Errorf("staged SQLite database did not pass VP08 compatibility validation for project %s", projectName)
}

func publishStagedDatabase(tmpPath, resolvedDest string, expectedParent os.FileInfo) error {
	currentParent, err := os.Stat(filepath.Dir(resolvedDest))
	if err != nil || !os.SameFile(expectedParent, currentParent) {
		return errors.New("destination parent directory changed during staging; publication aborted")
	}
	if err := sqlutil.LinkOrCreateExclusive(tmpPath, resolvedDest); err != nil {
		return fmt.Errorf("atomic no-overwrite link to %s failed: %w", resolvedDest, err)
	}
	return nil
}
