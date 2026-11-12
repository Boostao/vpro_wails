package diagnostics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	synthesisSUHistoryTable = "__VPRO_SynthesisSUHistory"
	synthesisSUHistorySQL   = `CREATE TABLE "__VPRO_SynthesisSUHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`
	foldedLumpCoverCap      = 99.0
)

type synthesisSUHistoryReceipt struct {
	Kind            string  `json:"kind"` // "synthesis_su_reassignment" | "synthesis_su_reassignment_undo"
	ContextID       string  `json:"contextId"`
	Project         string  `json:"project"`
	SU              string  `json:"su"`
	PlotNumber      string  `json:"plotNumber"`
	ExistedBefore   bool    `json:"existedBefore"`
	BeforeSiteUnit  *string `json:"beforeSiteUnit"`
	ExistedAfter    bool    `json:"existedAfter"`
	AfterSiteUnit   *string `json:"afterSiteUnit"`
	UndoneHistoryID int64   `json:"undoneHistoryId,omitempty"`
}

func normalizeAccessBooleanValue(raw any) bool {
	switch v := raw.(type) {
	case nil:
		return false
	case bool:
		return v
	case int64:
		return v != 0
	case int:
		return v != 0
	case float64:
		return v != 0
	case []byte:
		return normalizeAccessBooleanValue(string(v))
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(v))
		return trimmed == "-1" || trimmed == "1" || trimmed == "true" || trimmed == "yes"
	default:
		return false
	}
}

func validateWritableSUTarget(owner *vprocore.SqliteContext) (string, string, error) {
	if owner == nil || owner.Conn == nil {
		return "", "", errors.New("classification context is closed")
	}
	suName := strings.TrimSpace(owner.Selection.SU)
	if suName == "" || strings.EqualFold(suName, "None") {
		return "", "", errors.New("an active Site Unit (SU) table must be selected before reassigning plots")
	}
	lowerSU := strings.ToLower(suName)
	if lowerSU == "master" ||
		strings.HasPrefix(lowerSU, "master_") ||
		strings.HasSuffix(lowerSU, "_master") ||
		strings.EqualFold(suName, "MasterSiteUnitList") ||
		strings.HasPrefix(lowerSU, "usys") {
		return "", "", fmt.Errorf("SU table %q is a protected master or system table; writes are prohibited", suName+"_SU")
	}
	if err := owner.ValidateMetadataWriterFiles(); err != nil {
		return "", "", err
	}
	suInfo := owner.AttachmentInfo["su"]
	if suInfo == nil {
		return "", "", errors.New("active SU database is not attached")
	}
	for role, info := range owner.AttachmentInfo {
		if strings.HasPrefix(role, "V") && info != nil && os.SameFile(suInfo, info) {
			return "", "", fmt.Errorf("SU database aliases protected support database %s; writes are prohibited", role)
		}
	}
	suTable := suName + "_SU"
	for _, row := range owner.Descriptions["su"] {
		if tName, ok := row["table_name"].(string); ok && strings.EqualFold(tName, suTable) {
			if desc, ok := row["description"].(string); ok && strings.Contains(strings.ToLower(desc), "master") {
				return "", "", fmt.Errorf("SU table %s is marked as master in _table_metadata; writes are prohibited", suTable)
			}
		}
	}
	return suName, suTable, nil
}

// ListSynthesisLumpTables discovers *_Lump tables in the active project database for synthesis folding.
func ListSynthesisLumpTables(ctx context.Context, contexts *vprocore.ContextService, contextID string) ([]SynthesisLumpTableOption, error) {
	if ctx == nil {
		return nil, errors.New("lump table discovery requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ []SynthesisLumpTableOption, resultErr error) {
		owner := plots.Projects.Sqlite
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

		rows, err := tx.QueryContext(ctx,
			`SELECT name FROM "project".sqlite_master WHERE type='table' AND LOWER(name) LIKE '%\_lump' ESCAPE '\' ORDER BY name COLLATE NOCASE ASC`)
		if err != nil {
			return nil, err
		}
		var names []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return nil, err
			}
			names = append(names, n)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}

		out := make([]SynthesisLumpTableOption, 0, len(names))
		for _, tbl := range names {
			qTbl := vprocore.QuoteHeaderIdentifier(tbl)
			rRows, qErr := tx.QueryContext(ctx, `SELECT COALESCE(CAST("Use" AS TEXT), '0') FROM "project".`+qTbl)
			if qErr != nil {
				continue
			}
			total, active := 0, 0
			for rRows.Next() {
				var rawUse string
				if err := rRows.Scan(&rawUse); err == nil {
					total++
					if normalizeAccessBooleanValue(rawUse) {
						active++
					}
				}
			}
			rRows.Close()
			baseName := tbl
			if len(baseName) > 5 && strings.EqualFold(baseName[len(baseName)-5:], "_Lump") {
				baseName = baseName[:len(baseName)-5]
			}
			out = append(out, SynthesisLumpTableOption{
				Name:        baseName,
				TableName:   tbl,
				DBPath:      owner.Selection.ProjectPath,
				RowCount:    total,
				ActiveCount: active,
			})
		}

		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return out, nil
	})
}

func loadActiveLumpMap(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, rawLumpName, rawDBPath string) (map[string]string, string, error) {
	trimmed := strings.TrimSpace(rawLumpName)
	if trimmed == "" || strings.EqualFold(trimmed, "None") || strings.EqualFold(trimmed, "None_Lump") {
		return nil, "", nil
	}
	if !utf8.ValidString(trimmed) || strings.ContainsRune(trimmed, 0) {
		return nil, "", errors.New("lump table name must be valid UTF-8 without NUL")
	}
	base := trimmed
	if len(base) > 5 && strings.EqualFold(base[len(base)-5:], "_Lump") {
		base = base[:len(base)-5]
	}
	if !vprocore.ProjectNamePattern.MatchString(base) {
		return nil, "", fmt.Errorf("invalid lump table name %q", rawLumpName)
	}
	targetTable := base + "_Lump"

	scanLumpRows := func(rows *sql.Rows) (map[string]string, error) {
		defer rows.Close()
		mapping := map[string]string{}
		for rows.Next() {
			var lumpCode, sppCode sql.NullString
			var useRaw any
			if err := rows.Scan(&lumpCode, &sppCode, &useRaw); err != nil {
				return nil, err
			}
			if !normalizeAccessBooleanValue(useRaw) || !lumpCode.Valid || !sppCode.Valid {
				continue
			}
			cleanLump := strings.ToUpper(strings.TrimSpace(lumpCode.String))
			cleanSpp := strings.ToUpper(strings.TrimSpace(sppCode.String))
			if cleanLump != "" && cleanSpp != "" && cleanLump != cleanSpp {
				mapping[cleanSpp] = cleanLump
			}
		}
		return mapping, rows.Err()
	}

	dbPath := strings.TrimSpace(rawDBPath)
	if dbPath == "" || vprocore.SameDesktopPath(dbPath, owner.Selection.ProjectPath) {
		var actual string
		err := tx.QueryRowContext(ctx,
			`SELECT name FROM "project".sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`,
			targetTable,
		).Scan(&actual)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", fmt.Errorf("lump table %s does not exist in project %s", targetTable, owner.Selection.Project)
		}
		if err != nil {
			return nil, "", err
		}
		rows, err := tx.QueryContext(ctx,
			`SELECT CAST("LumpCode" AS TEXT), CAST("SppCode" AS TEXT), COALESCE(CAST("Use" AS TEXT), '0') FROM "project".`+vprocore.QuoteHeaderIdentifier(actual))
		if err != nil {
			return nil, "", err
		}
		mapping, err := scanLumpRows(rows)
		return mapping, actual, err
	}

	resolvedPath, err := vprocore.ExistingDatabasePath(dbPath)
	if err != nil {
		return nil, "", err
	}
	extDB, err := sqlutil.OpenReadOnly(resolvedPath)
	if err != nil {
		return nil, "", err
	}
	defer extDB.Close()

	var actual string
	if err := extDB.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`,
		targetTable,
	).Scan(&actual); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", fmt.Errorf("lump table %s does not exist in %s", targetTable, resolvedPath)
		}
		return nil, "", err
	}
	rows, err := extDB.QueryContext(ctx,
		`SELECT CAST("LumpCode" AS TEXT), CAST("SppCode" AS TEXT), COALESCE(CAST("Use" AS TEXT), '0') FROM `+vprocore.QuoteHeaderIdentifier(actual))
	if err != nil {
		return nil, "", err
	}
	mapping, err := scanLumpRows(rows)
	return mapping, actual, err
}

func loadSpeciesTaxonomyBatch(ctx context.Context, tx *sql.Tx, codes []string) (map[string]SpeciesTaxonMeta, error) {
	result := make(map[string]SpeciesTaxonMeta, len(codes))
	if len(codes) == 0 {
		return result, nil
	}

	// Determine VUser.USysUserSpp lifeform column spelling ("LifeForm" INT16 vs "Lifeform")
	userLifeformCol := ""
	pRows, pErr := tx.QueryContext(ctx, `PRAGMA "VUser".table_info("USysUserSpp")`)
	if pErr == nil {
		for pRows.Next() {
			var cid, notNull, pk int
			var name, ctype string
			var dflt any
			if err := pRows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err == nil {
				if name == "LifeForm" || name == "Lifeform" {
					userLifeformCol = name
					break
				}
			}
		}
		pRows.Close()
	}

	const chunkSize = 400
	for start := 0; start < len(codes); start += chunkSize {
		end := min(start+chunkSize, len(codes))
		batch := codes[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, c := range batch {
			placeholders[i] = "?"
			args[i] = strings.ToUpper(c)
		}
		inClause := strings.Join(placeholders, ",")

		// 1. VLists.USysAllSpecs (Lifeform is INT32 LONG; prefer non-synonym Codetype <> 'S')
		mRows, err := tx.QueryContext(ctx,
			`SELECT UPPER(TRIM(CAST("Code" AS TEXT))),
			        COALESCE(CAST("ScientificName" AS TEXT), ''),
			        COALESCE(CAST("EnglishName" AS TEXT), ''),
			        "Lifeform",
			        UPPER(TRIM(COALESCE(CAST("Codetype" AS TEXT), '')))
			 FROM "VLists"."USysAllSpecs"
			 WHERE UPPER(TRIM(CAST("Code" AS TEXT))) IN (`+inClause+`)
			 ORDER BY CASE WHEN UPPER(COALESCE(CAST("Codetype" AS TEXT), '')) = 'S' THEN 1 ELSE 0 END ASC, rowid ASC`,
			args...,
		)
		if err != nil {
			return nil, err
		}
		for mRows.Next() {
			var code, sci, eng, codetype string
			var lf sql.NullInt64
			if err := mRows.Scan(&code, &sci, &eng, &lf, &codetype); err != nil {
				mRows.Close()
				return nil, err
			}
			if _, exists := result[code]; exists {
				continue
			}
			var lfPtr *int
			if lf.Valid && lf.Int64 >= math.MinInt32 && lf.Int64 <= math.MaxInt32 {
				v := int(lf.Int64)
				lfPtr = &v
			}
			lfName, stratum := LifeformLabel(lfPtr)
			result[code] = SpeciesTaxonMeta{
				Species:         code,
				ScientificName:  strings.TrimSpace(sci),
				EnglishName:     strings.TrimSpace(eng),
				Lifeform:        lfPtr,
				LifeformName:    lfName,
				StratumGroup:    stratum,
				SourceCatalogue: "VLists",
			}
		}
		if err := errors.Join(mRows.Err(), mRows.Close()); err != nil {
			return nil, err
		}

		// 2. VUser.USysUserSpp fallback (LifeForm is INT16 INTEGER)
		if userLifeformCol != "" {
			qLF := vprocore.QuoteHeaderIdentifier(userLifeformCol)
			uRows, uErr := tx.QueryContext(ctx,
				`SELECT UPPER(TRIM(CAST("Code" AS TEXT))),
				        COALESCE(CAST("ScientificName" AS TEXT), ''),
				        COALESCE(CAST("EnglishName" AS TEXT), ''),
				        `+qLF+`
				 FROM "VUser"."USysUserSpp"
				 WHERE UPPER(TRIM(CAST("Code" AS TEXT))) IN (`+inClause+`)
				 ORDER BY rowid ASC`,
				args...,
			)
			if uErr == nil {
				for uRows.Next() {
					var code, sci, eng string
					var lf sql.NullInt64
					if err := uRows.Scan(&code, &sci, &eng, &lf); err != nil {
						uRows.Close()
						return nil, err
					}
					if _, exists := result[code]; exists {
						continue
					}
					var lfPtr *int
					if lf.Valid && lf.Int64 >= math.MinInt16 && lf.Int64 <= math.MaxInt16 {
						v := int(lf.Int64)
						lfPtr = &v
					}
					lfName, stratum := LifeformLabel(lfPtr)
					result[code] = SpeciesTaxonMeta{
						Species:         code,
						ScientificName:  strings.TrimSpace(sci),
						EnglishName:     strings.TrimSpace(eng),
						Lifeform:        lfPtr,
						LifeformName:    lfName,
						StratumGroup:    stratum,
						SourceCatalogue: "VUser",
					}
				}
				if err := errors.Join(uRows.Err(), uRows.Close()); err != nil {
					return nil, err
				}
			}
		}
	}

	for _, c := range codes {
		upper := strings.ToUpper(c)
		if _, exists := result[upper]; !exists {
			lfName, stratum := LifeformLabel(nil)
			result[upper] = SpeciesTaxonMeta{
				Species:         upper,
				ScientificName:  upper,
				EnglishName:     "",
				Lifeform:        nil,
				LifeformName:    lfName,
				StratumGroup:    stratum,
				SourceCatalogue: "Unlisted",
			}
		}
	}
	return result, nil
}

func readSynthesisUndoStatus(ctx context.Context, tx *sql.Tx, suName string) (bool, int, *SynthesisUndoEntry) {
	if suName == "" || strings.EqualFold(suName, "None") {
		return false, 0, nil
	}
	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM "su".sqlite_master WHERE type='table' AND name=?)`,
		synthesisSUHistoryTable,
	).Scan(&exists); err != nil || !exists {
		return false, 0, nil
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT ID, Created, Proposal FROM "su".`+vprocore.QuoteHeaderIdentifier(synthesisSUHistoryTable)+` ORDER BY ID ASC`)
	if err != nil {
		return false, 0, nil
	}
	defer rows.Close()

	type stackItem struct {
		id      int64
		created string
		rec     synthesisSUHistoryReceipt
	}
	var items []stackItem
	undoneSet := map[int64]bool{}

	for rows.Next() {
		var id int64
		var created, proposal string
		if err := rows.Scan(&id, &created, &proposal); err != nil {
			continue
		}
		var rec synthesisSUHistoryReceipt
		if err := json.Unmarshal([]byte(proposal), &rec); err != nil {
			continue
		}
		if !strings.EqualFold(rec.SU, suName) {
			continue
		}
		switch rec.Kind {
		case "synthesis_su_reassignment":
			items = append(items, stackItem{id: id, created: created, rec: rec})
		case "synthesis_su_reassignment_undo":
			if rec.UndoneHistoryID > 0 {
				undoneSet[rec.UndoneHistoryID] = true
			}
		}
	}

	active := make([]stackItem, 0, len(items))
	for _, it := range items {
		if !undoneSet[it.id] {
			active = append(active, it)
		}
	}
	if len(active) == 0 {
		return false, 0, nil
	}
	top := active[len(active)-1]
	before := ""
	if top.rec.BeforeSiteUnit != nil {
		before = *top.rec.BeforeSiteUnit
	}
	after := ""
	if top.rec.AfterSiteUnit != nil {
		after = *top.rec.AfterSiteUnit
	}
	return true, len(active), &SynthesisUndoEntry{
		HistoryID:      top.id,
		PlotNumber:     top.rec.PlotNumber,
		BeforeSiteUnit: before,
		AfterSiteUnit:  after,
		Created:        top.created,
	}
}

func computeSnapshotInReadTx(
	ctx context.Context,
	tx *sql.Tx,
	owner *vprocore.SqliteContext,
	contextID string,
	opts SynthesisOptions,
) (*SynthesisSnapshot, error) {
	started := time.Now()
	project := owner.Selection.Project

	sigBasis := strings.ToLower(strings.TrimSpace(opts.SignificanceBasis))
	if sigBasis == "" {
		sigBasis = "all_plots"
	}
	if sigBasis != "all_plots" && sigBasis != "present_plots" {
		return nil, fmt.Errorf("unsupported significanceBasis %q; expected 'all_plots' or 'present_plots'", opts.SignificanceBasis)
	}

	requestedSource := strings.ToLower(strings.TrimSpace(opts.UnitSource))
	if requestedSource == "" {
		requestedSource = "auto"
	}
	if requestedSource != "auto" && requestedSource != "su" && requestedSource != "user_site_unit" {
		return nil, fmt.Errorf("unsupported unitSource %q; expected 'auto', 'su', or 'user_site_unit'", opts.UnitSource)
	}

	hasActiveSU := owner.Selection.SU != "" && !strings.EqualFold(owner.Selection.SU, "None")
	if requestedSource == "su" && !hasActiveSU {
		return nil, errors.New("unitSource 'su' requires an active Site Unit (SU) table")
	}
	unitSourceUsed := "user_site_unit"
	suTableName := ""
	if hasActiveSU && (requestedSource == "auto" || requestedSource == "su") {
		unitSourceUsed = "su"
		suTableName = owner.Selection.SU + "_SU"
	}

	suWritable := false
	suReadOnlyReason := ""
	if unitSourceUsed == "su" {
		if _, _, wErr := validateWritableSUTarget(owner); wErr == nil {
			suWritable = true
		} else {
			suReadOnlyReason = wErr.Error()
		}
	} else {
		suReadOnlyReason = "Select an active Site Unit (SU) table to enable live plot reassignment"
	}

	// 1. Load optional Lump mapping
	lumpMap, lumpTableUsed, err := loadActiveLumpMap(ctx, tx, owner, opts.LumpTable, opts.LumpDBPath)
	if err != nil {
		return nil, err
	}

	// 2. Read scoped <Project>_Env + <Project>_Admin with literal BLOB PlotNumber join
	type envPlotInfo struct {
		plotNumber     string
		fieldNumber    string
		zone           string
		subZone        string
		siteSeries     string
		userSiteUnit   string
		moistureRegime string
		nutrientRegime string
	}
	envMap := map[string]envPlotInfo{}
	envOrder := []string{}

	envRows, err := tx.QueryContext(ctx,
		`SELECT CAST(env."PlotNumber" AS TEXT),
		        COALESCE(CAST(env."FieldNumber" AS TEXT), ''),
		        COALESCE(CAST(env."Zone" AS TEXT), ''),
		        COALESCE(CAST(env."SubZone" AS TEXT), ''),
		        COALESCE(CAST(env."SiteSeries" AS TEXT), ''),
		        COALESCE(CAST(admin."UserSiteUnit" AS TEXT), ''),
		        COALESCE(CAST(env."MoistureRegime" AS TEXT), ''),
		        COALESCE(CAST(env."NutrientRegime" AS TEXT), '')
		 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Env")+` AS env
		 INNER JOIN "project".`+vprocore.QuoteHeaderIdentifier(project+"_Admin")+` AS admin
		   ON typeof(env."PlotNumber")='text' AND typeof(admin."Plot")='text'
		  AND CAST(env."PlotNumber" AS BLOB)=CAST(admin."Plot" AS BLOB)
		 WHERE env."PlotNumber" IS NOT NULL AND TRIM(CAST(env."PlotNumber" AS TEXT)) <> ''
		 ORDER BY env."PlotNumber" COLLATE NOCASE ASC, env.rowid ASC`)
	if err != nil {
		return nil, err
	}
	for envRows.Next() {
		var rec envPlotInfo
		if err := envRows.Scan(
			&rec.plotNumber, &rec.fieldNumber, &rec.zone, &rec.subZone, &rec.siteSeries,
			&rec.userSiteUnit, &rec.moistureRegime, &rec.nutrientRegime,
		); err != nil {
			envRows.Close()
			return nil, err
		}
		if _, dup := envMap[rec.plotNumber]; !dup {
			envOrder = append(envOrder, rec.plotNumber)
			envMap[rec.plotNumber] = rec
		}
	}
	if err := errors.Join(envRows.Err(), envRows.Close()); err != nil {
		return nil, err
	}

	// 3. Resolve plot -> raw SiteUnit assignment
	rawUnitByPlot := make(map[string]string, len(envOrder))
	if unitSourceUsed == "su" {
		suRows, err := tx.QueryContext(ctx,
			`SELECT CAST("PlotNumber" AS TEXT), COALESCE(CAST("SiteUnit" AS TEXT), '')
			 FROM "su".`+vprocore.QuoteHeaderIdentifier(suTableName)+
				` WHERE typeof("PlotNumber")='text' ORDER BY rowid ASC`)
		if err != nil {
			return nil, err
		}
		for suRows.Next() {
			var p, u string
			if err := suRows.Scan(&p, &u); err != nil {
				suRows.Close()
				return nil, err
			}
			if _, inEnv := envMap[p]; inEnv {
				rawUnitByPlot[p] = strings.TrimSpace(u)
			}
		}
		if err := errors.Join(suRows.Err(), suRows.Close()); err != nil {
			return nil, err
		}
	} else {
		for _, p := range envOrder {
			rawUnitByPlot[p] = strings.TrimSpace(envMap[p].userSiteUnit)
		}
	}

	// 4. Filter excluded units (.support / .poor / .nudum + custom patterns) and partition plots
	excludedMap := map[string]*ExcludedUnitSummary{}
	excludedPlotCount := 0
	unassignedPlots := []SynthesisUnitPlot{}
	classifiedPlots := make([]classifiedPlotRecord, 0, len(envOrder))
	classifiedIndexByPlot := map[string]int{}
	unitSet := map[string]bool{}

	for _, p := range envOrder {
		info := envMap[p]
		unitCode := rawUnitByPlot[p]
		if unitCode == "" {
			unassignedPlots = append(unassignedPlots, SynthesisUnitPlot{
				PlotNumber:     info.plotNumber,
				FieldNumber:    info.fieldNumber,
				SiteUnit:       "",
				Zone:           info.zone,
				SubZone:        info.subZone,
				SiteSeries:     info.siteSeries,
				UserSiteUnit:   info.userSiteUnit,
				MoistureRegime: info.moistureRegime,
				NutrientRegime: info.nutrientRegime,
			})
			continue
		}
		if excluded, matched := MatchUnitExcluded(unitCode, opts.ExcludeStandardModifiers, opts.ExcludePatterns); excluded {
			excludedPlotCount++
			ex := excludedMap[unitCode]
			if ex == nil {
				ex = &ExcludedUnitSummary{
					SiteUnit:       unitCode,
					MatchedPattern: matched,
				}
				excludedMap[unitCode] = ex
			}
			ex.PlotCount++
			continue
		}
		unitSet[unitCode] = true
		classifiedIndexByPlot[p] = len(classifiedPlots)
		classifiedPlots = append(classifiedPlots, classifiedPlotRecord{
			plotNumber:     info.plotNumber,
			fieldNumber:    info.fieldNumber,
			siteUnit:       unitCode,
			zone:           info.zone,
			subZone:        info.subZone,
			siteSeries:     info.siteSeries,
			userSiteUnit:   info.userSiteUnit,
			moistureRegime: info.moistureRegime,
			nutrientRegime: info.nutrientRegime,
			speciesCovers:  map[string]float64{},
		})
	}

	excludedUnits := make([]ExcludedUnitSummary, 0, len(excludedMap))
	for _, ex := range excludedMap {
		excludedUnits = append(excludedUnits, *ex)
	}
	slices.SortFunc(excludedUnits, func(a, b ExcludedUnitSummary) int {
		return strings.Compare(strings.ToLower(a.SiteUnit), strings.ToLower(b.SiteUnit))
	})

	// Order SiteUnits deterministically (explicit UnitOrder first, then Access PIVOT case-insensitive alphabetical)
	orderedUnits := make([]string, 0, len(unitSet))
	placedUnit := map[string]bool{}
	for _, u := range opts.UnitOrder {
		trimmed := strings.TrimSpace(u)
		if unitSet[trimmed] && !placedUnit[trimmed] {
			placedUnit[trimmed] = true
			orderedUnits = append(orderedUnits, trimmed)
		}
	}
	remUnits := make([]string, 0, len(unitSet))
	for u := range unitSet {
		if !placedUnit[u] {
			remUnits = append(remUnits, u)
		}
	}
	slices.SortFunc(remUnits, func(a, b string) int {
		if cmp := strings.Compare(strings.ToLower(a), strings.ToLower(b)); cmp != 0 {
			return cmp
		}
		return strings.Compare(a, b)
	})
	orderedUnits = append(orderedUnits, remUnits...)

	// 5. Read unit long names from MasterSiteUnitList & UserSiteUnitList
	unitLongNames := map[string]string{}
	for _, src := range []struct{ role, table string }{
		{"VLists", "MasterSiteUnitList"},
		{"VUser", "UserSiteUnitList"},
	} {
		uRows, uErr := tx.QueryContext(ctx,
			`SELECT COALESCE(CAST("SiteSeries" AS TEXT), ''), COALESCE(CAST("SiteSeriesLongName" AS TEXT), '')
			 FROM `+vprocore.QuoteHeaderIdentifier(src.role)+`.`+vprocore.QuoteHeaderIdentifier(src.table))
		if uErr == nil {
			for uRows.Next() {
				var code, lname string
				if err := uRows.Scan(&code, &lname); err == nil && strings.TrimSpace(code) != "" {
					unitLongNames[strings.TrimSpace(code)] = strings.TrimSpace(lname)
				}
			}
			uRows.Close()
		}
	}

	// 6. Read <Project>_Veg for classified plots, applying float32 narrowing and optional *_Lump 99% cap
	type plotLayerKey struct {
		plotIdx int
		layerID int
		species string
	}
	type layerAccumulator struct {
		cover  float64
		lumped bool
		count  int
	}
	layerCells := map[plotLayerKey]*layerAccumulator{}
	lumpSourcesByTarget := map[string]map[string]bool{}
	foldedSourceTaxa := map[string]bool{}

	if len(classifiedPlots) > 0 {
		vegRows, err := tx.QueryContext(ctx,
			`SELECT CAST(v."PlotNumber" AS TEXT),
			        COALESCE(CAST(v."Species" AS TEXT), ''),
			        v."Cover1", v."Cover2", v."Cover3", v."TotalA",
			        v."Cover4", v."Cover5", v."Cover5a", v."Cover5b", v."Cover5c", v."TotalB",
			        v."Cover6", v."Cover7", v."Cover8", v."Cover9", v."Cover10"
			 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Veg")+` AS v
			 WHERE typeof(v."PlotNumber")='text' AND v."Species" IS NOT NULL`)
		if err != nil {
			return nil, err
		}

		narrowNullable := func(nf sql.NullFloat64) (float64, bool) {
			if !nf.Valid {
				return 0, false
			}
			s := float32(nf.Float64)
			if math.IsNaN(float64(s)) || math.IsInf(float64(s), 0) || s < 0 {
				return 0, false
			}
			return float64(s), true
		}

		addLayerCover := func(pIdx, layerID int, effSpecies string, val float64, mapped bool) {
			key := plotLayerKey{plotIdx: pIdx, layerID: layerID, species: effSpecies}
			acc := layerCells[key]
			if acc == nil {
				acc = &layerAccumulator{}
				layerCells[key] = acc
			}
			acc.cover += val
			acc.count++
			if mapped {
				acc.lumped = true
			}
		}

		for vegRows.Next() {
			var plotNum, rawSpp string
			var c1, c2, c3, totA, c4, c5, c5a, c5b, c5c, totB, c6, c7, c8, c9, c10 sql.NullFloat64
			if err := vegRows.Scan(
				&plotNum, &rawSpp,
				&c1, &c2, &c3, &totA,
				&c4, &c5, &c5a, &c5b, &c5c, &totB,
				&c6, &c7, &c8, &c9, &c10,
			); err != nil {
				vegRows.Close()
				return nil, err
			}
			pIdx, ok := classifiedIndexByPlot[plotNum]
			if !ok {
				continue
			}
			rawCode := strings.ToUpper(strings.TrimSpace(rawSpp))
			if rawCode == "" {
				continue
			}
			effCode := rawCode
			wasMapped := false
			if lumpMap != nil {
				if mapped, found := lumpMap[rawCode]; found && mapped != "" {
					effCode = mapped
					wasMapped = true
					foldedSourceTaxa[rawCode] = true
					if lumpSourcesByTarget[effCode] == nil {
						lumpSourcesByTarget[effCode] = map[string]bool{}
					}
					lumpSourcesByTarget[effCode][rawCode] = true
				}
			}

			// Stratum A: use Cover1..Cover3 when any sublayer is populated, else TotalA (SIVI compatibility)
			hasSubA := false
			for idx, nf := range []sql.NullFloat64{c1, c2, c3} {
				if v, valid := narrowNullable(nf); valid {
					hasSubA = true
					addLayerCover(pIdx, 1+idx, effCode, v, wasMapped)
				}
			}
			if !hasSubA {
				if v, valid := narrowNullable(totA); valid {
					addLayerCover(pIdx, 101, effCode, v, wasMapped)
				}
			}

			// Stratum B: use Cover4..Cover5c when any sublayer is populated, else TotalB (SIVI compatibility)
			hasSubB := false
			for idx, nf := range []sql.NullFloat64{c4, c5, c5a, c5b, c5c} {
				if v, valid := narrowNullable(nf); valid {
					hasSubB = true
					addLayerCover(pIdx, 4+idx, effCode, v, wasMapped)
				}
			}
			if !hasSubB {
				if v, valid := narrowNullable(totB); valid {
					addLayerCover(pIdx, 102, effCode, v, wasMapped)
				}
			}

			// Strata C & D (Cover6..Cover10)
			for idx, nf := range []sql.NullFloat64{c6, c7, c8, c9, c10} {
				if v, valid := narrowNullable(nf); valid {
					addLayerCover(pIdx, 20+idx, effCode, v, wasMapped)
				}
			}
		}
		if err := errors.Join(vegRows.Err(), vegRows.Close()); err != nil {
			return nil, err
		}
	}

	// Fold layers into per-(plot, species) cover with 99% cap on lumped observations
	lumpCappedCells := 0
	type plotSpeciesAcc struct {
		cover  float64
		lumped bool
	}
	plotSpeciesTotals := make([]map[string]*plotSpeciesAcc, len(classifiedPlots))
	allSpeciesSet := map[string]bool{}

	for key, acc := range layerCells {
		cov := acc.cover
		isLumpedCell := lumpMap != nil && (acc.lumped || acc.count > 1)
		if isLumpedCell && cov > foldedLumpCoverCap {
			cov = foldedLumpCoverCap
			lumpCappedCells++
		}
		m := plotSpeciesTotals[key.plotIdx]
		if m == nil {
			m = map[string]*plotSpeciesAcc{}
			plotSpeciesTotals[key.plotIdx] = m
		}
		ps := m[key.species]
		if ps == nil {
			ps = &plotSpeciesAcc{}
			m[key.species] = ps
		}
		ps.cover += cov
		if isLumpedCell {
			ps.lumped = true
		}
		allSpeciesSet[key.species] = true
	}

	unitPlotCounts := make(map[string]int, len(orderedUnits))
	unitPlotsMap := make(map[string][]SynthesisUnitPlot, len(orderedUnits))
	unitCoverSums := make(map[string]map[string]float64, len(orderedUnits))
	unitPresentCounts := make(map[string]map[string]int, len(orderedUnits))
	unitTotalSums := make(map[string]float64, len(orderedUnits))

	for _, u := range orderedUnits {
		unitCoverSums[u] = map[string]float64{}
		unitPresentCounts[u] = map[string]int{}
	}

	for i := range classifiedPlots {
		p := &classifiedPlots[i]
		u := p.siteUnit
		unitPlotCounts[u]++
		unitPlotsMap[u] = append(unitPlotsMap[u], SynthesisUnitPlot{
			PlotNumber:     p.plotNumber,
			FieldNumber:    p.fieldNumber,
			SiteUnit:       u,
			Zone:           p.zone,
			SubZone:        p.subZone,
			SiteSeries:     p.siteSeries,
			UserSiteUnit:   p.userSiteUnit,
			MoistureRegime: p.moistureRegime,
			NutrientRegime: p.nutrientRegime,
		})

		psMap := plotSpeciesTotals[i]
		for sp, ps := range psMap {
			cov := ps.cover
			if cov < 0 {
				continue
			}
			p.speciesCovers[sp] = cov
			p.totalCover += cov
			unitCoverSums[u][sp] += cov
			unitPresentCounts[u][sp]++
			unitTotalSums[u] += cov
		}
	}

	// 7. Batch-load taxonomy metadata for all observed species
	allSpeciesCodes := make([]string, 0, len(allSpeciesSet))
	for sp := range allSpeciesSet {
		allSpeciesCodes = append(allSpeciesCodes, sp)
	}
	slices.Sort(allSpeciesCodes)
	speciesMeta, err := loadSpeciesTaxonomyBatch(ctx, tx, allSpeciesCodes)
	if err != nil {
		return nil, err
	}

	// 8. Build per-unit & per-species constancy, mean cover, Domin-Krajina significance, and V7mdlDiagnostic codes
	unitSpeciesStats := make(map[string]map[string]unitSpeciesStat, len(orderedUnits))
	for _, u := range orderedUnits {
		unitSpeciesStats[u] = map[string]unitSpeciesStat{}
	}

	unitSummaryByCode := make(map[string]*SynthesisUnitSummary, len(orderedUnits))
	for _, u := range orderedUnits {
		plotsSlice := unitPlotsMap[u]
		if plotsSlice == nil {
			plotsSlice = []SynthesisUnitPlot{}
		}
		unitSummaryByCode[u] = &SynthesisUnitSummary{
			SiteUnit:  u,
			LongName:  unitLongNames[u],
			PlotCount: unitPlotCounts[u],
			Plots:     plotsSlice,
		}
	}

	speciesRows := make([]SynthesisSpeciesRow, 0, len(allSpeciesCodes))
	for _, sp := range allSpeciesCodes {
		meta := speciesMeta[sp]
		cells := make([]SynthesisUnitSpeciesCell, len(orderedUnits))
		orderedCodes := make([]OrderedUnitCode, len(orderedUnits))
		maxPresPct := 0.0
		maxMeanCov := 0.0

		for uIdx, u := range orderedUnits {
			nPlots := unitPlotCounts[u]
			nPres := unitPresentCounts[u][sp]
			totCov := unitCoverSums[u][sp]

			cell := SynthesisUnitSpeciesCell{
				SiteUnit:            u,
				PresentPlots:        nPres,
				PlotCount:           nPlots,
				PresencePctText:     "",
				MeanCoverAllText:    "",
				MeanCoverPresentText: "",
				CellDiagnosticCodes: []string{},
			}

			if nPlots > 0 && nPres > 0 {
				presRatio := float64(nPres) / float64(nPlots)
				presPct := presRatio * 100.0
				presClass, presRoman := DiagnosticPresenceClass(presRatio)

				meanAll := totCov / float64(nPlots)
				meanPres := totCov / float64(nPres)

				sigInput := meanAll
				if sigBasis == "present_plots" {
					sigInput = meanPres
				}
				sigClass, sigNum := DominKrajinaSignifClass(sigInput)
				ctCode := FormatUnitCrosstabCode(presClass, sigClass)

				diagPot := CalcDiagnosticPotential(presPct, meanAll, opts.Pairwise).DiagnosticPotential

				cell.PresencePct = RoundACE15(presPct, 1)
				cell.PresencePctText = FormatACE15(presPct, 0)
				cell.PresenceClass = presClass
				cell.PresenceRoman = presRoman
				cell.MeanCoverAll = RoundACE15(meanAll, 2)
				cell.MeanCoverAllText = FormatACE15(meanAll, 1)
				cell.MeanCoverPresent = RoundACE15(meanPres, 2)
				cell.MeanCoverPresentText = FormatACE15(meanPres, 1)
				cell.SignifClass = sigClass
				cell.SignifNumeric = sigNum
				cell.CrosstabCode = ctCode
				cell.DiagnosticPotential = RoundACE15(diagPot, 2)

				codeCopy := ctCode
				orderedCodes[uIdx] = OrderedUnitCode{Unit: u, Code: &codeCopy}

				unitSpeciesStats[u][sp] = unitSpeciesStat{
					presentPlots: nPres,
					totalCover:   totCov,
					meanCoverAll: meanAll,
					meanCoverPr:  meanPres,
					constancyPct: presPct,
					diagPot:      diagPot,
				}
				unitSummaryByCode[u].SpeciesRichness++
				unitSummaryByCode[u].DiagPotentialSum += diagPot
				if presClass == 5 {
					unitSummaryByCode[u].ConstantCount++
				}
				if presPct > maxPresPct {
					maxPresPct = presPct
				}
				if meanAll > maxMeanCov {
					maxMeanCov = meanAll
				}
			} else {
				orderedCodes[uIdx] = OrderedUnitCode{Unit: u, Code: nil}
			}
			cells[uIdx] = cell
		}

		if opts.MinMaxPresencePct > 0 && maxPresPct < opts.MinMaxPresencePct {
			continue
		}

		var diagRes DiagnosticClassification
		if len(orderedCodes) > 0 {
			var cErr error
			diagRes, cErr = ClassifySpeciesDiagnostic(orderedCodes)
			if cErr != nil {
				return nil, cErr
			}
		}

		for uIdx := range cells {
			c := &cells[uIdx]
			if c.PresentPlots == 0 {
				continue
			}
			if diagRes.Unit != "" && c.SiteUnit == diagRes.Unit {
				c.IsPrimaryDiagnosticUnit = true
				c.CellDiagnosticCodes = append(c.CellDiagnosticCodes, diagRes.Codes...)
				unitSummaryByCode[c.SiteUnit].DiagnosticCount++
			} else if c.PresenceClass == 5 {
				// Secondary unit constancy indicator for multi-unit constant/dominant taxa
				if c.SignifNumeric >= 5 {
					c.CellDiagnosticCodes = append(c.CellDiagnosticCodes, "cd")
				} else {
					c.CellDiagnosticCodes = append(c.CellDiagnosticCodes, "c")
				}
			}
		}

		var srcSpp []string
		if set := lumpSourcesByTarget[sp]; len(set) > 0 {
			srcSpp = make([]string, 0, len(set))
			for k := range set {
				srcSpp = append(srcSpp, k)
			}
			slices.Sort(srcSpp)
		}

		diagCodes := diagRes.Codes
		if diagCodes == nil {
			diagCodes = []string{}
		}

		speciesRows = append(speciesRows, SynthesisSpeciesRow{
			Species:         sp,
			ScientificName:  meta.ScientificName,
			EnglishName:     meta.EnglishName,
			Lifeform:        meta.Lifeform,
			LifeformName:    meta.LifeformName,
			StratumGroup:    meta.StratumGroup,
			SourceSpecies:   srcSpp,
			DiagnosticUnit:  diagRes.Unit,
			Diagnosis:       diagRes.Diagnosis,
			DiagnosticCodes: diagCodes,
			MaxPresencePct:  RoundACE15(maxPresPct, 1),
			MaxMeanCover:    RoundACE15(maxMeanCov, 2),
			Cells:           cells,
		})
	}

	// Order species: diagnostic units first (in orderedUnits order), then Non-Diagnostic, by stratum -> lifeform -> presence -> cover
	unitOrderIndex := make(map[string]int, len(orderedUnits))
	for i, u := range orderedUnits {
		unitOrderIndex[u] = i
	}
	slices.SortFunc(speciesRows, func(a, b SynthesisSpeciesRow) int {
		aDiag := a.DiagnosticUnit != ""
		bDiag := b.DiagnosticUnit != ""
		if aDiag != bDiag {
			if aDiag {
				return -1
			}
			return 1
		}
		if aDiag && bDiag && a.DiagnosticUnit != b.DiagnosticUnit {
			return unitOrderIndex[a.DiagnosticUnit] - unitOrderIndex[b.DiagnosticUnit]
		}
		if srA, srB := stratumSortRank(a.StratumGroup), stratumSortRank(b.StratumGroup); srA != srB {
			return srA - srB
		}
		lfA, lfB := 999, 999
		if a.Lifeform != nil {
			lfA = *a.Lifeform
		}
		if b.Lifeform != nil {
			lfB = *b.Lifeform
		}
		if lfA != lfB {
			return lfA - lfB
		}
		if a.MaxPresencePct != b.MaxPresencePct {
			if a.MaxPresencePct > b.MaxPresencePct {
				return -1
			}
			return 1
		}
		if a.MaxMeanCover != b.MaxMeanCover {
			if a.MaxMeanCover > b.MaxMeanCover {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ScientificName, b.ScientificName)
	})

	unitsOut := make([]SynthesisUnitSummary, 0, len(orderedUnits))
	for _, u := range orderedUnits {
		sum := unitSummaryByCode[u]
		sum.DiagPotentialSum = RoundACE15(sum.DiagPotentialSum, 2)
		unitsOut = append(unitsOut, *sum)
	}

	// 9. Pairwise Similarity Matrix + BEC do_pairwise comparisons
	simMatrix, pairwiseComps := ComputePairwiseAnalysis(
		orderedUnits,
		unitPlotCounts,
		speciesMeta,
		unitSpeciesStats,
		opts.Pairwise,
	)

	// 10. Candidate Misplaced Plots (Leave-One-Out Outlier Queue)
	outlierQueue := ComputeOutlierQueue(
		orderedUnits,
		classifiedPlots,
		unitCoverSums,
		unitTotalSums,
		unitPlotCounts,
	)

	// 11. Relative Edatopic Grids (rSMR 0..8 x rSNR A..F)
	overallEdatopic, unitEdatopic := ComputeEdatopicGrids(orderedUnits, classifiedPlots)

	// 12. Synthesis Undo status
	canUndo, undoDepth, latestUndo := readSynthesisUndoStatus(ctx, tx, owner.Selection.SU)

	elapsedMs := float64(time.Since(started).Microseconds()) / 1000.0

	return &SynthesisSnapshot{
		ContextID:           contextID,
		Project:             project,
		SU:                  owner.Selection.SU,
		SUTableName:         suTableName,
		UnitSourceUsed:      unitSourceUsed,
		SUWritable:          suWritable,
		SUReadOnlyReason:    suReadOnlyReason,
		LumpTableUsed:       lumpTableUsed,
		LumpFoldedTaxa:      len(foldedSourceTaxa),
		LumpCappedCells:     lumpCappedCells,
		SignificanceBasis:   sigBasis,
		TotalProjectPlots:   len(envOrder),
		ClassifiedPlotCount: len(classifiedPlots),
		UnassignedPlotCount: len(unassignedPlots),
		ExcludedPlotCount:   excludedPlotCount,
		ComputeMs:           RoundACE15(elapsedMs, 2),
		ExcludedUnits:       excludedUnits,
		UnassignedPlots:     unassignedPlots,
		Units:               unitsOut,
		Species:             speciesRows,
		SimilarityMatrix:    simMatrix,
		PairwiseComparisons: pairwiseComps,
		OutlierQueue:        outlierQueue,
		OverallEdatopicGrid: overallEdatopic,
		UnitEdatopicGrids:   unitEdatopic,
		CanUndo:             canUndo,
		UndoDepth:           undoDepth,
		LatestUndo:          latestUndo,
	}, nil
}

func computeSnapshotForPlots(
	ctx context.Context,
	owner *vprocore.SqliteContext,
	contextID string,
	opts SynthesisOptions,
) (_ *SynthesisSnapshot, resultErr error) {
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

	snap, err := computeSnapshotInReadTx(ctx, tx, owner, contextID, opts)
	if err != nil {
		return nil, err
	}
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return snap, nil
}

// ComputeSynthesisSnapshot computes the complete reactive BEC differential, pairwise similarity,
// Leave-One-Out outlier queue, and 9x6 edatopic grid snapshot for the active context.
func ComputeSynthesisSnapshot(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	opts SynthesisOptions,
) (*SynthesisSnapshot, error) {
	if ctx == nil {
		return nil, errors.New("synthesis snapshot requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*SynthesisSnapshot, error) {
		return computeSnapshotForPlots(ctx, plots.Projects.Sqlite, contextID, opts)
	})
}

func verifyPlotInProjectEnv(ctx context.Context, owner *vprocore.SqliteContext, plotNumber string) error {
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return err
	}
	defer owner.Mu.Unlock()
	tx, err := owner.BeginReadSnapshot(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	project := owner.Selection.Project
	var exists bool
	err = tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Env")+`
			WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)
		)`,
		plotNumber,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("plot %q does not exist in project %s", plotNumber, project)
	}
	return tx.Commit()
}

// ReassignSynthesisPlotUnit atomically updates <SU>_SU inside BEGIN IMMEDIATE, writes an undo receipt
// to __VPRO_SynthesisSUHistory, and immediately recomputes the updated SynthesisSnapshot (< 50 ms target).
func ReassignSynthesisPlotUnit(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	req ReassignSynthesisPlotRequest,
) (*ReassignSynthesisPlotResult, error) {
	started := time.Now()
	if ctx == nil {
		return nil, errors.New("synthesis plot reassignment requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	if req.PlotNumber == "" || !utf8.ValidString(req.PlotNumber) || strings.ContainsRune(req.PlotNumber, 0) {
		return nil, errors.New("plotNumber must be valid non-empty UTF-8 without NUL")
	}
	if len(utf16.Encode([]rune(req.PlotNumber))) > 7 {
		return nil, fmt.Errorf("plotNumber %q exceeds 7 UTF-16 code units", req.PlotNumber)
	}

	targetUnit := strings.TrimSpace(req.TargetSiteUnit)
	if !utf8.ValidString(targetUnit) || strings.ContainsRune(targetUnit, 0) {
		return nil, errors.New("targetSiteUnit must be valid UTF-8 without NUL")
	}
	if len(utf16.Encode([]rune(targetUnit))) > 255 {
		return nil, errors.New("targetSiteUnit exceeds 255 UTF-16 code units")
	}
	if targetUnit == "" {
		return nil, errors.New("targetSiteUnit must not be empty")
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*ReassignSynthesisPlotResult, error) {
		owner := plots.Projects.Sqlite
		if err := verifyPlotInProjectEnv(ctx, owner, req.PlotNumber); err != nil {
			return nil, err
		}

		var suName, suTable string
		validateFiles := func() error {
			var err error
			suName, suTable, err = validateWritableSUTarget(owner)
			return err
		}

		var beforeUnit string
		var changed bool
		var historyID int64

		err := owner.WithSelectedFileWriter(ctx, "su", validateFiles, func(conn *sql.Conn) (txErr error) {
			tx, bErr := conn.BeginTx(ctx, nil) // BEGIN IMMEDIATE via _txlock=immediate
			if bErr != nil {
				return bErr
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			qTable := vprocore.QuoteHeaderIdentifier(suTable)
			var rowID int64
			var existing sql.NullString
			qErr := tx.QueryRowContext(ctx,
				`SELECT rowid, CAST("SiteUnit" AS TEXT) FROM main.`+qTable+
					` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
				req.PlotNumber,
			).Scan(&rowID, &existing)

			existedBefore := qErr == nil
			if qErr != nil && !errors.Is(qErr, sql.ErrNoRows) {
				return qErr
			}
			var beforePtr *string
			if existedBefore && existing.Valid {
				v := existing.String
				beforeUnit = strings.TrimSpace(v)
				beforePtr = &v
			}

			afterCopy := targetUnit
			if existedBefore {
				if beforePtr != nil && *beforePtr == afterCopy {
					return tx.Commit()
				}
				if _, err := tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "SiteUnit" = ? WHERE rowid = ?`, afterCopy, rowID); err != nil {
					return err
				}
				changed = true
			} else {
				if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber","SiteUnit") VALUES (?,?)`, req.PlotNumber, afterCopy); err != nil {
					return err
				}
				changed = true
			}

			receipt := synthesisSUHistoryReceipt{
				Kind:           "synthesis_su_reassignment",
				ContextID:      contextID,
				Project:        owner.Selection.Project,
				SU:             suName,
				PlotNumber:     req.PlotNumber,
				ExistedBefore:  existedBefore,
				BeforeSiteUnit: beforePtr,
				ExistedAfter:   true,
				AfterSiteUnit:  &afterCopy,
			}
			rawReceipt, mErr := json.Marshal(receipt)
			if mErr != nil {
				return mErr
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, synthesisSUHistoryTable, synthesisSUHistorySQL, string(rawReceipt), "synthesis SU reassignment"); err != nil {
				return err
			}
			_ = tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM main.`+vprocore.QuoteHeaderIdentifier(synthesisSUHistoryTable)).Scan(&historyID)

			if vprocore.SameDesktopPath(owner.Selection.ProjectPath, owner.Selection.SUPath) && changed {
				actor := strings.TrimSpace(plots.CurrentUser)
				if actor == "" {
					actor = "VProUser"
				}
				qAudit := vprocore.QuoteHeaderIdentifier(owner.Selection.Project + "_Audit")
				_, _ = tx.ExecContext(ctx,
					`INSERT INTO main.`+qAudit+` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag") VALUES (?,?,?,?,?,?,?,?,0,0)`,
					owner.Selection.Project, actor, req.PlotNumber, "_SU", "SiteUnit", time.Now().Format("2006-01-02 15:04:05"), beforePtr, afterCopy,
				)
			}

			if err := validateFiles(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return tx.Commit()
		})
		if err != nil {
			return nil, err
		}

		recomputeOpts := req.Options
		if recomputeOpts.UnitSource == "" {
			recomputeOpts.UnitSource = "su"
		}
		snap, snapErr := computeSnapshotForPlots(ctx, owner, contextID, recomputeOpts)
		_ = snapErr // Commit already succeeded; return committed receipt even if optional refresh was cancelled.

		latencyMs := float64(time.Since(started).Microseconds()) / 1000.0
		return &ReassignSynthesisPlotResult{
			ContextID:      contextID,
			SU:             suName,
			TableName:      suTable,
			PlotNumber:     req.PlotNumber,
			BeforeSiteUnit: beforeUnit,
			AfterSiteUnit:  targetUnit,
			Changed:        changed,
			HistoryID:      historyID,
			LatencyMs:      RoundACE15(latencyMs, 2),
			Snapshot:       snap,
		}, nil
	})
}

// UndoSynthesisReassignment reverses the latest (or specified HistoryID) synthesis SU plot reassignment
// in __VPRO_SynthesisSUHistory and immediately recomputes the updated SynthesisSnapshot.
func UndoSynthesisReassignment(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	req UndoSynthesisReassignmentRequest,
) (*ReassignSynthesisPlotResult, error) {
	started := time.Now()
	if ctx == nil {
		return nil, errors.New("synthesis undo requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*ReassignSynthesisPlotResult, error) {
		owner := plots.Projects.Sqlite
		var suName, suTable string
		validateFiles := func() error {
			var err error
			suName, suTable, err = validateWritableSUTarget(owner)
			return err
		}

		var undonePlot, beforeUnit, afterUnit string
		var undoHistoryID int64

		err := owner.WithSelectedFileWriter(ctx, "su", validateFiles, func(conn *sql.Conn) (txErr error) {
			tx, bErr := conn.BeginTx(ctx, nil)
			if bErr != nil {
				return bErr
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			var hasHistory bool
			if err := tx.QueryRowContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM main.sqlite_master WHERE type='table' AND name=?)`,
				synthesisSUHistoryTable,
			).Scan(&hasHistory); err != nil || !hasHistory {
				return errors.New("no synthesis SU reassignment history exists for the active SU")
			}

			rows, err := tx.QueryContext(ctx,
				`SELECT ID, Proposal FROM main.`+vprocore.QuoteHeaderIdentifier(synthesisSUHistoryTable)+` ORDER BY ID ASC`)
			if err != nil {
				return err
			}
			type histRow struct {
				id  int64
				rec synthesisSUHistoryReceipt
			}
			var items []histRow
			undoneSet := map[int64]bool{}
			for rows.Next() {
				var id int64
				var prop string
				if sErr := rows.Scan(&id, &prop); sErr != nil {
					rows.Close()
					return sErr
				}
				var rec synthesisSUHistoryReceipt
				if err := json.Unmarshal([]byte(prop), &rec); err != nil || !strings.EqualFold(rec.SU, suName) {
					continue
				}
				switch rec.Kind {
				case "synthesis_su_reassignment":
					items = append(items, histRow{id: id, rec: rec})
				case "synthesis_su_reassignment_undo":
					if rec.UndoneHistoryID > 0 {
						undoneSet[rec.UndoneHistoryID] = true
					}
				}
			}
			if err := errors.Join(rows.Err(), rows.Close()); err != nil {
				return err
			}

			var target *histRow
			if req.HistoryID > 0 {
				for i := range items {
					if items[i].id == req.HistoryID {
						if undoneSet[items[i].id] {
							return fmt.Errorf("synthesis history #%d has already been undone", req.HistoryID)
						}
						target = &items[i]
						break
					}
				}
				if target == nil {
					return fmt.Errorf("synthesis history #%d not found for SU %s", req.HistoryID, suName)
				}
			} else {
				for i := len(items) - 1; i >= 0; i-- {
					if !undoneSet[items[i].id] {
						target = &items[i]
						break
					}
				}
				if target == nil {
					return errors.New("no reversible synthesis reassignment remaining on undo stack")
				}
			}

			qTable := vprocore.QuoteHeaderIdentifier(suTable)
			d := target.rec
			undonePlot = d.PlotNumber
			if d.AfterSiteUnit != nil {
				beforeUnit = *d.AfterSiteUnit
			}
			if d.BeforeSiteUnit != nil {
				afterUnit = *d.BeforeSiteUnit
			}

			var currentSU sql.NullString
			curErr := tx.QueryRowContext(ctx,
				`SELECT "SiteUnit" FROM main.`+qTable+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
				d.PlotNumber,
			).Scan(&currentSU)
			if curErr != nil && !errors.Is(curErr, sql.ErrNoRows) {
				return curErr
			}
			expectedAfter := ""
			if d.AfterSiteUnit != nil {
				expectedAfter = strings.TrimSpace(*d.AfterSiteUnit)
			}
			actualCurrent := ""
			if curErr == nil && currentSU.Valid {
				actualCurrent = strings.TrimSpace(currentSU.String)
			}
			if errors.Is(curErr, sql.ErrNoRows) || actualCurrent != expectedAfter {
				return fmt.Errorf("plot %q SiteUnit changed since reassignment (%q != %q); undo refused", d.PlotNumber, actualCurrent, expectedAfter)
			}

			if !d.ExistedBefore {
				if _, err := tx.ExecContext(ctx,
					`DELETE FROM main.`+qTable+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
					d.PlotNumber,
				); err != nil {
					return err
				}
			} else {
				var existingRowID int64
				rErr := tx.QueryRowContext(ctx,
					`SELECT rowid FROM main.`+qTable+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
					d.PlotNumber,
				).Scan(&existingRowID)
				if errors.Is(rErr, sql.ErrNoRows) {
					if _, err := tx.ExecContext(ctx,
						`INSERT INTO main.`+qTable+` ("PlotNumber","SiteUnit") VALUES (?,?)`,
						d.PlotNumber, d.BeforeSiteUnit,
					); err != nil {
						return err
					}
				} else if rErr != nil {
					return rErr
				} else {
					if _, err := tx.ExecContext(ctx,
						`UPDATE main.`+qTable+` SET "SiteUnit" = ? WHERE rowid = ?`,
						d.BeforeSiteUnit, existingRowID,
					); err != nil {
						return err
					}
				}
			}

			undoRec := synthesisSUHistoryReceipt{
				Kind:            "synthesis_su_reassignment_undo",
				ContextID:       contextID,
				Project:         owner.Selection.Project,
				SU:              suName,
				PlotNumber:      d.PlotNumber,
				ExistedBefore:   d.ExistedAfter,
				BeforeSiteUnit:  d.AfterSiteUnit,
				ExistedAfter:    d.ExistedBefore,
				AfterSiteUnit:   d.BeforeSiteUnit,
				UndoneHistoryID: target.id,
			}
			rawUndo, err := json.Marshal(undoRec)
			if err != nil {
				return err
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, synthesisSUHistoryTable, synthesisSUHistorySQL, string(rawUndo), "synthesis SU reassignment undo"); err != nil {
				return err
			}
			_ = tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM main.`+vprocore.QuoteHeaderIdentifier(synthesisSUHistoryTable)).Scan(&undoHistoryID)

			if err := validateFiles(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return tx.Commit()
		})
		if err != nil {
			return nil, err
		}

		recomputeOpts := req.Options
		if recomputeOpts.UnitSource == "" {
			recomputeOpts.UnitSource = "su"
		}
		snap, err := computeSnapshotForPlots(ctx, owner, contextID, recomputeOpts)
		if err != nil {
			return nil, err
		}

		latencyMs := float64(time.Since(started).Microseconds()) / 1000.0
		return &ReassignSynthesisPlotResult{
			ContextID:      contextID,
			SU:             suName,
			TableName:      suTable,
			PlotNumber:     undonePlot,
			BeforeSiteUnit: beforeUnit,
			AfterSiteUnit:  afterUnit,
			Changed:        true,
			HistoryID:      undoHistoryID,
			LatencyMs:      RoundACE15(latencyMs, 2),
			Snapshot:       snap,
		}, nil
	})
}
