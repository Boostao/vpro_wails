package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/boostao/vpro-wails/internal/accessreader"
)

type specsRow struct {
	YearOfChange        *string
	ChangeDescription   *string
	APCFamily           *string
	FamilyCode          *string
	OldCode             *string
	Code                *string
	SplCode             *string
	TreeCode            *string
	ScientificName      *string
	Lifeform            *int64
	Authority           *string
	EnglishNameOld      *string
	EnglishName         *string
	CommonNamePB        *string
	NotesOnNames        *string
	ShortenedGuideName  *string
	Native              *string
	Codetype            *string
	Province            *string
	ID                  *int64
	TaxonLevel          *string
	IsAttribute         int64
	Report              *int64
	SplCodeCalc         *string
	Codelength          *int64
	CombinedEnglishName *string
}

func deriveBECSynonymOldCode(sci string, targetCode string, usedOldCodes map[string]bool) string {
	fields := strings.Fields(sci)
	if len(fields) == 0 {
		return strings.ToUpper(targetCode)
	}
	lettersOnly := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if unicode.IsLetter(r) {
				b.WriteRune(unicode.ToUpper(r))
			}
		}
		return b.String()
	}
	genus := lettersOnly(fields[0])
	if len(genus) > 4 {
		genus = genus[:4]
	}
	species := ""
	if len(fields) >= 2 {
		ep := fields[1]
		if strings.EqualFold(ep, "x") && len(fields) >= 3 {
			species = "X" + lettersOnly(fields[2])
		} else {
			species = lettersOnly(ep)
		}
	}
	if len(species) > 3 {
		species = species[:3]
	}
	base := genus + species
	if base == "" {
		base = strings.ToUpper(targetCode)
	}
	// Check if infraspecific (ssp. / subsp. / var.)
	isInfra := false
	for _, f := range fields {
		lf := strings.ToLower(strings.TrimSuffix(f, "."))
		if lf == "ssp" || lf == "subsp" || lf == "var" {
			isInfra = true
			break
		}
	}
	if !isInfra && !strings.EqualFold(base, targetCode) && !usedOldCodes[base] {
		return base
	}
	if !isInfra && !strings.EqualFold(base, targetCode) {
		return base
	}
	for n := 1; n <= 9; n++ {
		cand := base + strconv.Itoa(n)
		if len(cand) <= 8 && !strings.EqualFold(cand, targetCode) && !usedOldCodes[cand] {
			return cand
		}
	}
	return base
}

func genusKey(sci string) string {
	fields := strings.Fields(sci)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}

func normSciKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func mergeTaxonomy2025(taxaAccdb *accessreader.DB, vlistsDB *sql.DB) ([]specsRow, error) {
	// Load historical USysAllSpecs (excluding newly minted 19000001+ IDs so re-runs are 100% idempotent!)
	rows, err := vlistsDB.Query(`SELECT
		YearOfChange, ChangeDescription, APCFamily, FamilyCode, OldCode, Code, SplCode, TreeCode,
		ScientificName, Lifeform, Authority, EnglishName_Old, EnglishName, common_name_pb,
		Notes_on_names, ShortenedGuideName, Native, Codetype, Province, ID, TaxonLevel,
		COALESCE(IsAttribute, 0), Report, SplCode_calc, Codelength, CombinedEnglishName
		FROM USysAllSpecs
		ORDER BY COALESCE(ID, 0), COALESCE(OldCode, ''), COALESCE(Code, ''), COALESCE(ScientificName, '')`)
	if err != nil {
		return nil, err
	}
	var hist []specsRow
	byCodeNonSyn := map[string]int{}
	bySci := map[string][]int{}
	genusFamily := map[string][2]*string{} // full lowercase genus word -> [APCFamily, FamilyCode]
	usedOldCodes := map[string]bool{}

	for rows.Next() {
		var r specsRow
		if err := rows.Scan(
			&r.YearOfChange, &r.ChangeDescription, &r.APCFamily, &r.FamilyCode, &r.OldCode, &r.Code, &r.SplCode, &r.TreeCode,
			&r.ScientificName, &r.Lifeform, &r.Authority, &r.EnglishNameOld, &r.EnglishName, &r.CommonNamePB,
			&r.NotesOnNames, &r.ShortenedGuideName, &r.Native, &r.Codetype, &r.Province, &r.ID, &r.TaxonLevel,
			&r.IsAttribute, &r.Report, &r.SplCodeCalc, &r.Codelength, &r.CombinedEnglishName,
		); err != nil {
			rows.Close()
			return nil, err
		}
		if r.ID != nil && *r.ID >= 19000001 {
			continue
		}
		idx := len(hist)
		hist = append(hist, r)
		if r.OldCode != nil && *r.OldCode != "" {
			usedOldCodes[strings.ToUpper(*r.OldCode)] = true
		}
		if r.Code != nil && *r.Code != "" && !strings.EqualFold(strVal(r.Codetype), "S") {
			cu := strings.ToUpper(*r.Code)
			if prevIdx, seen := byCodeNonSyn[cu]; !seen || (!strings.EqualFold(strVal(hist[prevIdx].Codetype), "U") && strings.EqualFold(strVal(r.Codetype), "U")) {
				byCodeNonSyn[cu] = idx
			}
		}
		if r.ScientificName != nil && *r.ScientificName != "" {
			sk := normSciKey(*r.ScientificName)
			bySci[sk] = append(bySci[sk], idx)
			if gk := genusKey(*r.ScientificName); gk != "" && r.APCFamily != nil && r.FamilyCode != nil {
				if _, ok := genusFamily[gk]; !ok {
					genusFamily[gk] = [2]*string{r.APCFamily, r.FamilyCode}
				}
			}
		}
	}
	rows.Close()

	_, taxa2025, err := taxaAccdb.ReadAll("USysAllSpecs")
	if err != nil {
		return nil, err
	}

	consumedHist := make([]bool, len(hist))
	var out []specsRow
	nextID := int64(19000001)
	provBC := "BC"
	repOne := int64(1)
	repZero := int64(0)

	// Pass 1: Process all non-synonym (accepted / excluded / extra) 2025 rows first so they deterministically claim their Code rows.
	for _, tr := range taxa2025 {
		ct := optStr(tr[8])
		if strings.EqualFold(strVal(ct), "S") {
			continue
		}
		yoc := optStr(tr[0])
		chg := optStr(tr[1])
		sci := optStr(tr[2])
		eng := optStr(tr[3])
		short := optStr(tr[4])
		code := optStr(tr[5])
		spl := optStr(tr[6])
		nat := optStr(tr[7])
		taxLvl := optStr(tr[9])
		var lf *int64
		if v, ok := tr[10].(int64); ok {
			vc := v
			lf = &vc
		}
		tree := optStr(tr[11])
		isAttr := int64(0)
		if b, ok := tr[12].(bool); ok && b {
			isAttr = -1
		}

		cu := strings.ToUpper(strVal(code))
		matchedIdx := -1
		if sci != nil {
			for _, candIdx := range bySci[normSciKey(*sci)] {
				if !consumedHist[candIdx] && !strings.EqualFold(strVal(hist[candIdx].Codetype), "S") && strings.EqualFold(strVal(hist[candIdx].Code), cu) {
					matchedIdx = candIdx
					break
				}
			}
		}
		if matchedIdx < 0 {
			if idx, ok := byCodeNonSyn[cu]; ok && !consumedHist[idx] {
				matchedIdx = idx
			}
		}
		if matchedIdx >= 0 {
			consumedHist[matchedIdx] = true
			base := hist[matchedIdx]
			if yoc != nil {
				base.YearOfChange = yoc
			}
			if chg != nil {
				base.ChangeDescription = chg
			}
			base.ScientificName = sci
			if eng != nil {
				base.EnglishName = eng
			}
			if short != nil {
				base.ShortenedGuideName = short
			}
			base.Code = code
			if base.OldCode == nil || strings.EqualFold(*base.OldCode, cu) {
				base.OldCode = code
			}
			base.SplCode = spl
			if nat != nil {
				base.Native = nat
			}
			base.Codetype = ct
			if taxLvl != nil {
				base.TaxonLevel = taxLvl
			}
			if lf != nil {
				base.Lifeform = lf
			}
			if tree != nil {
				base.TreeCode = tree
			}
			base.IsAttribute = isAttr
			out = append(out, base)
		} else {
			idVal := nextID
			nextID++
			var apcFam, famCode *string
			if gk := genusKey(strVal(sci)); gk != "" {
				if fam, ok := genusFamily[gk]; ok {
					apcFam, famCode = fam[0], fam[1]
				}
			}
			cl := int64(len(strVal(code)))
			out = append(out, specsRow{
				YearOfChange:       yoc,
				ChangeDescription:  chg,
				APCFamily:          apcFam,
				FamilyCode:         famCode,
				OldCode:            code,
				Code:               code,
				SplCode:            spl,
				TreeCode:           tree,
				ScientificName:     sci,
				Lifeform:           lf,
				EnglishName:        eng,
				ShortenedGuideName: short,
				Native:             nat,
				Codetype:           ct,
				Province:           &provBC,
				ID:                 &idVal,
				TaxonLevel:         taxLvl,
				IsAttribute:        isAttr,
				Report:             &repOne,
				SplCodeCalc:        spl,
				Codelength:         &cl,
			})
		}
	}

	// Pass 2: Process all synonym (Codetype == 'S') 2025 rows against remaining unconsumed historical rows.
	for _, tr := range taxa2025 {
		ct := optStr(tr[8])
		if !strings.EqualFold(strVal(ct), "S") {
			continue
		}
		yoc := optStr(tr[0])
		chg := optStr(tr[1])
		sci := optStr(tr[2])
		eng := optStr(tr[3])
		short := optStr(tr[4])
		code := optStr(tr[5])
		spl := optStr(tr[6])
		nat := optStr(tr[7])
		taxLvl := optStr(tr[9])
		var lf *int64
		if v, ok := tr[10].(int64); ok {
			vc := v
			lf = &vc
		}
		tree := optStr(tr[11])
		isAttr := int64(0)
		if b, ok := tr[12].(bool); ok && b {
			isAttr = -1
		}

		sk := normSciKey(strVal(sci))
		matchedIdx := -1
		for _, candIdx := range bySci[sk] {
			if !consumedHist[candIdx] {
				matchedIdx = candIdx
				break
			}
		}
		if matchedIdx >= 0 {
			consumedHist[matchedIdx] = true
			base := hist[matchedIdx]
			if yoc != nil {
				base.YearOfChange = yoc
			}
			if chg != nil {
				base.ChangeDescription = chg
			}
			base.ScientificName = sci
			if eng != nil {
				base.EnglishName = eng
			}
			if short != nil {
				base.ShortenedGuideName = short
			}
			base.Code = code
			base.SplCode = spl
			if nat != nil {
				base.Native = nat
			}
			base.Codetype = ct
			if taxLvl != nil {
				base.TaxonLevel = taxLvl
			}
			if lf != nil {
				base.Lifeform = lf
			}
			if tree != nil {
				base.TreeCode = tree
			}
			base.IsAttribute = isAttr
			if base.OldCode == nil || *base.OldCode == "" {
				oc := deriveBECSynonymOldCode(strVal(sci), strVal(code), usedOldCodes)
				usedOldCodes[oc] = true
				base.OldCode = &oc
			}
			out = append(out, base)
		} else {
			idVal := nextID
			nextID++
			oc := deriveBECSynonymOldCode(strVal(sci), strVal(code), usedOldCodes)
			usedOldCodes[oc] = true
			var apcFam, famCode *string
			if gk := genusKey(strVal(sci)); gk != "" {
				if fam, ok := genusFamily[gk]; ok {
					apcFam, famCode = fam[0], fam[1]
				}
			}
			cl := int64(len(oc))
			out = append(out, specsRow{
				YearOfChange:       yoc,
				ChangeDescription:  chg,
				APCFamily:          apcFam,
				FamilyCode:         famCode,
				OldCode:            &oc,
				Code:               code,
				SplCode:            spl,
				TreeCode:           tree,
				ScientificName:     sci,
				Lifeform:           lf,
				EnglishName:        eng,
				ShortenedGuideName: short,
				Native:             nat,
				Codetype:           ct,
				Province:           &provBC,
				ID:                 &idVal,
				TaxonLevel:         taxLvl,
				IsAttribute:        isAttr,
				Report:             &repZero,
				SplCodeCalc:        spl,
				Codelength:         &cl,
			})
		}
	}

	// Retain all unconsumed historical rows so ZERO OldCode -> Code synonyms or historical codes are lost!
	for i, hr := range hist {
		if consumedHist[i] {
			continue
		}
		out = append(out, hr)
	}

	// Resolve chained synonyms (e.g., CUPRNOO -> XANTNOO -> CALLNOO) where intermediate Code has no accepted/non-synonym row
	nonSynCodes := map[string]bool{}
	synTarget := map[string]string{}
	for _, r := range out {
		if r.Code != nil && *r.Code != "" && !strings.EqualFold(strVal(r.Codetype), "S") {
			nonSynCodes[strings.ToUpper(*r.Code)] = true
		}
		if strings.EqualFold(strVal(r.Codetype), "S") && r.OldCode != nil && r.Code != nil && !strings.EqualFold(*r.OldCode, *r.Code) {
			synTarget[strings.ToUpper(*r.OldCode)] = *r.Code
		}
	}
	for i := range out {
		if strings.EqualFold(strVal(out[i].Codetype), "S") && out[i].Code != nil {
			cur := *out[i].Code
			for hop := 0; hop < 4 && !nonSynCodes[strings.ToUpper(cur)]; hop++ {
				next, ok := synTarget[strings.ToUpper(cur)]
				if !ok || strings.EqualFold(next, cur) {
					break
				}
				cur = next
			}
			if cur != *out[i].Code {
				curCopy := cur
				out[i].Code = &curCopy
			}
		}
	}

	// Enrich newly-split genera (e.g., Mitella -> Brewerimitella, Thelypteris -> Amauropelta/Oreopteris) to fixed point
	for pass := 0; pass < 3; pass++ {
		codeFamily := map[string][2]*string{}
		for _, r := range out {
			if r.Code != nil && *r.Code != "" && r.FamilyCode != nil && *r.FamilyCode != "" {
				cu := strings.ToUpper(*r.Code)
				if _, seen := codeFamily[cu]; !seen {
					codeFamily[cu] = [2]*string{r.APCFamily, r.FamilyCode}
				}
			}
			if r.ScientificName != nil && *r.ScientificName != "" && r.FamilyCode != nil && *r.FamilyCode != "" {
				if gk := genusKey(*r.ScientificName); gk != "" {
					if _, seen := genusFamily[gk]; !seen {
						genusFamily[gk] = [2]*string{r.APCFamily, r.FamilyCode}
					}
				}
			}
		}
		for i := range out {
			if (out[i].FamilyCode == nil || *out[i].FamilyCode == "") && out[i].Code != nil {
				if fam, ok := codeFamily[strings.ToUpper(*out[i].Code)]; ok {
					if out[i].APCFamily == nil {
						out[i].APCFamily = fam[0]
					}
					out[i].FamilyCode = fam[1]
				}
			}
			if (out[i].FamilyCode == nil || *out[i].FamilyCode == "") && out[i].ScientificName != nil {
				if fam, ok := genusFamily[genusKey(*out[i].ScientificName)]; ok {
					if out[i].APCFamily == nil {
						out[i].APCFamily = fam[0]
					}
					out[i].FamilyCode = fam[1]
				}
			}
		}
	}

	// Ensure LMH25 (2023) Section 9.24 Version 2022 Tree Code Acx (Populus x hastata, black x balsam poplar hybrid) is present
	hasAcx := false
	for _, r := range out {
		if strings.EqualFold(strVal(r.TreeCode), "Acx") || strings.EqualFold(strVal(r.Code), "POPUHAS") {
			hasAcx = true
			break
		}
	}
	if !hasAcx {
		idVal := nextID
		nextID++
		yoc, chg := "2023", "LMH25 (2023) Sec 9.24 Version 2022 tree code addition"
		fam := "SALICAC"
		code := "POPUHAS"
		tc := "Acx"
		sci := "Populus x hastata"
		lf := int64(2)
		auth := "Dode"
		eng := "black x balsam poplar hybrid"
		nat := "N"
		ct := "U"
		tl := "hyb"
		cl := int64(7)
		out = append(out, specsRow{
			YearOfChange:      &yoc,
			ChangeDescription: &chg,
			APCFamily:         &fam,
			FamilyCode:        &fam,
			OldCode:           &code,
			Code:              &code,
			SplCode:           &code,
			TreeCode:          &tc,
			ScientificName:    &sci,
			Lifeform:          &lf,
			Authority:         &auth,
			EnglishName:       &eng,
			Native:            &nat,
			Codetype:          &ct,
			Province:          &provBC,
			ID:                &idVal,
			TaxonLevel:        &tl,
			IsAttribute:       0,
			Report:            &repOne,
			SplCodeCalc:       &code,
			Codelength:        &cl,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		ci, cj := strings.ToUpper(strVal(out[i].Code)), strings.ToUpper(strVal(out[j].Code))
		if ci != cj {
			return ci < cj
		}
		oi, oj := strings.ToUpper(strVal(out[i].OldCode)), strings.ToUpper(strVal(out[j].OldCode))
		if oi != oj {
			return oi < oj
		}
		return normSciKey(strVal(out[i].ScientificName)) < normSciKey(strVal(out[j].ScientificName))
	})
	return out, nil
}

type listUpsert struct {
	ListName      string
	ListFilter    *string
	ItemOrder     float64
	Item          string
	Description   string
	FieldUsedIn   *string
	ValidateLoops *string
	Validate      int64
	Note          *string
	Flag          int64
}

func lmh25ListAdditions() []listUpsert {
	sDist := "SiteDisturbance"
	exp := "Exposure"
	floodDur := "FloodingRegimeDur"
	floodFreq := "FloodingRegimeFreq"
	loops3 := "3"
	loops2 := "2"
	return []listUpsert{
		// StructuralStage 3c & 3d (verify/ensure present)
		{ListName: "StructuralStage", ItemOrder: 14.0, Item: "3c", Description: "Short stunted trees", FieldUsedIn: strPtrLocal("StructuralStage"), Validate: 1, Flag: 0},
		{ListName: "StructuralStage", ItemOrder: 15.0, Item: "3d", Description: "Tall stunted trees", FieldUsedIn: strPtrLocal("StructuralStage"), Validate: 1, Flag: 0},
		// SiviStructuralStage 3c & 3d
		{ListName: "SiviStructuralStage", ItemOrder: 8.1, Item: "3c", Description: "3c", Validate: 0, Flag: 0},
		{ListName: "SiviStructuralStage", ItemOrder: 8.2, Item: "3d", Description: "3d", Validate: 0, Flag: 0},
		// StructuralStageMod v & k
		{ListName: "StructuralStageMod", ItemOrder: 7.0, Item: "v", Description: "veteran canopy", Validate: 0, Flag: 0},
		{ListName: "StructuralStageMod", ItemOrder: 8.0, Item: "k", Description: "krummholz", Validate: 0, Flag: 0},
		// SiteDisturbance A.w.th & A.w.br
		{ListName: "SiteDisturbance", ItemOrder: 12.1, Item: "A.w.th", Description: "Windthrow (uprooting)", FieldUsedIn: &sDist, ValidateLoops: &loops3, Validate: 1, Flag: 0},
		{ListName: "SiteDisturbance", ItemOrder: 12.2, Item: "A.w.br", Description: "Windbreak (stem snap)", FieldUsedIn: &sDist, ValidateLoops: &loops3, Validate: 1, Flag: 0},
		// Exposure CO
		{ListName: "Exposure", ItemOrder: 3.5, Item: "CO", Description: "Cold aspect", FieldUsedIn: &exp, ValidateLoops: &loops2, Validate: 0, Flag: 0},
		// FloodingRegimeDur S & FloodingRegimeFreq S
		{ListName: "FloodingRegimeDur", ItemOrder: 7.0, Item: "S", Description: "Semi-permanent / Merely subirrigated during flood events", FieldUsedIn: &floodDur, Validate: 1, Flag: 0},
		{ListName: "FloodingRegimeFreq", ItemOrder: 5.5, Item: "S", Description: "Subirrigated / Seasonal seepage", FieldUsedIn: &floodFreq, Validate: 1, Flag: 0},
	}
}

func strPtrLocal(s string) *string { return &s }

func upgradeVListsFamilyAndReadModel(
	vlistsFamilyPath, vlistsReadModelPath string,
	specs []specsRow,
	zones []becZoneRow,
	series []becSeriesRow,
	masterUnits []wuRow,
) (string, string, int, int, error) {
	// Work on a deterministic clean copy of VLists.db
	srcBytes, err := os.ReadFile(vlistsFamilyPath)
	if err != nil {
		return "", "", 0, 0, err
	}
	tmpFamily, err := os.CreateTemp(filepath.Dir(vlistsFamilyPath), ".vlists-family-2026-*.db")
	if err != nil {
		return "", "", 0, 0, err
	}
	tmpFamilyPath := tmpFamily.Name()
	if _, err := tmpFamily.Write(srcBytes); err != nil {
		tmpFamily.Close()
		os.Remove(tmpFamilyPath)
		return "", "", 0, 0, err
	}
	tmpFamily.Close()
	defer os.Remove(tmpFamilyPath)

	db, err := sql.Open("sqlite3", tmpFamilyPath)
	if err != nil {
		return "", "", 0, 0, err
	}
	db.SetMaxOpenConns(1)

	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return "", "", 0, 0, err
	}

	// 1. Replace USysAllSpecs (preserving exact 26-column schema & INTEGER Lifeform)
	if _, err := tx.Exec(`DELETE FROM "USysAllSpecs"`); err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	spStmt, err := tx.Prepare(`INSERT INTO "USysAllSpecs" VALUES (` + strings.TrimSuffix(strings.Repeat("?,", 26), ",") + `)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	for _, r := range specs {
		if _, err := spStmt.Exec(
			r.YearOfChange, r.ChangeDescription, r.APCFamily, r.FamilyCode, r.OldCode, r.Code, r.SplCode, r.TreeCode,
			r.ScientificName, r.Lifeform, r.Authority, r.EnglishNameOld, r.EnglishName, r.CommonNamePB,
			r.NotesOnNames, r.ShortenedGuideName, r.Native, r.Codetype, r.Province, r.ID, r.TaxonLevel,
			r.IsAttribute, r.Report, r.SplCodeCalc, r.Codelength, r.CombinedEnglishName,
		); err != nil {
			spStmt.Close()
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
	}
	spStmt.Close()

	// 2. Replace USysZoneList
	if _, err := tx.Exec(`DELETE FROM "USysZoneList"`); err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	zStmt, err := tx.Prepare(`INSERT INTO "USysZoneList" ("Zone","SubZone","ZoneDescription","SubZoneVarDescription") VALUES (?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	for _, z := range zones {
		if _, err := zStmt.Exec(z.Zone, z.SubZone, z.ZoneDescription, z.Description); err != nil {
			zStmt.Close()
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
	}
	zStmt.Close()

	// 3. Replace USysSiteSeriesNames
	if _, err := tx.Exec(`DELETE FROM "USysSiteSeriesNames"`); err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	ssStmt, err := tx.Prepare(`INSERT INTO "USysSiteSeriesNames" VALUES (` + strings.TrimSuffix(strings.Repeat("?,", 26), ",") + `)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	for _, s := range series {
		var ssID, oSSID any
		if s.SourceID != nil {
			if f, err := strconv.ParseFloat(*s.SourceID, 64); err == nil {
				ssID = f
			}
		}
		if s.OriginalSourceID != nil {
			if f, err := strconv.ParseFloat(*s.OriginalSourceID, 64); err == nil {
				oSSID = f
			}
		}
		var added any
		if s.AddedDate != nil {
			d := *s.AddedDate
			if strings.HasSuffix(d, "T00:00:00") {
				d = strings.TrimSuffix(d, "T00:00:00")
			}
			added = d
		}
		var flagVal any
		if s.Flag != nil {
			if *s.Flag {
				flagVal = 1
			} else {
				flagVal = 0
			}
		}
		if _, err := ssStmt.Exec(
			ssID, s.SSCode, s.Name, s.Region, s.Zone, s.SubZone, s.BaseSubZone, s.Variant,
			s.Phase, s.SiteSeries, s.SiteSeriesPhase, s.Variation, s.Seral, s.Description,
			s.PlantAssociation, s.Comments, s.ReferenceID, added, s.ExpiredDate, oSSID,
			s.MergedBGC, s.Suballiance, s.Alliance, s.MissingNpeNa, s.TransferID, flagVal,
		); err != nil {
			ssStmt.Close()
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
	}
	ssStmt.Close()

	// 4. Replace MasterSiteUnitList
	if _, err := tx.Exec(`DELETE FROM "MasterSiteUnitList"`); err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	msuStmt, err := tx.Prepare(`INSERT INTO "MasterSiteUnitList" ("ID","SiteSeries","SiteSeriesLongName","SiteSeriesScientificName","Level") VALUES (?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}
	for _, m := range masterUnits {
		idInt, err := strconv.ParseInt(strVal(m.ID), 10, 64)
		if err != nil {
			msuStmt.Close()
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
		if _, err := msuStmt.Exec(idInt, m.SiteSeries, m.SiteSeriesLongName, m.SiteSeriesScientificName, m.Level); err != nil {
			msuStmt.Close()
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
	}
	msuStmt.Close()

	// 5. Upsert LMH25 (2023) vocabulary extensions in USysTableOfLists without changing existing row count on StructuralStage (21)!
	for _, up := range lmh25ListAdditions() {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM "USysTableOfLists" WHERE ListName=? AND Item=?`, up.ListName, up.Item).Scan(&exists); err != nil {
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
		if exists > 0 {
			if up.ListName == "FloodingRegimeDur" && up.Item == "S" {
				if _, err := tx.Exec(`UPDATE "USysTableOfLists" SET ItemDescription=? WHERE ListName=? AND Item=?`,
					up.Description, up.ListName, up.Item); err != nil {
					tx.Rollback()
					db.Close()
					return "", "", 0, 0, err
				}
			}
			continue
		}
		if _, err := tx.Exec(`INSERT INTO "USysTableOfLists"
			("ListName","ListFilter","ItemOrder","Item","ItemDescription","FieldUsedIn","ValidateLoops","Validate","Note","Flag")
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			up.ListName, up.ListFilter, up.ItemOrder, up.Item, up.Description,
			up.FieldUsedIn, up.ValidateLoops, up.Validate, up.Note, up.Flag); err != nil {
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
	}

	// 6. Stamp _table_metadata in VLists.db
	metaStamps := [][2]string{
		{"MasterSiteUnitList", "BECdb_v13_2026 (2026-05-28)"},
		{"USysSiteSeriesNames", "BECdb_v13_2026 (2026-05-28)"},
		{"USysZoneList", "BECdb_v13_2026 (2026-05-28)"},
		{"USysAllSpecs", "BCPlantTaxa_v19_2025 (2026-02-09)"},
		{"USysTableOfLists", "LMH25_2023 + BECdb_v13_2026"},
	}
	for _, st := range metaStamps {
		res, err := tx.Exec(`UPDATE "_table_metadata" SET "description"=? WHERE "table_name"=?`, st[1], st[0])
		if err != nil {
			tx.Rollback()
			db.Close()
			return "", "", 0, 0, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if _, err := tx.Exec(`INSERT INTO "_table_metadata" ("table_name","description") VALUES (?,?)`, st[0], st[1]); err != nil {
				tx.Rollback()
				db.Close()
				return "", "", 0, 0, err
			}
		}
	}

	var listTotal int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM "USysTableOfLists"`).Scan(&listTotal); err != nil {
		tx.Rollback()
		db.Close()
		return "", "", 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		db.Close()
		return "", "", 0, 0, err
	}
	freshFamily, err := os.CreateTemp(filepath.Dir(vlistsFamilyPath), ".vlists-family-clean-*.db")
	if err != nil {
		db.Close()
		return "", "", 0, 0, err
	}
	freshFamilyPath := freshFamily.Name()
	freshFamily.Close()
	defer os.Remove(freshFamilyPath)
	if err := cloneSQLiteDeterministic(db, freshFamilyPath); err != nil {
		db.Close()
		return "", "", 0, 0, err
	}

	// Build resources/vlists.db read-model deterministically from upgraded VLists.db
	tmpRead, err := os.CreateTemp(filepath.Dir(vlistsReadModelPath), ".vlists-read-2026-*.db")
	if err != nil {
		db.Close()
		return "", "", 0, 0, err
	}
	tmpReadPath := tmpRead.Name()
	tmpRead.Close()
	defer os.Remove(tmpReadPath)

	rmDB, err := sql.Open("sqlite3", tmpReadPath)
	if err != nil {
		db.Close()
		return "", "", 0, 0, err
	}
	rmDB.SetMaxOpenConns(1)
	rmTx, err := rmDB.Begin()
	if err != nil {
		rmDB.Close()
		db.Close()
		return "", "", 0, 0, err
	}
	if _, err := rmTx.Exec(`
CREATE TABLE Species (
    Code TEXT PRIMARY KEY,
    ScientificName TEXT,
    EnglishName TEXT,
    Lifeform INTEGER,
    FamilyCode TEXT,
    Authority TEXT,
    Native TEXT,
    Codetype TEXT
);
CREATE TABLE Lists (
    ListName TEXT,
    Item TEXT,
    ItemDescription TEXT,
    ItemOrder REAL,
    FieldUsedIn TEXT,
    PRIMARY KEY (ListName, Item)
);
CREATE INDEX idx_species_name ON Species(ScientificName);
CREATE INDEX idx_species_eng ON Species(EnglishName);
CREATE INDEX idx_lists_name ON Lists(ListName);
`); err != nil {
		rmTx.Rollback()
		rmDB.Close()
		db.Close()
		return "", "", 0, 0, err
	}

	// Populate Species (distinct non-synonym codes, preferring U > D/I > X/XX, non-null strings for ReferenceService.Scan)
	speciesRank := func(ct string) int {
		switch strings.ToUpper(ct) {
		case "U":
			return 0
		case "D", "I":
			return 1
		case "X":
			return 2
		case "XX":
			return 3
		default:
			return 4
		}
	}
	bestByCode := map[string]specsRow{}
	for _, r := range specs {
		c := strings.TrimSpace(strVal(r.Code))
		if c == "" || strings.EqualFold(strVal(r.Codetype), "S") {
			continue
		}
		cu := strings.ToUpper(c)
		if prev, exists := bestByCode[cu]; !exists || speciesRank(strVal(r.Codetype)) < speciesRank(strVal(prev.Codetype)) {
			bestByCode[cu] = r
		}
	}
	spCodes := make([]string, 0, len(bestByCode))
	for cu := range bestByCode {
		spCodes = append(spCodes, cu)
	}
	sort.Strings(spCodes)

	rmSpStmt, err := rmTx.Prepare(`INSERT INTO Species VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		rmTx.Rollback()
		rmDB.Close()
		db.Close()
		return "", "", 0, 0, err
	}
	for _, cu := range spCodes {
		r := bestByCode[cu]
		if _, err := rmSpStmt.Exec(
			cu,
			strVal(r.ScientificName),
			strVal(r.EnglishName),
			r.Lifeform,
			strVal(r.FamilyCode),
			strVal(r.Authority),
			strVal(r.Native),
			strVal(r.Codetype),
		); err != nil {
			rmSpStmt.Close()
			rmTx.Rollback()
			rmDB.Close()
			db.Close()
			return "", "", 0, 0, err
		}
	}
	rmSpStmt.Close()

	// Populate Lists from VLists.USysTableOfLists (non-null ListName and non-null, non-empty Item)
	lRows, err := db.Query(`SELECT ListName, Item, COALESCE(ItemDescription,''), ItemOrder, COALESCE(FieldUsedIn,'')
		FROM "USysTableOfLists"
		WHERE ListName IS NOT NULL AND ListName <> '' AND Item IS NOT NULL AND Item <> ''
		ORDER BY ListName COLLATE BINARY, ItemOrder, rowid`)
	if err != nil {
		rmTx.Rollback()
		rmDB.Close()
		db.Close()
		return "", "", 0, 0, err
	}
	rmLStmt, err := rmTx.Prepare(`INSERT OR IGNORE INTO Lists VALUES (?,?,?,?,?)`)
	if err != nil {
		lRows.Close()
		rmTx.Rollback()
		rmDB.Close()
		db.Close()
		return "", "", 0, 0, err
	}
	rmListCount := 0
	for lRows.Next() {
		var ln, it, desc, fui string
		var ord sql.NullFloat64
		if err := lRows.Scan(&ln, &it, &desc, &ord, &fui); err != nil {
			lRows.Close()
			rmLStmt.Close()
			rmTx.Rollback()
			rmDB.Close()
			db.Close()
			return "", "", 0, 0, err
		}
		var ordAny any
		if ord.Valid {
			ordAny = ord.Float64
		}
		res, err := rmLStmt.Exec(ln, it, desc, ordAny, fui)
		if err != nil {
			lRows.Close()
			rmLStmt.Close()
			rmTx.Rollback()
			rmDB.Close()
			db.Close()
			return "", "", 0, 0, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			rmListCount++
		}
	}
	lRows.Close()
	rmLStmt.Close()

	if err := rmTx.Commit(); err != nil {
		rmDB.Close()
		db.Close()
		return "", "", 0, 0, err
	}
	if _, err := rmDB.Exec(`VACUUM`); err != nil {
		rmDB.Close()
		db.Close()
		return "", "", 0, 0, err
	}
	if err := rmDB.Close(); err != nil {
		db.Close()
		return "", "", 0, 0, err
	}
	if err := db.Close(); err != nil {
		return "", "", 0, 0, err
	}

	familyBytes, err := os.ReadFile(freshFamilyPath)
	if err != nil {
		return "", "", 0, 0, err
	}
	readBytes, err := os.ReadFile(tmpReadPath)
	if err != nil {
		return "", "", 0, 0, err
	}
	familySHA := sha256Hex(familyBytes)
	readSHA := sha256Hex(readBytes)

	if err := os.Chmod(freshFamilyPath, 0644); err != nil {
		return "", "", 0, 0, err
	}
	if err := os.Rename(freshFamilyPath, vlistsFamilyPath); err != nil {
		return "", "", 0, 0, err
	}
	if err := os.Chmod(tmpReadPath, 0644); err != nil {
		return "", "", 0, 0, err
	}
	if err := os.Rename(tmpReadPath, vlistsReadModelPath); err != nil {
		return "", "", 0, 0, err
	}
	_ = rmListCount
	return familySHA, readSHA, listTotal, len(spCodes), nil
}

func cloneSQLiteDeterministic(srcDB *sql.DB, dstPath string) error {
	type ddlObj struct {
		kind, name, sql string
	}
	rows, err := srcDB.Query(`SELECT type, name, sql FROM sqlite_master WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY CASE type WHEN 'table' THEN 0 ELSE 1 END, name COLLATE BINARY`)
	if err != nil {
		return err
	}
	var objs []ddlObj
	for rows.Next() {
		var o ddlObj
		if err := rows.Scan(&o.kind, &o.name, &o.sql); err != nil {
			rows.Close()
			return err
		}
		objs = append(objs, o)
	}
	rows.Close()

	dstDB, err := sql.Open("sqlite3", dstPath)
	if err != nil {
		return err
	}
	defer dstDB.Close()
	dstDB.SetMaxOpenConns(1)

	tx, err := dstDB.Begin()
	if err != nil {
		return err
	}
	for _, o := range objs {
		if o.kind == "table" {
			if _, err := tx.Exec(o.sql); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Copy rows preserving rowid when no explicit integer PK aliases rowid, or full columns
	for _, o := range objs {
		if o.kind != "table" {
			continue
		}
		qName := `"` + strings.ReplaceAll(o.name, `"`, `""`) + `"`
		// Check if table has INTEGER PRIMARY KEY (which already aliases rowid)
		hasIPK := strings.Contains(strings.ToUpper(o.sql), "INTEGER PRIMARY KEY")
		var selectSQL, insertSQL string
		cRows, err := srcDB.Query(`PRAGMA table_info(` + qName + `)`)
		if err != nil {
			return err
		}
		var cols []string
		for cRows.Next() {
			var cid, notnull, pk int
			var cname, ctype string
			var dflt any
			if err := cRows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
				cRows.Close()
				return err
			}
			cols = append(cols, `"`+strings.ReplaceAll(cname, `"`, `""`)+`"`)
		}
		cRows.Close()

		colList := strings.Join(cols, ",")
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
		if hasIPK {
			selectSQL = `SELECT ` + colList + ` FROM ` + qName + ` ORDER BY rowid`
			insertSQL = `INSERT INTO ` + qName + ` (` + colList + `) VALUES (` + placeholders + `)`
		} else {
			selectSQL = `SELECT rowid,` + colList + ` FROM ` + qName + ` ORDER BY rowid`
			insertSQL = `INSERT INTO ` + qName + ` (rowid,` + colList + `) VALUES (?,` + placeholders + `)`
		}

		dataRows, err := srcDB.Query(selectSQL)
		if err != nil {
			return err
		}
		dTx, err := dstDB.Begin()
		if err != nil {
			dataRows.Close()
			return err
		}
		stmt, err := dTx.Prepare(insertSQL)
		if err != nil {
			dataRows.Close()
			dTx.Rollback()
			return err
		}
		nCols := len(cols)
		if !hasIPK {
			nCols++
		}
		for dataRows.Next() {
			vals := make([]any, nCols)
			ptrs := make([]any, nCols)
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := dataRows.Scan(ptrs...); err != nil {
				stmt.Close()
				dataRows.Close()
				dTx.Rollback()
				return err
			}
			if _, err := stmt.Exec(vals...); err != nil {
				stmt.Close()
				dataRows.Close()
				dTx.Rollback()
				return err
			}
		}
		dataRows.Close()
		stmt.Close()
		if err := dTx.Commit(); err != nil {
			return err
		}
	}

	idxTx, err := dstDB.Begin()
	if err != nil {
		return err
	}
	for _, o := range objs {
		if o.kind != "table" {
			if _, err := idxTx.Exec(o.sql); err != nil {
				idxTx.Rollback()
				return err
			}
		}
	}
	if err := idxTx.Commit(); err != nil {
		return err
	}
	_, err = dstDB.Exec(`VACUUM`)
	return err
}

func atomicWriteFile(dest string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(dest), ".std2026-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", dest, err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
