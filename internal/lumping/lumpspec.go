package lumping

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const PresetBECProvincialCorrelation2026 = "BEC_Provincial_Correlation_2026"

type LumpPresetInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Spec        LumpSpec `json:"spec"`
}

type LumpSpec struct {
	Preset                          string         `json:"preset,omitempty"`
	CollapseInfraspecific           bool           `json:"collapseInfraspecific"`
	CollapseSeedlingAndDeadSuffixes bool           `json:"collapseSeedlingAndDeadSuffixes"`
	GenusRollups                    []string       `json:"genusRollups,omitempty"`
	NonVascularLifeforms            []int          `json:"nonVascularLifeforms,omitempty"`
	BaseLumpTable                   string         `json:"baseLumpTable,omitempty"`
	BaseLumpDBPath                  string         `json:"baseLumpDbPath,omitempty"`
	CustomRules                     []LumpRowInput `json:"customRules,omitempty"`
	ScopeToProjectSpecies           bool           `json:"scopeToProjectSpecies"`
}

type LumpSpecPreview struct {
	EffectiveSpec LumpSpec          `json:"effectiveSpec"`
	Rows          []LumpRow         `json:"rows"`
	Impact        LumpImpactPreview `json:"impact"`
}

type LumpMaterializeResult struct {
	Table        LumpTableSummary  `json:"table"`
	RowsWritten  int               `json:"rowsWritten"`
	Impact       LumpImpactPreview `json:"impact"`
}

func ListLumpPresets() []LumpPresetInfo {
	return []LumpPresetInfo{
		{
			ID:          PresetBECProvincialCorrelation2026,
			Title:       "BEC Provincial Correlation 2026",
			Description: "Folds 8-character infraspecific subspecies/varieties (1..8), seedling/dead codes (_D/_X), and difficult Carex, Salix, Sphagnum, and Cladonia complexes to genus codes.",
			Spec: LumpSpec{
				Preset:                          PresetBECProvincialCorrelation2026,
				CollapseInfraspecific:           true,
				CollapseSeedlingAndDeadSuffixes: true,
				GenusRollups:                    []string{"CARE*", "SALI*", "SPHA*", "CLAD*"},
				ScopeToProjectSpecies:           true,
			},
		},
	}
}

func resolveEffectiveSpec(spec LumpSpec) (LumpSpec, error) {
	effective := spec
	switch strings.TrimSpace(spec.Preset) {
	case "":
	case PresetBECProvincialCorrelation2026:
		effective.CollapseInfraspecific = true
		effective.CollapseSeedlingAndDeadSuffixes = true
		merged := append([]string{"CARE*", "SALI*", "SPHA*", "CLAD*"}, spec.GenusRollups...)
		seen := map[string]bool{}
		dedup := make([]string, 0, len(merged))
		for _, g := range merged {
			u := strings.ToUpper(strings.TrimSpace(g))
			if u != "" && !seen[u] {
				seen[u] = true
				dedup = append(dedup, u)
			}
		}
		effective.GenusRollups = dedup
	default:
		return LumpSpec{}, fmt.Errorf("unknown LumpSpec preset %q", spec.Preset)
	}
	return effective, nil
}

type genusRollupRule struct {
	prefix     string
	targetCode string
}

func parseGenusRollups(tokens []string, lifeforms []int, genusByName map[string]string, catalog map[string]speciesTaxonomyEntry) ([]genusRollupRule, map[int]bool, error) {
	lfSet := map[int]bool{}
	for _, lf := range lifeforms {
		lfSet[lf] = true
	}
	var rules []genusRollupRule
	for _, raw := range tokens {
		tok := strings.ToUpper(strings.TrimSpace(raw))
		if tok == "" {
			continue
		}
		if strings.HasPrefix(tok, "LIFEFORM:") {
			numStr := strings.TrimSpace(strings.TrimPrefix(tok, "LIFEFORM:"))
			lf, err := strconv.Atoi(numStr)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid lifeform rollup token %q", raw)
			}
			lfSet[lf] = true
			continue
		}
		if tok == "NONVASCULAR" || tok == "NON-VASCULAR" {
			lfSet[9] = true
			lfSet[10] = true
			lfSet[11] = true
			continue
		}
		if tok == "9" || tok == "10" || tok == "11" {
			lf, _ := strconv.Atoi(tok)
			lfSet[lf] = true
			continue
		}

		var prefixPart, explicitTarget string
		if idx := strings.Index(tok, "->"); idx >= 0 {
			prefixPart = strings.TrimSpace(tok[:idx])
			explicitTarget = strings.TrimSpace(tok[idx+2:])
		} else {
			prefixPart = tok
		}
		prefixPart = strings.TrimSuffix(prefixPart, "*")
		if prefixPart == "" {
			return nil, nil, fmt.Errorf("invalid genus rollup pattern %q", raw)
		}
		target := explicitTarget
		if target == "" {
			target = inferGenusTargetCode(prefixPart, genusByName, catalog)
		}
		rules = append(rules, genusRollupRule{prefix: prefixPart, targetCode: target})
	}
	return rules, lfSet, nil
}

func inferGenusTargetCode(prefix string, genusByName map[string]string, catalog map[string]speciesTaxonomyEntry) string {
	wellKnown := map[string]string{
		"CARE": "CAREX",
		"SALI": "SALIX",
		"SPHA": "SPHAGNUM",
		"CLAD": "CLADONIA",
	}
	if known, ok := wellKnown[prefix]; ok {
		if _, exists := catalog[known]; exists {
			return known
		}
	}
	if entry, ok := catalog[prefix]; ok && (entry.TaxonLevel == "gen" || entry.TaxonLevel == "genus") {
		return entry.Code
	}
	// Match via ScientificName genus of species starting with prefix
	var candidateGenus string
	for code, entry := range catalog {
		if strings.HasPrefix(code, prefix) && entry.ScientificName != "" {
			g := strings.ToUpper(strings.Fields(entry.ScientificName)[0])
			if gCode, ok := genusByName[g]; ok {
				if candidateGenus == "" || len(gCode) < len(candidateGenus) {
					candidateGenus = gCode
				}
			}
		}
	}
	if candidateGenus != "" {
		return candidateGenus
	}
	return prefix
}

func foldSingleCode(code string, entry speciesTaxonomyEntry, hasEntry bool, spec LumpSpec, genusRules []genusRollupRule, lfSet map[int]bool, genusByName map[string]string, catalog map[string]speciesTaxonomyEntry) (string, string) {
	curr := code
	var reasons []string

	// Step 1: Collapse seedling (_D / ...D Codetype='D') and dead (_X / ...X Codetype='D') suffixes
	if spec.CollapseSeedlingAndDeadSuffixes {
		if strings.HasSuffix(curr, "_D") && len(curr) > 2 {
			curr = curr[:len(curr)-2]
			reasons = append(reasons, "seedling_suffix(_D)")
		} else if strings.HasSuffix(curr, "_X") && len(curr) > 2 {
			curr = curr[:len(curr)-2]
			reasons = append(reasons, "dead_suffix(_X)")
		} else if len(curr) == 8 && (curr[7] == 'D' || curr[7] == 'X') && hasEntry && entry.Codetype == "D" {
			parent := curr[:7]
			if _, ok := catalog[parent]; ok {
				if curr[7] == 'D' {
					reasons = append(reasons, "seedling_code(D)")
				} else {
					reasons = append(reasons, "dead_code(X)")
				}
				curr = parent
			}
		}
	}

	// Refresh entry if curr changed
	if curr != code {
		if e, ok := catalog[curr]; ok {
			entry = e
			hasEntry = true
		}
	}

	// Step 2: Collapse 8-char infraspecific subspecies/varieties ending in '1'..'8' to 7-char parent
	if spec.CollapseInfraspecific && len(curr) == 8 && curr[7] >= '1' && curr[7] <= '8' {
		parent := curr[:7]
		curr = parent
		reasons = append(reasons, "infraspecific_1_8")
		if e, ok := catalog[curr]; ok {
			entry = e
			hasEntry = true
		}
	}

	// Step 3: Non-vascular lifeform rollup (e.g., Lifeform IN (9, 10, 11))
	if len(lfSet) > 0 && hasEntry && entry.Lifeform != nil && lfSet[*entry.Lifeform] {
		if entry.TaxonLevel != "gen" && entry.TaxonLevel != "genus" && entry.TaxonLevel != "fam" {
			target := ""
			if entry.ScientificName != "" {
				genusWord := strings.ToUpper(strings.Fields(entry.ScientificName)[0])
				target = genusByName[genusWord]
			}
			if target == "" && len(curr) >= 4 {
				target = curr[:4]
			}
			if target != "" && target != curr {
				curr = target
				reasons = append(reasons, fmt.Sprintf("lifeform_%d_genus", *entry.Lifeform))
			}
		}
	}

	// Step 4: Wildcard genus prefix rollups (e.g., CARE*, SALI*, SPHA*, CLAD*)
	for _, gr := range genusRules {
		if strings.HasPrefix(curr, gr.prefix) && curr != gr.targetCode {
			// Verify scientific name compatibility if available so unrelated genera sharing 4 letters aren't accidentally merged when both have distinct genera in VLists
			if hasEntry && entry.ScientificName != "" {
				if targetEntry, ok := catalog[gr.targetCode]; ok && targetEntry.ScientificName != "" {
					srcGenus := strings.ToUpper(strings.Fields(entry.ScientificName)[0])
					dstGenus := strings.ToUpper(strings.Fields(targetEntry.ScientificName)[0])
					if srcGenus != dstGenus {
						continue
					}
				}
			}
			curr = gr.targetCode
			reasons = append(reasons, "genus_rollup("+gr.prefix+"*)")
			break
		}
	}

	if curr == code {
		return "", ""
	}
	return curr, strings.Join(reasons, "+")
}

func compileLumpSpecTx(ctx context.Context, tx *sql.Tx, project, projectPath string, spec LumpSpec) (LumpSpec, []LumpRow, map[string]string, error) {
	effective, err := resolveEffectiveSpec(spec)
	if err != nil {
		return LumpSpec{}, nil, nil, err
	}
	catalog, genusByName, err := loadTaxonomyCatalog(ctx, tx)
	if err != nil {
		return LumpSpec{}, nil, nil, err
	}
	genusRules, lfSet, err := parseGenusRollups(effective.GenusRollups, effective.NonVascularLifeforms, genusByName, catalog)
	if err != nil {
		return LumpSpec{}, nil, nil, err
	}

	// Read project species set
	projectSpp := map[string]bool{}
	vegTable := "project." + vprocore.QuoteHeaderIdentifier(project+"_Veg")
	pRows, err := tx.QueryContext(ctx, `SELECT DISTINCT Species FROM `+vegTable+` WHERE Species IS NOT NULL AND TRIM(Species) <> ''`)
	if err != nil {
		return LumpSpec{}, nil, nil, err
	}
	for pRows.Next() {
		var s string
		if err := pRows.Scan(&s); err != nil {
			pRows.Close()
			return LumpSpec{}, nil, nil, err
		}
		projectSpp[strings.ToUpper(strings.TrimSpace(s))] = true
	}
	if err := pRows.Close(); err != nil {
		return LumpSpec{}, nil, nil, err
	}

	candidates := make([]string, 0, len(projectSpp)+len(catalog))
	seenCand := map[string]bool{}
	for s := range projectSpp {
		seenCand[s] = true
		candidates = append(candidates, s)
	}
	if !effective.ScopeToProjectSpecies {
		for code, entry := range catalog {
			if entry.SourceCatalog == "Synonym" {
				continue
			}
			if !seenCand[code] {
				seenCand[code] = true
				candidates = append(candidates, code)
			}
		}
	}
	slices.Sort(candidates)

	rowBySpp := map[string]LumpRow{}
	for _, code := range candidates {
		entry, hasEntry := catalog[code]
		target, reason := foldSingleCode(code, entry, hasEntry, effective, genusRules, lfSet, genusByName, catalog)
		if target == "" || strings.EqualFold(target, code) {
			continue
		}
		lumpCode, sppCode, vErr := ValidateLumpCodes(target, code)
		if vErr != nil {
			continue
		}
		rowBySpp[sppCode] = enrichLumpRow(LumpRow{
			LumpCode:   lumpCode,
			SppCode:    sppCode,
			Use:        true,
			RuleOrigin: reason,
		}, catalog)
	}

	// Base lump table overrides (matching V7mdlLumping.CombineSpp priority)
	if baseTbl := strings.TrimSpace(effective.BaseLumpTable); baseTbl != "" && !strings.EqualFold(baseTbl, "None") && !strings.EqualFold(baseTbl, "None_Lump") {
		baseDB := strings.TrimSpace(effective.BaseLumpDBPath)
		if baseDB == "" {
			baseDB = projectPath
		}
		baseRows, actual, _, bErr := readRawLumpRowsFromDB(ctx, baseDB, baseTbl)
		if bErr != nil {
			return LumpSpec{}, nil, nil, bErr
		}
		for _, br := range baseRows {
			if !br.Use {
				continue
			}
			lumpCode, sppCode, vErr := ValidateLumpCodes(br.LumpCode, br.SppCode)
			if vErr != nil {
				continue
			}
			if effective.ScopeToProjectSpecies && !projectSpp[sppCode] {
				continue
			}
			rowBySpp[sppCode] = enrichLumpRow(LumpRow{
				LumpCode:   lumpCode,
				SppCode:    sppCode,
				Use:        true,
				RuleOrigin: "base_table(" + actual + ")",
			}, catalog)
		}
	}

	// Custom explicit overrides
	for _, cr := range effective.CustomRules {
		lumpCode, sppCode, vErr := ValidateLumpCodes(cr.LumpCode, cr.SppCode)
		if vErr != nil {
			return LumpSpec{}, nil, nil, vErr
		}
		if _, ok := catalog[sppCode]; !ok && !cr.AllowUnlisted {
			return LumpSpec{}, nil, nil, fmt.Errorf("custom LumpSpec species code %q was not found in VLists.USysAllSpecs or VUser.USysUserSpp", sppCode)
		}
		rowBySpp[sppCode] = enrichLumpRow(LumpRow{
			LumpCode:   lumpCode,
			SppCode:    sppCode,
			Use:        cr.Use,
			RuleOrigin: "custom_rule",
		}, catalog)
	}

	compiled := make([]LumpRow, 0, len(rowBySpp))
	activeMap := make(map[string]string, len(rowBySpp))
	for _, r := range rowBySpp {
		compiled = append(compiled, r)
		if r.Use {
			activeMap[r.SppCode] = r.LumpCode
		}
	}
	slices.SortFunc(compiled, func(a, b LumpRow) int {
		if c := strings.Compare(a.LumpCode, b.LumpCode); c != 0 {
			return c
		}
		return strings.Compare(a.SppCode, b.SppCode)
	})
	return effective, compiled, activeMap, nil
}

func PreviewLumpSpec(s *vprocore.ContextService, ctx context.Context, contextID string, spec LumpSpec) (LumpSpecPreview, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (LumpSpecPreview, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return LumpSpecPreview{}, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return LumpSpecPreview{}, err
		}
		defer func() { _ = tx.Rollback() }()

		effective, rows, activeMap, err := compileLumpSpecTx(ctx, tx, owner.Selection.Project, owner.Selection.ProjectPath, spec)
		if err != nil {
			return LumpSpecPreview{}, err
		}
		impact, err := computeFoldingImpactFromTx(ctx, tx, owner.Selection.Project, activeMap, "LumpSpec_Preview", owner.Selection.ProjectPath)
		if err != nil {
			return LumpSpecPreview{}, err
		}
		if err := tx.Commit(); err != nil {
			return LumpSpecPreview{}, err
		}
		return LumpSpecPreview{
			EffectiveSpec: effective,
			Rows:          rows,
			Impact:        impact,
		}, nil
	})
}

func MaterializeLumpSpec(s *vprocore.ContextService, ctx context.Context, contextID, targetName string, spec LumpSpec, replaceExisting bool) (LumpMaterializeResult, error) {
	base, tableName, err := normalizeLumpBaseName(targetName)
	if err != nil {
		return LumpMaterializeResult{}, err
	}
	preview, err := PreviewLumpSpec(s, ctx, contextID, spec)
	if err != nil {
		return LumpMaterializeResult{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (LumpMaterializeResult, error) {
		owner := plots.Projects.Sqlite
		activeCount := 0
		lumpSet := map[string]struct{}{}
		if err := withImmediateProjectWriteTx(ctx, owner, func(conn *sql.Conn) error {
			var existingActual string
			qErr := conn.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name COLLATE NOCASE=?`, tableName).Scan(&existingActual)
			if qErr == nil {
				if !replaceExisting {
					return fmt.Errorf("lump table %s already exists; set replaceExisting=true to overwrite", existingActual)
				}
				if _, err := conn.ExecContext(ctx, `DROP TABLE `+vprocore.QuoteHeaderIdentifier(existingActual)); err != nil {
					return err
				}
			} else if !errors.Is(qErr, sql.ErrNoRows) {
				return qErr
			}

			created, err := createLumpTableInTx(ctx, conn, base)
			if err != nil {
				return err
			}
			tableName = created
			quoted := vprocore.QuoteHeaderIdentifier(created)
			stmt, err := conn.PrepareContext(ctx, `INSERT INTO `+quoted+` ("LumpCode", "SppCode", "Use") VALUES (?, ?, ?)`)
			if err != nil {
				return err
			}
			defer stmt.Close()
			for _, r := range preview.Rows {
				if _, err := stmt.ExecContext(ctx, r.LumpCode, r.SppCode, accessBoolToInt(r.Use)); err != nil {
					return err
				}
				lumpSet[r.LumpCode] = struct{}{}
				if r.Use {
					activeCount++
				}
			}
			return nil
		}); err != nil {
			return LumpMaterializeResult{}, err
		}
		impact := preview.Impact
		impact.TableName = tableName
		impact.DatabasePath = owner.Selection.ProjectPath
		return LumpMaterializeResult{
			Table: LumpTableSummary{
				Name:              base,
				TableName:         tableName,
				DatabasePath:      owner.Selection.ProjectPath,
				IsActiveProject:   true,
				MetadataVersion:   LumpTableMetadataVersion,
				RowCount:          len(preview.Rows),
				ActiveCount:       activeCount,
				DistinctLumpCodes: len(lumpSet),
			},
			RowsWritten: len(preview.Rows),
			Impact:      impact,
		}, nil
	})
}
