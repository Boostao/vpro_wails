package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/boostao/vpro-wails/internal/accessreader"
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

type becSourceCol struct {
	Name    string `json:"name"`
	DAOType int    `json:"daoType"`
	Size    int    `json:"size"`
}

type becZoneRow struct {
	RowID           string  `json:"rowId"`
	Zone            *string `json:"zone"`
	SubZone         *string `json:"subZone"`
	ZoneDescription *string `json:"zoneDescription"`
	Description     *string `json:"description"`
}

type becSeriesRow struct {
	RowID            string   `json:"rowId"`
	SourceID         *string  `json:"sourceId"`
	SSCode           *string  `json:"ssCode"`
	Name             *string  `json:"name"`
	Region           *string  `json:"region"`
	Zone             *string  `json:"zone"`
	SubZone          *string  `json:"subZone"`
	BaseSubZone      *string  `json:"baseSubZone"`
	Variant          *string  `json:"variant"`
	Phase            *string  `json:"phase"`
	SiteSeries       *string  `json:"siteSeries"`
	SiteSeriesPhase  *string  `json:"siteSeriesPhase"`
	Variation        *string  `json:"variation"`
	Seral            *string  `json:"seral"`
	Description      *string  `json:"description"`
	PlantAssociation *string  `json:"plantAssociation"`
	Comments         *string  `json:"comments"`
	ReferenceID      *float64 `json:"referenceId"`
	AddedDate        *string  `json:"addedDate"`
	ExpiredDate      *string  `json:"expiredDate"`
	OriginalSourceID *string  `json:"originalSourceId"`
	MergedBGC        *string  `json:"mergedBGC"`
	Suballiance      *string  `json:"suballiance"`
	Alliance         *string  `json:"alliance"`
	MissingNpeNa     *string  `json:"missingNpeNa"`
	TransferID       *string  `json:"transferId"`
	Flag             *bool    `json:"flag"`
	Selectable       bool     `json:"selectable"`
	Diagnostic       string   `json:"diagnostic"`
}

type becSnapDoc struct {
	Version            int            `json:"version"`
	Exporter           string         `json:"exporter"`
	SourceAccessSHA256 string         `json:"sourceAccessSha256"`
	ZoneSchema         []becSourceCol `json:"zoneSchema"`
	SeriesSchema       []becSourceCol `json:"seriesSchema"`
	Zones              []becZoneRow   `json:"zones"`
	SiteSeries         []becSeriesRow `json:"siteSeries"`
}

type becProvDoc struct {
	Version        int    `json:"version"`
	Exporter       string `json:"exporter"`
	SourceSHA256   string `json:"sourceSha256"`
	SnapshotSHA256 string `json:"snapshotSha256"`
	DatabaseSHA256 string `json:"databaseSha256"`
	ZoneRows       int    `json:"zoneRows"`
	SiteSeriesRows int    `json:"siteSeriesRows"`
	DuplicateKeys  int    `json:"duplicateKeys"`
}

type wuCopy struct {
	Path string `json:"Path"`
	Hash string `json:"Hash"`
}

type wuSchema struct {
	Kind            string   `json:"kind"`
	Table           string   `json:"table"`
	Name            string   `json:"name"`
	Connect         string   `json:"connect"`
	SourceTable     string   `json:"sourceTable"`
	Type            int      `json:"type"`
	Size            int      `json:"size"`
	Required        bool     `json:"required"`
	AllowZeroLength bool     `json:"allowZeroLength"`
	Default         string   `json:"default"`
	Unique          bool     `json:"unique"`
	Primary         bool     `json:"primary"`
	Fields          []string `json:"fields"`
}

type wuRow struct {
	SourceOrdinal            string  `json:"sourceOrdinal"`
	ID                       *string `json:"id"`
	SiteSeries               *string `json:"siteSeries"`
	SiteSeriesLongName       *string `json:"siteSeriesLongName"`
	SiteSeriesScientificName *string `json:"siteSeriesScientificName"`
	Level                    *int    `json:"level"`
}

type wuSnapDoc struct {
	Version                       int                   `json:"version"`
	Exporter                      string                `json:"exporter"`
	ReadOnlyNativeReferenceAccess bool                  `json:"readOnlyNativeReferenceAccess"`
	SourceCopies                  []wuCopy              `json:"sourceCopies"`
	Schema                        map[string][]wuSchema `json:"schema"`
	Tables                        map[string][]wuRow    `json:"tables"`
	IdentityEncoding              string                `json:"identityEncoding"`
	SourceContext                 string                `json:"sourceContext"`
}

type wuProvDoc struct {
	Version          int                   `json:"version"`
	Exporter         string                `json:"exporter"`
	SnapshotSHA256   string                `json:"snapshotSha256"`
	DatabaseSHA256   string                `json:"databaseSha256"`
	MasterSHA256     string                `json:"masterSha256"`
	UserSHA256       string                `json:"userSha256"`
	MasterRows       int                   `json:"masterRows"`
	UserRows         int                   `json:"userRows"`
	MasterDuplicates int                   `json:"masterDuplicates"`
	UserDuplicates   int                   `json:"userDuplicates"`
	TypedCellsSHA256 string                `json:"typedCellsSha256"`
	SourceCopies     []wuCopy              `json:"sourceCopies"`
	SourceSchema     map[string][]wuSchema `json:"sourceSchema"`
	IdentityEncoding string                `json:"identityEncoding"`
	SourceContext    string                `json:"sourceContext"`
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func optStr(v any) *string {
	if v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return &s
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func truncateUTF16(p *string, maxUnits int) *string {
	if p == nil {
		return nil
	}
	units := utf16.Encode([]rune(*p))
	if len(units) <= maxUnits {
		return p
	}
	runes := utf16.Decode(units[:maxUnits])
	s := string(runes)
	return &s
}

func formatSubZoneDesc(subName, varName, phaseName *string) *string {
	parts := []string{}
	if varName != nil && strings.TrimSpace(*varName) != "" {
		parts = append(parts, strings.TrimSpace(*varName))
	}
	if subName != nil && strings.TrimSpace(*subName) != "" {
		parts = append(parts, strings.TrimSpace(*subName))
	}
	desc := strings.Join(parts, " ")
	if phaseName != nil && strings.TrimSpace(*phaseName) != "" {
		if desc != "" {
			desc += ", " + strings.TrimSpace(*phaseName) + " Phase"
		} else {
			desc = strings.TrimSpace(*phaseName) + " Phase"
		}
	}
	if desc == "" {
		return nil
	}
	return truncateUTF16(&desc, 80)
}

func buildBECSchemas() ([]becSourceCol, []becSourceCol) {
	zones := []becSourceCol{{"Zone", 10, 4}, {"SubZone", 10, 10}, {"ZoneDescription", 10, 50}, {"SubZoneVarDescription", 10, 80}}
	names := []string{"SiteSeries_ID", "SSCode", "BGCName", "BEC_Region", "BGC_Zone", "SubzVarPh", "BGC_Subzone", "BGC_Variant",
		"BGC_Phase", "SiteSeries", "SSPhase", "SSVariation", "Seral", "SiteSeriesDescription", "PlantAssociation", "Comments",
		"Ref_ID", "AddedDate", "ExpiredDate", "O_SS_ID", "MergedBGC_SS", "BECSuballiance", "BECAlliance", "MissingNpeNa", "TransferID", "Flag"}
	series := make([]becSourceCol, len(names))
	for i, name := range names {
		series[i] = becSourceCol{name, 10, 255}
		switch name {
		case "SiteSeries_ID", "Ref_ID", "O_SS_ID":
			series[i].DAOType, series[i].Size = 7, 8
		case "AddedDate":
			series[i].DAOType, series[i].Size = 8, 8
		case "Flag":
			series[i].DAOType, series[i].Size = 1, 1
		}
	}
	return zones, series
}

func loadExistingBEC(becDBPath string) ([]becZoneRow, []becSeriesRow, error) {
	db, err := sqlutil.OpenReadOnly(becDBPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	zRows, err := db.Query(`SELECT RowID, Zone, SubZone, ZoneDescription, Description FROM BECZoneList ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		return nil, nil, err
	}
	var zones []becZoneRow
	for zRows.Next() {
		var r becZoneRow
		if err := zRows.Scan(&r.RowID, &r.Zone, &r.SubZone, &r.ZoneDescription, &r.Description); err != nil {
			zRows.Close()
			return nil, nil, err
		}
		zones = append(zones, r)
	}
	zRows.Close()

	sRows, err := db.Query(`SELECT RowID, SourceID, SSCode, Name, Region, Zone, SubZone, BaseSubZone, Variant, Phase, SiteSeries,
		SiteSeriesPhase, Variation, Seral, Description, PlantAssociation, Comments, ReferenceID, AddedDate, ExpiredDate,
		OriginalSourceID, MergedBGC, Suballiance, Alliance, MissingNpeNa, TransferID, Flag
		FROM BECSiteSeries ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		return nil, nil, err
	}
	defer sRows.Close()
	var series []becSeriesRow
	for sRows.Next() {
		var r becSeriesRow
		if err := sRows.Scan(&r.RowID, &r.SourceID, &r.SSCode, &r.Name, &r.Region, &r.Zone, &r.SubZone,
			&r.BaseSubZone, &r.Variant, &r.Phase, &r.SiteSeries, &r.SiteSeriesPhase, &r.Variation,
			&r.Seral, &r.Description, &r.PlantAssociation, &r.Comments, &r.ReferenceID, &r.AddedDate,
			&r.ExpiredDate, &r.OriginalSourceID, &r.MergedBGC, &r.Suballiance, &r.Alliance,
			&r.MissingNpeNa, &r.TransferID, &r.Flag); err != nil {
			return nil, nil, err
		}
		series = append(series, r)
	}
	return zones, series, sRows.Err()
}

func buildBECv13Data(becAccdb *accessreader.DB, becAccdbSHA, existingBECPath string) ([]becZoneRow, []becSeriesRow, becProvDoc, error) {
	histZones, histSeries, err := loadExistingBEC(existingBECPath)
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}

	_, zonesTbl, err := becAccdb.ReadAll("tblBGC_Zones")
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}
	zoneNames := map[string]string{}
	zonePrefixes := []string{"AT", "FG", "MG"}
	for _, r := range zonesTbl {
		z := strVal(optStr(r[0]))
		zn := strings.Join(strings.Fields(strVal(optStr(r[1]))), " ")
		if z != "" {
			zoneNames[strings.ToUpper(z)] = zn
			zonePrefixes = append(zonePrefixes, z)
		}
	}

	_, curBGC, err := becAccdb.ReadAll("Current_BGCunits_v13")
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}
	_, retBGC, err := becAccdb.ReadAll("Retired_BGCunits_v13")
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}

	// Build merged zone/subzone map keyed case-insensitively by (Zone, SubZone)
	zoneMap := map[string]becZoneRow{}
	for _, hz := range histZones {
		k := strings.ToUpper(strVal(hz.Zone)) + "|" + strings.ToLower(strVal(hz.SubZone))
		zoneMap[k] = hz
	}
	for _, r := range curBGC {
		z := strVal(optStr(r[9]))
		sz := strVal(optStr(r[10]))
		v := strVal(optStr(r[11]))
		ph := strVal(optStr(r[12]))
		sub := sz + v + ph
		zDesc := zoneNames[strings.ToUpper(z)]
		if zDesc == "" {
			zDesc = strings.Join(strings.Fields(strVal(optStr(r[5]))), " ")
		}
		zPtr := truncateUTF16(&z, 4)
		subPtr := truncateUTF16(&sub, 10)
		zDescPtr := truncateUTF16(&zDesc, 50)
		descPtr := formatSubZoneDesc(optStr(r[6]), optStr(r[7]), optStr(r[8]))
		k := strings.ToUpper(z) + "|" + strings.ToLower(sub)
		zoneMap[k] = becZoneRow{
			Zone:            zPtr,
			SubZone:         subPtr,
			ZoneDescription: zDescPtr,
			Description:     descPtr,
		}
	}
	for _, r := range retBGC {
		ns := strVal(optStr(r[1]))
		matchedZ, matchedSub := "", ""
		for _, zp := range zonePrefixes {
			if strings.HasPrefix(ns, zp) && len(zp) > len(matchedZ) {
				matchedZ = zp
				matchedSub = ns[len(zp):]
			}
		}
		if matchedZ == "" {
			continue
		}
		k := strings.ToUpper(matchedZ) + "|" + strings.ToLower(matchedSub)
		if _, exists := zoneMap[k]; !exists {
			zDesc := zoneNames[strings.ToUpper(matchedZ)]
			if zDesc == "" {
				zDesc = strings.Join(strings.Fields(strVal(optStr(r[3]))), " ")
			}
			zCopy, subCopy, zDescCopy := matchedZ, matchedSub, zDesc
			zoneMap[k] = becZoneRow{
				Zone:            truncateUTF16(&zCopy, 4),
				SubZone:         truncateUTF16(&subCopy, 10),
				ZoneDescription: truncateUTF16(&zDescCopy, 50),
				Description:     formatSubZoneDesc(optStr(r[4]), optStr(r[5]), optStr(r[6])),
			}
		}
	}

	mergedZones := make([]becZoneRow, 0, len(zoneMap))
	for _, zr := range zoneMap {
		if canon, ok := zoneNames[strings.ToUpper(strVal(zr.Zone))]; ok && canon != "" {
			canonCopy := canon
			zr.ZoneDescription = truncateUTF16(&canonCopy, 50)
		}
		mergedZones = append(mergedZones, zr)
	}
	sort.Slice(mergedZones, func(i, j int) bool {
		zi, zj := strVal(mergedZones[i].Zone), strVal(mergedZones[j].Zone)
		if !strings.EqualFold(zi, zj) {
			return strings.ToLower(zi) < strings.ToLower(zj)
		}
		if zi != zj {
			return zi < zj
		}
		si, sj := strVal(mergedZones[i].SubZone), strVal(mergedZones[j].SubZone)
		if !strings.EqualFold(si, sj) {
			return strings.ToLower(si) < strings.ToLower(sj)
		}
		return si < sj
	})
	for i := range mergedZones {
		mergedZones[i].RowID = strconv.Itoa(i + 1)
	}

	// Build tblBEC_References map
	_, refsTbl, err := becAccdb.ReadAll("tblBEC_References")
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}
	refByShort := map[string]float64{}
	for _, r := range refsTbl {
		short := strings.TrimSpace(strVal(optStr(r[1])))
		if short == "" {
			continue
		}
		if id, ok := r[9].(int64); ok {
			refByShort[short] = float64(id)
		} else if idAuto, ok := r[0].(int64); ok {
			refByShort[short] = float64(idAuto)
		}
	}

	_, curSU, err := becAccdb.ReadAll("Current_SiteUnits_v13")
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}
	_, retSU, err := becAccdb.ReadAll("Retired_SiteUnits_v13")
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}
	retiredIDs := map[string]int64{}
	for _, r := range retSU {
		if cid, ok := r[6].(int64); ok {
			ver, _ := r[7].(int64)
			retiredIDs[strconv.FormatInt(cid, 10)] = ver
		}
	}

	histBySourceID := map[string]becSeriesRow{}
	for _, hs := range histSeries {
		if hs.SourceID != nil {
			histBySourceID[*hs.SourceID] = hs
		}
	}

	curBySourceID := map[string]becSeriesRow{}
	seenCatIDs := map[string]int{}
	for idx, r := range curSU {
		var srcID string
		if cid, ok := r[17].(int64); ok {
			srcID = strconv.FormatInt(cid, 10)
		} else {
			srcID = strconv.Itoa(1300000 + idx + 1)
		}
		if seenCatIDs[srcID] > 0 {
			seenCatIDs[srcID]++
			srcID = fmt.Sprintf("%s.%d", srcID, seenCatIDs[srcID])
		} else {
			seenCatIDs[srcID] = 1
		}

		suNoSpace := strVal(optStr(r[1]))
		var ssCodePtr *string
		if slash := strings.IndexByte(suNoSpace, '/'); slash >= 0 {
			s := suNoSpace[slash+1:]
			ssCodePtr = &s
		} else if s := strVal(optStr(r[12])); s != "" {
			ssCodePtr = &s
		}

		suLabel := strVal(optStr(r[3]))
		var bgcNamePtr *string
		if slash := strings.IndexByte(suLabel, '/'); slash >= 0 {
			b := strings.TrimRight(suLabel[:slash], " ")
			bgcNamePtr = &b
		}

		zPtr := optStr(r[8])
		baseSubPtr := optStr(r[9])
		varPtr := optStr(r[10])
		phasePtr := optStr(r[11])
		subCombined := strVal(baseSubPtr) + strVal(varPtr) + strVal(phasePtr)
		var subCombinedPtr *string
		if subCombined != "" {
			subCombinedPtr = &subCombined
		}

		var variationPtr *string
		if vInt, ok := r[14].(int64); ok {
			vs := strconv.FormatInt(vInt, 10)
			variationPtr = &vs
		}

		var refIDPtr *float64
		if refShort := strings.TrimSpace(strVal(optStr(r[6]))); refShort != "" {
			if rid, ok := refByShort[refShort]; ok {
				ridCopy := rid
				refIDPtr = &ridCopy
			}
		}

		var addedPtr *string
		if dt, ok := r[16].(time.Time); ok && !dt.IsZero() {
			ds := dt.UTC().Format("2006-01-02T15:04:05")
			addedPtr = &ds
		}

		flagFalse := false
		srcCopy := srcID
		mergedCopy := suLabel
		var mergedPtr *string
		if mergedCopy != "" {
			mergedPtr = &mergedCopy
		}

		row := becSeriesRow{
			SourceID:         &srcCopy,
			SSCode:           truncateUTF16(ssCodePtr, 255),
			Name:             truncateUTF16(bgcNamePtr, 255),
			Region:           truncateUTF16(optStr(r[7]), 255),
			Zone:             truncateUTF16(zPtr, 255),
			SubZone:          truncateUTF16(subCombinedPtr, 255),
			BaseSubZone:      truncateUTF16(baseSubPtr, 255),
			Variant:          truncateUTF16(varPtr, 255),
			Phase:            truncateUTF16(phasePtr, 255),
			SiteSeries:       truncateUTF16(optStr(r[12]), 255),
			SiteSeriesPhase:  truncateUTF16(optStr(r[13]), 255),
			Variation:        variationPtr,
			Seral:            truncateUTF16(optStr(r[15]), 255),
			Description:      truncateUTF16(optStr(r[4]), 255),
			PlantAssociation: truncateUTF16(optStr(r[5]), 255),
			ReferenceID:      refIDPtr,
			AddedDate:        addedPtr,
			OriginalSourceID: &srcCopy,
			MergedBGC:        truncateUTF16(mergedPtr, 255),
			Flag:             &flagFalse,
		}
		if prev, hasPrev := histBySourceID[srcID]; hasPrev {
			row.Comments = prev.Comments
			row.OriginalSourceID = prev.OriginalSourceID
			row.Suballiance = prev.Suballiance
			row.Alliance = prev.Alliance
			row.MissingNpeNa = prev.MissingNpeNa
			row.TransferID = prev.TransferID
			if prev.Flag != nil {
				row.Flag = prev.Flag
			}
		}
		curBySourceID[srcID] = row
	}

	// Merge: keep all current v13 rows + retain historical rows whose SourceID is not in Current_SiteUnits_v13
	// (excluding any synthetic 1300000+ IDs if re-run idempotently).
	mergedSeries := make([]becSeriesRow, 0, len(curBySourceID)+len(histSeries))
	for _, r := range curBySourceID {
		mergedSeries = append(mergedSeries, r)
	}
	for _, hs := range histSeries {
		sid := strVal(hs.SourceID)
		if _, inCur := curBySourceID[sid]; inCur {
			continue
		}
		if ver, isRet := retiredIDs[sid]; isRet && (hs.ExpiredDate == nil || *hs.ExpiredDate == "") {
			exp := fmt.Sprintf("BECv%d", ver)
			hs.ExpiredDate = &exp
		}
		hs.Selectable = false
		hs.Diagnostic = ""
		mergedSeries = append(mergedSeries, hs)
	}

	sort.Slice(mergedSeries, func(i, j int) bool {
		a, b := mergedSeries[i], mergedSeries[j]
		for _, pair := range [][2]string{
			{strVal(a.Zone), strVal(b.Zone)},
			{strVal(a.SubZone), strVal(b.SubZone)},
			{strVal(a.SiteSeries), strVal(b.SiteSeries)},
			{strVal(a.SSCode), strVal(b.SSCode)},
			{strVal(a.Region), strVal(b.Region)},
			{strVal(a.SourceID), strVal(b.SourceID)},
		} {
			if !strings.EqualFold(pair[0], pair[1]) {
				return strings.ToLower(pair[0]) < strings.ToLower(pair[1])
			}
			if pair[0] != pair[1] {
				return pair[0] < pair[1]
			}
		}
		return false
	})

	for i := range mergedSeries {
		mergedSeries[i].RowID = strconv.Itoa(i + 1)
		mergedSeries[i].Selectable = false
		mergedSeries[i].Diagnostic = ""
	}

	zSchema, sSchema := buildBECSchemas()
	snap := becSnapDoc{
		Version:            2,
		Exporter:           "accessreader-becdb-v13-2026",
		SourceAccessSHA256: becAccdbSHA,
		ZoneSchema:         zSchema,
		SeriesSchema:       sSchema,
		Zones:              mergedZones,
		SiteSeries:         mergedSeries,
	}
	snapBytes, err := json.Marshal(snap)
	if err != nil {
		return nil, nil, becProvDoc{}, err
	}
	prov := becProvDoc{
		Version:        2,
		Exporter:       "accessreader-becdb-v13-2026",
		SourceSHA256:   becAccdbSHA,
		SnapshotSHA256: sha256Hex(snapBytes),
		ZoneRows:       len(mergedZones),
		SiteSeriesRows: len(mergedSeries),
	}
	return mergedZones, mergedSeries, prov, nil
}

func writeBECDatabase(dbPath, provPath string, zones []becZoneRow, series []becSeriesRow, prov becProvDoc) (becProvDoc, error) {
	tmpFile, err := os.CreateTemp(filepath.Dir(dbPath), ".bec2026-*.db")
	if err != nil {
		return prov, err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	db, err := sql.Open("sqlite3", tmpPath)
	if err != nil {
		return prov, err
	}
	db.SetMaxOpenConns(1)

	schemaSQL := `
CREATE TABLE BECZoneList (RowID TEXT PRIMARY KEY NOT NULL, Zone TEXT, SubZone TEXT, ZoneDescription TEXT, Description TEXT);
CREATE TABLE BECSiteSeries (
 RowID TEXT PRIMARY KEY NOT NULL, SourceID TEXT, SSCode TEXT, Name TEXT, Region TEXT, Zone TEXT, SubZone TEXT,
 BaseSubZone TEXT, Variant TEXT, Phase TEXT, SiteSeries TEXT, SiteSeriesPhase TEXT, Variation TEXT, Seral TEXT,
 Description TEXT, PlantAssociation TEXT, Comments TEXT, ReferenceID REAL, AddedDate TEXT, ExpiredDate TEXT,
 OriginalSourceID TEXT, MergedBGC TEXT, Suballiance TEXT, Alliance TEXT, MissingNpeNa TEXT, TransferID TEXT,
 Flag BOOLEAN CHECK (Flag IS NULL OR Flag IN (0,1)));
CREATE INDEX BECZoneLookup ON BECZoneList(Zone COLLATE NOCASE, SubZone COLLATE NOCASE);
CREATE INDEX BECSiteSeriesLookup ON BECSiteSeries(Zone COLLATE NOCASE, SubZone COLLATE NOCASE);
CREATE TABLE BECProvenance (Version INTEGER NOT NULL, Exporter TEXT NOT NULL, SourceSHA256 TEXT NOT NULL, SnapshotSHA256 TEXT NOT NULL);
`
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return prov, err
	}
	if _, err := tx.Exec(schemaSQL); err != nil {
		tx.Rollback()
		db.Close()
		return prov, err
	}
	zStmt, err := tx.Prepare(`INSERT INTO BECZoneList VALUES (?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return prov, err
	}
	for _, r := range zones {
		if _, err := zStmt.Exec(r.RowID, r.Zone, r.SubZone, r.ZoneDescription, r.Description); err != nil {
			zStmt.Close()
			tx.Rollback()
			db.Close()
			return prov, err
		}
	}
	zStmt.Close()

	sStmt, err := tx.Prepare(`INSERT INTO BECSiteSeries VALUES (` + strings.TrimSuffix(strings.Repeat("?,", 27), ",") + `)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return prov, err
	}
	for _, r := range series {
		v := reflect.ValueOf(r)
		args := make([]any, 27)
		for i := range args {
			args[i] = v.Field(i).Interface()
		}
		if _, err := sStmt.Exec(args...); err != nil {
			sStmt.Close()
			tx.Rollback()
			db.Close()
			return prov, err
		}
	}
	sStmt.Close()

	if _, err := tx.Exec(`INSERT INTO BECProvenance VALUES (?,?,?,?)`,
		prov.Version, prov.Exporter, prov.SourceSHA256, prov.SnapshotSHA256); err != nil {
		tx.Rollback()
		db.Close()
		return prov, err
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM (SELECT Zone,SubZone,SiteSeries FROM BECSiteSeries GROUP BY Zone,SubZone,SiteSeries HAVING COUNT(*)>1)`).Scan(&prov.DuplicateKeys); err != nil {
		tx.Rollback()
		db.Close()
		return prov, err
	}
	if err := tx.Commit(); err != nil {
		db.Close()
		return prov, err
	}
	if _, err := db.Exec(`VACUUM`); err != nil {
		db.Close()
		return prov, err
	}
	if err := db.Close(); err != nil {
		return prov, err
	}

	rawDB, err := os.ReadFile(tmpPath)
	if err != nil {
		return prov, err
	}
	prov.DatabaseSHA256 = sha256Hex(rawDB)
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return prov, err
	}
	if err := os.Rename(tmpPath, dbPath); err != nil {
		return prov, err
	}

	provBytes, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return prov, err
	}
	provBytes = append(provBytes, '\n')
	if err := atomicWriteFile(provPath, provBytes, 0644); err != nil {
		return prov, err
	}
	return prov, nil
}

func buildAndWriteWorkingUnitv13(becAccdb *accessreader.DB, becAccdbSHA, existingWUPath, existingWUProvPath, outDBPath, outProvPath string) ([]wuRow, wuProvDoc, error) {
	rawProv, err := os.ReadFile(existingWUProvPath)
	if err != nil {
		return nil, wuProvDoc{}, err
	}
	var oldProv wuProvDoc
	dec := json.NewDecoder(bytes.NewReader(rawProv))
	if err := dec.Decode(&oldProv); err != nil {
		return nil, wuProvDoc{}, err
	}

	oldDB, err := sqlutil.OpenReadOnly(existingWUPath)
	if err != nil {
		return nil, wuProvDoc{}, err
	}

	// Load historical master rows (excluding reserved 13000001..13999999 so re-runs are idempotent!)
	mRows, err := oldDB.Query(`SELECT RowID, SourceID, Code, Description, ScientificName, Level
		FROM WorkingUnits WHERE Origin='master' ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		oldDB.Close()
		return nil, wuProvDoc{}, err
	}
	var masterRows []wuRow
	histLabels := map[string]int{} // SU_Label -> index in masterRows
	for mRows.Next() {
		var r wuRow
		if err := mRows.Scan(&r.SourceOrdinal, &r.ID, &r.SiteSeries, &r.SiteSeriesLongName, &r.SiteSeriesScientificName, &r.Level); err != nil {
			mRows.Close()
			oldDB.Close()
			return nil, wuProvDoc{}, err
		}
		if r.ID != nil {
			idVal, _ := strconv.ParseInt(*r.ID, 10, 64)
			if idVal >= 13000001 && idVal <= 13999999 {
				continue
			}
		}
		idx := len(masterRows)
		masterRows = append(masterRows, r)
		if r.SiteSeries != nil {
			if _, seen := histLabels[*r.SiteSeries]; !seen {
				histLabels[*r.SiteSeries] = idx
			}
		}
	}
	mRows.Close()

	uRows, err := oldDB.Query(`SELECT RowID, SourceID, Code, Description, ScientificName, Level
		FROM WorkingUnits WHERE Origin='user' ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		oldDB.Close()
		return nil, wuProvDoc{}, err
	}
	var userRows []wuRow
	for uRows.Next() {
		var r wuRow
		if err := uRows.Scan(&r.SourceOrdinal, &r.ID, &r.SiteSeries, &r.SiteSeriesLongName, &r.SiteSeriesScientificName, &r.Level); err != nil {
			uRows.Close()
			oldDB.Close()
			return nil, wuProvDoc{}, err
		}
		userRows = append(userRows, r)
	}
	uRows.Close()
	if err := oldDB.Close(); err != nil {
		return nil, wuProvDoc{}, err
	}

	_, uniqSU, err := becAccdb.ReadAll("Current_Unique_SiteUnits_v13")
	if err != nil {
		return nil, wuProvDoc{}, err
	}

	type uniqCandidate struct {
		label, noSpace string
		longName, sci  *string
	}
	candidates := make([]uniqCandidate, 0, len(uniqSU))
	for _, r := range uniqSU {
		lbl := strVal(optStr(r[3]))
		ns := strVal(optStr(r[1]))
		if lbl == "" {
			continue
		}
		candidates = append(candidates, uniqCandidate{
			label:    lbl,
			noSpace:  ns,
			longName: truncateUTF16(optStr(r[4]), 120),
			sci:      truncateUTF16(optStr(r[5]), 255),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].label != candidates[j].label {
			return candidates[i].label < candidates[j].label
		}
		return candidates[i].noSpace < candidates[j].noSpace
	})

	nextID := int64(13000001)
	seenNewLabels := map[string]bool{}
	lvl11 := 11
	for _, c := range candidates {
		if idx, exists := histLabels[c.label]; exists {
			if c.longName != nil {
				masterRows[idx].SiteSeriesLongName = c.longName
			}
			if c.sci != nil {
				masterRows[idx].SiteSeriesScientificName = c.sci
			}
			continue
		}
		if seenNewLabels[c.label] {
			continue
		}
		seenNewLabels[c.label] = true
		idStr := strconv.FormatInt(nextID, 10)
		nextID++
		lblCopy := c.label
		masterRows = append(masterRows, wuRow{
			ID:                       &idStr,
			SiteSeries:               truncateUTF16(&lblCopy, 50),
			SiteSeriesLongName:       c.longName,
			SiteSeriesScientificName: c.sci,
			Level:                    &lvl11,
		})
	}

	for i := range masterRows {
		masterRows[i].SourceOrdinal = strconv.Itoa(i + 1)
	}
	for i := range userRows {
		userRows[i].SourceOrdinal = strconv.Itoa(i + 1)
	}

	// Build sourceCopies ensuring BECdb_v13_2026.accdb and VUser.accda are recorded with portable basenames
	copies := []wuCopy{
		{Path: "BECdb_v13_2026.accdb", Hash: strings.ToUpper(becAccdbSHA)},
	}
	seenCopy := map[string]bool{"becdb_v13_2026.accdb": true}
	for _, sc := range oldProv.SourceCopies {
		portableName := filepath.Base(strings.ReplaceAll(sc.Path, `\`, `/`))
		base := strings.ToLower(portableName)
		if seenCopy[base] {
			continue
		}
		seenCopy[base] = true
		copies = append(copies, wuCopy{Path: portableName, Hash: sc.Hash})
	}

	tables := map[string][]wuRow{
		"MasterSiteUnitList": masterRows,
		"UserSiteUnitList":   userRows,
	}
	cellsJSON, err := json.Marshal(tables)
	if err != nil {
		return nil, wuProvDoc{}, err
	}

	srcContext := "BECdb_v13_2026.accdb Current_Unique_SiteUnits_v13 merged with historical VLists.MasterSiteUnitList; VUser.UserSiteUnitList preserved"
	snap := wuSnapDoc{
		Version:                       2,
		Exporter:                      "accessreader-becdb-v13-working-unit-v2",
		ReadOnlyNativeReferenceAccess: true,
		SourceCopies:                  copies,
		Schema:                        oldProv.SourceSchema,
		Tables:                        tables,
		IdentityEncoding:              oldProv.IdentityEncoding,
		SourceContext:                 srcContext,
	}
	snapBytes, err := json.Marshal(snap)
	if err != nil {
		return nil, wuProvDoc{}, err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(outDBPath), ".wu2026-*.db")
	if err != nil {
		return nil, wuProvDoc{}, err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	db, err := sql.Open("sqlite3", tmpPath)
	if err != nil {
		return nil, wuProvDoc{}, err
	}
	db.SetMaxOpenConns(1)

	wuSchemaSQL := `
CREATE TABLE WorkingUnits(
 Origin TEXT NOT NULL CHECK(Origin IN ('master','user')),
 RowID TEXT NOT NULL,
 SourceID TEXT,
 Code TEXT,
 Description TEXT,
 ScientificName TEXT,
 Level INTEGER CHECK(Level IS NULL OR Level BETWEEN -32768 AND 32767),
 PRIMARY KEY(Origin,RowID), UNIQUE(Origin,SourceID));
CREATE INDEX WorkingUnitChoices ON WorkingUnits(Origin,Level,Code COLLATE NOCASE);
CREATE TABLE WorkingUnitProvenance(Version INTEGER NOT NULL, Exporter TEXT NOT NULL,
 SnapshotSHA256 TEXT NOT NULL, MasterSHA256 TEXT NOT NULL, UserSHA256 TEXT NOT NULL);
`
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, wuProvDoc{}, err
	}
	if _, err := tx.Exec(wuSchemaSQL); err != nil {
		tx.Rollback()
		db.Close()
		return nil, wuProvDoc{}, err
	}
	stmt, err := tx.Prepare(`INSERT INTO WorkingUnits VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, wuProvDoc{}, err
	}
	for _, r := range masterRows {
		if _, err := stmt.Exec("master", r.SourceOrdinal, r.ID, r.SiteSeries, r.SiteSeriesLongName, r.SiteSeriesScientificName, r.Level); err != nil {
			stmt.Close()
			tx.Rollback()
			db.Close()
			return nil, wuProvDoc{}, err
		}
	}
	for _, r := range userRows {
		if _, err := stmt.Exec("user", r.SourceOrdinal, r.ID, r.SiteSeries, r.SiteSeriesLongName, r.SiteSeriesScientificName, r.Level); err != nil {
			stmt.Close()
			tx.Rollback()
			db.Close()
			return nil, wuProvDoc{}, err
		}
	}
	stmt.Close()

	snapSHA := sha256Hex(snapBytes)
	if _, err := tx.Exec(`INSERT INTO WorkingUnitProvenance VALUES(?,?,?,?,?)`,
		2, "accessreader-becdb-v13-working-unit-v2", snapSHA, becAccdbSHA, oldProv.UserSHA256); err != nil {
		tx.Rollback()
		db.Close()
		return nil, wuProvDoc{}, err
	}
	var masterDups, userDups int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM (SELECT Code FROM WorkingUnits WHERE Origin='master' GROUP BY Code HAVING COUNT(*)>1)`).Scan(&masterDups); err != nil {
		tx.Rollback()
		db.Close()
		return nil, wuProvDoc{}, err
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM (SELECT Code FROM WorkingUnits WHERE Origin='user' GROUP BY Code HAVING COUNT(*)>1)`).Scan(&userDups); err != nil {
		tx.Rollback()
		db.Close()
		return nil, wuProvDoc{}, err
	}
	if err := tx.Commit(); err != nil {
		db.Close()
		return nil, wuProvDoc{}, err
	}
	if _, err := db.Exec(`VACUUM`); err != nil {
		db.Close()
		return nil, wuProvDoc{}, err
	}
	if err := db.Close(); err != nil {
		return nil, wuProvDoc{}, err
	}

	rawDB, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, wuProvDoc{}, err
	}
	newProv := wuProvDoc{
		Version:          2,
		Exporter:         "accessreader-becdb-v13-working-unit-v2",
		SnapshotSHA256:   snapSHA,
		DatabaseSHA256:   sha256Hex(rawDB),
		MasterSHA256:     becAccdbSHA,
		UserSHA256:       oldProv.UserSHA256,
		MasterRows:       len(masterRows),
		UserRows:         len(userRows),
		MasterDuplicates: masterDups,
		UserDuplicates:   userDups,
		TypedCellsSHA256: sha256Hex(cellsJSON),
		SourceCopies:     copies,
		SourceSchema:     oldProv.SourceSchema,
		IdentityEncoding: oldProv.IdentityEncoding,
		SourceContext:    srcContext,
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return nil, wuProvDoc{}, err
	}
	if err := os.Rename(tmpPath, outDBPath); err != nil {
		return nil, wuProvDoc{}, err
	}
	provBytes, err := json.MarshalIndent(newProv, "", "  ")
	if err != nil {
		return nil, wuProvDoc{}, err
	}
	provBytes = append(provBytes, '\n')
	if err := atomicWriteFile(outProvPath, provBytes, 0644); err != nil {
		return nil, wuProvDoc{}, err
	}
	return masterRows, newProv, nil
}
