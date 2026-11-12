package listcatalog

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	LMH252023PDFSHA256         = "ebf47444eab4d0862d3209e5108963c579c13e196ccb1d99f89a857bfcb6617a"
	BECdbV132026AccdbSHA256    = "7b19bd03e0a7cf0a9df453118fe42d8fecd9433f9c85634df3e556e6632161ba"
	BCPlantTaxa2025AccdbSHA256 = "41d60607c2e84487d95d49a73d425555d16679d1513e4c027690b4729b89281a"

	SiteLegacyDatabaseSHA256   = "d6bb8beb21066a04203b8f453c595a080b6f6751503d14a67df1cbd35693fb89"
	SoilLegacyDatabaseSHA256   = "31fa3688dcb60766308f9f79ecf190a9b95242eec569d7cc3c0c14965920e65a"
	ParentLegacyDatabaseSHA256 = "6e155d54cefb8540feecf28e5c7abb5938bac7394fe26c5a4a32a4950c5c6d69"

	SiteExporter2026   = "standards-2026-lmh25-site-codes-v2"
	SoilExporter2026   = "standards-2026-lmh25-soil-codes-v2"
	ParentExporter2026 = "standards-2026-lmh25-parent-codes-v2"

	SiteSnapshot2026SHA256   = "7b781e672ff997211c1f55a683d0f1ee138f0001a836dd1096e62d232c993dae"
	SoilSnapshot2026SHA256   = "e505284c9533f1679a00e0a8b98fd019e5105d28e96928e56b449e0e5d0fe654"
	ParentSnapshot2026SHA256 = "59259d5bc5da1fab6d71a2ecabb60e1497ea8acf4c72d2ba22ce255dbfb02953"
)

func SiteBundledProfile2026() Profile {
	return Profile{
		Version:              2,
		Exporter:             SiteExporter2026,
		SourceSHA256:         LMH252023PDFSHA256,
		SnapshotSHA256:       SiteSnapshot2026SHA256,
		LegacyDatabaseSHA256: SiteLegacyDatabaseSHA256,
		SQL:                  SourceSQL,
		Lists:                []ListDefinition{{"Exposure", 13}, {"SiteDisturbance", 129}},
	}
}

func SoilBundledProfile2026() Profile {
	return Profile{
		Version:              2,
		Exporter:             SoilExporter2026,
		SourceSHA256:         LMH252023PDFSHA256,
		SnapshotSHA256:       SoilSnapshot2026SHA256,
		LegacyDatabaseSHA256: SoilLegacyDatabaseSHA256,
		SQL:                  SoilSourceSQL("SoilClassGroup") + " " + SoilSourceSQL("SoilClassSubgroup"),
		Lists:                []ListDefinition{{"SoilClassGroup", 39}, {"SoilClassSubgroup", 62}},
	}
}

func ParentBundledProfile2026() Profile {
	base := ParentProfile()
	return Profile{
		Version:              2,
		Exporter:             ParentExporter2026,
		SourceSHA256:         LMH252023PDFSHA256,
		SnapshotSHA256:       ParentSnapshot2026SHA256,
		LegacyDatabaseSHA256: ParentLegacyDatabaseSHA256,
		SQL:                  base.SQL,
		Lists:                append([]ListDefinition(nil), base.Lists...),
	}
}

func strPtr(v string) *string    { return &v }
func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }

func renumberLists(rows []Choice, profile Profile) ([]Choice, error) {
	byList := map[string][]Choice{}
	for _, r := range rows {
		if r.ListName == nil {
			return nil, fmt.Errorf("row %s missing ListName", r.RowID)
		}
		byList[*r.ListName] = append(byList[*r.ListName], r)
	}
	total := 0
	for _, def := range profile.Lists {
		total += def.Rows
	}
	out := make([]Choice, 0, total)
	for _, def := range profile.Lists {
		items := byList[def.Name]
		if len(items) != def.Rows {
			return nil, fmt.Errorf("list %s count %d != expected %d", def.Name, len(items), def.Rows)
		}
		for i, it := range items {
			it.RowID = strconv.Itoa(i + 1)
			it.Selectable = false
			it.Diagnostic = ""
			out = append(out, it)
		}
	}
	return out, nil
}

// SiteBundled2026Rows upgrades the verified 2023 DAO SiteCode snapshot with LMH25 (2023) vocabulary:
// Exposure: CO ("Cold aspect"); SiteDisturbance: A.w.th ("Windthrow (uprooting)") and A.w.br ("Windbreak (stem snap)").
func SiteBundled2026Rows(daoSnapshot []byte) ([]Choice, Provenance, error) {
	baseP := Provenance{
		Version:           1,
		Exporter:          Exporter,
		SourceSHA256:      SourceSHA256,
		SnapshotSHA256:    SnapshotSHA256,
		DatabaseSHA256:    strings.Repeat("0", 64),
		TypedCellsSHA256:  strings.Repeat("0", 64),
		Rows:              139,
		Cells:             1390,
		SourceTable:       "USysTableOfLists",
		SQL:               SourceSQL,
		SourceSchema:      ExpectedSchema(),
		ItemOrderEncoding: ItemOrderEncoding,
	}
	baseRows, err := SnapshotRows(daoSnapshot, baseP)
	if err != nil {
		return nil, Provenance{}, err
	}
	upgraded := make([]Choice, 0, 142)
	insertedCO, insertedWind := false, false
	for _, r := range baseRows {
		upgraded = append(upgraded, r)
		if r.ListName != nil && *r.ListName == "Exposure" && r.Code != nil && *r.Code == "CA" {
			upgraded = append(upgraded, Choice{
				ListName:      strPtr("Exposure"),
				ListFilter:    strPtr(""),
				ItemOrder:     floatPtr(3.5),
				Code:          strPtr("CO"),
				Description:   strPtr("Cold aspect"),
				FieldUsedIn:   strPtr("Exposure"),
				ValidateLoops: strPtr("2"),
				Validate:      boolPtr(false),
				Note:          strPtr(""),
				Flag:          boolPtr(false),
			})
			insertedCO = true
		}
		if r.ListName != nil && *r.ListName == "SiteDisturbance" && r.Code != nil && *r.Code == "A.w" {
			upgraded = append(upgraded,
				Choice{
					ListName:      strPtr("SiteDisturbance"),
					ListFilter:    nil,
					ItemOrder:     floatPtr(12.1),
					Code:          strPtr("A.w.th"),
					Description:   strPtr("Windthrow (uprooting)"),
					FieldUsedIn:   strPtr("SiteDisturbance"),
					ValidateLoops: strPtr("3"),
					Validate:      boolPtr(true),
					Note:          nil,
					Flag:          boolPtr(false),
				},
				Choice{
					ListName:      strPtr("SiteDisturbance"),
					ListFilter:    nil,
					ItemOrder:     floatPtr(12.2),
					Code:          strPtr("A.w.br"),
					Description:   strPtr("Windbreak (stem snap)"),
					FieldUsedIn:   strPtr("SiteDisturbance"),
					ValidateLoops: strPtr("3"),
					Validate:      boolPtr(true),
					Note:          nil,
					Flag:          boolPtr(false),
				},
			)
			insertedWind = true
		}
	}
	if !insertedCO || !insertedWind {
		return nil, Provenance{}, fmt.Errorf("base site-codes snapshot missing CA or A.w anchor")
	}
	profile := SiteBundledProfile2026()
	finalRows, err := renumberLists(upgraded, profile)
	if err != nil {
		return nil, Provenance{}, err
	}
	p, err := Bundled2026Provenance(finalRows, profile)
	return finalRows, p, err
}

// SoilBundled2026Rows verifies CSSC / LMH25 (2023) SoilClassGroup (39) and SoilClassSubgroup (62) codes.
func SoilBundled2026Rows(daoGzip []byte) ([]Choice, Provenance, error) {
	histProfile := SoilProfile()
	baseP := Provenance{
		Version:           1,
		Exporter:          histProfile.Exporter,
		SourceSHA256:      histProfile.SourceSHA256,
		SnapshotSHA256:    histProfile.SnapshotSHA256,
		DatabaseSHA256:    strings.Repeat("0", 64),
		TypedCellsSHA256:  strings.Repeat("0", 64),
		Rows:              101,
		Cells:             1010,
		SourceTable:       "USysTableOfLists",
		SQL:               histProfile.SQL,
		SourceSchema:      ExpectedSchema(),
		ItemOrderEncoding: ItemOrderEncoding,
	}
	rows, err := SoilSnapshotRows(daoGzip, baseP)
	if err != nil {
		return nil, Provenance{}, err
	}
	profile := SoilBundledProfile2026()
	finalRows, err := renumberLists(rows, profile)
	if err != nil {
		return nil, Provenance{}, err
	}
	p, err := Bundled2026Provenance(finalRows, profile)
	return finalRows, p, err
}

// ParentBundled2026Rows verifies the 16 parent lists against LMH25 (2023), updating FloodingRegimeDur 'S'
// per LMH25 (2023) Table 2.19 ("Semi-permanent / Merely subirrigated during flood events").
func ParentBundled2026Rows(data, captureProvenance, manifest, bedrock []byte) ([]Choice, Provenance, error) {
	histProfile := ParentProfile()
	baseP := Provenance{
		Version:           1,
		Exporter:          histProfile.Exporter,
		SourceSHA256:      histProfile.SourceSHA256,
		SnapshotSHA256:    histProfile.SnapshotSHA256,
		DatabaseSHA256:    strings.Repeat("0", 64),
		TypedCellsSHA256:  strings.Repeat("0", 64),
		Rows:              347,
		Cells:             3470,
		SourceTable:       "USysTableOfLists",
		SQL:               histProfile.SQL,
		SourceSchema:      ExpectedSchema(),
		ItemOrderEncoding: ItemOrderEncoding,
	}
	rows, err := ParentSnapshotRows(data, captureProvenance, manifest, bedrock, baseP)
	if err != nil {
		return nil, Provenance{}, err
	}
	foundFloodS := false
	for i := range rows {
		if rows[i].ListName != nil && *rows[i].ListName == "FloodingRegimeDur" && rows[i].Code != nil && *rows[i].Code == "S" {
			rows[i].Description = strPtr("Semi-permanent / Merely subirrigated during flood events")
			foundFloodS = true
		}
	}
	if !foundFloodS {
		return nil, Provenance{}, fmt.Errorf("FloodingRegimeDur missing code S")
	}
	profile := ParentBundledProfile2026()
	finalRows, err := renumberLists(rows, profile)
	if err != nil {
		return nil, Provenance{}, err
	}
	p, err := Bundled2026Provenance(finalRows, profile)
	return finalRows, p, err
}

func Bundled2026Provenance(rows []Choice, profile Profile) (Provenance, error) {
	typedHash, err := TypedHash(rows)
	if err != nil {
		return Provenance{}, err
	}
	snapBytes, err := json.Marshal(rows)
	if err != nil {
		return Provenance{}, err
	}
	return Provenance{
		Version:           profile.Version,
		Exporter:          profile.Exporter,
		SourceSHA256:      profile.SourceSHA256,
		SnapshotSHA256:    checksum(snapBytes),
		DatabaseSHA256:    strings.Repeat("0", 64),
		TypedCellsSHA256:  typedHash,
		Rows:              len(rows),
		Cells:             len(rows) * 10,
		SourceTable:       "USysTableOfLists",
		SQL:               profile.SQL,
		SourceSchema:      ExpectedSchema(),
		ItemOrderEncoding: ItemOrderEncoding,
	}, nil
}
