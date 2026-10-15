package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type plotLabelPDFRuntime struct {
	now    func() time.Time
	format func(time.Time) (plotLabelNativeDate, error)
	fonts  func(context.Context) (plotLabelPDFFonts, error)
}

type ownedPlotLabelPDFPreparation struct {
	Selection  desktopSelection
	Source     [2]ProjectMetadataTable
	Preview    PlotLabelRegionalPreview
	FontSHA256 [2]string
	Bytes      []byte
}

func plotLabelPDFHeader(source [2]ProjectMetadataTable, plot string) (SavedPlotLabelHeader, error) {
	var header SavedPlotLabelHeader
	required := []string{"PlotNumber", "Zone", "SubZone", "SiteSeries", "PlotRepresenting", "ProjectID"}
	index, err := siteUnitTransferColumns(source[0], required...)
	if err != nil {
		return header, err
	}
	admin, err := siteUnitTransferColumns(source[1], "Plot")
	if err != nil {
		return header, err
	}
	if len(source[0].Rows) != 1 || len(source[1].Rows) != 1 {
		return header, errors.New("label PDF requires one complete literal physical Env/Admin pair")
	}
	identity := source[1].Rows[0].Cells[admin["Plot"]]
	if identity.Storage != "text" || identity.Text == nil || *identity.Text != plot {
		return header, errors.New("label PDF requires a byte-exact TEXT Admin parent")
	}
	var text [6]*string
	for i, name := range required {
		cell := source[0].Rows[0].Cells[index[name]]
		if _, err := metadataCellValue(cell); err != nil {
			return header, err
		}
		switch cell.Storage {
		case "null":
		case "text":
			text[i] = cell.Text
		default:
			return header, fmt.Errorf("label PDF %s requires original TEXT or NULL storage", name)
		}
	}
	if text[0] == nil || *text[0] != plot {
		return header, errors.New("label PDF requires a byte-exact TEXT Env parent")
	}
	return SavedPlotLabelHeader{*text[0], text[1], text[2], text[3], text[4], text[5]}, nil
}

func readPlotLabelPDFPreparationSnapshot(ctx context.Context, contextID, plot string, owner *sqliteContext, tx *sql.Tx, runtime plotLabelPDFRuntime) (*ownedPlotLabelPDFPreparation, error) {
	if plot == "" {
		return nil, errors.New("label PDF requires a literal plot number")
	}
	if err := plotLabelText(plot, "PlotNumber1", 7); err != nil {
		return nil, err
	}
	result := &ownedPlotLabelPDFPreparation{Selection: owner.selection}
	for i, suffix := range []string{"_Env", "_Admin"} {
		table, column := owner.selection.Project+suffix, "PlotNumber"
		if i == 1 {
			column = "Plot"
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, fmt.Errorf("label PDF requires original physical project table %q", table)
		}
		if err := validateSIVIPhysicalSchema(ctx, tx, "project", table, "label PDF originals"); err != nil {
			return nil, err
		}
		rows, err := readSQLiteLiteralTextRows(ctx, tx, "project", table, column, plot, "")
		if err != nil {
			return nil, err
		}
		result.Source[i] = rows
	}
	header, err := plotLabelPDFHeader(result.Source, plot)
	if err != nil {
		return nil, err
	}
	if runtime.now == nil {
		runtime.now = time.Now
	}
	if runtime.format == nil {
		runtime.format = formatPlotLabelNativeDate
	}
	if runtime.fonts == nil {
		runtime.fonts = loadPlotLabelPDFFonts
	}
	date := runtime.now().Local()
	formatted, err := runtime.format(date)
	if err != nil {
		return nil, fmt.Errorf("label PDF native date: %w", err)
	}
	result.Preview = PlotLabelRegionalPreview{
		ContextID: contextID, PlotNumber: plot, Header: header,
		LabelDate: formatted.text, ISODate: date.Format("2006-01-02"),
		Locale: formatted.locale, Pattern: formatted.pattern, Source: "windows-user-long-date",
	}
	fonts, err := runtime.fonts(ctx)
	if err != nil {
		return nil, err
	}
	for i, data := range [][]byte{fonts.regular, fonts.bold} {
		hash := sha256.Sum256(data)
		result.FontSHA256[i] = hex.EncodeToString(hash[:])
	}
	result.Bytes, err = renderPlotLabelPDF(ctx, result.Preview, fonts)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ContextService) readPlotLabelPDFPreparation(ctx context.Context, contextID, plot string, runtime plotLabelPDFRuntime) (*ownedPlotLabelPDFPreparation, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*ownedPlotLabelPDFPreparation, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*ownedPlotLabelPDFPreparation, error) {
			return readPlotLabelPDFPreparationSnapshot(ctx, contextID, plot, owner, tx, runtime)
		})
	})
}
