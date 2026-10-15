package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/boostao/vpro-wails/internal/reports/breakreport"
)

const breakReportMaxOriginalBytes = 8 << 20
const breakReportMaxPageSize = 100

type BreakReportReview struct {
	ContextID    string             `json:"contextId"`
	SourcePath   string             `json:"sourcePath"`
	SourceSHA256 string             `json:"sourceSHA256"`
	Offset       int                `json:"offset"`
	Limit        int                `json:"limit"`
	Total        int                `json:"total"`
	Rows         []breakreport.Pair `json:"rows"`
}

func (s *ContextService) reviewBreakReport(ctx context.Context, contextID string, offset, limit int) (BreakReportReview, error) {
	if ctx == nil {
		return BreakReportReview{}, errors.New("PC-ORD Break Report requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return BreakReportReview{}, err
	}
	if s == nil || s.projects == nil || s.plots == nil {
		return BreakReportReview{}, errors.New("PC-ORD Break Report owned context is unavailable")
	}
	if offset < 0 || limit < 1 || limit > breakReportMaxPageSize {
		return BreakReportReview{}, errors.New("PC-ORD Break Report page requires a nonnegative offset and limit from 1 to 100")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (BreakReportReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, publicationReadSnapshotHooks{}, func(owner *sqliteContext, tx *sql.Tx) (BreakReportReview, error) {
			original, err := readBreakReportOriginal(ctx, tx)
			if err != nil {
				return BreakReportReview{}, err
			}
			columns, err := siteUnitTransferColumns(original, "Reference", "BreakCode")
			if err != nil {
				return BreakReportReview{}, err
			}
			input := make([]breakreport.Pair, 0, len(original.Rows))
			for _, row := range original.Rows {
				if err := ctx.Err(); err != nil {
					return BreakReportReview{}, err
				}
				values := make([]*string, 0, 2)
				for _, field := range []string{"Reference", "BreakCode"} {
					cell := row.Cells[columns[field]]
					switch cell.Storage {
					case "null":
						values = append(values, nil)
					case "text":
						values = append(values, cell.Text)
					default:
						return BreakReportReview{}, fmt.Errorf("PC-ORD Break Report row %s %s requires literal TEXT or NULL storage", row.RowID, field)
					}
				}
				input = append(input, breakreport.Pair{Reference: values[0], BreakCode: values[1]})
			}
			rows, err := breakreport.Plan(ctx, input)
			if err != nil {
				return BreakReportReview{}, err
			}
			raw, err := json.Marshal(original)
			if err != nil {
				return BreakReportReview{}, fmt.Errorf("PC-ORD Break Report original witness: %w", err)
			}
			digest := sha256.Sum256(raw)
			total := len(rows)
			if offset < total {
				end := total
				if limit < total-offset {
					end = offset + limit
				}
				rows = rows[offset:end]
			} else {
				rows = []breakreport.Pair{}
			}
			if err := ctx.Err(); err != nil {
				return BreakReportReview{}, err
			}
			return BreakReportReview{
				ContextID: contextID, SourcePath: owner.attachments["VPro64"], SourceSHA256: hex.EncodeToString(digest[:]),
				Offset: offset, Limit: limit, Total: total, Rows: rows,
			}, nil
		})
	})
}

func readBreakReportOriginal(ctx context.Context, tx *sql.Tx) (ProjectMetadataTable, error) {
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM "VPro64".sqlite_master WHERE name='USysBreakTable' COLLATE BINARY`).Scan(&kind); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("PC-ORD Break Report physical source unavailable: %w", err)
	}
	if kind != "table" {
		return ProjectMetadataTable{}, errors.New("PC-ORD Break Report requires its original physical table, not a view")
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, "VPro64", "USysBreakTable", "PC-ORD Break Report sources"); err != nil {
		return ProjectMetadataTable{}, err
	}
	columns, err := readSQLiteStorageColumns(ctx, tx, "VPro64", "USysBreakTable")
	if err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("PC-ORD Break Report schema unavailable: %w", err)
	}
	required := map[string]bool{"Order": false, "Reference": false, "BreakCode": false, "PlotNumber": false}
	if len(columns) != len(required) {
		return ProjectMetadataTable{}, errors.New("PC-ORD Break Report requires exactly its four original physical columns")
	}
	for _, column := range columns {
		if _, exists := required[column.Name]; !exists {
			return ProjectMetadataTable{}, fmt.Errorf("PC-ORD Break Report has unsupported physical column %q", column.Name)
		}
		required[column.Name] = true
	}
	for name, present := range required {
		if !present {
			return ProjectMetadataTable{}, fmt.Errorf("PC-ORD Break Report physical column %q is unavailable", name)
		}
	}
	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "VPro64"."USysBreakTable"`).Scan(&count); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("PC-ORD Break Report physical bounds unavailable: %w", err)
	}
	if count > breakreport.MaxRows {
		return ProjectMetadataTable{}, errors.New("PC-ORD Break Report exceeds the 10000-row physical bound; no rows were truncated")
	}
	var bytes int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(
		COALESCE(length(CAST("Order" AS BLOB)),0)+COALESCE(length(CAST("Reference" AS BLOB)),0)+
		COALESCE(length(CAST("BreakCode" AS BLOB)),0)+COALESCE(length(CAST("PlotNumber" AS BLOB)),0)),0)
		FROM "VPro64"."USysBreakTable"`).Scan(&bytes); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("PC-ORD Break Report complete-original bounds unavailable: %w", err)
	}
	if bytes > breakReportMaxOriginalBytes {
		return ProjectMetadataTable{}, errors.New("PC-ORD Break Report exceeds the 8MiB complete-original bound; no partial source was read")
	}
	original, err := readSQLiteStorageRows(ctx, tx, "VPro64", "USysBreakTable", "", nil, "")
	if err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("PC-ORD Break Report original rows unavailable: %w", err)
	}
	return original, nil
}
