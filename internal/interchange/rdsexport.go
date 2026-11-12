package interchange

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	rSxpSym      uint32 = 1
	rSxpList     uint32 = 2
	rSxpChar     uint32 = 9
	rSxpLgl      uint32 = 10
	rSxpInt      uint32 = 13
	rSxpReal     uint32 = 14
	rSxpStr      uint32 = 16
	rSxpVec      uint32 = 19
	rSxpNilValue uint32 = 254
	rSxpRef      uint32 = 255

	rObjectBit uint32 = 1 << 8
	rHasAttr   uint32 = 1 << 9
	rHasTag    uint32 = 1 << 10

	rCharUTF8Mask  uint32 = (1 << 3) << 12
	rCharASCIIMask uint32 = (1 << 6) << 12

	rNAInteger int32  = -2147483648
	rNARealHex uint64 = 0x7ff00000000007a2
)

type rdsColumnKind int

const (
	rdsColCharacter rdsColumnKind = iota
	rdsColInteger
	rdsColReal
	rdsColLogical
)

type rdsColumn struct {
	Name    string
	Kind    rdsColumnKind
	Strings []*string
	Ints    []*int32
	Reals   []*float64
	Logics  []*bool
}

type rdsDataFrame struct {
	Name    string
	NumRows int
	Columns []rdsColumn
}

type rdsWriter struct {
	buf     bytes.Buffer
	symRefs map[string]int
	nextRef int
}

func newRDSWriter() *rdsWriter {
	w := &rdsWriter{
		symRefs: make(map[string]int, 8),
		nextRef: 1,
	}
	// XDR v2 stream header ("X\n", format v2, R 4.4.0 writer, R 2.3.0 min reader)
	w.buf.Write([]byte{'X', '\n'})
	w.writeU32(2)
	w.writeU32(0x00040400)
	w.writeU32(0x00020300)
	return w
}

func (w *rdsWriter) writeU32(v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	w.buf.Write(b[:])
}

func (w *rdsWriter) writeI32(v int32) {
	w.writeU32(uint32(v))
}

func (w *rdsWriter) writeF64Bits(bits uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], bits)
	w.buf.Write(b[:])
}

func isAllASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func (w *rdsWriter) writeCharSXP(s *string) {
	if s == nil {
		w.writeU32(rSxpChar)
		w.writeI32(-1)
		return
	}
	val := *s
	if isAllASCII(val) {
		w.writeU32(rSxpChar | rCharASCIIMask)
	} else {
		w.writeU32(rSxpChar | rCharUTF8Mask)
	}
	w.writeI32(int32(len(val)))
	w.buf.WriteString(val)
}

func (w *rdsWriter) writeSymbol(name string) {
	if refIdx, exists := w.symRefs[name]; exists {
		w.writeU32((uint32(refIdx) << 8) | rSxpRef)
		return
	}
	w.symRefs[name] = w.nextRef
	w.nextRef++
	w.writeU32(rSxpSym)
	w.writeCharSXP(&name)
}

func (w *rdsWriter) writeStringVector(vals []string) {
	w.writeU32(rSxpStr)
	w.writeI32(int32(len(vals)))
	for i := range vals {
		s := vals[i]
		w.writeCharSXP(&s)
	}
}

func (w *rdsWriter) writeColumn(col rdsColumn, nrow int) error {
	switch col.Kind {
	case rdsColCharacter:
		if len(col.Strings) != nrow {
			return fmt.Errorf("column %s length %d != nrow %d", col.Name, len(col.Strings), nrow)
		}
		w.writeU32(rSxpStr)
		w.writeI32(int32(nrow))
		for _, s := range col.Strings {
			w.writeCharSXP(s)
		}
	case rdsColInteger:
		if len(col.Ints) != nrow {
			return fmt.Errorf("column %s length %d != nrow %d", col.Name, len(col.Ints), nrow)
		}
		w.writeU32(rSxpInt)
		w.writeI32(int32(nrow))
		for _, iv := range col.Ints {
			if iv == nil {
				w.writeI32(rNAInteger)
			} else {
				w.writeI32(*iv)
			}
		}
	case rdsColReal:
		if len(col.Reals) != nrow {
			return fmt.Errorf("column %s length %d != nrow %d", col.Name, len(col.Reals), nrow)
		}
		w.writeU32(rSxpReal)
		w.writeI32(int32(nrow))
		for _, rv := range col.Reals {
			if rv == nil || math.IsNaN(*rv) {
				w.writeF64Bits(rNARealHex)
			} else {
				w.writeF64Bits(math.Float64bits(*rv))
			}
		}
	case rdsColLogical:
		if len(col.Logics) != nrow {
			return fmt.Errorf("column %s length %d != nrow %d", col.Name, len(col.Logics), nrow)
		}
		w.writeU32(rSxpLgl)
		w.writeI32(int32(nrow))
		for _, lv := range col.Logics {
			if lv == nil {
				w.writeI32(rNAInteger)
			} else if *lv {
				w.writeI32(1)
			} else {
				w.writeI32(0)
			}
		}
	default:
		return fmt.Errorf("unsupported RDS column kind for %s", col.Name)
	}
	return nil
}

func (w *rdsWriter) writeDataFrame(df rdsDataFrame) error {
	w.writeU32(rSxpVec | rObjectBit | rHasAttr)
	w.writeI32(int32(len(df.Columns)))
	colNames := make([]string, len(df.Columns))
	for i, col := range df.Columns {
		colNames[i] = col.Name
		if err := w.writeColumn(col, df.NumRows); err != nil {
			return err
		}
	}

	// Attribute 1: names
	w.writeU32(rSxpList | rHasTag)
	w.writeSymbol("names")
	w.writeStringVector(colNames)

	// Attribute 2: class = "data.frame"
	w.writeU32(rSxpList | rHasTag)
	w.writeSymbol("class")
	w.writeStringVector([]string{"data.frame"})

	// Attribute 3: row.names (compact integer representation)
	w.writeU32(rSxpList | rHasTag)
	w.writeSymbol("row.names")
	w.writeU32(rSxpInt)
	if df.NumRows == 0 {
		w.writeI32(0)
	} else {
		w.writeI32(2)
		w.writeI32(rNAInteger)
		w.writeI32(-int32(df.NumRows))
	}

	// End of pairlist
	w.writeU32(rSxpNilValue)
	return nil
}

func EncodeNamedDataFramesToRDS(frames []rdsDataFrame) ([]byte, error) {
	w := newRDSWriter()
	w.writeU32(rSxpVec | rHasAttr)
	w.writeI32(int32(len(frames)))

	names := make([]string, len(frames))
	for i, df := range frames {
		names[i] = df.Name
		if err := w.writeDataFrame(df); err != nil {
			return nil, err
		}
	}

	// Outer list names attribute
	w.writeU32(rSxpList | rHasTag)
	w.writeSymbol("names")
	w.writeStringVector(names)
	w.writeU32(rSxpNilValue)

	var gzBuf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&gzBuf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := gz.Write(w.buf.Bytes()); err != nil {
		gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return gzBuf.Bytes(), nil
}

type RDSExportRequest struct {
	DestinationPath string `json:"destinationPath"`
	LumpTable       string `json:"lumpTable,omitempty"`
	LumpDBPath      string `json:"lumpDbPath,omitempty"`
}

type RDSTableSummary struct {
	Name        string `json:"name"`
	RowCount    int    `json:"rowCount"`
	ColumnCount int    `json:"columnCount"`
}

type RDSExportOutcome struct {
	Path      string            `json:"path"`
	SHA256    string            `json:"sha256"`
	ByteSize  int               `json:"byteSize"`
	PlotCount int               `json:"plotCount"`
	Tables    []RDSTableSummary `json:"tables"`
}

func classifySQLiteDeclaredType(decl string) rdsColumnKind {
	u := strings.ToUpper(strings.TrimSpace(decl))
	switch {
	case strings.Contains(u, "BOOL") || strings.Contains(u, "BIT"):
		return rdsColLogical
	case strings.Contains(u, "INT"):
		return rdsColInteger
	case strings.Contains(u, "REAL") || strings.Contains(u, "DOUB") || strings.Contains(u, "FLOA") || strings.Contains(u, "SING") || strings.Contains(u, "NUM") || strings.Contains(u, "DEC"):
		return rdsColReal
	default:
		return rdsColCharacter
	}
}

func readSQLiteTableAsRDSDataFrame(
	ctx context.Context,
	tx *sql.Tx,
	dfName string,
	alias string,
	table string,
	whereClause string,
	orderClause string,
) (rdsDataFrame, error) {
	pRows, err := tx.QueryContext(ctx, `PRAGMA `+vprocore.QuoteHeaderIdentifier(alias)+`.table_info(`+vprocore.QuoteHeaderIdentifier(table)+`)`)
	if err != nil {
		return rdsDataFrame{}, err
	}
	type colMeta struct {
		name string
		kind rdsColumnKind
	}
	var metas []colMeta
	for pRows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt any
		if err := pRows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			pRows.Close()
			return rdsDataFrame{}, err
		}
		metas = append(metas, colMeta{name: name, kind: classifySQLiteDeclaredType(ctype)})
	}
	if err := pRows.Close(); err != nil {
		return rdsDataFrame{}, err
	}

	selectExprs := make([]string, len(metas))
	cols := make([]rdsColumn, len(metas))
	for i, m := range metas {
		cols[i] = rdsColumn{Name: m.name, Kind: m.kind}
		qCol := "t." + vprocore.QuoteHeaderIdentifier(m.name)
		switch m.kind {
		case rdsColLogical:
			selectExprs[i] = `CASE WHEN ` + qCol + ` IS NULL THEN NULL WHEN typeof(` + qCol + `)='text' THEN ` + qCol + ` ELSE CAST(` + qCol + ` AS INTEGER) END`
		case rdsColCharacter:
			selectExprs[i] = `CASE WHEN ` + qCol + ` IS NULL THEN NULL ELSE CAST(` + qCol + ` AS TEXT) END`
		default:
			selectExprs[i] = qCol
		}
	}

	fullTable := vprocore.QuoteHeaderIdentifier(alias) + "." + vprocore.QuoteHeaderIdentifier(table) + " AS t"
	query := `SELECT ` + strings.Join(selectExprs, ", ") + ` FROM ` + fullTable
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	if orderClause != "" {
		query += " ORDER BY " + orderClause
	}

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return rdsDataFrame{}, fmt.Errorf("read %s for RDS: %w", table, err)
	}
	defer rows.Close()

	nrow := 0
	for rows.Next() {
		rawVals := make([]any, len(metas))
		ptrs := make([]any, len(metas))
		for i := range rawVals {
			ptrs[i] = &rawVals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return rdsDataFrame{}, err
		}
		nrow++
		for i, m := range metas {
			raw := rawVals[i]
			switch m.kind {
			case rdsColCharacter:
				if raw == nil {
					cols[i].Strings = append(cols[i].Strings, nil)
				} else {
					var s string
					switch v := raw.(type) {
					case string:
						s = v
					case []byte:
						s = string(v)
					default:
						s = fmt.Sprint(v)
					}
					cols[i].Strings = append(cols[i].Strings, &s)
				}
			case rdsColInteger:
				if raw == nil {
					cols[i].Ints = append(cols[i].Ints, nil)
				} else {
					var iv int64
					ok := true
					switch v := raw.(type) {
					case int64:
						iv = v
					case int:
						iv = int64(v)
					case float64:
						iv = int64(v)
					default:
						ok = false
					}
					if !ok || iv <= int64(rNAInteger) || iv > 2147483647 {
						cols[i].Ints = append(cols[i].Ints, nil)
					} else {
						v32 := int32(iv)
						cols[i].Ints = append(cols[i].Ints, &v32)
					}
				}
			case rdsColReal:
				if raw == nil {
					cols[i].Reals = append(cols[i].Reals, nil)
				} else {
					var fv float64
					ok := true
					switch v := raw.(type) {
					case float64:
						fv = v
					case int64:
						fv = float64(v)
					case int:
						fv = float64(v)
					default:
						ok = false
					}
					if !ok || math.IsNaN(fv) {
						cols[i].Reals = append(cols[i].Reals, nil)
					} else {
						cols[i].Reals = append(cols[i].Reals, &fv)
					}
				}
			case rdsColLogical:
				if raw == nil {
					cols[i].Logics = append(cols[i].Logics, nil)
				} else {
					b := isAccessTruthy(raw)
					cols[i].Logics = append(cols[i].Logics, &b)
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return rdsDataFrame{}, err
	}
	return rdsDataFrame{
		Name:    dfName,
		NumRows: nrow,
		Columns: cols,
	}, nil
}

func buildVegLongDataFrame(obs []longVegObservation) rdsDataFrame {
	n := len(obs)
	plots := make([]*string, n)
	sus := make([]*string, n)
	ids := make([]*int32, n)
	spps := make([]*string, n)
	scis := make([]*string, n)
	lfs := make([]*int32, n)
	layers := make([]*string, n)
	covers := make([]*float64, n)
	heights := make([]*float64, n)
	colls := make([]*string, n)

	for i, o := range obs {
		p := o.PlotNumber
		plots[i] = &p
		if o.SiteUnit != "" {
			su := o.SiteUnit
			sus[i] = &su
		}
		id32 := int32(o.ID)
		ids[i] = &id32
		sp := o.Species
		spps[i] = &sp
		if o.ScientificName != "" {
			sc := o.ScientificName
			scis[i] = &sc
		}
		if o.Lifeform != nil {
			lf32 := int32(*o.Lifeform)
			lfs[i] = &lf32
		}
		ly := o.Layer
		layers[i] = &ly
		cv := o.Cover
		covers[i] = &cv
		if o.Height != nil {
			ht := *o.Height
			heights[i] = &ht
		}
		if o.Collected != "" {
			cl := o.Collected
			colls[i] = &cl
		}
	}

	return rdsDataFrame{
		Name:    "veg_long",
		NumRows: n,
		Columns: []rdsColumn{
			{Name: "PlotNumber", Kind: rdsColCharacter, Strings: plots},
			{Name: "SiteUnit", Kind: rdsColCharacter, Strings: sus},
			{Name: "ID", Kind: rdsColInteger, Ints: ids},
			{Name: "Species", Kind: rdsColCharacter, Strings: spps},
			{Name: "ScientificName", Kind: rdsColCharacter, Strings: scis},
			{Name: "Lifeform", Kind: rdsColInteger, Ints: lfs},
			{Name: "Layer", Kind: rdsColCharacter, Strings: layers},
			{Name: "Cover", Kind: rdsColReal, Reals: covers},
			{Name: "Height", Kind: rdsColReal, Reals: heights},
			{Name: "Collected", Kind: rdsColCharacter, Strings: colls},
		},
	}
}

func buildSUDataFrame(plotOrder []string, suByPlot map[string]string, hasActiveSU bool) rdsDataFrame {
	if !hasActiveSU {
		var plots []*string
		var sus []*string
		for _, p := range plotOrder {
			su := suByPlot[p]
			if su == "" {
				continue
			}
			pc, sc := p, su
			plots = append(plots, &pc)
			sus = append(sus, &sc)
		}
		return rdsDataFrame{
			Name:    "su",
			NumRows: len(plots),
			Columns: []rdsColumn{
				{Name: "PlotNumber", Kind: rdsColCharacter, Strings: plots},
				{Name: "SiteUnit", Kind: rdsColCharacter, Strings: sus},
			},
		}
	}
	plots := make([]*string, len(plotOrder))
	sus := make([]*string, len(plotOrder))
	for i, p := range plotOrder {
		pc := p
		plots[i] = &pc
		if su := suByPlot[p]; su != "" {
			sc := su
			sus[i] = &sc
		}
	}
	return rdsDataFrame{
		Name:    "su",
		NumRows: len(plotOrder),
		Columns: []rdsColumn{
			{Name: "PlotNumber", Kind: rdsColCharacter, Strings: plots},
			{Name: "SiteUnit", Kind: rdsColCharacter, Strings: sus},
		},
	}
}

func ExportProjectRDS(s *vprocore.ContextService, ctx context.Context, contextID string, request RDSExportRequest) (RDSExportOutcome, error) {
	if strings.TrimSpace(request.DestinationPath) == "" {
		return RDSExportOutcome{}, errors.New("RDS export requires a non-empty destinationPath")
	}
	dest := strings.TrimSpace(request.DestinationPath)
	if !strings.HasSuffix(strings.ToLower(dest), ".rds") {
		dest += ".rds"
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (RDSExportOutcome, error) {
		owner := plots.Projects.Sqlite
		lumpMap, err := loadOptionalLumpMap(ctx, owner.Selection.ProjectPath, request.LumpTable, request.LumpDBPath)
		if err != nil {
			return RDSExportOutcome{}, err
		}

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return RDSExportOutcome{}, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		defer func() { _ = tx.Rollback() }()

		project := owner.Selection.Project
		scopePlotPred := `EXISTS (SELECT 1 FROM USysEnv AS e WHERE typeof(e.PlotNumber)=typeof(t."PlotNumber") AND CAST(e.PlotNumber AS BLOB)=CAST(t."PlotNumber" AS BLOB))`
		scopeAdminPred := `EXISTS (SELECT 1 FROM USysEnv AS e WHERE typeof(e.PlotNumber)=typeof(t."Plot") AND CAST(e.PlotNumber AS BLOB)=CAST(t."Plot" AS BLOB))`

		envDF, err := readSQLiteTableAsRDSDataFrame(ctx, tx, "env", "project", project+"_Env", scopePlotPred, `t."PlotNumber"`)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		adminDF, err := readSQLiteTableAsRDSDataFrame(ctx, tx, "admin", "project", project+"_Admin", scopeAdminPred, `t."Plot"`)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		humusDF, err := readSQLiteTableAsRDSDataFrame(ctx, tx, "humus", "project", project+"_Humus", scopePlotPred, `t."PlotNumber", t."UpperDepth" DESC, t."Horizon" DESC, t."ID"`)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		mineralDF, err := readSQLiteTableAsRDSDataFrame(ctx, tx, "mineral", "project", project+"_Mineral", scopePlotPred, `t."PlotNumber", t."UpperDepth" ASC, t."Horizon" ASC, t."ID"`)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		metadataDF, err := readSQLiteTableAsRDSDataFrame(ctx, tx, "metadata", "project", project+"_Metadata", "", `t."ID"`)
		if err != nil {
			return RDSExportOutcome{}, err
		}

		plotOrder, suByPlot, err := loadScopedPlotSiteUnits(ctx, tx, owner)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		sppMeta, err := loadSpeciesLookupMeta(ctx, tx)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		vegObs, err := extractLongVegObservations(ctx, tx, owner, suByPlot, sppMeta, lumpMap)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		if err := tx.Commit(); err != nil {
			return RDSExportOutcome{}, err
		}

		vegDF := buildVegLongDataFrame(vegObs)
		suDF := buildSUDataFrame(plotOrder, suByPlot, owner.Selection.SU != "None")

		frames := []rdsDataFrame{envDF, adminDF, vegDF, humusDF, mineralDF, suDF, metadataDF}
		rdsBytes, err := EncodeNamedDataFramesToRDS(frames)
		if err != nil {
			return RDSExportOutcome{}, err
		}
		writtenPath, shaHex, err := writeAtomicExportFile(dest, rdsBytes)
		if err != nil {
			return RDSExportOutcome{}, err
		}

		summaries := make([]RDSTableSummary, len(frames))
		for i, f := range frames {
			summaries[i] = RDSTableSummary{
				Name:        f.Name,
				RowCount:    f.NumRows,
				ColumnCount: len(f.Columns),
			}
		}
		return RDSExportOutcome{
			Path:      writtenPath,
			SHA256:    shaHex,
			ByteSize:  len(rdsBytes),
			PlotCount: len(plotOrder),
			Tables:    summaries,
		}, nil
	})
}
