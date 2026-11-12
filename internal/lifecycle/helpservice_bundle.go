package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/resources"
)

var helpBundleJSON = resources.HelpBundleJSON

const whatsNewConfigKey = "last_seen_whats_new_version"

type HelpCodeOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type HelpFieldTopic struct {
	ID        string           `json:"id"`
	Code      string           `json:"code"`
	Section   string           `json:"section"`
	Title     string           `json:"title"`
	Column    string           `json:"column"`
	Unit      string           `json:"unit"`
	LMH25Page int              `json:"lmh25Page"`
	KeyRef    string           `json:"keyRef,omitempty"`
	Summary   string           `json:"summary"`
	Rules     []string         `json:"rules"`
	Codes     []HelpCodeOption `json:"codes"`
}

type HelpDecisionStep struct {
	Step   int    `json:"step"`
	Prompt string `json:"prompt"`
	Yes    string `json:"yes,omitempty"`
	No     string `json:"no,omitempty"`
}

type HelpDecisionKey struct {
	ID        string             `json:"id"`
	Code      string             `json:"code"`
	Title     string             `json:"title"`
	LMH25Page int                `json:"lmh25Page"`
	Steps     []HelpDecisionStep `json:"steps"`
}

type HelpGuideSection struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

type HelpGuide struct {
	ID       string             `json:"id"`
	Slug     string             `json:"slug"`
	Category string             `json:"category"`
	Title    string             `json:"title"`
	Summary  string             `json:"summary"`
	Sections []HelpGuideSection `json:"sections"`
}

type HelpTourStep struct {
	Target string `json:"target"`
	View   string `json:"view"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type HelpTour struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Duration    string         `json:"duration"`
	Description string         `json:"description"`
	Steps       []HelpTourStep `json:"steps"`
}

type HelpShortcut struct {
	Keys   string `json:"keys"`
	Scope  string `json:"scope"`
	Action string `json:"action"`
}

type HelpWhatsNewEntry struct {
	Version    string   `json:"version"`
	Date       string   `json:"date"`
	Channel    string   `json:"channel"`
	Highlights []string `json:"highlights"`
}

type HelpDataUseAgreement struct {
	Title     string   `json:"title"`
	Authority string   `json:"authority"`
	SourceURL string   `json:"sourceUrl"`
	Citation  string   `json:"citation"`
	Clauses   []string `json:"clauses"`
}

type HelpBundle struct {
	SchemaVersion    int                  `json:"schemaVersion"`
	ProductName      string               `json:"productName"`
	FullName         string               `json:"fullName"`
	Organization     string               `json:"organization"`
	LMH25Edition     string               `json:"lmh25Edition"`
	LibraryStamps    LibraryStamps        `json:"libraryStamps"`
	Fields           []HelpFieldTopic     `json:"fields"`
	DecisionKeys     []HelpDecisionKey    `json:"decisionKeys"`
	Guides           []HelpGuide          `json:"guides"`
	Tours            []HelpTour           `json:"tours"`
	Shortcuts        []HelpShortcut       `json:"shortcuts"`
	WhatsNew         []HelpWhatsNewEntry  `json:"whatsNew"`
	DataUseAgreement HelpDataUseAgreement `json:"dataUseAgreement"`
}

type HelpLookupResult struct {
	RequestedID   string           `json:"requestedId"`
	ResolvedID    string           `json:"resolvedId"`
	MatchKind     string           `json:"matchKind"`
	Field         *HelpFieldTopic  `json:"field,omitempty"`
	DecisionKey   *HelpDecisionKey `json:"decisionKey,omitempty"`
	RelatedGuides []HelpGuide      `json:"relatedGuides"`
}

type HelpSearchHit struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Code      string `json:"code,omitempty"`
	Title     string `json:"title"`
	Section   string `json:"section,omitempty"`
	Snippet   string `json:"snippet"`
	LMH25Page int    `json:"lmh25Page,omitempty"`
	Score     int    `json:"score"`
}

type WhatsNewStatus struct {
	LatestVersion   string              `json:"latestVersion"`
	LastSeenVersion string              `json:"lastSeenVersion"`
	HasUnread       bool                `json:"hasUnread"`
	UnreadCount     int                 `json:"unreadCount"`
	Entries         []HelpWhatsNewEntry `json:"entries"`
}

func parseHelpBundle(data []byte, stamps LibraryStamps) (HelpBundle, error) {
	var raw struct {
		SchemaVersion    int                  `json:"schemaVersion"`
		ProductName      string               `json:"productName"`
		FullName         string               `json:"fullName"`
		Organization     string               `json:"organization"`
		LMH25Edition     string               `json:"lmh25Edition"`
		Provenance       json.RawMessage      `json:"provenance"`
		Fields           []HelpFieldTopic     `json:"fields"`
		DecisionKeys     []HelpDecisionKey    `json:"decisionKeys"`
		Guides           []HelpGuide          `json:"guides"`
		Tours            []HelpTour           `json:"tours"`
		Shortcuts        []HelpShortcut       `json:"shortcuts"`
		WhatsNew         []HelpWhatsNewEntry  `json:"whatsNew"`
		DataUseAgreement HelpDataUseAgreement `json:"dataUseAgreement"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return HelpBundle{}, fmt.Errorf("decode help bundle: %w", err)
	}
	if raw.SchemaVersion != 1 ||
		raw.ProductName == "" ||
		len(raw.Fields) == 0 ||
		len(raw.DecisionKeys) == 0 ||
		len(raw.Guides) == 0 ||
		len(raw.Tours) < 4 ||
		len(raw.Shortcuts) == 0 ||
		len(raw.WhatsNew) == 0 ||
		len(raw.DataUseAgreement.Clauses) == 0 {
		return HelpBundle{}, errors.New("embedded help bundle is incomplete")
	}
	bundle := HelpBundle{
		SchemaVersion:    raw.SchemaVersion,
		ProductName:      raw.ProductName,
		FullName:         raw.FullName,
		Organization:     raw.Organization,
		LMH25Edition:     raw.LMH25Edition,
		LibraryStamps:    cloneLibraryStamps(stamps),
		Fields:           raw.Fields,
		DecisionKeys:     raw.DecisionKeys,
		Guides:           raw.Guides,
		Tours:            raw.Tours,
		Shortcuts:        raw.Shortcuts,
		WhatsNew:         raw.WhatsNew,
		DataUseAgreement: raw.DataUseAgreement,
	}
	return cloneHelpBundle(bundle), nil
}

func cloneHelpFieldTopic(src HelpFieldTopic) HelpFieldTopic {
	dst := src
	dst.Rules = make([]string, len(src.Rules))
	copy(dst.Rules, src.Rules)
	dst.Codes = make([]HelpCodeOption, len(src.Codes))
	copy(dst.Codes, src.Codes)
	return dst
}

func cloneHelpDecisionKey(src HelpDecisionKey) HelpDecisionKey {
	dst := src
	dst.Steps = make([]HelpDecisionStep, len(src.Steps))
	copy(dst.Steps, src.Steps)
	return dst
}

func cloneHelpGuide(src HelpGuide) HelpGuide {
	dst := src
	dst.Sections = make([]HelpGuideSection, len(src.Sections))
	copy(dst.Sections, src.Sections)
	return dst
}

func cloneHelpTour(src HelpTour) HelpTour {
	dst := src
	dst.Steps = make([]HelpTourStep, len(src.Steps))
	copy(dst.Steps, src.Steps)
	return dst
}

func cloneHelpWhatsNewEntry(src HelpWhatsNewEntry) HelpWhatsNewEntry {
	dst := src
	dst.Highlights = make([]string, len(src.Highlights))
	copy(dst.Highlights, src.Highlights)
	return dst
}

func cloneHelpDataUseAgreement(src HelpDataUseAgreement) HelpDataUseAgreement {
	dst := src
	dst.Clauses = make([]string, len(src.Clauses))
	copy(dst.Clauses, src.Clauses)
	return dst
}

func cloneHelpBundle(src HelpBundle) HelpBundle {
	dst := src
	dst.LibraryStamps = cloneLibraryStamps(src.LibraryStamps)
	dst.Fields = make([]HelpFieldTopic, len(src.Fields))
	for i, f := range src.Fields {
		dst.Fields[i] = cloneHelpFieldTopic(f)
	}
	dst.DecisionKeys = make([]HelpDecisionKey, len(src.DecisionKeys))
	for i, k := range src.DecisionKeys {
		dst.DecisionKeys[i] = cloneHelpDecisionKey(k)
	}
	dst.Guides = make([]HelpGuide, len(src.Guides))
	for i, g := range src.Guides {
		dst.Guides[i] = cloneHelpGuide(g)
	}
	dst.Tours = make([]HelpTour, len(src.Tours))
	for i, t := range src.Tours {
		dst.Tours[i] = cloneHelpTour(t)
	}
	dst.Shortcuts = make([]HelpShortcut, len(src.Shortcuts))
	copy(dst.Shortcuts, src.Shortcuts)
	dst.WhatsNew = make([]HelpWhatsNewEntry, len(src.WhatsNew))
	for i, w := range src.WhatsNew {
		dst.WhatsNew[i] = cloneHelpWhatsNewEntry(w)
	}
	dst.DataUseAgreement = cloneHelpDataUseAgreement(src.DataUseAgreement)
	return dst
}

func validateCleanText(input, label string) error {
	if !utf8.ValidString(input) || strings.ContainsRune(input, 0) {
		return fmt.Errorf("%s contains malformed UTF-8 or NUL characters", label)
	}
	return nil
}

func (s *HelpService) requireReady(ctx context.Context, op string) error {
	if ctx == nil {
		return fmt.Errorf("%s requires a context", op)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return errors.New("help service is unavailable")
	}
	return nil
}

func (s *HelpService) GetHelpBundle(ctx context.Context) (HelpBundle, error) {
	if err := s.requireReady(ctx, "help bundle lookup"); err != nil {
		return HelpBundle{}, err
	}
	return cloneHelpBundle(s.bundle), nil
}

var helpTopicAliases = map[string]string{
	"site.plot":                  "site.plot_number",
	"site.plotnumber":            "site.plot_number",
	"site.project_id":            "site.field_number",
	"site.projectid":             "site.field_number",
	"site.field_no":              "site.field_number",
	"site.surveyor":              "site.date",
	"site.start_date":            "site.date",
	"site.zone":                  "site.bgc_zone",
	"site.subzone":               "site.bgc_zone",
	"site.site_series":           "site.bgc_zone",
	"site.bec_site_unit":         "site.bgc_zone",
	"site.user_site_unit":        "site.bgc_zone",
	"site.map_unit":              "site.bgc_zone",
	"site.plot_representing":     "site.bgc_zone",
	"site.forest_district":       "site.ecosection",
	"site.fs_region_district":    "site.ecosection",
	"site.latitude":              "site.coordinates",
	"site.longitude":             "site.coordinates",
	"site.utm":                   "site.coordinates",
	"site.utm_zone":              "site.coordinates",
	"site.easting":               "site.coordinates",
	"site.northing":              "site.coordinates",
	"site.map_sheet":             "site.coordinates",
	"site.location":              "site.coordinates",
	"site.slope":                 "site.elevation",
	"site.slope_gradient":        "site.elevation",
	"site.aspect":                "site.elevation",
	"site.surface_shape":         "site.meso_slope",
	"site.microtopography":       "site.meso_slope",
	"site.rsmr":                  "site.moisture_regime",
	"site.smr":                   "site.moisture_regime",
	"site.rsnr":                  "site.nutrient_regime",
	"site.snr":                   "site.nutrient_regime",
	"site.substrate":             "site.substrate_cover",
	"site.structural_stage":      "site.successional_structural",
	"site.successional_status":   "site.successional_structural",
	"site.stand_age":             "site.successional_structural",
	"site.snow_cover_regime":     "site.realm_class",
	"site.active_layer_depth":    "site.realm_class",
	"site.chars":                 "site.realm_class",
	"humus.upper_depth":          "humus.upper_lower_depth",
	"humus.lower_depth":          "humus.upper_lower_depth",
	"humus.depth":                "humus.upper_lower_depth",
	"humus.thickness":            "humus.upper_lower_depth",
	"humus.ph":                   "humus.humus_form",
	"mineral.upper_depth":        "mineral.horizon",
	"mineral.lower_depth":        "mineral.horizon",
	"mineral.depth":              "mineral.horizon",
	"mineral.texture":            "mineral.texture_cf",
	"mineral.colour":             "mineral.texture_cf",
	"mineral.coarse_fragments":   "mineral.texture_cf",
	"soil.rooting_depth":         "soil.rooting_seepage",
	"soil.seepage_depth":         "soil.rooting_seepage",
	"soil.drainage":              "soil.rooting_seepage",
	"veg.cover":                  "veg.covers",
	"veg.cover1":                 "veg.covers",
	"veg.cover2":                 "veg.covers",
	"veg.cover3":                 "veg.covers",
	"veg.cover4":                 "veg.covers",
	"veg.cover5":                 "veg.covers",
	"veg.cover5a":                "veg.covers",
	"veg.cover5b":                "veg.covers",
	"veg.cover5c":                "veg.covers",
	"veg.cover6":                 "veg.covers",
	"veg.cover7":                 "veg.covers",
	"veg.cover8":                 "veg.covers",
	"veg.cover9":                 "veg.covers",
	"veg.cover10":                "veg.covers",
	"veg.totala":                 "veg.covers",
	"veg.totalb":                 "veg.covers",
	"veg.height":                 "veg.covers",
	"veg.strata":                 "veg.covers",
}

var implicitFieldKeyRefs = map[string]string{
	"site.meso_slope":      "key.rsmr_9_4",
	"soil.rooting_seepage": "key.rsmr_9_4",
	"humus.humus_form":     "key.rsnr_9_6",
}

func splitColumnTokens(spec string) []string {
	raw := strings.FieldsFunc(spec, func(r rune) bool {
		return r == '/' || r == ',' || r == ' ' || r == '.' || r == '(' || r == ')'
	})
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		trimmed := strings.TrimSpace(token)
		if trimmed != "" {
			out = append(out, strings.ToLower(trimmed))
		}
	}
	return out
}

func (s *HelpService) findKeyByID(id string) *HelpDecisionKey {
	if id == "" {
		return nil
	}
	for _, k := range s.bundle.DecisionKeys {
		if strings.EqualFold(k.ID, id) {
			cloned := cloneHelpDecisionKey(k)
			return &cloned
		}
	}
	return nil
}

func (s *HelpService) relatedGuidesForTopic(topicID string) []HelpGuide {
	wanted := []string{"fs882-sivi-datasheet-entry"}
	lower := strings.ToLower(topicID)
	switch {
	case strings.Contains(lower, "coordinates") || strings.Contains(lower, "ecosection") || strings.Contains(lower, "moisture") || strings.Contains(lower, "rsmr"):
		wanted = append(wanted, "geospatial-and-r-sidecar")
	case strings.Contains(lower, "bgc") || strings.HasPrefix(lower, "veg.") || strings.Contains(lower, "cover"):
		wanted = append(wanted, "classification-lumping-diagnostics", "standards-and-interchange")
	case strings.HasPrefix(lower, "sivi."):
		wanted = append(wanted, "migrating-from-vpro64")
	default:
		wanted = append(wanted, "migrating-from-vpro64")
	}
	out := make([]HelpGuide, 0, len(wanted))
	for _, gid := range wanted {
		for _, g := range s.bundle.Guides {
			if g.ID == gid {
				out = append(out, cloneHelpGuide(g))
				break
			}
		}
	}
	return out
}

func (s *HelpService) buildFieldLookupResult(requested, matchKind string, field HelpFieldTopic) HelpLookupResult {
	clonedField := cloneHelpFieldTopic(field)
	keyID := field.KeyRef
	if keyID == "" {
		keyID = implicitFieldKeyRefs[field.ID]
	}
	return HelpLookupResult{
		RequestedID:   requested,
		ResolvedID:    field.ID,
		MatchKind:     matchKind,
		Field:         &clonedField,
		DecisionKey:   s.findKeyByID(keyID),
		RelatedGuides: s.relatedGuidesForTopic(field.ID),
	}
}

func (s *HelpService) LookupHelpTopic(ctx context.Context, helpID string) (HelpLookupResult, error) {
	if err := s.requireReady(ctx, "help topic lookup"); err != nil {
		return HelpLookupResult{}, err
	}
	if err := validateCleanText(helpID, "help topic ID"); err != nil {
		return HelpLookupResult{}, err
	}
	trimmed := strings.TrimSpace(helpID)
	if trimmed == "" {
		return HelpLookupResult{}, errors.New("help topic ID is required")
	}
	normalized := strings.ToLower(trimmed)

	// 1. Exact field ID match
	for _, f := range s.bundle.Fields {
		if strings.ToLower(f.ID) == normalized {
			return s.buildFieldLookupResult(trimmed, "exact", f), nil
		}
	}

	// 2. Exact decision key ID match
	for _, k := range s.bundle.DecisionKeys {
		if strings.ToLower(k.ID) == normalized {
			clonedKey := cloneHelpDecisionKey(k)
			var linkedField *HelpFieldTopic
			for _, f := range s.bundle.Fields {
				if strings.EqualFold(f.KeyRef, k.ID) {
					cf := cloneHelpFieldTopic(f)
					linkedField = &cf
					break
				}
			}
			return HelpLookupResult{
				RequestedID:   trimmed,
				ResolvedID:    k.ID,
				MatchKind:     "key",
				Field:         linkedField,
				DecisionKey:   &clonedKey,
				RelatedGuides: s.relatedGuidesForTopic(k.ID),
			}, nil
		}
	}

	// 3. Curated alias map (e.g. humus.upper_depth -> humus.upper_lower_depth, veg.cover6 -> veg.covers)
	if aliasTarget, ok := helpTopicAliases[normalized]; ok {
		for _, f := range s.bundle.Fields {
			if f.ID == aliasTarget {
				return s.buildFieldLookupResult(trimmed, "alias", f), nil
			}
		}
	}

	// 4. Exact LMH25 code or physical column match (e.g. "SITE 20", "MoistureRegime", "Cover5a")
	for _, f := range s.bundle.Fields {
		if strings.ToLower(f.Code) == normalized {
			return s.buildFieldLookupResult(trimmed, "code", f), nil
		}
		for _, col := range splitColumnTokens(f.Column) {
			if col == normalized {
				return s.buildFieldLookupResult(trimmed, "column", f), nil
			}
		}
	}
	if strings.HasPrefix(normalized, "cover") || strings.HasPrefix(normalized, "veg.cover") {
		for _, f := range s.bundle.Fields {
			if f.ID == "veg.covers" {
				return s.buildFieldLookupResult(trimmed, "prefix", f), nil
			}
		}
	}
	if strings.HasPrefix(normalized, "substrate") || strings.HasPrefix(normalized, "site.substrate") {
		for _, f := range s.bundle.Fields {
			if f.ID == "site.substrate_cover" {
				return s.buildFieldLookupResult(trimmed, "prefix", f), nil
			}
		}
	}

	// 5. Bidirectional prefix match on topic IDs (e.g. "humus.upper" -> "humus.upper_lower_depth", "sivi.observer" -> "sivi.plot_type")
	for _, f := range s.bundle.Fields {
		fid := strings.ToLower(f.ID)
		if strings.HasPrefix(fid, normalized) || strings.HasPrefix(normalized, fid+".") || strings.HasPrefix(normalized, fid+"_") {
			return s.buildFieldLookupResult(trimmed, "prefix", f), nil
		}
	}
	if dot := strings.IndexByte(normalized, '.'); dot > 0 {
		domainPrefix := normalized[:dot+1]
		for _, f := range s.bundle.Fields {
			if strings.HasPrefix(strings.ToLower(f.ID), domainPrefix) {
				return s.buildFieldLookupResult(trimmed, "prefix", f), nil
			}
		}
	}

	return HelpLookupResult{}, fmt.Errorf("unknown help topic %q", trimmed)
}

func tokenizeSearchQuery(query string) []string {
	raw := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', ',', ';', '(', ')', '[', ']', '"', '\'', '—', '–':
			return true
		default:
			return false
		}
	})
	tokens := make([]string, 0, len(raw))
	for _, tok := range raw {
		trimmed := strings.Trim(tok, ".:")
		if trimmed != "" && !slices.Contains(tokens, trimmed) {
			tokens = append(tokens, trimmed)
		}
	}
	return tokens
}

func kindPriority(kind string) int {
	switch kind {
	case "field":
		return 1
	case "key":
		return 2
	case "guide":
		return 3
	case "shortcut":
		return 4
	case "tour":
		return 5
	default:
		return 9
	}
}

func hasExactSlashCode(codeSpec, needle string) bool {
	for _, part := range strings.FieldsFunc(strings.ToLower(codeSpec), func(r rune) bool {
		return r == '/' || r == ',' || r == '(' || r == ')' || r == ' '
	}) {
		if part == needle {
			return true
		}
	}
	return false
}

func (s *HelpService) SearchHelp(ctx context.Context, query string) ([]HelpSearchHit, error) {
	if err := s.requireReady(ctx, "help search"); err != nil {
		return nil, err
	}
	if err := validateCleanText(query, "help search query"); err != nil {
		return nil, err
	}
	phrase := strings.ToLower(strings.TrimSpace(query))
	if phrase == "" {
		return []HelpSearchHit{}, nil
	}
	tokens := tokenizeSearchQuery(phrase)
	if len(tokens) == 0 {
		return []HelpSearchHit{}, nil
	}

	hits := make([]HelpSearchHit, 0, 16)

	// 1. Search LMH25 Fields
	for _, f := range s.bundle.Fields {
		idLower := strings.ToLower(f.ID)
		codeLower := strings.ToLower(f.Code)
		titleLower := strings.ToLower(f.Title)
		colLower := strings.ToLower(f.Column)
		unitLower := strings.ToLower(f.Unit)
		secLower := strings.ToLower(f.Section)
		sumLower := strings.ToLower(f.Summary)

		var rulesBuf strings.Builder
		for _, r := range f.Rules {
			rulesBuf.WriteString(strings.ToLower(r))
			rulesBuf.WriteByte('\n')
		}
		rulesLower := rulesBuf.String()

		var codesBuf strings.Builder
		for _, c := range f.Codes {
			codesBuf.WriteString(strings.ToLower(c.Code))
			codesBuf.WriteByte(' ')
			codesBuf.WriteString(strings.ToLower(c.Label))
			codesBuf.WriteByte('\n')
		}
		codesLower := codesBuf.String()

		corpus := strings.Join([]string{idLower, codeLower, titleLower, colLower, unitLower, secLower, sumLower, rulesLower, codesLower}, " ")
		allMatch := true
		for _, tok := range tokens {
			if !strings.Contains(corpus, tok) {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}

		score := 0
		snippet := f.Summary
		if codeLower == phrase || idLower == phrase {
			score += 650
		} else if strings.HasPrefix(codeLower, phrase) || strings.HasPrefix(idLower, phrase) {
			score += 320
		}
		for _, col := range splitColumnTokens(f.Column) {
			if col == phrase {
				score += 520
				break
			}
		}
		if strings.Contains(titleLower, phrase) {
			score += 360
		}
		for _, c := range f.Codes {
			if strings.ToLower(c.Code) == phrase || hasExactSlashCode(c.Code, phrase) {
				score += 480
				snippet = fmt.Sprintf("%s — %s", c.Code, c.Label)
				break
			}
			if strings.Contains(strings.ToLower(c.Label), phrase) {
				score += 260
				snippet = fmt.Sprintf("%s — %s", c.Code, c.Label)
				break
			}
		}
		if snippet == f.Summary && !strings.Contains(sumLower, phrase) {
			for _, r := range f.Rules {
				if strings.Contains(strings.ToLower(r), phrase) {
					snippet = r
					break
				}
			}
		}
		for _, tok := range tokens {
			if strings.Contains(codeLower, tok) || strings.Contains(idLower, tok) {
				score += 95
			}
			if strings.Contains(titleLower, tok) {
				score += 75
			}
			if strings.Contains(colLower, tok) || strings.Contains(codesLower, tok) {
				score += 55
			}
			if strings.Contains(sumLower, tok) {
				score += 30
			}
			if strings.Contains(rulesLower, tok) || strings.Contains(unitLower, tok) {
				score += 20
			}
		}
		hits = append(hits, HelpSearchHit{
			Kind:      "field",
			ID:        f.ID,
			Code:      f.Code,
			Title:     f.Title,
			Section:   f.Section,
			Snippet:   snippet,
			LMH25Page: f.LMH25Page,
			Score:     score,
		})
	}

	// 2. Search LMH25 Decision Keys
	for _, k := range s.bundle.DecisionKeys {
		idLower := strings.ToLower(k.ID)
		codeLower := strings.ToLower(k.Code)
		titleLower := strings.ToLower(k.Title)
		var stepsBuf strings.Builder
		for _, st := range k.Steps {
			stepsBuf.WriteString(strings.ToLower(st.Prompt))
			stepsBuf.WriteByte(' ')
			stepsBuf.WriteString(strings.ToLower(st.Yes))
			stepsBuf.WriteByte(' ')
			stepsBuf.WriteString(strings.ToLower(st.No))
			stepsBuf.WriteByte('\n')
		}
		stepsLower := stepsBuf.String()
		corpus := strings.Join([]string{idLower, codeLower, titleLower, stepsLower}, " ")
		allMatch := true
		for _, tok := range tokens {
			if !strings.Contains(corpus, tok) {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}
		score := 0
		if idLower == phrase || codeLower == phrase {
			score += 620
		}
		if strings.Contains(codeLower, phrase) || strings.Contains(titleLower, phrase) {
			score += 350
		}
		snippet := k.Steps[0].Prompt
		for _, st := range k.Steps {
			combined := strings.ToLower(st.Prompt + " " + st.Yes + " " + st.No)
			if strings.Contains(combined, phrase) {
				snippet = st.Prompt
				if st.Yes != "" {
					snippet = fmt.Sprintf("Step %d: %s -> %s", st.Step, st.Prompt, st.Yes)
				}
				score += 120
				break
			}
		}
		for _, tok := range tokens {
			if strings.Contains(codeLower, tok) || strings.Contains(titleLower, tok) {
				score += 80
			}
			if strings.Contains(stepsLower, tok) {
				score += 30
			}
		}
		hits = append(hits, HelpSearchHit{
			Kind:      "key",
			ID:        k.ID,
			Code:      k.Code,
			Title:     k.Title,
			Section:   "LMH25 Decision Keys",
			Snippet:   snippet,
			LMH25Page: k.LMH25Page,
			Score:     score,
		})
	}

	// 3. Search Guides
	for _, g := range s.bundle.Guides {
		idLower := strings.ToLower(g.ID)
		catLower := strings.ToLower(g.Category)
		titleLower := strings.ToLower(g.Title)
		sumLower := strings.ToLower(g.Summary)
		var secBuf strings.Builder
		for _, sec := range g.Sections {
			secBuf.WriteString(strings.ToLower(sec.Heading))
			secBuf.WriteByte(' ')
			secBuf.WriteString(strings.ToLower(sec.Body))
			secBuf.WriteByte('\n')
		}
		secLower := secBuf.String()
		corpus := strings.Join([]string{idLower, catLower, titleLower, sumLower, secLower}, " ")
		allMatch := true
		for _, tok := range tokens {
			if !strings.Contains(corpus, tok) {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}
		score := 0
		if idLower == phrase {
			score += 500
		}
		if strings.Contains(titleLower, phrase) {
			score += 300
		}
		snippet := g.Summary
		for _, sec := range g.Sections {
			if strings.Contains(strings.ToLower(sec.Heading+" "+sec.Body), phrase) {
				snippet = fmt.Sprintf("%s: %s", sec.Heading, sec.Body)
				score += 90
				break
			}
		}
		for _, tok := range tokens {
			if strings.Contains(titleLower, tok) {
				score += 65
			}
			if strings.Contains(sumLower, tok) {
				score += 35
			}
			if strings.Contains(secLower, tok) {
				score += 20
			}
		}
		hits = append(hits, HelpSearchHit{
			Kind:    "guide",
			ID:      g.ID,
			Title:   g.Title,
			Section: g.Category,
			Snippet: snippet,
			Score:   score,
		})
	}

	// 4. Search Shortcuts
	for i, sc := range s.bundle.Shortcuts {
		keysLower := strings.ToLower(sc.Keys)
		scopeLower := strings.ToLower(sc.Scope)
		actLower := strings.ToLower(sc.Action)
		corpus := strings.Join([]string{keysLower, scopeLower, actLower}, " ")
		allMatch := true
		for _, tok := range tokens {
			if !strings.Contains(corpus, tok) {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}
		score := 0
		if keysLower == phrase {
			score += 550
		} else if strings.Contains(keysLower, phrase) {
			score += 280
		}
		for _, tok := range tokens {
			if strings.Contains(keysLower, tok) {
				score += 75
			}
			if strings.Contains(actLower, tok) {
				score += 35
			}
		}
		hits = append(hits, HelpSearchHit{
			Kind:    "shortcut",
			ID:      fmt.Sprintf("shortcut-%d", i+1),
			Code:    sc.Keys,
			Title:   fmt.Sprintf("%s — %s", sc.Keys, sc.Scope),
			Section: "Keyboard Shortcuts",
			Snippet: sc.Action,
			Score:   score,
		})
	}

	// 5. Search Guided Tours
	for _, tr := range s.bundle.Tours {
		idLower := strings.ToLower(tr.ID)
		titleLower := strings.ToLower(tr.Title)
		descLower := strings.ToLower(tr.Description)
		var stepBuf strings.Builder
		for _, st := range tr.Steps {
			stepBuf.WriteString(strings.ToLower(st.Title))
			stepBuf.WriteByte(' ')
			stepBuf.WriteString(strings.ToLower(st.Body))
			stepBuf.WriteByte('\n')
		}
		stepsLower := stepBuf.String()
		corpus := strings.Join([]string{idLower, titleLower, descLower, stepsLower}, " ")
		allMatch := true
		for _, tok := range tokens {
			if !strings.Contains(corpus, tok) {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}
		score := 0
		if strings.Contains(titleLower, phrase) {
			score += 260
		}
		for _, tok := range tokens {
			if strings.Contains(titleLower, tok) {
				score += 55
			}
			if strings.Contains(descLower, tok) || strings.Contains(stepsLower, tok) {
				score += 20
			}
		}
		hits = append(hits, HelpSearchHit{
			Kind:    "tour",
			ID:      tr.ID,
			Code:    tr.Duration,
			Title:   tr.Title,
			Section: "Interactive Walkthroughs",
			Snippet: tr.Description,
			Score:   score,
		})
	}

	slices.SortFunc(hits, func(a, b HelpSearchHit) int {
		if a.Score != b.Score {
			return b.Score - a.Score
		}
		if pa, pb := kindPriority(a.Kind), kindPriority(b.Kind); pa != pb {
			return pa - pb
		}
		return strings.Compare(a.ID, b.ID)
	})

	return hits, nil
}

func (s *HelpService) readLastSeenWhatsNewVersion() (string, error) {
	if s.preferences == nil {
		return "", nil
	}
	values, err := s.preferences.Snapshot()
	if err != nil {
		return "", err
	}
	if msgSection, ok := values["Message"].(map[string]any); ok {
		if raw, exists := msgSection[whatsNewConfigKey]; exists {
			if v, ok := raw.(string); ok {
				return strings.TrimSpace(v), nil
			}
		}
	}
	if raw, exists := values[whatsNewConfigKey]; exists {
		if v, ok := raw.(string); ok {
			return strings.TrimSpace(v), nil
		}
	}
	return "", nil
}

func (s *HelpService) GetWhatsNewStatus(ctx context.Context) (WhatsNewStatus, error) {
	if err := s.requireReady(ctx, "what's new lookup"); err != nil {
		return WhatsNewStatus{}, err
	}
	lastSeen, err := s.readLastSeenWhatsNewVersion()
	if err != nil {
		return WhatsNewStatus{}, err
	}
	entries := make([]HelpWhatsNewEntry, len(s.bundle.WhatsNew))
	for i, e := range s.bundle.WhatsNew {
		entries[i] = cloneHelpWhatsNewEntry(e)
	}
	latest := ""
	if len(entries) > 0 {
		latest = entries[0].Version
	}
	unreadCount := len(entries)
	if lastSeen != "" {
		for idx, e := range entries {
			if e.Version == lastSeen {
				unreadCount = idx
				break
			}
		}
	}
	return WhatsNewStatus{
		LatestVersion:   latest,
		LastSeenVersion: lastSeen,
		HasUnread:       unreadCount > 0,
		UnreadCount:     unreadCount,
		Entries:         entries,
	}, nil
}

func (s *HelpService) AcknowledgeWhatsNew(ctx context.Context, version string) (WhatsNewStatus, error) {
	if err := s.requireReady(ctx, "what's new acknowledgement"); err != nil {
		return WhatsNewStatus{}, err
	}
	if err := validateCleanText(version, "what's new version"); err != nil {
		return WhatsNewStatus{}, err
	}
	target := strings.TrimSpace(version)
	if target == "" {
		return WhatsNewStatus{}, errors.New("what's new version is required")
	}
	if strings.EqualFold(target, "latest") && len(s.bundle.WhatsNew) > 0 {
		target = s.bundle.WhatsNew[0].Version
	}
	known := false
	for _, entry := range s.bundle.WhatsNew {
		if entry.Version == target {
			known = true
			break
		}
	}
	if !known {
		return WhatsNewStatus{}, fmt.Errorf("unknown release notes version %q", target)
	}
	if s.preferences == nil {
		return WhatsNewStatus{}, errors.New("desktop configuration is unavailable")
	}
	if err := s.preferences.Update("Message", map[string]any{
		whatsNewConfigKey: target,
	}); err != nil {
		return WhatsNewStatus{}, err
	}
	return s.GetWhatsNewStatus(ctx)
}

func writePristineSampleFile(destination string) (resultErr error) {
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sample-pristine-*.db")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if tmp != nil {
			resultErr = errors.Join(resultErr, tmp.Close())
		}
		if removeErr := os.Remove(tmpName); !errors.Is(removeErr, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, removeErr)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err := tmp.Write(resources.SampleDB); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		tmp = nil
		return err
	}
	tmp = nil

	for _, sidecar := range []string{destination + "-wal", destination + "-shm"} {
		if err := os.Remove(sidecar); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("clear SQLite WAL sidecar %s: %w", filepath.Base(sidecar), err)
		}
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("install pristine Sample database: %w", err)
	}
	return nil
}

func removeSampleStagingFiles(stagingPath string) {
	for _, p := range []string{stagingPath + "-wal", stagingPath + "-shm", stagingPath} {
		_ = os.Remove(p)
	}
	_ = os.Remove(filepath.Dir(stagingPath))
}

func (s *HelpService) ResetWorkspaceToSample(ctx context.Context, contextID string) (vprocore.ProjectState, error) {
	if err := s.requireReady(ctx, "workspace sample reset"); err != nil {
		return vprocore.ProjectState{}, err
	}
	if err := validateCleanText(contextID, "context ID"); err != nil {
		return vprocore.ProjectState{}, err
	}
	if strings.TrimSpace(contextID) == "" {
		return vprocore.ProjectState{}, errors.New("context ID is required before resetting workspace to Sample")
	}
	if s.contexts == nil || s.contexts.Projects == nil {
		return vprocore.ProjectState{}, errors.New("project context service is unavailable")
	}

	projects := s.contexts.Projects
	projectsDir := filepath.Join(projects.Root, "projects")
	samplePath, err := filepath.Abs(filepath.Join(projectsDir, "Sample.db"))
	if err != nil {
		return vprocore.ProjectState{}, err
	}

	projects.Mu.RLock()
	currentID := projects.ContextID
	sqliteCtx := projects.Sqlite
	sampleAttached := false
	profileUsesSample := false
	if sqliteCtx != nil {
		if vprocore.SameDesktopPath(sqliteCtx.Selection.ProjectPath, samplePath) ||
			vprocore.SameDesktopPath(sqliteCtx.Selection.SUPath, samplePath) ||
			vprocore.SameDesktopPath(sqliteCtx.Selection.HierarchyPath, samplePath) {
			sampleAttached = true
		}
		for _, attachedPath := range sqliteCtx.Attachments {
			if vprocore.SameDesktopPath(attachedPath, samplePath) {
				sampleAttached = true
				break
			}
		}
		if sqliteCtx.Profile != nil && vprocore.SameDesktopPath(sqliteCtx.Profile.Path, samplePath) {
			sampleAttached = true
			profileUsesSample = true
		}
	}
	projects.Mu.RUnlock()

	if currentID == "" || contextID != currentID {
		return vprocore.ProjectState{}, errors.New("project context changed; reload before resetting workspace to Sample")
	}
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectState{}, err
	}

	activeID := contextID
	stagingPath := filepath.Join(projects.Root, ".sample-reset-staging", "Sample_Staging.db")
	if sampleAttached {
		defer removeSampleStagingFiles(stagingPath)
		if err := writePristineSampleFile(stagingPath); err != nil {
			return vprocore.ProjectState{}, err
		}
		if profileUsesSample {
			profState, err := s.contexts.SelectPlotProfile(activeID, vprocore.PlotProfileSource{
				Name: "Sample",
				Path: stagingPath,
			})
			if err != nil {
				return vprocore.ProjectState{}, err
			}
			activeID = profState.ContextID
		}
		stagedState, err := s.contexts.SwitchContext(activeID, vprocore.ContextSelection{
			Project:       "Sample",
			ProjectPath:   stagingPath,
			SU:            "Sample",
			SUPath:        stagingPath,
			Hierarchy:     "Sample",
			HierarchyPath: stagingPath,
		})
		if err != nil {
			return vprocore.ProjectState{}, err
		}
		activeID = stagedState.ContextID
	}

	if err := writePristineSampleFile(samplePath); err != nil {
		return vprocore.ProjectState{}, err
	}

	finalState, err := s.contexts.SwitchContext(activeID, vprocore.ContextSelection{
		Project:       "Sample",
		ProjectPath:   samplePath,
		SU:            "Sample",
		SUPath:        samplePath,
		Hierarchy:     "Sample",
		HierarchyPath: samplePath,
	})
	if err != nil {
		return vprocore.ProjectState{}, err
	}

	if profileUsesSample {
		finalState, err = s.contexts.SelectPlotProfile(finalState.ContextID, vprocore.PlotProfileSource{
			Name: "Sample",
			Path: samplePath,
		})
		if err != nil {
			return vprocore.ProjectState{}, err
		}
	}

	return finalState, nil
}
