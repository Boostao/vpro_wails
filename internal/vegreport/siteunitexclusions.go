package vegreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

// DefaultSiteUnitExclusionPatterns returns the BEC standard excluded SiteUnit suffixes
// per VPro2.0_Functions pp. 12-13 (P054, P060).
func DefaultSiteUnitExclusionPatterns() []string {
	return []string{".support", ".poor", ".nudum"}
}

type SiteUnitExclusionRule struct {
	Pattern   string `json:"pattern"`
	MatchMode string `json:"matchMode"` // "auto" (default) | "suffix" | "contains" | "wildcard"
	Enabled   bool   `json:"enabled"`
}

func (r *SiteUnitExclusionRule) UnmarshalJSON(data []byte) error {
	type plain SiteUnitExclusionRule
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type ExcludedSiteUnitInfo struct {
	SiteUnit       string `json:"siteUnit"`
	MatchedPattern string `json:"matchedPattern"`
	PlotCount      int    `json:"plotCount"`
}

type RetainedSiteUnitInfo struct {
	SiteUnit  string `json:"siteUnit"`
	PlotCount int    `json:"plotCount"`
}

type SiteUnitExclusionSummary struct {
	AppliedRules       []SiteUnitExclusionRule `json:"appliedRules"`
	TotalPlotsBefore   int                     `json:"totalPlotsBefore"`
	RetainedPlots      int                     `json:"retainedPlots"`
	ExcludedPlots      int                     `json:"excludedPlots"`
	RetainedUnits      []RetainedSiteUnitInfo  `json:"retainedUnits"`
	ExcludedUnits      []ExcludedSiteUnitInfo  `json:"excludedUnits"`
}

type HierarchyBreakpointOptions struct {
	BreakpointPolicy   string `json:"breakpointPolicy"`   // "lowest" (default) | "highest"
	UnbrokenUnitPolicy string `json:"unbrokenUnitPolicy"` // "keep_original" (default) | "exclude_unbroken"
	StripTildePrefix   bool   `json:"stripTildePrefix"`
}

func (r *HierarchyBreakpointOptions) UnmarshalJSON(data []byte) error {
	type plain HierarchyBreakpointOptions
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type HierarchyBreakpointUnitMapping struct {
	OriginalUnit   string `json:"originalUnit"`
	EffectiveUnit  string `json:"effectiveUnit"`
	RolledUp       bool   `json:"rolledUp"`
	MatchedInTree  bool   `json:"matchedInTree"`
	Excluded       bool   `json:"excluded"`
	PlotCount      int    `json:"plotCount"`
}

type HierarchyBreakpointBucket struct {
	BreakpointUnit string   `json:"breakpointUnit"`
	SourceUnits    []string `json:"sourceUnits"`
	PlotCount      int      `json:"plotCount"`
}

type HierarchyBreakpointResolution struct {
	ContextID         string                           `json:"contextId"`
	Project           string                           `json:"project"`
	SU                string                           `json:"su"`
	Hierarchy         string                           `json:"hierarchy"`
	BreakpointPolicy  string                           `json:"breakpointPolicy"`
	TotalUnitsBefore  int                              `json:"totalUnitsBefore"`
	TotalUnitsAfter   int                              `json:"totalUnitsAfter"`
	RolledUpPlotCount int                              `json:"rolledUpPlotCount"`
	UnmappedUnits     []string                         `json:"unmappedUnits"`
	Mappings          []HierarchyBreakpointUnitMapping `json:"mappings"`
	Buckets           []HierarchyBreakpointBucket      `json:"buckets"`
}

type HierarchyBreakpointRequest struct {
	Rules             []SiteUnitExclusionRule    `json:"rules"`
	UseDefaultPresets bool                       `json:"useDefaultPresets"`
	Options           HierarchyBreakpointOptions `json:"options"`
}

func (r *HierarchyBreakpointRequest) UnmarshalJSON(data []byte) error {
	type plain HierarchyBreakpointRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type SiteUnitExclusionPreviewRequest struct {
	Rules                   []SiteUnitExclusionRule    `json:"rules"`
	UseDefaultPresets       bool                       `json:"useDefaultPresets"`
	UseHierarchyBreakpoints bool                       `json:"useHierarchyBreakpoints"`
	BreakpointOptions       HierarchyBreakpointOptions `json:"breakpointOptions"`
}

func (r *SiteUnitExclusionPreviewRequest) UnmarshalJSON(data []byte) error {
	type plain SiteUnitExclusionPreviewRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type SiteUnitExclusionPreview struct {
	ContextID   string                         `json:"contextId"`
	Project     string                         `json:"project"`
	SU          string                         `json:"su"`
	Hierarchy   string                         `json:"hierarchy"`
	Exclusions  SiteUnitExclusionSummary       `json:"exclusions"`
	Breakpoints *HierarchyBreakpointResolution `json:"breakpoints,omitempty"`
}

type SummaryVegetationExclusionRequest struct {
	Average                 int                                `json:"average"`
	Display                 int                                `json:"display"`
	ShowClass               int                                `json:"showClass"`
	GroupAndOrder           int                                `json:"groupAndOrder"`
	Rules                   []SiteUnitExclusionRule            `json:"rules"`
	UseDefaultPresets       bool                               `json:"useDefaultPresets"`
	UseHierarchyBreakpoints bool                               `json:"useHierarchyBreakpoints"`
	BreakpointOptions       HierarchyBreakpointOptions         `json:"breakpointOptions"`
	References              *SummaryVegetationReferenceOptions `json:"references,omitempty"`
}

func (r *SummaryVegetationExclusionRequest) UnmarshalJSON(data []byte) error {
	type plain SummaryVegetationExclusionRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type SummaryVegetationExclusionPreview struct {
	Preview     SummaryVegetationPreview       `json:"preview"`
	Exclusions  SiteUnitExclusionSummary       `json:"exclusions"`
	Breakpoints *HierarchyBreakpointResolution `json:"breakpoints,omitempty"`
}

type LongVegetationExclusionRequest struct {
	Rules                   []SiteUnitExclusionRule    `json:"rules"`
	UseDefaultPresets       bool                       `json:"useDefaultPresets"`
	UseHierarchyBreakpoints bool                       `json:"useHierarchyBreakpoints"`
	BreakpointOptions       HierarchyBreakpointOptions `json:"breakpointOptions"`
}

func (r *LongVegetationExclusionRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationExclusionRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type LongVegetationExclusionPreview struct {
	Preview     LongVegetationPreview          `json:"preview"`
	Exclusions  SiteUnitExclusionSummary       `json:"exclusions"`
	Breakpoints *HierarchyBreakpointResolution `json:"breakpoints,omitempty"`
}

func NormalizeExclusionRules(rules []SiteUnitExclusionRule, useDefaults bool) ([]SiteUnitExclusionRule, error) {
	if len(rules) == 0 && useDefaults {
		defaults := DefaultSiteUnitExclusionPatterns()
		out := make([]SiteUnitExclusionRule, len(defaults))
		for i, p := range defaults {
			out[i] = SiteUnitExclusionRule{Pattern: p, MatchMode: "auto", Enabled: true}
		}
		return out, nil
	}
	out := make([]SiteUnitExclusionRule, 0, len(rules))
	for _, r := range rules {
		if !utf8.ValidString(r.Pattern) || strings.ContainsRune(r.Pattern, 0) {
			return nil, fmt.Errorf("SiteUnit exclusion pattern %q must be valid UTF-8 without NUL", r.Pattern)
		}
		p := strings.TrimSpace(r.Pattern)
		if p == "" {
			continue
		}
		mode := strings.ToLower(strings.TrimSpace(r.MatchMode))
		if mode == "" {
			mode = "auto"
		}
		if mode != "auto" && mode != "suffix" && mode != "contains" && mode != "wildcard" {
			return nil, fmt.Errorf("unsupported SiteUnit exclusion matchMode %q", r.MatchMode)
		}
		out = append(out, SiteUnitExclusionRule{
			Pattern:   p,
			MatchMode: mode,
			Enabled:   r.Enabled,
		})
	}
	return out, nil
}

// wildcardCaseInsensitiveMatch matches '*' (zero or more runes) and '?' (single rune) on lowercased runes.
func wildcardCaseInsensitiveMatch(pattern, target string) bool {
	p := []rune(strings.ToLower(pattern))
	s := []rune(strings.ToLower(target))
	pi, si := 0, 0
	starIdx, matchIdx := -1, 0
	for si < len(s) {
		if pi < len(p) && (p[pi] == '?' || p[pi] == s[si]) {
			pi++
			si++
		} else if pi < len(p) && p[pi] == '*' {
			starIdx = pi
			matchIdx = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
		} else {
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

func MatchSiteUnitExclusion(siteUnit string, rules []SiteUnitExclusionRule) (bool, string) {
	trimmedUnit := strings.TrimSpace(siteUnit)
	if trimmedUnit == "" {
		return false, ""
	}
	lowerUnit := strings.ToLower(trimmedUnit)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		pat := strings.TrimSpace(rule.Pattern)
		if pat == "" {
			continue
		}
		lowerPat := strings.ToLower(pat)
		mode := rule.MatchMode
		if mode == "" || mode == "auto" {
			if strings.ContainsAny(pat, "*?") {
				mode = "wildcard"
			} else if strings.HasPrefix(pat, ".") {
				// Dot-prefixed BEC unit qualifier (e.g. .support, .poor, .nudum):
				// match suffix OR dot-delimited segment (e.g. CWHvm1/01.support-a, CWHvm1/01.poor.1).
				if strings.HasSuffix(lowerUnit, lowerPat) ||
					strings.Contains(lowerUnit, lowerPat+".") ||
					strings.Contains(lowerUnit, lowerPat+"-") ||
					strings.Contains(lowerUnit, lowerPat+"_") ||
					strings.Contains(lowerUnit, lowerPat+" ") {
					return true, rule.Pattern
				}
				continue
			} else {
				mode = "contains"
			}
		}

		switch mode {
		case "suffix":
			if strings.HasSuffix(lowerUnit, lowerPat) {
				return true, rule.Pattern
			}
		case "contains":
			if strings.Contains(lowerUnit, lowerPat) {
				return true, rule.Pattern
			}
		case "wildcard":
			if wildcardCaseInsensitiveMatch(lowerPat, lowerUnit) {
				return true, rule.Pattern
			}
		}
	}
	return false, ""
}

func FilterSUTableByExclusions(
	su vprocore.ProjectMetadataTable,
	rules []SiteUnitExclusionRule,
) (vprocore.ProjectMetadataTable, SiteUnitExclusionSummary, error) {
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, SiteUnitExclusionSummary{}, err
	}
	filtered := vprocore.ProjectMetadataTable{
		Columns: append([]vprocore.ProjectMetadataColumn{}, su.Columns...),
		Rows:    make([]vprocore.ProjectMetadataRow, 0, len(su.Rows)),
	}
	summary := SiteUnitExclusionSummary{
		AppliedRules:  append([]SiteUnitExclusionRule{}, rules...),
		RetainedUnits: []RetainedSiteUnitInfo{},
		ExcludedUnits: []ExcludedSiteUnitInfo{},
	}

	type excludedBucket struct {
		unit    string
		pattern string
		count   int
	}
	excludedByUnit := map[string]*excludedBucket{}
	retainedByUnit := map[string]int{}

	for _, row := range su.Rows {
		plotCell := row.Cells[sc["PlotNumber"]]
		unitCell := row.Cells[sc["SiteUnit"]]
		hasPlot := plotCell.Storage == "text" && plotCell.Text != nil && strings.TrimSpace(*plotCell.Text) != ""
		unitStr := ""
		if unitCell.Storage == "text" && unitCell.Text != nil {
			unitStr = *unitCell.Text
		}
		if hasPlot && strings.TrimSpace(unitStr) != "" {
			summary.TotalPlotsBefore++
		}

		if strings.TrimSpace(unitStr) != "" {
			if excluded, matchedPat := MatchSiteUnitExclusion(unitStr, rules); excluded {
				if hasPlot {
					summary.ExcludedPlots++
				}
				b := excludedByUnit[unitStr]
				if b == nil {
					b = &excludedBucket{unit: unitStr, pattern: matchedPat}
					excludedByUnit[unitStr] = b
				}
				if hasPlot {
					b.count++
				}
				continue
			}
		}

		if hasPlot && strings.TrimSpace(unitStr) != "" {
			summary.RetainedPlots++
			retainedByUnit[unitStr]++
		}
		clonedCells := make([]vprocore.ProjectMetadataCell, len(row.Cells))
		for i, c := range row.Cells {
			clonedCells[i] = vprocore.CloneSiteUnitCell(c)
		}
		filtered.Rows = append(filtered.Rows, vprocore.ProjectMetadataRow{
			RowID: row.RowID,
			Cells: clonedCells,
		})
	}

	retKeys := make([]string, 0, len(retainedByUnit))
	for k := range retainedByUnit {
		retKeys = append(retKeys, k)
	}
	slices.Sort(retKeys)
	for _, k := range retKeys {
		summary.RetainedUnits = append(summary.RetainedUnits, RetainedSiteUnitInfo{
			SiteUnit:  k,
			PlotCount: retainedByUnit[k],
		})
	}

	excKeys := make([]string, 0, len(excludedByUnit))
	for k := range excludedByUnit {
		excKeys = append(excKeys, k)
	}
	slices.Sort(excKeys)
	for _, k := range excKeys {
		b := excludedByUnit[k]
		summary.ExcludedUnits = append(summary.ExcludedUnits, ExcludedSiteUnitInfo{
			SiteUnit:       b.unit,
			MatchedPattern: b.pattern,
			PlotCount:      b.count,
		})
	}

	return filtered, summary, nil
}

type hierarchyBreakNode struct {
	id     int64
	name   string
	parent *int64
	level  int64
}

func cleanBreakNodeName(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if strings.HasPrefix(trimmed, "~") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "~")), true
	}
	return trimmed, false
}

func loadHierarchyNodesFromTx(ctx context.Context, tx *sql.Tx, hierName string) ([]hierarchyBreakNode, error) {
	tableName := hierName + "_Hierarchy"
	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM "hierarchy".sqlite_master WHERE type='table' AND name COLLATE BINARY=?`,
		tableName,
	).Scan(&count); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, fmt.Errorf("active hierarchy table %s does not exist in attached hierarchy database", tableName)
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT "ID", COALESCE(CAST("Name" AS TEXT), ''), "Parent", COALESCE("Level", 11)
		 FROM "hierarchy".`+vprocore.QuoteHeaderIdentifier(tableName)+` ORDER BY "ID" ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []hierarchyBreakNode
	for rows.Next() {
		var n hierarchyBreakNode
		var p sql.NullInt64
		if err := rows.Scan(&n.id, &n.name, &p, &n.level); err != nil {
			return nil, err
		}
		if p.Valid {
			pv := p.Int64
			n.parent = &pv
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Verify hierarchy is acyclic before resolving breakpoints
	byID := make(map[int64]hierarchyBreakNode, len(nodes))
	for _, n := range nodes {
		byID[n.id] = n
	}
	for _, n := range nodes {
		visited := map[int64]bool{n.id: true}
		curr := n.parent
		for curr != nil {
			if visited[*curr] {
				return nil, fmt.Errorf("active hierarchy %s contains a parent cycle involving node %d", hierName, *curr)
			}
			next, ok := byID[*curr]
			if !ok {
				break
			}
			visited[*curr] = true
			curr = next.parent
		}
	}
	return nodes, nil
}

func resolveHierarchyBreakpointsOnSU(
	su vprocore.ProjectMetadataTable,
	nodes []hierarchyBreakNode,
	options HierarchyBreakpointOptions,
) (vprocore.ProjectMetadataTable, HierarchyBreakpointResolution, error) {
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, HierarchyBreakpointResolution{}, err
	}

	policy := strings.ToLower(strings.TrimSpace(options.BreakpointPolicy))
	if policy == "" {
		policy = "lowest"
	}
	if policy != "lowest" && policy != "highest" {
		return vprocore.ProjectMetadataTable{}, HierarchyBreakpointResolution{}, fmt.Errorf("unsupported breakpointPolicy %q; expected 'lowest' or 'highest'", options.BreakpointPolicy)
	}
	unbrokenPolicy := strings.ToLower(strings.TrimSpace(options.UnbrokenUnitPolicy))
	if unbrokenPolicy == "" {
		unbrokenPolicy = "keep_original"
	}
	if unbrokenPolicy != "keep_original" && unbrokenPolicy != "exclude_unbroken" {
		return vprocore.ProjectMetadataTable{}, HierarchyBreakpointResolution{}, fmt.Errorf("unsupported unbrokenUnitPolicy %q; expected 'keep_original' or 'exclude_unbroken'", options.UnbrokenUnitPolicy)
	}

	byID := make(map[int64]hierarchyBreakNode, len(nodes))
	byCleanLower := make(map[string]hierarchyBreakNode, len(nodes))
	for _, n := range nodes {
		byID[n.id] = n
		clean, _ := cleanBreakNodeName(n.name)
		if clean != "" {
			byCleanLower[strings.ToLower(clean)] = n
		}
	}

	// Compute unit plot counts & distinct original units
	unitPlotCounts := map[string]int{}
	distinctOriginal := []string{}
	seenOrig := map[string]bool{}
	for _, row := range su.Rows {
		pCell := row.Cells[sc["PlotNumber"]]
		uCell := row.Cells[sc["SiteUnit"]]
		if uCell.Storage != "text" || uCell.Text == nil || strings.TrimSpace(*uCell.Text) == "" {
			continue
		}
		u := *uCell.Text
		if !seenOrig[u] {
			seenOrig[u] = true
			distinctOriginal = append(distinctOriginal, u)
		}
		if pCell.Storage == "text" && pCell.Text != nil && strings.TrimSpace(*pCell.Text) != "" {
			unitPlotCounts[u]++
		}
	}
	slices.Sort(distinctOriginal)

	mappingByUnit := map[string]HierarchyBreakpointUnitMapping{}
	mappings := make([]HierarchyBreakpointUnitMapping, 0, len(distinctOriginal))
	unmapped := []string{}
	bucketSources := map[string]map[string]bool{}
	bucketPlots := map[string]int{}
	afterUnitSet := map[string]bool{}
	rolledUpPlots := 0

	for _, orig := range distinctOriginal {
		cleanOrig, _ := cleanBreakNodeName(orig)
		startNode, matched := byCleanLower[strings.ToLower(cleanOrig)]

		var breaksBottomUp []hierarchyBreakNode
		if matched {
			curr := &startNode
			visited := map[int64]bool{}
			for curr != nil && !visited[curr.id] {
				visited[curr.id] = true
				if _, isBrk := cleanBreakNodeName(curr.name); isBrk {
					breaksBottomUp = append(breaksBottomUp, *curr)
				}
				if curr.parent == nil {
					break
				}
				parentNode, ok := byID[*curr.parent]
				if !ok {
					break
				}
				curr = &parentNode
			}
		} else {
			unmapped = append(unmapped, orig)
		}

		pCount := unitPlotCounts[orig]
		if len(breaksBottomUp) > 0 {
			chosen := breaksBottomUp[0] // lowest '~' ancestor (nearest to leaf)
			if policy == "highest" {
				chosen = breaksBottomUp[len(breaksBottomUp)-1] // highest '~' ancestor (nearest to root)
			}
			effective := strings.TrimSpace(chosen.name)
			if options.StripTildePrefix {
				effective, _ = cleanBreakNodeName(effective)
			}
			if len(utf16.Encode([]rune(effective))) > 255 {
				return vprocore.ProjectMetadataTable{}, HierarchyBreakpointResolution{}, fmt.Errorf("breakpoint unit %q exceeds 255 UTF-16 code units", effective)
			}
			m := HierarchyBreakpointUnitMapping{
				OriginalUnit:  orig,
				EffectiveUnit: effective,
				RolledUp:      effective != orig,
				MatchedInTree: true,
				Excluded:      false,
				PlotCount:     pCount,
			}
			mappingByUnit[orig] = m
			mappings = append(mappings, m)
			afterUnitSet[effective] = true
			if m.RolledUp {
				rolledUpPlots += pCount
			}
			if bucketSources[effective] == nil {
				bucketSources[effective] = map[string]bool{}
			}
			bucketSources[effective][orig] = true
			bucketPlots[effective] += pCount
		} else {
			if unbrokenPolicy == "exclude_unbroken" {
				m := HierarchyBreakpointUnitMapping{
					OriginalUnit:  orig,
					EffectiveUnit: "",
					RolledUp:      false,
					MatchedInTree: matched,
					Excluded:      true,
					PlotCount:     pCount,
				}
				mappingByUnit[orig] = m
				mappings = append(mappings, m)
			} else {
				m := HierarchyBreakpointUnitMapping{
					OriginalUnit:  orig,
					EffectiveUnit: orig,
					RolledUp:      false,
					MatchedInTree: matched,
					Excluded:      false,
					PlotCount:     pCount,
				}
				mappingByUnit[orig] = m
				mappings = append(mappings, m)
				afterUnitSet[orig] = true
			}
		}
	}

	rolledSU := vprocore.ProjectMetadataTable{
		Columns: append([]vprocore.ProjectMetadataColumn{}, su.Columns...),
		Rows:    make([]vprocore.ProjectMetadataRow, 0, len(su.Rows)),
	}
	sIdx := sc["SiteUnit"]
	for _, row := range su.Rows {
		uCell := row.Cells[sIdx]
		if uCell.Storage == "text" && uCell.Text != nil && strings.TrimSpace(*uCell.Text) != "" {
			m, ok := mappingByUnit[*uCell.Text]
			if ok && m.Excluded {
				continue
			}
			cloned := make([]vprocore.ProjectMetadataCell, len(row.Cells))
			for i, c := range row.Cells {
				cloned[i] = vprocore.CloneSiteUnitCell(c)
			}
			if ok && m.EffectiveUnit != "" {
				eff := m.EffectiveUnit
				cloned[sIdx] = vprocore.ProjectMetadataCell{Storage: "text", Text: &eff}
			}
			rolledSU.Rows = append(rolledSU.Rows, vprocore.ProjectMetadataRow{
				RowID: row.RowID,
				Cells: cloned,
			})
		} else {
			if unbrokenPolicy == "exclude_unbroken" {
				continue
			}
			cloned := make([]vprocore.ProjectMetadataCell, len(row.Cells))
			for i, c := range row.Cells {
				cloned[i] = vprocore.CloneSiteUnitCell(c)
			}
			rolledSU.Rows = append(rolledSU.Rows, vprocore.ProjectMetadataRow{
				RowID: row.RowID,
				Cells: cloned,
			})
		}
	}

	bucketKeys := make([]string, 0, len(bucketSources))
	for k := range bucketSources {
		bucketKeys = append(bucketKeys, k)
	}
	slices.Sort(bucketKeys)
	buckets := make([]HierarchyBreakpointBucket, 0, len(bucketKeys))
	for _, bk := range bucketKeys {
		srcs := make([]string, 0, len(bucketSources[bk]))
		for s := range bucketSources[bk] {
			srcs = append(srcs, s)
		}
		slices.Sort(srcs)
		buckets = append(buckets, HierarchyBreakpointBucket{
			BreakpointUnit: bk,
			SourceUnits:    srcs,
			PlotCount:      bucketPlots[bk],
		})
	}

	res := HierarchyBreakpointResolution{
		BreakpointPolicy:  policy,
		TotalUnitsBefore:  len(distinctOriginal),
		TotalUnitsAfter:   len(afterUnitSet),
		RolledUpPlotCount: rolledUpPlots,
		UnmappedUnits:     unmapped,
		Mappings:          mappings,
		Buckets:           buckets,
	}
	return rolledSU, res, nil
}

func ResolveHierarchyBreakpointUnits(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	request HierarchyBreakpointRequest,
) (*HierarchyBreakpointResolution, error) {
	if ctx == nil {
		return nil, errors.New("hierarchy breakpoint resolution requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("vegetation report context service is unavailable")
	}
	rules, err := NormalizeExclusionRules(request.Rules, request.UseDefaultPresets)
	if err != nil {
		return nil, err
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ *HierarchyBreakpointResolution, resultErr error) {
		owner := plots.Projects.Sqlite
		if owner.Selection.SU == "" || owner.Selection.SU == "None" {
			return nil, errors.New("an active Site Unit (SU) table must be selected")
		}
		if owner.Selection.Hierarchy == "" || owner.Selection.Hierarchy == "None" {
			return nil, errors.New("an active Hierarchy table must be selected to resolve '~' breakpoints")
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

		suTable, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "su", owner.Selection.SU+"_SU", "", nil, "")
		if err != nil {
			return nil, err
		}
		filteredSU, _, err := FilterSUTableByExclusions(suTable, rules)
		if err != nil {
			return nil, err
		}
		nodes, err := loadHierarchyNodesFromTx(ctx, tx, owner.Selection.Hierarchy)
		if err != nil {
			return nil, err
		}
		_, res, err := resolveHierarchyBreakpointsOnSU(filteredSU, nodes, request.Options)
		if err != nil {
			return nil, err
		}
		res.ContextID = contextID
		res.Project = owner.Selection.Project
		res.SU = owner.Selection.SU
		res.Hierarchy = owner.Selection.Hierarchy

		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &res, nil
	})
}

func PreviewSiteUnitExclusions(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	request SiteUnitExclusionPreviewRequest,
) (*SiteUnitExclusionPreview, error) {
	if ctx == nil {
		return nil, errors.New("SiteUnit exclusion preview requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("vegetation report context service is unavailable")
	}
	rules, err := NormalizeExclusionRules(request.Rules, request.UseDefaultPresets)
	if err != nil {
		return nil, err
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ *SiteUnitExclusionPreview, resultErr error) {
		owner := plots.Projects.Sqlite
		if owner.Selection.SU == "" || owner.Selection.SU == "None" {
			return nil, errors.New("an active Site Unit (SU) table must be selected")
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

		suTable, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "su", owner.Selection.SU+"_SU", "", nil, "")
		if err != nil {
			return nil, err
		}
		filteredSU, summary, err := FilterSUTableByExclusions(suTable, rules)
		if err != nil {
			return nil, err
		}

		out := &SiteUnitExclusionPreview{
			ContextID:  contextID,
			Project:    owner.Selection.Project,
			SU:         owner.Selection.SU,
			Hierarchy:  owner.Selection.Hierarchy,
			Exclusions: summary,
		}

		if request.UseHierarchyBreakpoints {
			if owner.Selection.Hierarchy == "" || owner.Selection.Hierarchy == "None" {
				return nil, errors.New("an active Hierarchy table must be selected when UseHierarchyBreakpoints is enabled")
			}
			nodes, hErr := loadHierarchyNodesFromTx(ctx, tx, owner.Selection.Hierarchy)
			if hErr != nil {
				return nil, hErr
			}
			_, bpRes, bErr := resolveHierarchyBreakpointsOnSU(filteredSU, nodes, request.BreakpointOptions)
			if bErr != nil {
				return nil, bErr
			}
			bpRes.ContextID = contextID
			bpRes.Project = owner.Selection.Project
			bpRes.SU = owner.Selection.SU
			bpRes.Hierarchy = owner.Selection.Hierarchy
			out.Breakpoints = &bpRes
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

func PreviewSummaryVegetationWithExclusions(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	request SummaryVegetationExclusionRequest,
) (*SummaryVegetationExclusionPreview, error) {
	if ctx == nil {
		return nil, errors.New("Summary Vegetation requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil || contexts.Projects == nil || contexts.Projects.Sqlite == nil || contexts.Plots == nil {
		return nil, errors.New("Summary Vegetation context service is unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}

	average := request.Average
	if average == 0 {
		average = 1
	}
	display := request.Display
	if display == 0 {
		display = 6
	}
	showClass := request.ShowClass
	groupAndOrder := request.GroupAndOrder
	if groupAndOrder == 0 {
		groupAndOrder = 11
	}
	if groupAndOrder == 11 {
		if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
			return nil, err
		}
	} else {
		if err := validateSummaryVegetationGroupedOptions(ctx, average, groupAndOrder, display, showClass); err != nil {
			return nil, err
		}
	}

	rules, err := NormalizeExclusionRules(request.Rules, request.UseDefaultPresets)
	if err != nil {
		return nil, err
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*SummaryVegetationExclusionPreview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SummaryVegetationExclusionPreview, error) {
			if owner.Selection.SU == "" || owner.Selection.SU == "None" {
				return nil, errors.New("Summary Vegetation requires a selected Site Unit (SU) table")
			}

			tables := make([]vprocore.ProjectMetadataTable, 5)
			for i, source := range []struct{ role, table string }{
				{"project", owner.Selection.Project + "_Veg"},
				{"su", owner.Selection.SU + "_SU"},
				{"VLists", "USysAllSpecs"},
				{"VUser", "USysUserSpp"},
				{"VPro64", "LayerCode"},
			} {
				var count int
				if err := tx.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`,
					source.table,
				).Scan(&count); err != nil {
					return nil, err
				}
				if count != 1 {
					return nil, fmt.Errorf("Summary Vegetation requires original physical table %s.%s", source.role, source.table)
				}
				tbl, tErr := vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
				if tErr != nil {
					return nil, tErr
				}
				tables[i] = tbl
			}

			effectiveSU, exclusionSummary, err := FilterSUTableByExclusions(tables[1], rules)
			if err != nil {
				return nil, err
			}

			var bpResolution *HierarchyBreakpointResolution
			if request.UseHierarchyBreakpoints {
				if owner.Selection.Hierarchy == "" || owner.Selection.Hierarchy == "None" {
					return nil, errors.New("an active Hierarchy table must be selected when UseHierarchyBreakpoints is enabled")
				}
				nodes, hErr := loadHierarchyNodesFromTx(ctx, tx, owner.Selection.Hierarchy)
				if hErr != nil {
					return nil, hErr
				}
				rolledSU, bpRes, bErr := resolveHierarchyBreakpointsOnSU(effectiveSU, nodes, request.BreakpointOptions)
				if bErr != nil {
					return nil, bErr
				}
				bpRes.ContextID = contextID
				bpRes.Project = owner.Selection.Project
				bpRes.SU = owner.Selection.SU
				bpRes.Hierarchy = owner.Selection.Hierarchy
				effectiveSU = rolledSU
				bpResolution = &bpRes
			}

			var references *summaryVegetationReferencePlan
			refOpts := SummaryVegetationReferenceOptions{}
			if request.References != nil {
				refOpts = *request.References
			}
			var hasDesc int
			_ = tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM "VLists".sqlite_master WHERE type='table' AND name COLLATE BINARY='USysSppAttributeDescription'`,
			).Scan(&hasDesc)
			_, hasIsAttrErr := vprocore.SiteUnitTransferColumns(tables[2], "IsAttribute")
			if request.References != nil || (hasDesc == 1 && hasIsAttrErr == nil && summaryVegetationReferenceGroupingAvailable(groupAndOrder, refOpts.ShowCommon || refOpts.UseGuideNames)) {
				descTable, dErr := vprocore.ReadSQLiteStorageRows(ctx, tx, "VLists", "USysSppAttributeDescription", "", nil, "")
				if dErr != nil {
					return nil, dErr
				}
				plan, pErr := prepareSummaryVegetationReferences(ctx, tables[2], tables[3], descTable, refOpts)
				if pErr != nil {
					return nil, pErr
				}
				references = &plan
			}

			var report summaryVegetationLayerReport
			var grouped summaryVegetationGroupedReport
			if groupAndOrder == 11 {
				report, err = prepareSummaryVegetationLayerReferences(
					ctx, owner.Selection.Project, owner.Selection.SU,
					tables[0], effectiveSU, tables[2], tables[3], tables[4],
					average, display, showClass, references,
				)
			} else {
				grouped, err = prepareSummaryVegetationGroupedReferences(
					ctx, owner.Selection.Project, owner.Selection.SU,
					tables[0], effectiveSU, tables[2], tables[3], tables[4],
					average, groupAndOrder, display, showClass, references,
				)
				report = grouped.Report
			}
			if err != nil {
				return nil, err
			}

			sourceTag := "literal-normal-su-layer"
			if len(exclusionSummary.ExcludedUnits) > 0 && bpResolution != nil {
				sourceTag = "literal-filtered-su-hierarchy-breakpoints"
			} else if bpResolution != nil {
				sourceTag = "literal-normal-su-hierarchy-breakpoints"
			} else if len(exclusionSummary.ExcludedUnits) > 0 {
				sourceTag = "literal-filtered-su-layer"
			}

			preview := SummaryVegetationPreview{
				ContextID:   contextID,
				Project:     owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath,
				SU:          owner.Selection.SU,
				SUPath:      owner.Selection.SUPath,
				Average:     average,
				Display:     display,
				ShowClass:   showClass,
				Source:      sourceTag,
				Ordering:    "literal-null-first-layer-species",
				Memberships: []reportcore.VegetationReportMembership{},
				Units:       []SummaryVegetationUnit{},
				Columns:     append([]string{}, report.Columns...),
				Rows:        []SummaryVegetationRow{},
			}
			if references != nil {
				preview.References = &references.Review
			}
			if groupAndOrder != 11 {
				preview.GroupAndOrder = groupAndOrder
				preview.GroupField = &SummaryVegetationField{grouped.GroupField.Name, grouped.GroupField.DAOType, grouped.GroupField.Size}
				preview.OrderField = &SummaryVegetationField{grouped.OrderField.Name, grouped.OrderField.DAOType, grouped.OrderField.Size}
				preview.Ordering = "literal-null-first-group-species"
			}

			uc, err := vprocore.SiteUnitTransferColumns(effectiveSU, "PlotNumber", "SiteUnit")
			if err != nil {
				return nil, err
			}
			for _, row := range effectiveSU.Rows {
				preview.Memberships = append(preview.Memberships, reportcore.VegetationReportMembership{
					RowID:      row.RowID,
					PlotNumber: vprocore.CloneSiteUnitCell(row.Cells[uc["PlotNumber"]]),
					SiteUnit:   vprocore.CloneSiteUnitCell(row.Cells[uc["SiteUnit"]]),
				})
			}
			for _, u := range report.Units {
				preview.Units = append(preview.Units, SummaryVegetationUnit{
					Code:          u.Code,
					NumPlots:      u.NumPlots,
					MembershipIDs: append([]string{}, u.MembershipIDs...),
					RawVegRowIDs:  append([]string{}, u.RawVegRowIDs...),
				})
			}
			for _, row := range report.Rows {
				transport := SummaryVegetationRow{
					Layer:     vprocore.CloneSiteUnitCell(row.Layer),
					Species:   vprocore.CloneSiteUnitCell(row.Species),
					Order:     vprocore.CloneSiteUnitCell(row.Order),
					Covers:    make([]vprocore.ProjectMetadataCell, 0, len(row.Covers)),
					Witnesses: make([]SummaryVegetationWitness, 0, len(row.Witnesses)),
				}
				if row.CommonName != nil {
					v := vprocore.CloneSiteUnitCell(*row.CommonName)
					transport.CommonName = &v
				}
				if row.RawSpecies != nil {
					v := vprocore.CloneSiteUnitCell(*row.RawSpecies)
					transport.RawSpecies = &v
				}
				for _, c := range row.Covers {
					transport.Covers = append(transport.Covers, vprocore.CloneSiteUnitCell(c))
				}
				for _, w := range row.Witnesses {
					origins := make([]SummaryVegetationReferenceOrigin, 0, len(w.ReferenceOrigins))
					for _, o := range w.ReferenceOrigins {
						origins = append(origins, SummaryVegetationReferenceOrigin{Table: o.Table, RowID: o.RowID})
					}
					transport.Witnesses = append(transport.Witnesses, SummaryVegetationWitness{
						StageRowID:        w.StageRowID,
						ConversionLayerID: w.ConversionLayerID,
						MembershipID:      w.MembershipID,
						Unit:              w.Unit,
						LayerMapID:        w.LayerMapID,
						ReferenceOrigins:  origins,
						RawVegRowIDs:      append([]string{}, w.RawVegRowIDs...),
						Cover:             vprocore.CloneSiteUnitCell(w.Cover),
					})
				}
				preview.Rows = append(preview.Rows, transport)
			}

			return &SummaryVegetationExclusionPreview{
				Preview:     preview,
				Exclusions:  exclusionSummary,
				Breakpoints: bpResolution,
			}, nil
		})
	})
}

func PreviewLongVegetationWithExclusions(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	request LongVegetationExclusionRequest,
) (*LongVegetationExclusionPreview, error) {
	if ctx == nil {
		return nil, errors.New("Long Vegetation requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil || contexts.Projects == nil || contexts.Projects.Sqlite == nil || contexts.Plots == nil {
		return nil, errors.New("Long Vegetation context service is unavailable")
	}
	rules, err := NormalizeExclusionRules(request.Rules, request.UseDefaultPresets)
	if err != nil {
		return nil, err
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationExclusionPreview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationExclusionPreview, error) {
			if owner.Selection.SU == "" || owner.Selection.SU == "None" {
				return nil, errors.New("Long Vegetation requires an explicitly selected SU, not an all-project fallback")
			}

			values, err := plots.Projects.Preferences.Snapshot()
			if err != nil {
				return nil, err
			}
			options, err := DecodeOwnedLongVegetationOptions(contexts, ctx, plots, tx, values)
			if err != nil {
				return nil, err
			}
			if err := CheckLongVegetationGrouping(contexts, options); err != nil {
				return nil, err
			}

			sources := []struct{ role, table string }{
				{"project", owner.Selection.Project + "_Veg"},
				{"su", owner.Selection.SU + "_SU"},
				{"VLists", "USysAllSpecs"},
				{"VPro64", "LayerCode"},
				{"VLists", "MasterSiteUnitList"},
			}
			if options.Quality != nil || options.NoneUnits || options.SelectedUnitFields != nil {
				sources = append(sources, struct{ role, table string }{"project", owner.Selection.Project + "_Env"})
			}
			if options.Quality != nil || options.SelectedUnitFields != nil {
				sources = append(sources, struct{ role, table string }{"project", owner.Selection.Project + "_Admin"})
			}
			if options.Quality != nil {
				sources = append(sources, struct{ role, table string }{"VLists", "USysTableOfLists"})
			}
			personalIndex := -1
			if options.LifeformGrouping {
				personalIndex = len(sources)
				sources = append(sources, struct{ role, table string }{"VUser", "USysUserSpp"})
			}

			tables := make([]vprocore.ProjectMetadataTable, len(sources))
			for i, source := range sources {
				var count int
				if err := tx.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`,
					source.table,
				).Scan(&count); err != nil {
					return nil, err
				}
				if count != 1 {
					return nil, fmt.Errorf("Long Vegetation requires original physical table %s.%s", source.role, source.table)
				}
				tbl, tErr := vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
				if tErr != nil {
					return nil, tErr
				}
				tables[i] = tbl
			}

			effectiveSU, exclusionSummary, err := FilterSUTableByExclusions(tables[1], rules)
			if err != nil {
				return nil, err
			}

			var bpResolution *HierarchyBreakpointResolution
			if request.UseHierarchyBreakpoints {
				if owner.Selection.Hierarchy == "" || owner.Selection.Hierarchy == "None" {
					return nil, errors.New("an active Hierarchy table must be selected when UseHierarchyBreakpoints is enabled")
				}
				nodes, hErr := loadHierarchyNodesFromTx(ctx, tx, owner.Selection.Hierarchy)
				if hErr != nil {
					return nil, hErr
				}
				rolledSU, bpRes, bErr := resolveHierarchyBreakpointsOnSU(effectiveSU, nodes, request.BreakpointOptions)
				if bErr != nil {
					return nil, bErr
				}
				bpRes.ContextID = contextID
				bpRes.Project = owner.Selection.Project
				bpRes.SU = owner.Selection.SU
				bpRes.Hierarchy = owner.Selection.Hierarchy
				effectiveSU = rolledSU
				bpResolution = &bpRes
			}

			var prepared VegetationReportPreparation
			var quality vegetationQualitySelection
			var noneUnits []vegetationNoneUnitOccurrence
			var selectedUnits []vegetationSelectedUnitOccurrence

			if options.SelectedUnitFields != nil {
				var qualified *vegetationQualitySelection
				if options.Quality != nil {
					quality, err = planLongVegetationQuality(ctx, effectiveSU, tables[5], tables[6], tables[7], *options.Quality)
					if err != nil {
						return nil, err
					}
					qualified = &quality
				}
				prepared, selectedUnits, err = prepareLongVegetationSelectedUnits(ctx, owner.Selection.Project, owner.Selection.SU,
					tables[0], effectiveSU, tables[3], tables[5], tables[6], options.SelectedUnitFields, qualified)
				if err == nil && len(selectedUnits) == 0 {
					err = errors.New("Long Vegetation selected report units have no derived memberships after SiteUnit exclusions")
				}
			} else if options.NoneUnits {
				var qualified *vegetationQualitySelection
				if options.Quality != nil {
					quality, err = planLongVegetationQuality(ctx, effectiveSU, tables[5], tables[6], tables[7], *options.Quality)
					if err != nil {
						return nil, err
					}
					qualified = &quality
				}
				prepared, noneUnits, err = prepareLongVegetationNoneUnits(ctx, owner.Selection.Project, owner.Selection.SU,
					tables[0], effectiveSU, tables[3], tables[5], qualified)
				if err == nil && len(noneUnits) == 0 {
					err = errors.New("Long Vegetation None report units have no joined Env/SU plots after SiteUnit exclusions")
				}
			} else if options.Quality == nil {
				prepared, err = prepareLongVegetation(ctx, owner.Selection.Project, owner.Selection.SU, tables[0], effectiveSU, tables[3])
			} else {
				prepared, quality, err = prepareQualityLongVegetation(ctx, owner.Selection.Project, owner.Selection.SU,
					tables[0], effectiveSU, tables[3], tables[5], tables[6], tables[7], *options.Quality)
			}
			if err != nil {
				return nil, err
			}
			if options.Quality != nil && len(quality.Occurrences) == 0 {
				return nil, errors.New("no plots remain after applying plot quality standards and SiteUnit exclusions")
			}

			var personal vprocore.ProjectMetadataTable
			if personalIndex >= 0 {
				personal = tables[personalIndex]
			}
			report, err := planLongVegetationPreparedSource(ctx, prepared, tables[2], personal, tables[3], options)
			if err != nil {
				return nil, err
			}
			if options.Quality != nil {
				report.Quality = &quality
				if options.NoneUnits || options.SelectedUnitFields != nil {
					if err := addLongVegetationQualityMultiplicity(ctx, &report, quality); err != nil {
						return nil, err
					}
				}
			}
			report.NoneUnits = noneUnits
			report.SelectedUnits = selectedUnits
			if err := addLongVegetationUnitNames(ctx, &report, tables[4]); err != nil {
				return nil, err
			}

			transport, err := longVegetationTransport(ctx, report)
			if err != nil {
				return nil, err
			}
			lvPreview := LongVegetationPreview{
				ContextID:   contextID,
				ProjectPath: owner.Selection.ProjectPath,
				SUPath:      owner.Selection.SUPath,
				Settings:    vegetationSettings(options),
				Report:      transport,
			}

			return &LongVegetationExclusionPreview{
				Preview:     lvPreview,
				Exclusions:  exclusionSummary,
				Breakpoints: bpResolution,
			}, nil
		})
	})
}
