package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type plotProfilePresenceSelector struct {
	rowID, table, species string
}

type plotProfilePresencePlan struct {
	seed      bool
	selectors []plotProfilePresenceSelector
}

type plotProfilePresenceResult struct {
	totalPlots  int
	plotNumbers []string
}

func compilePlotProfilePresence(table ProjectMetadataTable) (plotProfilePresencePlan, error) {
	indices := map[string]int{}
	for index, column := range table.Columns {
		if _, duplicate := indices[column.Name]; duplicate {
			return plotProfilePresencePlan{}, errors.New("species-only profile schema is ambiguous")
		}
		indices[column.Name] = index
	}
	for _, name := range []string{"Table", "Species"} {
		if _, present := indices[name]; !present {
			return plotProfilePresencePlan{}, fmt.Errorf("species-only profile schema is missing %s", name)
		}
	}
	plan := plotProfilePresencePlan{}
	seen := map[string]bool{}
	for _, row := range table.Rows {
		identity, err := strconv.ParseInt(row.RowID, 10, 64)
		if err != nil || strconv.FormatInt(identity, 10) != row.RowID || seen[row.RowID] || len(row.Cells) != len(table.Columns) {
			return plotProfilePresencePlan{}, errors.New("species-only profile requires distinct physical rows and complete cells")
		}
		seen[row.RowID] = true
		cell := row.Cells[indices["Table"]]
		if cell.Storage == "null" {
			continue
		}
		if cell.Storage != "text" || cell.Text == nil || !profileASCII(*cell.Text) || len(*cell.Text) > 255 {
			return plotProfilePresencePlan{}, fmt.Errorf("species-only profile row %s requires bounded ASCII Table text; no value was converted", row.RowID)
		}
		name := *cell.Text
		if strings.EqualFold(strings.TrimRight(name, " "), "Env") {
			continue
		}
		plan.seed = true
		normalized := strings.TrimRight(name, " ")
		if name != "Veg" && name != "Lump" {
			if strings.EqualFold(normalized, "Veg") || strings.EqualFold(normalized, "Lump") {
				return plotProfilePresencePlan{}, fmt.Errorf("species-only profile row %s has an unqualified Table spelling", row.RowID)
			}
			continue
		}
		species := row.Cells[indices["Species"]]
		if species.Storage != "text" || species.Text == nil || !profileASCII(*species.Text) ||
			len(*species.Text) > 255 || strings.Contains(*species.Text, "'") {
			return plotProfilePresencePlan{}, fmt.Errorf("species-only profile row %s requires bounded literal ASCII Species; NULL, raw SQL and historical coercion remain unavailable", row.RowID)
		}
		plan.selectors = append(plan.selectors, plotProfilePresenceSelector{row.RowID, name, *species.Text})
	}
	return plan, nil
}

func executePlotProfilePresence(ctx context.Context, tx *sql.Tx, plan plotProfilePresencePlan, lumpRelation string) (plotProfilePresenceResult, error) {
	if err := ctx.Err(); err != nil {
		return plotProfilePresenceResult{}, err
	}
	if tx == nil {
		return plotProfilePresenceResult{}, errors.New("species-only profile requires a caller-owned read snapshot")
	}
	scope, err := queryProfilePlots(ctx, tx, `SELECT DISTINCT PlotNumber FROM USysEnv ORDER BY PlotNumber COLLATE BINARY`, nil)
	if err != nil {
		return plotProfilePresenceResult{}, err
	}
	identities := map[string]bool{}
	for _, plot := range scope {
		key := strings.ToUpper(strings.TrimRight(plot, " "))
		if !profileASCII(plot) || len(plot) > 7 || identities[key] {
			return plotProfilePresenceResult{}, errors.New("species-only profile scope has unqualified or case-ambiguous plot identities; no identity was repaired")
		}
		identities[key] = true
	}
	result := plotProfilePresenceResult{totalPlots: len(scope), plotNumbers: []string{}}
	if !plan.seed {
		if err := ctx.Err(); err != nil {
			return plotProfilePresenceResult{}, err
		}
		return result, nil
	}
	remaining := map[string]bool{}
	for _, plot := range scope {
		remaining[plot] = true
	}
	if len(plan.selectors) > 0 {
		var ambiguous int
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv e JOIN USysVeg v
			ON rtrim(vpro_profile_text(e.PlotNumber),' ') COLLATE NOCASE=rtrim(vpro_profile_text(v.PlotNumber),' ') COLLATE NOCASE
			WHERE CAST(e.PlotNumber AS BLOB)<>CAST(v.PlotNumber AS BLOB))`).Scan(&ambiguous)
		if err != nil {
			return plotProfilePresenceResult{}, err
		}
		if ambiguous != 0 {
			return plotProfilePresenceResult{}, errors.New("species-only profile child identity differs from its literal parent; source collation adoption remains unavailable")
		}
	}
	// USysTVeg exposes these nine columns, not Cover5a/5b/5c/8/9/10 or computed sums.
	presence := `(v.Cover1 IS NOT NULL OR v.Cover2 IS NOT NULL OR v.Cover3 IS NOT NULL OR
		v.Cover4 IS NOT NULL OR v.Cover5 IS NOT NULL OR v.Cover6 IS NOT NULL OR
		v.Cover7 IS NOT NULL OR v.TotalA IS NOT NULL OR v.TotalB IS NOT NULL)`
	for _, selector := range plan.selectors {
		if err := ctx.Err(); err != nil {
			return plotProfilePresenceResult{}, err
		}
		from := ` FROM USysEnv e JOIN USysVeg v ON CAST(e.PlotNumber AS BLOB)=CAST(v.PlotNumber AS BLOB)`
		// Source text equality ignores terminal ASCII spaces; the reviewed values stay unchanged.
		where := `rtrim(vpro_profile_text(v.Species),' ') COLLATE NOCASE=?`
		if selector.table == "Lump" {
			if lumpRelation == "" {
				return plotProfilePresenceResult{}, errors.New("species-only Lump profile requires an explicitly reviewed project-local relation; stale global state is not inherited")
			}
			from += ` JOIN ` + lumpRelation + ` l ON rtrim(vpro_profile_text(v.Species),' ') COLLATE NOCASE=rtrim(vpro_profile_text(l.SppCode),' ') COLLATE NOCASE`
			where = `rtrim(vpro_profile_text(l.LumpCode),' ') COLLATE NOCASE=? AND vpro_profile_bool(l."Use")=-1`
		}
		matches, err := queryProfilePlots(ctx, tx, `SELECT DISTINCT e.PlotNumber`+from+
			` WHERE `+presence+` AND `+where, []any{strings.TrimRight(selector.species, " ")})
		if err != nil {
			return plotProfilePresenceResult{}, fmt.Errorf("species-only profile row %s failed; no stored data changed: %w", selector.rowID, err)
		}
		matched := map[string]bool{}
		for _, plot := range matches {
			matched[plot] = true
		}
		for plot := range remaining {
			if !matched[plot] {
				delete(remaining, plot)
			}
		}
	}
	for _, plot := range scope {
		if remaining[plot] {
			result.plotNumbers = append(result.plotNumbers, plot)
		}
	}
	if err := ctx.Err(); err != nil {
		return plotProfilePresenceResult{}, err
	}
	return result, nil
}
