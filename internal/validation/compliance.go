package validation

import (
	"context"
	"crypto/rand"
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
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type ComplianceOptions struct {
	UseActiveSU bool `json:"useActiveSu"`
}

type EnvironmentCodeFinding struct {
	ListName   string `json:"listName"`
	Field      string `json:"field"`
	PlotNumber string `json:"plotNumber"`
	Value      string `json:"value"`
}

type SkippedListDefinition struct {
	ListName string `json:"listName"`
	Field    string `json:"field"`
	Reason   string `json:"reason"`
}

type BoundsFinding struct {
	Table      string `json:"table"`
	PlotNumber string `json:"plotNumber"`
	RowID      int64  `json:"rowId"`
	Field      string `json:"field"`
	Value      string `json:"value"`
	Rule       string `json:"rule"`
}

type EnvAdminCompletenessFinding struct {
	PlotNumber string `json:"plotNumber"`
	Issue      string `json:"issue"` // "missing_admin" | "orphan_admin"
}

type OrphanChildFinding struct {
	Table      string `json:"table"` // "Veg" | "Humus" | "Mineral" | "Other" | "Audit" | "SU"
	PlotNumber string `json:"plotNumber"`
	RowCount   int    `json:"rowCount"`
}

type DuplicateVegSpeciesFinding struct {
	PlotNumber        string   `json:"plotNumber"`
	Species           string   `json:"species"`
	RowCount          int      `json:"rowCount"`
	OverlappingLayers []string `json:"overlappingLayers"`
}

type TaxonomyComplianceFinding struct {
	RowID         int64   `json:"rowId"`
	ID            int64   `json:"id"`
	PlotNumber    string  `json:"plotNumber"`
	Species       string  `json:"species"`
	Status        string  `json:"status"` // "missing" | "synonym" | "user" | "unlisted"
	SuggestedCode *string `json:"suggestedCode,omitempty"`
}

type ComplianceSummaryCounts struct {
	EnvCodeErrors         int `json:"envCodeErrors"`
	BoundsErrors          int `json:"boundsErrors"`
	EnvAdminMismatches    int `json:"envAdminMismatches"`
	OrphanChildRows       int `json:"orphanChildRows"`
	DuplicateSpeciesSplits int `json:"duplicateSpeciesSplits"`
	UnlistedSpecies       int `json:"unlistedSpecies"`
	SynonymSpecies        int `json:"synonymSpecies"`
	UserSpecies           int `json:"userSpecies"`
	PlotsWithoutVeg       int `json:"plotsWithoutVeg"`
}

type ComplianceValidationReport struct {
	Project                string                        `json:"project"`
	SU                     string                        `json:"su"`
	UseActiveSU            bool                          `json:"useActiveSu"`
	Passed                 bool                          `json:"passed"`
	Summary                ComplianceSummaryCounts       `json:"summary"`
	EnvCodeFindings        []EnvironmentCodeFinding      `json:"envCodeFindings"`
	SkippedDefinitions     []SkippedListDefinition       `json:"skippedDefinitions"`
	BoundsFindings         []BoundsFinding               `json:"boundsFindings"`
	EnvAdminFindings       []EnvAdminCompletenessFinding `json:"envAdminFindings"`
	OrphanFindings         []OrphanChildFinding          `json:"orphanFindings"`
	PlotsWithoutVegetation []string                      `json:"plotsWithoutVegetation"`
	DuplicateVegFindings   []DuplicateVegSpeciesFinding  `json:"duplicateVegFindings"`
	TaxonomyFindings       []TaxonomyComplianceFinding   `json:"taxonomyFindings"`
}

func utf16CodeUnitLen(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

func isTextOrNoneAffinityType(declType string) bool {
	u := strings.ToUpper(strings.TrimSpace(declType))
	if u == "" {
		return true
	}
	if strings.Contains(u, "INT") {
		return false
	}
	if strings.Contains(u, "CHAR") || strings.Contains(u, "CLOB") || strings.Contains(u, "TEXT") || strings.Contains(u, "BLOB") {
		return true
	}
	return false
}

func readTableColumnTypes(ctx context.Context, tx *sql.Tx, alias, table string) (map[string]string, map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA "+vprocore.QuoteHeaderIdentifier(alias)+".table_info("+vprocore.QuoteHeaderIdentifier(table)+")")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	names := map[string]string{}
	types := map[string]string{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
			return nil, nil, err
		}
		lower := strings.ToLower(name)
		names[lower] = name
		types[lower] = typ
	}
	return names, types, rows.Err()
}

func buildSUPlotInPreFilter(colExpr string, declType string, suPlots []string) (string, []any) {
	if len(suPlots) == 0 || len(suPlots) > 900 || !isTextOrNoneAffinityType(declType) {
		return "", nil
	}
	placeholders := make([]string, len(suPlots))
	args := make([]any, len(suPlots))
	for i, p := range suPlots {
		if !utf8.ValidString(p) || strings.ContainsRune(p, 0) {
			return "", nil
		}
		placeholders[i] = "?"
		args[i] = p
	}
	return colExpr + " IN (" + strings.Join(placeholders, ",") + ")", args
}

func isValidSingleCover(val float64) bool {
	if math.IsNaN(val) || math.IsInf(val, 0) || val < 0 || math.Abs(val) > math.MaxFloat32 {
		return false
	}
	f32 := float32(val)
	return f32 >= 0 && f32 < 100
}

// RunComplianceValidation executes the unified Pre-Flight & Standards Compliance gate (Auto-4):
// 1. VLists.USysTableOfLists (Validate <> 0) code compliance
// 2. Numeric, IEEE-754 float32 <100 cover, LMH25 soil horizon sign, and UTF-16 length bounds
// 3. 1:1 _Env <-> _Admin completeness
// 4. 1:N foreign key integrity (_Veg, _Humus, _Mineral, _Other, _Audit, *_SU)
// 5. Duplicate (PlotNumber, Species) strata splits & same-layer cover duplicates
// 6. USysAllSpecs & USysUserSpp species code taxonomy checks (including OldCode -> Code synonyms).
func RunComplianceValidation(s *vprocore.ContextService, ctx context.Context, contextID string, opts ComplianceOptions) (ComplianceValidationReport, error) {
	if s == nil {
		return ComplianceValidationReport{}, errors.New("validation context service is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (report ComplianceValidationReport, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return report, err
		}
		defer owner.Mu.Unlock()
		if owner.Conn == nil {
			return report, errors.New("project context is closed")
		}

		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return report, err
		}
		defer func() {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, rb)
			}
		}()

		project := owner.Selection.Project
		activeSU := owner.Selection.SU
		useSU := opts.UseActiveSU && activeSU != "None"

		report = ComplianceValidationReport{
			Project:                project,
			SU:                     activeSU,
			UseActiveSU:            useSU,
			EnvCodeFindings:        []EnvironmentCodeFinding{},
			SkippedDefinitions:     []SkippedListDefinition{},
			BoundsFindings:         []BoundsFinding{},
			EnvAdminFindings:       []EnvAdminCompletenessFinding{},
			OrphanFindings:         []OrphanChildFinding{},
			PlotsWithoutVegetation: []string{},
			DuplicateVegFindings:   []DuplicateVegSpeciesFinding{},
			TaxonomyFindings:       []TaxonomyComplianceFinding{},
		}

		envTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Env")
		adminTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Admin")

		type plotKeyEntry struct {
			isText  bool
			rawKey  string
			display string
		}
		var envOrdered []plotKeyEntry
		envTextPlots := map[string]struct{}{}

		envScanRows, err := tx.QueryContext(ctx, `SELECT typeof("PlotNumber"), CAST("PlotNumber" AS BLOB), COALESCE(CAST("PlotNumber" AS TEXT), '<NULL>')
			FROM `+envTbl+`
			ORDER BY "PlotNumber"`)
		if err != nil {
			return report, err
		}
		for envScanRows.Next() {
			var pType, disp string
			var raw []byte
			if err := envScanRows.Scan(&pType, &raw, &disp); err != nil {
				envScanRows.Close()
				return report, err
			}
			isText := pType == "text"
			rawStr := string(raw)
			if isText {
				envTextPlots[rawStr] = struct{}{}
			}
			envOrdered = append(envOrdered, plotKeyEntry{isText: isText, rawKey: rawStr, display: disp})
		}
		if err := errors.Join(envScanRows.Err(), envScanRows.Close()); err != nil {
			return report, err
		}

		// 1. 1:1 _Env <-> _Admin completeness (always project-wide on physical tables).
		adminTextPlots := make(map[string]struct{}, len(envTextPlots))
		var orphanAdmins []string
		adminScanRows, err := tx.QueryContext(ctx, `SELECT typeof("Plot"), CAST("Plot" AS BLOB), COALESCE(CAST("Plot" AS TEXT), '<NULL>')
			FROM `+adminTbl+`
			ORDER BY "Plot"`)
		if err != nil {
			return report, err
		}
		for adminScanRows.Next() {
			var pType, disp string
			var raw []byte
			if err := adminScanRows.Scan(&pType, &raw, &disp); err != nil {
				adminScanRows.Close()
				return report, err
			}
			rawStr := string(raw)
			if pType == "text" {
				adminTextPlots[rawStr] = struct{}{}
				if _, ok := envTextPlots[rawStr]; !ok {
					orphanAdmins = append(orphanAdmins, disp)
				}
			} else {
				orphanAdmins = append(orphanAdmins, disp)
			}
		}
		if err := errors.Join(adminScanRows.Err(), adminScanRows.Close()); err != nil {
			return report, err
		}

		for _, eRow := range envOrdered {
			if !eRow.isText {
				report.EnvAdminFindings = append(report.EnvAdminFindings, EnvAdminCompletenessFinding{
					PlotNumber: eRow.display,
					Issue:      "missing_admin",
				})
				continue
			}
			if _, ok := adminTextPlots[eRow.rawKey]; !ok {
				report.EnvAdminFindings = append(report.EnvAdminFindings, EnvAdminCompletenessFinding{
					PlotNumber: eRow.display,
					Issue:      "missing_admin",
				})
			}
		}
		for _, p := range orphanAdmins {
			report.EnvAdminFindings = append(report.EnvAdminFindings, EnvAdminCompletenessFinding{
				PlotNumber: p,
				Issue:      "orphan_admin",
			})
		}
		report.Summary.EnvAdminMismatches = len(report.EnvAdminFindings)

		// 2. 1:N Foreign Key Integrity across _Veg, _Humus, _Mineral, _Other, _Audit, and active *_SU.
		vegTextPlots := make(map[string]struct{}, len(envTextPlots))
		for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other", "Audit"} {
			childTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_"+suffix)
			whereClause := ""
			if suffix == "Audit" {
				// System-level audit entries (e.g. NewProject, USysAllSpecs) legitimately have NULL PlotNumber.
				whereClause = ` WHERE "PlotNumber" IS NOT NULL AND CAST("PlotNumber" AS TEXT) <> ''`
			}
			cRows, cErr := tx.QueryContext(ctx, `SELECT typeof("PlotNumber"), CAST("PlotNumber" AS BLOB), COALESCE(CAST("PlotNumber" AS TEXT), '<NULL>'), COUNT(*)
				FROM `+childTbl+whereClause+`
				GROUP BY "PlotNumber", typeof("PlotNumber"), CAST("PlotNumber" AS BLOB)`)
			if cErr != nil {
				return report, cErr
			}
			orphanCounts := map[string]int{}
			for cRows.Next() {
				var pType, disp string
				var raw []byte
				var grpCnt int
				if err := cRows.Scan(&pType, &raw, &disp, &grpCnt); err != nil {
					cRows.Close()
					return report, err
				}
				rawStr := string(raw)
				if suffix == "Veg" && pType == "text" {
					vegTextPlots[rawStr] = struct{}{}
				}
				if pType != "text" {
					orphanCounts[disp] += grpCnt
				} else if _, ok := envTextPlots[rawStr]; !ok {
					orphanCounts[disp] += grpCnt
				}
			}
			if err := errors.Join(cRows.Err(), cRows.Close()); err != nil {
				return report, err
			}
			if len(orphanCounts) > 0 {
				keys := make([]string, 0, len(orphanCounts))
				for k := range orphanCounts {
					keys = append(keys, k)
				}
				slices.Sort(keys)
				for _, k := range keys {
					cnt := orphanCounts[k]
					report.OrphanFindings = append(report.OrphanFindings, OrphanChildFinding{
						Table:      suffix,
						PlotNumber: k,
						RowCount:   cnt,
					})
					if suffix != "Audit" {
						report.Summary.OrphanChildRows += cnt
					}
				}
			}
		}

		suTextPlots := map[string]struct{}{}
		var suPlotList []string
		if activeSU != "None" {
			suTbl := `"su".` + vprocore.QuoteHeaderIdentifier(activeSU+"_SU")
			suRows, suErr := tx.QueryContext(ctx, `SELECT typeof("PlotNumber"), CAST("PlotNumber" AS BLOB), COALESCE(CAST("PlotNumber" AS TEXT), '<NULL>')
				FROM `+suTbl)
			if suErr != nil {
				return report, suErr
			}
			suOrphanCounts := map[string]int{}
			for suRows.Next() {
				var pType, disp string
				var raw []byte
				if err := suRows.Scan(&pType, &raw, &disp); err != nil {
					suRows.Close()
					return report, err
				}
				rawStr := string(raw)
				if pType == "text" {
					if _, seen := suTextPlots[rawStr]; !seen {
						suTextPlots[rawStr] = struct{}{}
						suPlotList = append(suPlotList, rawStr)
					}
					if _, ok := envTextPlots[rawStr]; !ok {
						suOrphanCounts[disp]++
					}
				} else {
					suOrphanCounts[disp]++
				}
			}
			if err := errors.Join(suRows.Err(), suRows.Close()); err != nil {
				return report, err
			}
			if len(suOrphanCounts) > 0 {
				keys := make([]string, 0, len(suOrphanCounts))
				for k := range suOrphanCounts {
					keys = append(keys, k)
				}
				slices.Sort(keys)
				for _, k := range keys {
					report.OrphanFindings = append(report.OrphanFindings, OrphanChildFinding{
						Table:      "SU",
						PlotNumber: k,
						RowCount:   suOrphanCounts[k],
					})
				}
			}
		}

		// Plots without vegetation (respects UseActiveSU).
		for _, eRow := range envOrdered {
			if useSU {
				if !eRow.isText {
					continue
				}
				if _, inSU := suTextPlots[eRow.rawKey]; !inSU {
					continue
				}
			}
			if !eRow.isText {
				report.PlotsWithoutVegetation = append(report.PlotsWithoutVegetation, eRow.display)
				continue
			}
			if _, hasVeg := vegTextPlots[eRow.rawKey]; !hasVeg {
				report.PlotsWithoutVegetation = append(report.PlotsWithoutVegetation, eRow.display)
			}
		}
		report.Summary.PlotsWithoutVeg = len(report.PlotsWithoutVegetation)

		// 3. VLists.USysTableOfLists (Validate <> 0) code compliance.
		envCols, envTypes, err := readTableColumnTypes(ctx, tx, "project", project+"_Env")
		if err != nil {
			return report, err
		}
		adminCols, adminTypes, err := readTableColumnTypes(ctx, tx, "project", project+"_Admin")
		if err != nil {
			return report, err
		}

		defRows, err := tx.QueryContext(ctx, `SELECT COALESCE(MIN("ListName"), ''), COALESCE(MIN("FieldUsedIn"), ''), COALESCE(CAST("ValidateLoops" AS TEXT), '')
			FROM "VLists"."USysTableOfLists"
			WHERE "Validate" <> 0
			GROUP BY "ListName" COLLATE NOCASE, "FieldUsedIn" COLLATE NOCASE, "ValidateLoops"
			ORDER BY "ListName" COLLATE NOCASE, "FieldUsedIn" COLLATE NOCASE, "ValidateLoops"`)
		if err != nil {
			return report, err
		}
		type listDef struct {
			listName, fieldUsedIn, loops string
		}
		var defs []listDef
		for defRows.Next() {
			var d listDef
			if err := defRows.Scan(&d.listName, &d.fieldUsedIn, &d.loops); err != nil {
				defRows.Close()
				return report, err
			}
			defs = append(defs, d)
		}
		if err := errors.Join(defRows.Err(), defRows.Close()); err != nil {
			return report, err
		}

		validItemsByList := map[string]map[string]struct{}{}
		itemRows, err := tx.QueryContext(ctx, `SELECT COALESCE(CAST("ListName" AS TEXT), ''), COALESCE(CAST("Item" AS TEXT), '')
			FROM "VLists"."USysTableOfLists"
			WHERE "ListName" IS NOT NULL AND "Item" IS NOT NULL`)
		if err != nil {
			return report, err
		}
		for itemRows.Next() {
			var ln, it string
			if err := itemRows.Scan(&ln, &it); err != nil {
				itemRows.Close()
				return report, err
			}
			lnKey := asciiLower(ln)
			m := validItemsByList[lnKey]
			if m == nil {
				m = map[string]struct{}{}
				validItemsByList[lnKey] = m
			}
			m[asciiLower(it)] = struct{}{}
		}
		if err := errors.Join(itemRows.Err(), itemRows.Close()); err != nil {
			return report, err
		}

		type expandedCodeCheck struct {
			listName    string
			listLower   string
			field       string
			inAdmin     bool
			colActual   string
			colScanSlot int
			findingsSet map[[2]string]struct{}
		}
		var checks []*expandedCodeCheck
		var envCheckCols []string
		envColSlot := map[string]int{}
		var adminCheckCols []string
		adminColSlot := map[string]int{}

		for _, d := range defs {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			if strings.TrimSpace(d.listName) == "" {
				report.SkippedDefinitions = append(report.SkippedDefinitions, SkippedListDefinition{d.listName, d.fieldUsedIn, "ListName is blank"})
				continue
			}
			if strings.TrimSpace(d.fieldUsedIn) == "" {
				report.SkippedDefinitions = append(report.SkippedDefinitions, SkippedListDefinition{d.listName, d.fieldUsedIn, "FieldUsedIn is blank"})
				continue
			}
			expandedFields := []string{d.fieldUsedIn}
			if strings.TrimSpace(d.loops) != "" {
				nLoops, pErr := strconv.Atoi(strings.TrimSpace(d.loops))
				if pErr != nil {
					report.SkippedDefinitions = append(report.SkippedDefinitions, SkippedListDefinition{d.listName, d.fieldUsedIn, "ValidateLoops is not an integer"})
					continue
				}
				if nLoops > 0 {
					expandedFields = make([]string, nLoops)
					for idx := 1; idx <= nLoops; idx++ {
						expandedFields[idx-1] = fmt.Sprintf("%s%d", d.fieldUsedIn, idx)
					}
				}
			}

			for _, field := range expandedFields {
				lowerF := strings.ToLower(field)
				envActual, inEnv := envCols[lowerF]
				adminActual, inAdmin := adminCols[lowerF]
				if inEnv && inAdmin {
					report.SkippedDefinitions = append(report.SkippedDefinitions, SkippedListDefinition{d.listName, field, "Field is ambiguous across Env and Admin"})
					continue
				}
				if !inEnv && !inAdmin {
					report.SkippedDefinitions = append(report.SkippedDefinitions, SkippedListDefinition{d.listName, field, "Field does not exist in active Env/Admin schema"})
					continue
				}
				chk := &expandedCodeCheck{
					listName:    d.listName,
					listLower:   asciiLower(d.listName),
					field:       field,
					inAdmin:     inAdmin,
					findingsSet: map[[2]string]struct{}{},
				}
				if inAdmin {
					chk.colActual = adminActual
					slot, ok := adminColSlot[adminActual]
					if !ok {
						slot = len(adminCheckCols)
						adminColSlot[adminActual] = slot
						adminCheckCols = append(adminCheckCols, adminActual)
					}
					chk.colScanSlot = slot
				} else {
					chk.colActual = envActual
					slot, ok := envColSlot[envActual]
					if !ok {
						slot = len(envCheckCols)
						envColSlot[envActual] = slot
						envCheckCols = append(envCheckCols, envActual)
					}
					chk.colScanSlot = slot
				}
				checks = append(checks, chk)
			}
		}

		qualifiesForEnvCodeCheck := func(rawPlot string) bool {
			if _, ok := envTextPlots[rawPlot]; !ok {
				return false
			}
			if _, ok := adminTextPlots[rawPlot]; !ok {
				return false
			}
			if useSU {
				if _, ok := suTextPlots[rawPlot]; !ok {
					return false
				}
			}
			return true
		}

		scanTableForCodeChecks := func(tblExpr, plotCol, plotDecl string, cols []string, wantAdmin bool) error {
			if len(cols) == 0 || (useSU && len(suTextPlots) == 0) {
				return nil
			}
			qPlot := vprocore.QuoteHeaderIdentifier(plotCol)
			selParts := make([]string, 0, 2+len(cols))
			selParts = append(selParts, `CAST(`+qPlot+` AS BLOB)`, `CAST(`+qPlot+` AS TEXT)`)
			for _, c := range cols {
				selParts = append(selParts, `CAST(`+vprocore.QuoteHeaderIdentifier(c)+` AS TEXT)`)
			}
			whereSQL := ` WHERE typeof(` + qPlot + `)='text'`
			var qArgs []any
			if useSU {
				if pre, args := buildSUPlotInPreFilter(qPlot, plotDecl, suPlotList); pre != "" {
					whereSQL += ` AND ` + pre
					qArgs = append(qArgs, args...)
				}
			}
			rows, err := tx.QueryContext(ctx, `SELECT `+strings.Join(selParts, ", ")+` FROM `+tblExpr+whereSQL, qArgs...)
			if err != nil {
				return err
			}
			defer rows.Close()

			vals := make([]sql.NullString, len(cols))
			var rawPlot []byte
			var plotStr string
			dest := make([]any, 2+len(cols))
			dest[0] = &rawPlot
			dest[1] = &plotStr
			for i := range vals {
				dest[2+i] = &vals[i]
			}

			for rows.Next() {
				if err := rows.Scan(dest...); err != nil {
					return err
				}
				if !qualifiesForEnvCodeCheck(string(rawPlot)) {
					continue
				}
				for _, chk := range checks {
					if chk.inAdmin != wantAdmin {
						continue
					}
					cell := vals[chk.colScanSlot]
					if !cell.Valid || cell.String == "" {
						continue
					}
					allowed := validItemsByList[chk.listLower]
					if _, ok := allowed[asciiLower(cell.String)]; !ok {
						chk.findingsSet[[2]string{plotStr, cell.String}] = struct{}{}
					}
				}
			}
			return rows.Err()
		}

		if err := scanTableForCodeChecks(envTbl, "PlotNumber", envTypes["plotnumber"], envCheckCols, false); err != nil {
			return report, err
		}
		if err := scanTableForCodeChecks(adminTbl, "Plot", adminTypes["plot"], adminCheckCols, true); err != nil {
			return report, err
		}

		for _, chk := range checks {
			if len(chk.findingsSet) == 0 {
				continue
			}
			pairs := make([][2]string, 0, len(chk.findingsSet))
			for pair := range chk.findingsSet {
				pairs = append(pairs, pair)
			}
			slices.SortFunc(pairs, func(a, b [2]string) int {
				if cmp := strings.Compare(a[0], b[0]); cmp != 0 {
					return cmp
				}
				return strings.Compare(a[1], b[1])
			})
			for _, pair := range pairs {
				report.EnvCodeFindings = append(report.EnvCodeFindings, EnvironmentCodeFinding{
					ListName:   chk.listName,
					Field:      chk.field,
					PlotNumber: pair[0],
					Value:      pair[1],
				})
			}
		}
		report.Summary.EnvCodeErrors = len(report.EnvCodeFindings)

		// 4. UTF-16, Numeric, IEEE-754 float32 <100 cover, and LMH25 soil horizon sign bounds.
		if err := checkProjectBounds(ctx, tx, project, useSU, suTextPlots, suPlotList, envCols, envTypes, adminCols, adminTypes, &report); err != nil {
			return report, err
		}
		report.Summary.BoundsErrors = len(report.BoundsFindings)

		// 5. Duplicate (PlotNumber, Species) strata splits & overlapping layer covers.
		if err := checkDuplicateVegSpecies(ctx, tx, project, &report); err != nil {
			return report, err
		}
		report.Summary.DuplicateSpeciesSplits = len(report.DuplicateVegFindings)

		// 6. USysAllSpecs & USysUserSpp species taxonomy compliance.
		if err := checkTaxonomyCompliance(ctx, tx, project, useSU, suTextPlots, suPlotList, &report); err != nil {
			return report, err
		}

		// Passed is true when there are zero compliance violations.
		report.Passed = report.Summary.EnvCodeErrors == 0 &&
			report.Summary.BoundsErrors == 0 &&
			report.Summary.EnvAdminMismatches == 0 &&
			report.Summary.OrphanChildRows == 0 &&
			report.Summary.DuplicateSpeciesSplits == 0 &&
			report.Summary.UnlistedSpecies == 0 &&
			report.Summary.SynonymSpecies == 0

		if err := ctx.Err(); err != nil {
			return report, err
		}
		if err := tx.Commit(); err != nil {
			return report, err
		}
		return report, nil
	})
}

func checkSingleCoverColumns(
	ctx context.Context,
	tx *sql.Tx,
	project string,
	suffix string,
	plotCol string,
	plotDecl string,
	cols []string,
	useSU bool,
	suTextPlots map[string]struct{},
	suPlotList []string,
	report *ComplianceValidationReport,
) error {
	if len(cols) == 0 || (useSU && len(suTextPlots) == 0) {
		return nil
	}
	tbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_"+suffix)
	qPlot := vprocore.QuoteHeaderIdentifier(plotCol)

	selParts := make([]string, 0, 4+3*len(cols))
	selParts = append(selParts,
		`t.rowid`,
		`typeof(t.`+qPlot+`)`,
		`CAST(t.`+qPlot+` AS BLOB)`,
		`COALESCE(CAST(t.`+qPlot+` AS TEXT), '')`,
	)
	candClauses := make([]string, 0, len(cols))
	for _, col := range cols {
		qCol := vprocore.QuoteHeaderIdentifier(col)
		selParts = append(selParts,
			`typeof(t.`+qCol+`)`,
			`CAST(t.`+qCol+` AS REAL)`,
			`COALESCE(CAST(t.`+qCol+` AS TEXT), '')`,
		)
		// Any valid IEEE-754 float32 cover in [0, 100) has float64 < 99.99999618530273.
		// Pre-filtering candidates >= 99.9999 (or < 0 or non-numeric) in C-level SQLite avoids scanning clean cells into Go.
		candClauses = append(candClauses,
			`(t.`+qCol+` IS NOT NULL AND (typeof(t.`+qCol+`) NOT IN ('real','integer') OR t.`+qCol+` < 0 OR t.`+qCol+` >= 99.9999))`,
		)
	}

	whereSQL := ` WHERE (` + strings.Join(candClauses, " OR ") + `)`
	var qArgs []any
	if useSU {
		if pre, args := buildSUPlotInPreFilter(`t.`+qPlot, plotDecl, suPlotList); pre != "" {
			whereSQL += ` AND ` + pre
			qArgs = append(qArgs, args...)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+strings.Join(selParts, ", ")+` FROM `+tbl+` t`+whereSQL+` ORDER BY t.rowid`, qArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	perColFindings := make([][]BoundsFinding, len(cols))
	colTypes := make([]string, len(cols))
	colVals := make([]sql.NullFloat64, len(cols))
	colTexts := make([]string, len(cols))
	var rid int64
	var plotType, plotStr string
	var rawPlot []byte
	dest := make([]any, 4+3*len(cols))
	dest[0] = &rid
	dest[1] = &plotType
	dest[2] = &rawPlot
	dest[3] = &plotStr
	for i := range cols {
		dest[4+3*i] = &colTypes[i]
		dest[4+3*i+1] = &colVals[i]
		dest[4+3*i+2] = &colTexts[i]
	}

	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		if useSU {
			if plotType != "text" {
				continue
			}
			if _, ok := suTextPlots[string(rawPlot)]; !ok {
				continue
			}
		}
		for i, col := range cols {
			typ := colTypes[i]
			if typ == "null" {
				continue
			}
			fVal := colVals[i]
			if (typ != "real" && typ != "integer") || !fVal.Valid || !isValidSingleCover(fVal.Float64) {
				perColFindings[i] = append(perColFindings[i], BoundsFinding{
					Table:      suffix,
					PlotNumber: plotStr,
					RowID:      rid,
					Field:      col,
					Value:      colTexts[i],
					Rule:       "IEEE-754 SINGLE cover must be >= 0 and < 100",
				})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, list := range perColFindings {
		report.BoundsFindings = append(report.BoundsFindings, list...)
	}
	return nil
}

func checkProjectBounds(
	ctx context.Context,
	tx *sql.Tx,
	project string,
	useSU bool,
	suTextPlots map[string]struct{},
	suPlotList []string,
	envCols, envTypes, adminCols, adminTypes map[string]string,
	report *ComplianceValidationReport,
) error {
	if useSU && len(suTextPlots) == 0 {
		return nil
	}

	_, vegTypes, err := readTableColumnTypes(ctx, tx, "project", project+"_Veg")
	if err != nil {
		return err
	}
	_, humusTypes, err := readTableColumnTypes(ctx, tx, "project", project+"_Humus")
	if err != nil {
		return err
	}
	_, minTypes, err := readTableColumnTypes(ctx, tx, "project", project+"_Mineral")
	if err != nil {
		return err
	}

	// Text UTF-16 length limits on core tables (grouped per table so each table is scanned at most once).
	type textColRule struct {
		col    string
		maxUTF int
	}
	type textTableGroup struct {
		suffix   string
		plotCol  string
		plotDecl string
		rules    []textColRule
	}
	textGroups := []textTableGroup{
		{"Env", "PlotNumber", envTypes["plotnumber"], []textColRule{
			{"PlotNumber", 7}, {"FieldNumber", 50}, {"ProjectID", 30},
			{"Zone", 4}, {"SubZone", 8}, {"SiteSeries", 5},
			{"Ecosection", 3}, {"NtsMapSheet", 8},
		}},
		{"Admin", "Plot", adminTypes["plot"], []textColRule{
			{"Plot", 7}, {"PlotType", 10},
		}},
		{"Veg", "PlotNumber", vegTypes["plotnumber"], []textColRule{
			{"PlotNumber", 7}, {"Species", 8}, {"Layer", 2}, {"Collected", 1},
		}},
		{"Humus", "PlotNumber", humusTypes["plotnumber"], []textColRule{
			{"PlotNumber", 7}, {"Horizon", 8},
		}},
		{"Mineral", "PlotNumber", minTypes["plotnumber"], []textColRule{
			{"PlotNumber", 7}, {"Horizon", 8}, {"Texture", 4}, {"Colour", 14},
		}},
	}

	for _, grp := range textGroups {
		tbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_"+grp.suffix)
		qPlot := vprocore.QuoteHeaderIdentifier(grp.plotCol)

		selParts := make([]string, 0, 4+len(grp.rules))
		selParts = append(selParts,
			`t.rowid`,
			`typeof(t.`+qPlot+`)`,
			`CAST(t.`+qPlot+` AS BLOB)`,
			`COALESCE(CAST(t.`+qPlot+` AS TEXT), '')`,
		)
		nonNullParts := make([]string, len(grp.rules))
		for i, r := range grp.rules {
			qCol := vprocore.QuoteHeaderIdentifier(r.col)
			selParts = append(selParts, `CAST(t.`+qCol+` AS BLOB)`)
			nonNullParts[i] = `t.` + qCol + ` IS NOT NULL`
		}
		whereSQL := ` WHERE (` + strings.Join(nonNullParts, " OR ") + `)`
		var qArgs []any
		if useSU {
			if pre, args := buildSUPlotInPreFilter(`t.`+qPlot, grp.plotDecl, suPlotList); pre != "" {
				whereSQL += ` AND ` + pre
				qArgs = append(qArgs, args...)
			}
		}

		rows, err := tx.QueryContext(ctx, `SELECT `+strings.Join(selParts, ", ")+` FROM `+tbl+` t`+whereSQL+` ORDER BY t.rowid`, qArgs...)
		if err != nil {
			return err
		}
		perRuleFindings := make([][]BoundsFinding, len(grp.rules))
		rawCells := make([][]byte, len(grp.rules))
		var rid int64
		var plotType, plotStr string
		var rawPlot []byte
		dest := make([]any, 4+len(grp.rules))
		dest[0] = &rid
		dest[1] = &plotType
		dest[2] = &rawPlot
		dest[3] = &plotStr
		for i := range rawCells {
			dest[4+i] = &rawCells[i]
		}

		for rows.Next() {
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return err
			}
			if useSU {
				if plotType != "text" {
					continue
				}
				if _, ok := suTextPlots[string(rawPlot)]; !ok {
					continue
				}
			}
			for i, r := range grp.rules {
				raw := rawCells[i]
				if raw == nil {
					continue
				}
				if !utf8.Valid(raw) {
					perRuleFindings[i] = append(perRuleFindings[i], BoundsFinding{
						Table:      grp.suffix,
						PlotNumber: plotStr,
						RowID:      rid,
						Field:      r.col,
						Value:      fmt.Sprintf("0x%x", raw),
						Rule:       "Malformed UTF-8 byte sequence",
					})
					continue
				}
				s := string(raw)
				if utf16CodeUnitLen(s) > r.maxUTF {
					perRuleFindings[i] = append(perRuleFindings[i], BoundsFinding{
						Table:      grp.suffix,
						PlotNumber: plotStr,
						RowID:      rid,
						Field:      r.col,
						Value:      s,
						Rule:       fmt.Sprintf("Exceeds %d UTF-16 code units", r.maxUTF),
					})
				}
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
		for _, list := range perRuleFindings {
			report.BoundsFindings = append(report.BoundsFindings, list...)
		}
	}

	// Check IEEE-754 SINGLE (<100 and >=0) cover bounds in _Veg, _Env.StrataCover*, and _Admin.StrataCoverTotal.
	vegCoverCols := []string{
		"Cover1", "Cover2", "Cover3", "TotalA",
		"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB",
		"Cover6", "Cover7", "Cover8", "Cover9", "Cover10",
	}
	if err := checkSingleCoverColumns(ctx, tx, project, "Veg", "PlotNumber", vegTypes["plotnumber"], vegCoverCols, useSU, suTextPlots, suPlotList, report); err != nil {
		return err
	}

	var envStrataCols []string
	for _, col := range []string{"StrataCoverTree", "StrataCoverShrub", "StrataCoverHerb", "StrataCoverMoss"} {
		if actual, ok := envCols[strings.ToLower(col)]; ok {
			envStrataCols = append(envStrataCols, actual)
		}
	}
	if err := checkSingleCoverColumns(ctx, tx, project, "Env", "PlotNumber", envTypes["plotnumber"], envStrataCols, useSU, suTextPlots, suPlotList, report); err != nil {
		return err
	}

	var adminStrataCols []string
	if actual, ok := adminCols["stratacovertotal"]; ok {
		adminStrataCols = append(adminStrataCols, actual)
	}
	if err := checkSingleCoverColumns(ctx, tx, project, "Admin", "Plot", adminTypes["plot"], adminStrataCols, useSU, suTextPlots, suPlotList, report); err != nil {
		return err
	}

	// Check numeric bounds on _Env (Latitude, Longitude, Aspect, SlopeGradient) & _Admin (StartDate).
	envTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Env")
	envWhere := ` WHERE ("Latitude" IS NOT NULL OR "Longitude" IS NOT NULL OR "Aspect" IS NOT NULL OR "SlopeGradient" IS NOT NULL)`
	var envArgs []any
	if useSU {
		if pre, args := buildSUPlotInPreFilter(`t."PlotNumber"`, envTypes["plotnumber"], suPlotList); pre != "" {
			envWhere += ` AND ` + pre
			envArgs = append(envArgs, args...)
		}
	}
	envNumRows, err := tx.QueryContext(ctx, `SELECT t.rowid, typeof(t."PlotNumber"), CAST(t."PlotNumber" AS BLOB), COALESCE(CAST(t."PlotNumber" AS TEXT), ''),
		t."Latitude", t."Longitude", t."Aspect", t."SlopeGradient"
		FROM `+envTbl+` t`+envWhere+` ORDER BY t.rowid`, envArgs...)
	if err != nil {
		return err
	}
	for envNumRows.Next() {
		var rid int64
		var plotType, plot string
		var rawPlot []byte
		var lat, lon, slope sql.NullFloat64
		var aspect sql.NullInt64
		if err := envNumRows.Scan(&rid, &plotType, &rawPlot, &plot, &lat, &lon, &aspect, &slope); err != nil {
			envNumRows.Close()
			return err
		}
		if useSU {
			if plotType != "text" {
				continue
			}
			if _, ok := suTextPlots[string(rawPlot)]; !ok {
				continue
			}
		}
		if lat.Valid && (lat.Float64 < -90 || lat.Float64 > 90) {
			report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
				Table: "Env", PlotNumber: plot, RowID: rid, Field: "Latitude",
				Value: vprocore.SourceDoubleDecimalString(lat.Float64), Rule: "Latitude must be between -90 and 90",
			})
		}
		if lon.Valid && (lon.Float64 < -180 || lon.Float64 > 180) {
			report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
				Table: "Env", PlotNumber: plot, RowID: rid, Field: "Longitude",
				Value: vprocore.SourceDoubleDecimalString(lon.Float64), Rule: "Longitude must be between -180 and 180",
			})
		}
		if aspect.Valid && aspect.Int64 != 999 && (aspect.Int64 < 0 || aspect.Int64 > 359) {
			report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
				Table: "Env", PlotNumber: plot, RowID: rid, Field: "Aspect",
				Value: strconv.FormatInt(aspect.Int64, 10), Rule: "Aspect must be 0..359 or 999 (level)",
			})
		}
		if slope.Valid && slope.Float64 < 0 {
			report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
				Table: "Env", PlotNumber: plot, RowID: rid, Field: "SlopeGradient",
				Value: vprocore.SourceDoubleDecimalString(slope.Float64), Rule: "SlopeGradient must be >= 0",
			})
		}
	}
	if err := errors.Join(envNumRows.Err(), envNumRows.Close()); err != nil {
		return err
	}

	adminTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Admin")
	adminWhere := ` WHERE t."StartDate" IS NOT NULL AND (typeof(t."StartDate") <> 'integer' OR t."StartDate" < 1900 OR t."StartDate" > 2500)`
	var adminArgs []any
	if useSU {
		if pre, args := buildSUPlotInPreFilter(`t."Plot"`, adminTypes["plot"], suPlotList); pre != "" {
			adminWhere += ` AND ` + pre
			adminArgs = append(adminArgs, args...)
		}
	}
	adminNumRows, err := tx.QueryContext(ctx, `SELECT t.rowid, typeof(t."Plot"), CAST(t."Plot" AS BLOB), COALESCE(CAST(t."Plot" AS TEXT), ''), t."StartDate"
		FROM `+adminTbl+` t`+adminWhere+` ORDER BY t.rowid`, adminArgs...)
	if err != nil {
		return err
	}
	for adminNumRows.Next() {
		var rid int64
		var plotType, plot string
		var rawPlot []byte
		var sd sql.NullInt64
		if err := adminNumRows.Scan(&rid, &plotType, &rawPlot, &plot, &sd); err != nil {
			adminNumRows.Close()
			return err
		}
		if useSU {
			if plotType != "text" {
				continue
			}
			if _, ok := suTextPlots[string(rawPlot)]; !ok {
				continue
			}
		}
		if sd.Valid && (sd.Int64 < 1900 || sd.Int64 > 2500) {
			report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
				Table: "Admin", PlotNumber: plot, RowID: rid, Field: "StartDate",
				Value: strconv.FormatInt(sd.Int64, 10), Rule: "StartDate must be between 1900 and 2500",
			})
		}
	}
	if err := errors.Join(adminNumRows.Err(), adminNumRows.Close()); err != nil {
		return err
	}

	// Check LMH25 soil horizon directional depth signs:
	// Upland L/F/H humus horizons measure upward from 0 cm (UpperDepth > LowerDepth);
	// Wetland Of/Om/Oh and Mineral A/B/C/R/W measure downward (UpperDepth < LowerDepth).
	humusTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Humus")
	hWhere := ` WHERE t."UpperDepth" IS NOT NULL AND t."LowerDepth" IS NOT NULL`
	var hArgs []any
	if useSU {
		if pre, args := buildSUPlotInPreFilter(`t."PlotNumber"`, humusTypes["plotnumber"], suPlotList); pre != "" {
			hWhere += ` AND ` + pre
			hArgs = append(hArgs, args...)
		}
	}
	hRows, err := tx.QueryContext(ctx, `SELECT t.rowid, typeof(t."PlotNumber"), CAST(t."PlotNumber" AS BLOB), COALESCE(CAST(t."PlotNumber" AS TEXT), ''), COALESCE(CAST(t."Horizon" AS TEXT), ''), t."UpperDepth", t."LowerDepth"
		FROM `+humusTbl+` t`+hWhere+` ORDER BY t.rowid`, hArgs...)
	if err != nil {
		return err
	}
	for hRows.Next() {
		var rid int64
		var plotType, plot, hz string
		var rawPlot []byte
		var upper, lower float64
		if err := hRows.Scan(&rid, &plotType, &rawPlot, &plot, &hz, &upper, &lower); err != nil {
			hRows.Close()
			return err
		}
		if useSU {
			if plotType != "text" {
				continue
			}
			if _, ok := suTextPlots[string(rawPlot)]; !ok {
				continue
			}
		}
		trimmedUpper := strings.ToUpper(strings.TrimSpace(hz))
		if strings.HasPrefix(trimmedUpper, "OF") || strings.HasPrefix(trimmedUpper, "OM") || strings.HasPrefix(trimmedUpper, "OH") {
			if upper >= lower {
				report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
					Table: "Humus", PlotNumber: plot, RowID: rid, Field: "UpperDepth",
					Value: fmt.Sprintf("%g..%g (%s)", upper, lower, strings.TrimSpace(hz)),
					Rule:  "LMH25 wetland Of/Om/Oh horizons measure downward (UpperDepth < LowerDepth)",
				})
			}
		} else if strings.HasPrefix(trimmedUpper, "L") || strings.HasPrefix(trimmedUpper, "F") || strings.HasPrefix(trimmedUpper, "H") {
			if upper <= lower {
				report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
					Table: "Humus", PlotNumber: plot, RowID: rid, Field: "UpperDepth",
					Value: fmt.Sprintf("%g..%g (%s)", upper, lower, strings.TrimSpace(hz)),
					Rule:  "LMH25 upland L/F/H horizons measure upward from 0 cm (UpperDepth > LowerDepth)",
				})
			}
		}
	}
	if err := errors.Join(hRows.Err(), hRows.Close()); err != nil {
		return err
	}

	minTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Mineral")
	mWhere := ` WHERE t."UpperDepth" IS NOT NULL AND t."LowerDepth" IS NOT NULL AND t."UpperDepth" >= t."LowerDepth"`
	var mArgs []any
	if useSU {
		if pre, args := buildSUPlotInPreFilter(`t."PlotNumber"`, minTypes["plotnumber"], suPlotList); pre != "" {
			mWhere += ` AND ` + pre
			mArgs = append(mArgs, args...)
		}
	}
	mRows, err := tx.QueryContext(ctx, `SELECT t.rowid, typeof(t."PlotNumber"), CAST(t."PlotNumber" AS BLOB), COALESCE(CAST(t."PlotNumber" AS TEXT), ''), COALESCE(CAST(t."Horizon" AS TEXT), ''), t."UpperDepth", t."LowerDepth"
		FROM `+minTbl+` t`+mWhere+` ORDER BY t.rowid`, mArgs...)
	if err != nil {
		return err
	}
	for mRows.Next() {
		var rid int64
		var plotType, plot, hz string
		var rawPlot []byte
		var upper, lower float64
		if err := mRows.Scan(&rid, &plotType, &rawPlot, &plot, &hz, &upper, &lower); err != nil {
			mRows.Close()
			return err
		}
		if useSU {
			if plotType != "text" {
				continue
			}
			if _, ok := suTextPlots[string(rawPlot)]; !ok {
				continue
			}
		}
		if upper >= lower {
			report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
				Table: "Mineral", PlotNumber: plot, RowID: rid, Field: "UpperDepth",
				Value: fmt.Sprintf("%g..%g (%s)", upper, lower, strings.TrimSpace(hz)),
				Rule:  "LMH25 mineral horizons measure downward (UpperDepth < LowerDepth)",
			})
		}
	}
	return errors.Join(mRows.Err(), mRows.Close())
}

func checkDuplicateVegSpecies(ctx context.Context, tx *sql.Tx, project string, report *ComplianceValidationReport) error {
	vegTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Veg")

	// Pre-filter duplicate (PlotNumber, Species) plots in C-level SQLite (streaming index group-by on PlotNumber)
	// so clean 1.65M-row datasets never materialize per-row layer bitmasks in Go.
	dupPlotRows, err := tx.QueryContext(ctx, `SELECT DISTINCT COALESCE(CAST("PlotNumber" AS TEXT), '')
		FROM `+vegTbl+`
		WHERE "PlotNumber" IS NOT NULL AND "Species" IS NOT NULL
		GROUP BY "PlotNumber", "Species"
		HAVING COUNT(*) > 1`)
	if err != nil {
		return err
	}
	var candidatePlots []string
	for dupPlotRows.Next() {
		var p string
		if err := dupPlotRows.Scan(&p); err != nil {
			dupPlotRows.Close()
			return err
		}
		candidatePlots = append(candidatePlots, p)
	}
	if err := errors.Join(dupPlotRows.Err(), dupPlotRows.Close()); err != nil {
		return err
	}
	if len(candidatePlots) == 0 {
		return nil
	}

	layerCols := []struct {
		code string
		col  string
	}{
		{"1", "Cover1"}, {"2", "Cover2"}, {"3", "Cover3"}, {"A", "TotalA"},
		{"4", "Cover4"}, {"5", "Cover5"}, {"5a", "Cover5a"}, {"5b", "Cover5b"}, {"5c", "Cover5c"}, {"B", "TotalB"},
		{"6", "Cover6"}, {"7", "Cover7"}, {"8", "Cover8"}, {"9", "Cover9"}, {"10", "Cover10"},
	}

	var colExprs []string
	for _, lc := range layerCols {
		colExprs = append(colExprs, `CASE WHEN `+vprocore.QuoteHeaderIdentifier(lc.col)+` IS NOT NULL THEN 1 ELSE 0 END`)
	}

	_, vegTypes, err := readTableColumnTypes(ctx, tx, "project", project+"_Veg")
	if err != nil {
		return err
	}
	whereExtra := ""
	var qArgs []any
	if pre, args := buildSUPlotInPreFilter(`"PlotNumber"`, vegTypes["plotnumber"], candidatePlots); pre != "" {
		whereExtra = ` AND ` + pre
		qArgs = append(qArgs, args...)
	}

	q := `SELECT COALESCE(CAST("PlotNumber" AS TEXT), ''), COALESCE(CAST("Species" AS TEXT), ''), ` + strings.Join(colExprs, ", ") + `
		FROM ` + vegTbl + `
		WHERE "PlotNumber" IS NOT NULL AND "Species" IS NOT NULL` + whereExtra

	rows, err := tx.QueryContext(ctx, q, qArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type grpKey struct{ plot, spp string }
	type grpStat struct {
		count       int
		layerCounts []int
	}
	stats := map[grpKey]*grpStat{}
	var dupKeys []grpKey

	var plot, spp string
	flags := make([]int, len(layerCols))
	dest := make([]any, 2+len(layerCols))
	dest[0], dest[1] = &plot, &spp
	for i := range flags {
		dest[2+i] = &flags[i]
	}

	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		k := grpKey{plot, spp}
		st, exists := stats[k]
		if !exists {
			st = &grpStat{layerCounts: make([]int, len(layerCols))}
			stats[k] = st
		} else if st.count == 1 {
			dupKeys = append(dupKeys, k)
		}
		st.count++
		for i, f := range flags {
			st.layerCounts[i] += f
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	slices.SortFunc(dupKeys, func(a, b grpKey) int {
		if cmp := strings.Compare(a.plot, b.plot); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.spp, b.spp)
	})
	for _, k := range dupKeys {
		st := stats[k]
		var overlaps []string
		for i, lc := range layerCols {
			if st.layerCounts[i] > 1 {
				overlaps = append(overlaps, lc.code)
			}
		}
		report.DuplicateVegFindings = append(report.DuplicateVegFindings, DuplicateVegSpeciesFinding{
			PlotNumber:        k.plot,
			Species:           k.spp,
			RowCount:          st.count,
			OverlappingLayers: overlaps,
		})
	}
	return nil
}

func checkTaxonomyCompliance(
	ctx context.Context,
	tx *sql.Tx,
	project string,
	useSU bool,
	suTextPlots map[string]struct{},
	suPlotList []string,
	report *ComplianceValidationReport,
) error {
	if useSU && len(suTextPlots) == 0 {
		return nil
	}
	vegTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Veg")
	_, vegTypes, err := readTableColumnTypes(ctx, tx, "project", project+"_Veg")
	if err != nil {
		return err
	}

	// Build fast lookup maps from USysAllSpecs and USysUserSpp.
	acceptedMaster := map[string]bool{}
	synonymToAccepted := map[string]string{}

	specRows, err := tx.QueryContext(ctx, `SELECT COALESCE(CAST("Code" AS TEXT), ''), COALESCE(CAST("OldCode" AS TEXT), ''), COALESCE(LOWER(CAST("Codetype" AS TEXT)), '') FROM "VLists"."USysAllSpecs"`)
	if err != nil {
		return err
	}
	for specRows.Next() {
		var code, oldCode, codeType string
		if err := specRows.Scan(&code, &oldCode, &codeType); err != nil {
			specRows.Close()
			return err
		}
		if code != "" && codeType != "s" {
			acceptedMaster[code] = true
		}
		if oldCode != "" && code != "" && oldCode != code {
			if _, exists := synonymToAccepted[oldCode]; !exists {
				synonymToAccepted[oldCode] = code
			}
		}
	}
	if err := errors.Join(specRows.Err(), specRows.Close()); err != nil {
		return err
	}

	userSpecies := map[string]bool{}
	uRows, err := tx.QueryContext(ctx, `SELECT COALESCE(CAST("Code" AS TEXT), '') FROM "VUser"."USysUserSpp"`)
	if err != nil {
		return err
	}
	for uRows.Next() {
		var code string
		if err := uRows.Scan(&code); err != nil {
			uRows.Close()
			return err
		}
		if code != "" {
			userSpecies[code] = true
		}
	}
	if err := errors.Join(uRows.Err(), uRows.Close()); err != nil {
		return err
	}

	whereSQL := ""
	var qArgs []any
	if useSU {
		if pre, args := buildSUPlotInPreFilter(`v."PlotNumber"`, vegTypes["plotnumber"], suPlotList); pre != "" {
			whereSQL = ` WHERE ` + pre
			qArgs = append(qArgs, args...)
		}
	}

	vRows, err := tx.QueryContext(ctx, `SELECT v.rowid, COALESCE(v."ID", 0), typeof(v."PlotNumber"), CAST(v."PlotNumber" AS BLOB), COALESCE(CAST(v."PlotNumber" AS TEXT), ''), v."Species"
		FROM `+vegTbl+` v`+whereSQL, qArgs...)
	if err != nil {
		return err
	}
	defer vRows.Close()

	for vRows.Next() {
		var rid, id int64
		var plotType, plot string
		var rawPlot []byte
		var spp sql.NullString
		if err := vRows.Scan(&rid, &id, &plotType, &rawPlot, &plot, &spp); err != nil {
			return err
		}
		if useSU {
			if plotType != "text" {
				continue
			}
			if _, ok := suTextPlots[string(rawPlot)]; !ok {
				continue
			}
		}
		if !spp.Valid || strings.TrimSpace(spp.String) == "" {
			report.TaxonomyFindings = append(report.TaxonomyFindings, TaxonomyComplianceFinding{
				RowID: rid, ID: id, PlotNumber: plot, Species: "", Status: "missing",
			})
			report.Summary.UnlistedSpecies++
			continue
		}
		code := spp.String
		if acceptedMaster[code] {
			continue
		}
		if target, isSyn := synonymToAccepted[code]; isSyn {
			tCopy := target
			report.TaxonomyFindings = append(report.TaxonomyFindings, TaxonomyComplianceFinding{
				RowID: rid, ID: id, PlotNumber: plot, Species: code, Status: "synonym", SuggestedCode: &tCopy,
			})
			report.Summary.SynonymSpecies++
			continue
		}
		if userSpecies[code] {
			report.TaxonomyFindings = append(report.TaxonomyFindings, TaxonomyComplianceFinding{
				RowID: rid, ID: id, PlotNumber: plot, Species: code, Status: "user",
			})
			report.Summary.UserSpecies++
			continue
		}
		report.TaxonomyFindings = append(report.TaxonomyFindings, TaxonomyComplianceFinding{
			RowID: rid, ID: id, PlotNumber: plot, Species: code, Status: "unlisted",
		})
		report.Summary.UnlistedSpecies++
	}
	if err := vRows.Err(); err != nil {
		return err
	}
	slices.SortFunc(report.TaxonomyFindings, func(a, b TaxonomyComplianceFinding) int {
		if cmp := strings.Compare(a.PlotNumber, b.PlotNumber); cmp != 0 {
			return cmp
		}
		if a.RowID < b.RowID {
			return -1
		}
		if a.RowID > b.RowID {
			return 1
		}
		return 0
	})
	return nil
}

type QuarantineOrphanRequest struct {
	Tables []string `json:"tables,omitempty"` // Optional subset of ["Veg", "Humus", "Mineral", "Other"]; defaults to all 4
}

type QuarantinedTableSummary struct {
	Table           string   `json:"table"`
	QuarantineTable string   `json:"quarantineTable"`
	MovedRows       int      `json:"movedRows"`
	PlotNumbers     []string `json:"plotNumbers"`
}

type QuarantineOrphanResult struct {
	Project            string                    `json:"project"`
	BatchID            string                    `json:"batchId"`
	TotalQuarantined   int                       `json:"totalQuarantined"`
	ReservedDeletedIDs []int64                   `json:"reservedDeletedIds"`
	AuditRowID         int64                     `json:"auditRowId,omitempty"`
	Tables             []QuarantinedTableSummary `json:"tables"`
}

var allowedQuarantineTables = []string{"Veg", "Humus", "Mineral", "Other"}

// QuarantineOrphanRecords transactionally moves orphan child rows (whose PlotNumber is missing
// from <Project>_Env) into __VPRO_Quarantine_<Table>, preserving all original columns and values,
// reserving positive child IDs in __VPRO_ChildIdentity, and writing an atomic <Project>_Audit receipt.
func QuarantineOrphanRecords(s *vprocore.ContextService, ctx context.Context, contextID string, req QuarantineOrphanRequest) (QuarantineOrphanResult, error) {
	if s == nil {
		return QuarantineOrphanResult{}, errors.New("validation context service is unavailable")
	}
	targetSuffixes := req.Tables
	if len(targetSuffixes) == 0 {
		targetSuffixes = append([]string(nil), allowedQuarantineTables...)
	} else {
		for _, t := range targetSuffixes {
			if !slices.Contains(allowedQuarantineTables, t) {
				return QuarantineOrphanResult{}, fmt.Errorf("unsupported quarantine table suffix %q (allowed: Veg, Humus, Mineral, Other)", t)
			}
		}
	}

	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return QuarantineOrphanResult{}, err
	}
	batchID := hex.EncodeToString(entropy[:])

	var out QuarantineOrphanResult
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
			envTbl := vprocore.QuoteHeaderIdentifier(project + "_Env")
			auditTbl := vprocore.QuoteHeaderIdentifier(project + "_Audit")
			now := time.Now().Format("2006-01-02 15:04:05")

			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
				"ChildTable" TEXT NOT NULL,
				"ID" INTEGER NOT NULL,
				PRIMARY KEY ("ChildTable", "ID")
			)`); err != nil {
				return err
			}

			envTextPlots := map[string]struct{}{}
			eRows, eErr := tx.QueryContext(ctx, `SELECT CAST("PlotNumber" AS BLOB) FROM `+envTbl+` WHERE typeof("PlotNumber")='text'`)
			if eErr != nil {
				return eErr
			}
			for eRows.Next() {
				var raw []byte
				if err := eRows.Scan(&raw); err != nil {
					eRows.Close()
					return err
				}
				envTextPlots[string(raw)] = struct{}{}
			}
			if err := errors.Join(eRows.Err(), eRows.Close()); err != nil {
				return err
			}

			var summaries []QuarantinedTableSummary
			var reservedIDs []int64
			totalMoved := 0

			for _, suffix := range targetSuffixes {
				if err := ctx.Err(); err != nil {
					return err
				}
				srcTableName := project + "_" + suffix
				quotedSrc := vprocore.QuoteHeaderIdentifier(srcTableName)
				quarantineTableName := "__VPRO_Quarantine_" + suffix
				quotedQuarantine := vprocore.QuoteHeaderIdentifier(quarantineTableName)

				// Inspect source columns & types so __VPRO_Quarantine_<Suffix> preserves full schema.
				pRows, pErr := tx.QueryContext(ctx, "PRAGMA table_info("+quotedSrc+")")
				if pErr != nil {
					return pErr
				}
				var colNames []string
				var colDefs []string
				hasID := false
				for pRows.Next() {
					var cid, notNull, pk int
					var cName, cType string
					var defVal sql.NullString
					if err := pRows.Scan(&cid, &cName, &cType, &notNull, &defVal, &pk); err != nil {
						pRows.Close()
						return err
					}
					if cName == "ID" {
						hasID = true
					}
					colNames = append(colNames, cName)
					decl := cType
					if strings.TrimSpace(decl) == "" {
						decl = "BLOB"
					}
					colDefs = append(colDefs, vprocore.QuoteHeaderIdentifier(cName)+" "+decl)
				}
				if err := errors.Join(pRows.Err(), pRows.Close()); err != nil {
					return err
				}

				// Single O(N) pass collecting orphan rowids, distinct orphan plot numbers, & positive IDs.
				var plotSet []string
				seenPlots := map[string]bool{}
				idExpr := "0"
				if hasID {
					idExpr = `CASE WHEN typeof("ID")='integer' THEN "ID" ELSE 0 END`
				}
				scanRows, sErr := tx.QueryContext(ctx,
					`SELECT rowid, typeof("PlotNumber"), CAST("PlotNumber" AS BLOB), COALESCE(CAST("PlotNumber" AS TEXT), '<NULL>'), `+idExpr+
						` FROM `+quotedSrc+` ORDER BY rowid`)
				if sErr != nil {
					return sErr
				}
				var orphanRowIDs []int64
				var tableIDs []int64
				for scanRows.Next() {
					var rowID, id int64
					var plotType, p string
					var rawPlot []byte
					if err := scanRows.Scan(&rowID, &plotType, &rawPlot, &p, &id); err != nil {
						scanRows.Close()
						return err
					}
					if plotType == "text" {
						if _, inEnv := envTextPlots[string(rawPlot)]; inEnv {
							continue
						}
					}
					orphanRowIDs = append(orphanRowIDs, rowID)
					if !seenPlots[p] {
						seenPlots[p] = true
						plotSet = append(plotSet, p)
					}
					if id > 0 {
						tableIDs = append(tableIDs, id)
					}
				}
				if err := errors.Join(scanRows.Err(), scanRows.Close()); err != nil {
					return err
				}

				rowCount := len(orphanRowIDs)
				if rowCount == 0 {
					summaries = append(summaries, QuarantinedTableSummary{
						Table:           suffix,
						QuarantineTable: quarantineTableName,
						MovedRows:       0,
						PlotNumbers:     []string{},
					})
					continue
				}

				metaDefs := []string{
					`"__QuarantineBatchID" TEXT NOT NULL`,
					`"__QuarantinedAt" TEXT NOT NULL`,
					`"__QuarantinedBy" TEXT NOT NULL`,
					`"__SourceRowID" INTEGER NOT NULL`,
				}
				createQuarantineSQL := `CREATE TABLE IF NOT EXISTS ` + quotedQuarantine + ` (` +
					strings.Join(append(metaDefs, colDefs...), ", ") + `)`
				if _, err := tx.ExecContext(ctx, createQuarantineSQL); err != nil {
					return err
				}

				quotedCols := make([]string, len(colNames))
				for i, c := range colNames {
					quotedCols[i] = vprocore.QuoteHeaderIdentifier(c)
				}
				joinedCols := strings.Join(quotedCols, ", ")

				const batchSize = 500
				movedTotal := 0
				deletedTotal := 0
				for start := 0; start < len(orphanRowIDs); start += batchSize {
					end := start + batchSize
					if end > len(orphanRowIDs) {
						end = len(orphanRowIDs)
					}
					chunk := orphanRowIDs[start:end]
					ph := make([]string, len(chunk))
					insArgs := make([]any, 0, 3+len(chunk))
					insArgs = append(insArgs, batchID, now, user)
					delArgs := make([]any, len(chunk))
					for i, rid := range chunk {
						ph[i] = "?"
						insArgs = append(insArgs, rid)
						delArgs[i] = rid
					}
					inList := strings.Join(ph, ",")
					insertQuarantineSQL := `INSERT INTO ` + quotedQuarantine +
						` ("__QuarantineBatchID", "__QuarantinedAt", "__QuarantinedBy", "__SourceRowID", ` + joinedCols + `)
						SELECT ?, ?, ?, rowid, ` + joinedCols + `
						FROM ` + quotedSrc + ` WHERE rowid IN (` + inList + `) ORDER BY rowid`
					insRes, insErr := tx.ExecContext(ctx, insertQuarantineSQL, insArgs...)
					if insErr != nil {
						return insErr
					}
					moved, _ := insRes.RowsAffected()
					movedTotal += int(moved)

					delRes, delErr := tx.ExecContext(ctx, `DELETE FROM `+quotedSrc+` WHERE rowid IN (`+inList+`)`, delArgs...)
					if delErr != nil {
						return delErr
					}
					deleted, _ := delRes.RowsAffected()
					deletedTotal += int(deleted)
				}
				if movedTotal != rowCount {
					return errors.New("quarantine insert row count mismatch; transaction aborted")
				}
				if deletedTotal != rowCount {
					return errors.New("quarantine delete row count mismatch; transaction aborted")
				}

				for _, id := range tableIDs {
					if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ChildIdentity" ("ChildTable", "ID") VALUES (?, ?) ON CONFLICT DO NOTHING`,
						quotedSrc, id); err != nil {
						return err
					}
					reservedIDs = append(reservedIDs, id)
				}

				totalMoved += rowCount
				slices.Sort(plotSet)
				summaries = append(summaries, QuarantinedTableSummary{
					Table:           suffix,
					QuarantineTable: quarantineTableName,
					MovedRows:       rowCount,
					PlotNumbers:     plotSet,
				})
			}

			var auditID int64
			if totalMoved > 0 {
				receiptBytes, _ := json.Marshal(map[string]any{
					"batchId":          batchID,
					"totalQuarantined": totalMoved,
					"tables":           summaries,
				})
				// Note: PlotNumber is kept NULL because <Project>_Audit has a foreign key to <Project>_Env(PlotNumber).
				aRes, aErr := tx.ExecContext(ctx, `INSERT INTO `+auditTbl+` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,NULL,?,?,?,?,?,0,0,NULL)`,
					project, user, "__VPRO_Quarantine", "QuarantineOrphanRecords", now, nil, string(receiptBytes))
				if aErr != nil {
					return aErr
				}
				auditID, _ = aRes.LastInsertId()
			}

			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			slices.Sort(reservedIDs)
			reservedIDs = slices.Compact(reservedIDs)
			out = QuarantineOrphanResult{
				Project:            project,
				BatchID:            batchID,
				TotalQuarantined:   totalMoved,
				ReservedDeletedIDs: reservedIDs,
				AuditRowID:         auditID,
				Tables:             summaries,
			}
			return nil
		})
	})
	if err != nil {
		return QuarantineOrphanResult{}, err
	}
	return out, nil
}
