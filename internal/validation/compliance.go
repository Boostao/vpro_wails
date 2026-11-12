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
		vegTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Veg")

		// 1. 1:1 _Env <-> _Admin completeness (always project-wide on physical tables).
		missingAdminRows, err := tx.QueryContext(ctx, `SELECT CAST(e."PlotNumber" AS TEXT)
			FROM `+envTbl+` e
			WHERE NOT EXISTS (
				SELECT 1 FROM `+adminTbl+` a
				WHERE typeof(a."Plot")='text' AND typeof(e."PlotNumber")='text'
				  AND CAST(a."Plot" AS BLOB)=CAST(e."PlotNumber" AS BLOB)
			)
			ORDER BY e."PlotNumber"`)
		if err != nil {
			return report, err
		}
		for missingAdminRows.Next() {
			var plot string
			if err := missingAdminRows.Scan(&plot); err != nil {
				missingAdminRows.Close()
				return report, err
			}
			report.EnvAdminFindings = append(report.EnvAdminFindings, EnvAdminCompletenessFinding{
				PlotNumber: plot,
				Issue:      "missing_admin",
			})
		}
		if err := errors.Join(missingAdminRows.Err(), missingAdminRows.Close()); err != nil {
			return report, err
		}

		orphanAdminRows, err := tx.QueryContext(ctx, `SELECT COALESCE(CAST(a."Plot" AS TEXT), '<NULL>')
			FROM `+adminTbl+` a
			WHERE a."Plot" IS NULL OR typeof(a."Plot") <> 'text' OR NOT EXISTS (
				SELECT 1 FROM `+envTbl+` e
				WHERE typeof(e."PlotNumber")='text'
				  AND CAST(e."PlotNumber" AS BLOB)=CAST(a."Plot" AS BLOB)
			)
			ORDER BY a."Plot"`)
		if err != nil {
			return report, err
		}
		for orphanAdminRows.Next() {
			var plot string
			if err := orphanAdminRows.Scan(&plot); err != nil {
				orphanAdminRows.Close()
				return report, err
			}
			report.EnvAdminFindings = append(report.EnvAdminFindings, EnvAdminCompletenessFinding{
				PlotNumber: plot,
				Issue:      "orphan_admin",
			})
		}
		if err := errors.Join(orphanAdminRows.Err(), orphanAdminRows.Close()); err != nil {
			return report, err
		}
		report.Summary.EnvAdminMismatches = len(report.EnvAdminFindings)

		// 2. 1:N Foreign Key Integrity across _Veg, _Humus, _Mineral, _Other, _Audit, and active *_SU.
		for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other", "Audit"} {
			childTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_"+suffix)
			whereExtra := ""
			if suffix == "Audit" {
				// System-level audit entries (e.g. NewProject, USysAllSpecs) legitimately have NULL PlotNumber.
				whereExtra = `c."PlotNumber" IS NOT NULL AND CAST(c."PlotNumber" AS TEXT) <> '' AND `
			}
			oRows, oErr := tx.QueryContext(ctx, `SELECT COALESCE(CAST(c."PlotNumber" AS TEXT), '<NULL>'), COUNT(*)
				FROM `+childTbl+` c
				WHERE `+whereExtra+`(c."PlotNumber" IS NULL OR typeof(c."PlotNumber") <> 'text' OR NOT EXISTS (
					SELECT 1 FROM `+envTbl+` e
					WHERE typeof(e."PlotNumber")='text'
					  AND CAST(e."PlotNumber" AS BLOB)=CAST(c."PlotNumber" AS BLOB)
				))
				GROUP BY COALESCE(CAST(c."PlotNumber" AS TEXT), '<NULL>')
				ORDER BY 1`)
			if oErr != nil {
				return report, oErr
			}
			for oRows.Next() {
				var plot string
				var cnt int
				if err := oRows.Scan(&plot, &cnt); err != nil {
					oRows.Close()
					return report, err
				}
				report.OrphanFindings = append(report.OrphanFindings, OrphanChildFinding{
					Table:      suffix,
					PlotNumber: plot,
					RowCount:   cnt,
				})
				if suffix != "Audit" {
					report.Summary.OrphanChildRows += cnt
				}
			}
			if err := errors.Join(oRows.Err(), oRows.Close()); err != nil {
				return report, err
			}
		}

		if activeSU != "None" {
			suTbl := `"su".` + vprocore.QuoteHeaderIdentifier(activeSU+"_SU")
			suOrphans, suErr := tx.QueryContext(ctx, `SELECT COALESCE(CAST(s."PlotNumber" AS TEXT), '<NULL>'), COUNT(*)
				FROM `+suTbl+` s
				WHERE s."PlotNumber" IS NULL OR typeof(s."PlotNumber") <> 'text' OR NOT EXISTS (
					SELECT 1 FROM `+envTbl+` e
					WHERE typeof(e."PlotNumber")='text'
					  AND CAST(e."PlotNumber" AS BLOB)=CAST(s."PlotNumber" AS BLOB)
				)
				GROUP BY COALESCE(CAST(s."PlotNumber" AS TEXT), '<NULL>')
				ORDER BY 1`)
			if suErr != nil {
				return report, suErr
			}
			for suOrphans.Next() {
				var plot string
				var cnt int
				if err := suOrphans.Scan(&plot, &cnt); err != nil {
					suOrphans.Close()
					return report, err
				}
				report.OrphanFindings = append(report.OrphanFindings, OrphanChildFinding{
					Table:      "SU",
					PlotNumber: plot,
					RowCount:   cnt,
				})
			}
			if err := errors.Join(suOrphans.Err(), suOrphans.Close()); err != nil {
				return report, err
			}
		}

		// Plots without vegetation (respects UseActiveSU).
		suFilterSQL := ""
		if useSU {
			suTbl := `"su".` + vprocore.QuoteHeaderIdentifier(activeSU+"_SU")
			suFilterSQL = ` AND EXISTS (SELECT 1 FROM ` + suTbl + ` s WHERE typeof(s."PlotNumber")='text' AND CAST(s."PlotNumber" AS BLOB)=CAST(e."PlotNumber" AS BLOB))`
		}
		noVegRows, err := tx.QueryContext(ctx, `SELECT CAST(e."PlotNumber" AS TEXT)
			FROM `+envTbl+` e
			WHERE NOT EXISTS (
				SELECT 1 FROM `+vegTbl+` v
				WHERE typeof(v."PlotNumber")='text' AND typeof(e."PlotNumber")='text'
				  AND CAST(v."PlotNumber" AS BLOB)=CAST(e."PlotNumber" AS BLOB)
			)`+suFilterSQL+`
			ORDER BY e."PlotNumber"`)
		if err != nil {
			return report, err
		}
		for noVegRows.Next() {
			var p string
			if err := noVegRows.Scan(&p); err != nil {
				noVegRows.Close()
				return report, err
			}
			report.PlotsWithoutVegetation = append(report.PlotsWithoutVegetation, p)
		}
		if err := errors.Join(noVegRows.Err(), noVegRows.Close()); err != nil {
			return report, err
		}
		report.Summary.PlotsWithoutVeg = len(report.PlotsWithoutVegetation)

		// 3. VLists.USysTableOfLists (Validate <> 0) code compliance.
		envCols, err := readTableColumnMap(ctx, tx, "project", project+"_Env")
		if err != nil {
			return report, err
		}
		adminCols, err := readTableColumnMap(ctx, tx, "project", project+"_Admin")
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

		joinsSQL := `FROM ` + envTbl + ` env INNER JOIN ` + adminTbl + ` admin ON typeof(env."PlotNumber")='text' AND typeof(admin."Plot")='text' AND CAST(env."PlotNumber" AS BLOB)=CAST(admin."Plot" AS BLOB)`
		if useSU {
			suTbl := `"su".` + vprocore.QuoteHeaderIdentifier(activeSU+"_SU")
			joinsSQL += ` INNER JOIN ` + suTbl + ` su ON typeof(su."PlotNumber")='text' AND CAST(su."PlotNumber" AS BLOB)=CAST(env."PlotNumber" AS BLOB)`
		}

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
				expr := `env.` + vprocore.QuoteHeaderIdentifier(envActual)
				if inAdmin {
					expr = `admin.` + vprocore.QuoteHeaderIdentifier(adminActual)
				}
				q := `SELECT DISTINCT CAST(env."PlotNumber" AS TEXT), CAST(` + expr + ` AS TEXT)
					` + joinsSQL + `
					WHERE ` + expr + ` IS NOT NULL AND CAST(` + expr + ` AS TEXT) <> ''
					  AND NOT EXISTS (
						SELECT 1 FROM "VLists"."USysTableOfLists" lists
						WHERE lists."ListName" = ? COLLATE NOCASE
						  AND lists."Item" = CAST(` + expr + ` AS TEXT) COLLATE NOCASE
					  )
					ORDER BY 1, 2`
				fRows, fErr := tx.QueryContext(ctx, q, d.listName)
				if fErr != nil {
					return report, fErr
				}
				for fRows.Next() {
					var plot, val string
					if err := fRows.Scan(&plot, &val); err != nil {
						fRows.Close()
						return report, err
					}
					report.EnvCodeFindings = append(report.EnvCodeFindings, EnvironmentCodeFinding{
						ListName:   d.listName,
						Field:      field,
						PlotNumber: plot,
						Value:      val,
					})
				}
				if err := errors.Join(fRows.Err(), fRows.Close()); err != nil {
					return report, err
				}
			}
		}
		report.Summary.EnvCodeErrors = len(report.EnvCodeFindings)

		// 4. UTF-16, Numeric, IEEE-754 float32 <100 cover, and LMH25 soil horizon sign bounds.
		if err := checkProjectBounds(ctx, tx, project, useSU, activeSU, &report); err != nil {
			return report, err
		}
		report.Summary.BoundsErrors = len(report.BoundsFindings)

		// 5. Duplicate (PlotNumber, Species) strata splits & overlapping layer covers.
		if err := checkDuplicateVegSpecies(ctx, tx, project, &report); err != nil {
			return report, err
		}
		report.Summary.DuplicateSpeciesSplits = len(report.DuplicateVegFindings)

		// 6. USysAllSpecs & USysUserSpp species taxonomy compliance.
		if err := checkTaxonomyCompliance(ctx, tx, project, useSU, activeSU, &report); err != nil {
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

func readTableColumnMap(ctx context.Context, tx *sql.Tx, alias, table string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA "+vprocore.QuoteHeaderIdentifier(alias)+".table_info("+vprocore.QuoteHeaderIdentifier(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = name
	}
	return out, rows.Err()
}

func checkProjectBounds(ctx context.Context, tx *sql.Tx, project string, useSU bool, activeSU string, report *ComplianceValidationReport) error {
	// Text UTF-16 length limits on core tables.
	textRules := []struct {
		suffix  string
		plotCol string
		col     string
		maxUTF  int
	}{
		{"Env", "PlotNumber", "PlotNumber", 7},
		{"Env", "PlotNumber", "FieldNumber", 50},
		{"Env", "PlotNumber", "ProjectID", 30},
		{"Env", "PlotNumber", "Zone", 4},
		{"Env", "PlotNumber", "SubZone", 8},
		{"Env", "PlotNumber", "SiteSeries", 5},
		{"Env", "PlotNumber", "Ecosection", 3},
		{"Env", "PlotNumber", "NtsMapSheet", 8},
		{"Admin", "Plot", "Plot", 7},
		{"Admin", "Plot", "PlotType", 10},
		{"Veg", "PlotNumber", "PlotNumber", 7},
		{"Veg", "PlotNumber", "Species", 8},
		{"Veg", "PlotNumber", "Layer", 2},
		{"Veg", "PlotNumber", "Collected", 1},
		{"Humus", "PlotNumber", "PlotNumber", 7},
		{"Humus", "PlotNumber", "Horizon", 8},
		{"Mineral", "PlotNumber", "PlotNumber", 7},
		{"Mineral", "PlotNumber", "Horizon", 8},
		{"Mineral", "PlotNumber", "Texture", 4},
		{"Mineral", "PlotNumber", "Colour", 14},
	}

	suJoin := ""
	if useSU {
		suTbl := `"su".` + vprocore.QuoteHeaderIdentifier(activeSU+"_SU")
		suJoin = ` AND EXISTS (SELECT 1 FROM ` + suTbl + ` s WHERE typeof(s."PlotNumber")='text' AND CAST(s."PlotNumber" AS BLOB)=CAST(t.__plot_col__ AS BLOB))`
	}

	for _, rule := range textRules {
		tbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_"+rule.suffix)
		qCol := vprocore.QuoteHeaderIdentifier(rule.col)
		qPlot := vprocore.QuoteHeaderIdentifier(rule.plotCol)
		filter := strings.ReplaceAll(suJoin, "t.__plot_col__", "t."+qPlot)
		rows, err := tx.QueryContext(ctx, `SELECT t.rowid, COALESCE(CAST(t.`+qPlot+` AS TEXT), ''), CAST(t.`+qCol+` AS BLOB)
			FROM `+tbl+` t
			WHERE t.`+qCol+` IS NOT NULL`+filter+`
			ORDER BY t.rowid`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var rid int64
			var plot string
			var raw []byte
			if err := rows.Scan(&rid, &plot, &raw); err != nil {
				rows.Close()
				return err
			}
			if !utf8.Valid(raw) {
				report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
					Table:      rule.suffix,
					PlotNumber: plot,
					RowID:      rid,
					Field:      rule.col,
					Value:      fmt.Sprintf("0x%x", raw),
					Rule:       "Malformed UTF-8 byte sequence",
				})
				continue
			}
			s := string(raw)
			if utf16CodeUnitLen(s) > rule.maxUTF {
				report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
					Table:      rule.suffix,
					PlotNumber: plot,
					RowID:      rid,
					Field:      rule.col,
					Value:      s,
					Rule:       fmt.Sprintf("Exceeds %d UTF-16 code units", rule.maxUTF),
				})
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
	}

	// Check IEEE-754 SINGLE (<100) cover bounds in _Veg.
	vegTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Veg")
	vegCoverCols := []string{
		"Cover1", "Cover2", "Cover3", "TotalA",
		"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB",
		"Cover6", "Cover7", "Cover8", "Cover9", "Cover10",
	}
	vegFilter := strings.ReplaceAll(suJoin, "t.__plot_col__", `t."PlotNumber"`)
	for _, col := range vegCoverCols {
		qCol := vprocore.QuoteHeaderIdentifier(col)
		rows, err := tx.QueryContext(ctx, `SELECT t.rowid, COALESCE(CAST(t."PlotNumber" AS TEXT), ''), typeof(t.`+qCol+`), CAST(t.`+qCol+` AS REAL), CAST(t.`+qCol+` AS TEXT)
			FROM `+vegTbl+` t
			WHERE t.`+qCol+` IS NOT NULL`+vegFilter+`
			ORDER BY t.rowid`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var rid int64
			var plot, typ, rawTxt string
			var fVal sql.NullFloat64
			if err := rows.Scan(&rid, &plot, &typ, &fVal, &rawTxt); err != nil {
				rows.Close()
				return err
			}
			if typ != "real" && typ != "integer" || !fVal.Valid || !isValidSingleCover(fVal.Float64) {
				report.BoundsFindings = append(report.BoundsFindings, BoundsFinding{
					Table:      "Veg",
					PlotNumber: plot,
					RowID:      rid,
					Field:      col,
					Value:      rawTxt,
					Rule:       "IEEE-754 SINGLE cover must be >= 0 and < 100",
				})
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
	}

	// Check numeric bounds on _Env (Latitude, Longitude, Aspect, SlopeGradient) & _Admin (StartDate).
	envTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Env")
	envFilter := strings.ReplaceAll(suJoin, "t.__plot_col__", `t."PlotNumber"`)
	envNumRows, err := tx.QueryContext(ctx, `SELECT t.rowid, COALESCE(CAST(t."PlotNumber" AS TEXT), ''),
		t."Latitude", t."Longitude", t."Aspect", t."SlopeGradient"
		FROM `+envTbl+` t WHERE 1=1`+envFilter+` ORDER BY t.rowid`)
	if err != nil {
		return err
	}
	for envNumRows.Next() {
		var rid int64
		var plot string
		var lat, lon, slope sql.NullFloat64
		var aspect sql.NullInt64
		if err := envNumRows.Scan(&rid, &plot, &lat, &lon, &aspect, &slope); err != nil {
			envNumRows.Close()
			return err
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
	adminFilter := strings.ReplaceAll(suJoin, "t.__plot_col__", `t."Plot"`)
	adminNumRows, err := tx.QueryContext(ctx, `SELECT t.rowid, COALESCE(CAST(t."Plot" AS TEXT), ''), t."StartDate"
		FROM `+adminTbl+` t WHERE t."StartDate" IS NOT NULL`+adminFilter+` ORDER BY t.rowid`)
	if err != nil {
		return err
	}
	for adminNumRows.Next() {
		var rid int64
		var plot string
		var sd sql.NullInt64
		if err := adminNumRows.Scan(&rid, &plot, &sd); err != nil {
			adminNumRows.Close()
			return err
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
	hRows, err := tx.QueryContext(ctx, `SELECT t.rowid, COALESCE(CAST(t."PlotNumber" AS TEXT), ''), COALESCE(CAST(t."Horizon" AS TEXT), ''), t."UpperDepth", t."LowerDepth"
		FROM `+humusTbl+` t WHERE t."UpperDepth" IS NOT NULL AND t."LowerDepth" IS NOT NULL`+vegFilter+` ORDER BY t.rowid`)
	if err != nil {
		return err
	}
	for hRows.Next() {
		var rid int64
		var plot, hz string
		var upper, lower float64
		if err := hRows.Scan(&rid, &plot, &hz, &upper, &lower); err != nil {
			hRows.Close()
			return err
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
	mRows, err := tx.QueryContext(ctx, `SELECT t.rowid, COALESCE(CAST(t."PlotNumber" AS TEXT), ''), COALESCE(CAST(t."Horizon" AS TEXT), ''), t."UpperDepth", t."LowerDepth"
		FROM `+minTbl+` t WHERE t."UpperDepth" IS NOT NULL AND t."LowerDepth" IS NOT NULL`+vegFilter+` ORDER BY t.rowid`)
	if err != nil {
		return err
	}
	for mRows.Next() {
		var rid int64
		var plot, hz string
		var upper, lower float64
		if err := mRows.Scan(&rid, &plot, &hz, &upper, &lower); err != nil {
			mRows.Close()
			return err
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
	q := `SELECT COALESCE(CAST("PlotNumber" AS TEXT), ''), COALESCE(CAST("Species" AS TEXT), ''), ` + strings.Join(colExprs, ", ") + `
		FROM ` + vegTbl + `
		WHERE "PlotNumber" IS NOT NULL AND "Species" IS NOT NULL
		ORDER BY "PlotNumber", "Species", rowid`

	rows, err := tx.QueryContext(ctx, q)
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
	var order []grpKey

	for rows.Next() {
		var plot, spp string
		flags := make([]int, len(layerCols))
		dest := make([]any, 2+len(layerCols))
		dest[0], dest[1] = &plot, &spp
		for i := range flags {
			dest[2+i] = &flags[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		k := grpKey{plot, spp}
		st, exists := stats[k]
		if !exists {
			st = &grpStat{layerCounts: make([]int, len(layerCols))}
			stats[k] = st
			order = append(order, k)
		}
		st.count++
		for i, f := range flags {
			st.layerCounts[i] += f
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, k := range order {
		st := stats[k]
		if st.count <= 1 {
			continue
		}
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

func checkTaxonomyCompliance(ctx context.Context, tx *sql.Tx, project string, useSU bool, activeSU string, report *ComplianceValidationReport) error {
	vegTbl := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Veg")
	suFilter := ""
	if useSU {
		suTbl := `"su".` + vprocore.QuoteHeaderIdentifier(activeSU+"_SU")
		suFilter = ` WHERE EXISTS (SELECT 1 FROM ` + suTbl + ` s WHERE typeof(s."PlotNumber")='text' AND CAST(s."PlotNumber" AS BLOB)=CAST(v."PlotNumber" AS BLOB))`
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

	vRows, err := tx.QueryContext(ctx, `SELECT v.rowid, COALESCE(v."ID", 0), COALESCE(CAST(v."PlotNumber" AS TEXT), ''), v."Species"
		FROM `+vegTbl+` v`+suFilter+` ORDER BY v."PlotNumber", v.rowid`)
	if err != nil {
		return err
	}
	defer vRows.Close()

	for vRows.Next() {
		var rid, id int64
		var plot string
		var spp sql.NullString
		if err := vRows.Scan(&rid, &id, &plot, &spp); err != nil {
			return err
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
	return vRows.Err()
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

				orphanWhere := `("PlotNumber" IS NULL OR typeof("PlotNumber") <> 'text' OR NOT EXISTS (
					SELECT 1 FROM ` + envTbl + ` e
					WHERE typeof(e."PlotNumber")='text'
					  AND CAST(e."PlotNumber" AS BLOB)=CAST(` + quotedSrc + `."PlotNumber" AS BLOB)
				))`

				// Collect distinct orphan plot numbers & positive IDs before moving.
				var plotSet []string
				seenPlots := map[string]bool{}
				idExpr := "0"
				if hasID {
					idExpr = `CASE WHEN typeof("ID")='integer' THEN "ID" ELSE 0 END`
				}
				scanRows, sErr := tx.QueryContext(ctx, `SELECT COALESCE(CAST("PlotNumber" AS TEXT), '<NULL>'), `+idExpr+` FROM `+quotedSrc+` WHERE `+orphanWhere+` ORDER BY rowid`)
				if sErr != nil {
					return sErr
				}
				rowCount := 0
				var tableIDs []int64
				for scanRows.Next() {
					var p string
					var id int64
					if err := scanRows.Scan(&p, &id); err != nil {
						scanRows.Close()
						return err
					}
					rowCount++
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
				insertQuarantineSQL := `INSERT INTO ` + quotedQuarantine +
					` ("__QuarantineBatchID", "__QuarantinedAt", "__QuarantinedBy", "__SourceRowID", ` + strings.Join(quotedCols, ", ") + `)
					SELECT ?, ?, ?, rowid, ` + strings.Join(quotedCols, ", ") + `
					FROM ` + quotedSrc + ` WHERE ` + orphanWhere
				insRes, insErr := tx.ExecContext(ctx, insertQuarantineSQL, batchID, now, user)
				if insErr != nil {
					return insErr
				}
				moved, _ := insRes.RowsAffected()
				if int(moved) != rowCount {
					return errors.New("quarantine insert row count mismatch; transaction aborted")
				}

				delRes, delErr := tx.ExecContext(ctx, `DELETE FROM `+quotedSrc+` WHERE `+orphanWhere)
				if delErr != nil {
					return delErr
				}
				deleted, _ := delRes.RowsAffected()
				if int(deleted) != rowCount {
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
