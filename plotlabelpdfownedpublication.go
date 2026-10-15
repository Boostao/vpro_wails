package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

type plotLabelPDFPublicationHooks struct {
	observe     func(string) error
	runtime     plotLabelPDFRuntime
	snapshot    publicationReadSnapshotHooks
	publication artifactPublicationHooks
}

func snapshotPlotLabelPDFPreparation(expected ownedPlotLabelPDFPreparation) (ownedPlotLabelPDFPreparation, error) {
	for i, table := range expected.Source {
		table.Columns = slices.Clone(table.Columns)
		table.Rows = slices.Clone(table.Rows)
		for j, row := range table.Rows {
			row.Cells = slices.Clone(row.Cells)
			for k, cell := range row.Cells {
				row.Cells[k] = cloneSiteUnitCell(cell)
			}
			table.Rows[j] = row
		}
		expected.Source[i] = table
	}
	header, err := plotLabelPDFHeader(expected.Source, expected.Preview.PlotNumber)
	if err != nil {
		return ownedPlotLabelPDFPreparation{}, err
	}
	if !reflect.DeepEqual(expected.Preview.Header, header) {
		return ownedPlotLabelPDFPreparation{}, errors.New("label PDF reviewed header disagrees with its complete physical source")
	}
	expected.Preview.Header = header
	expected.Bytes = bytes.Clone(expected.Bytes)
	return expected, nil
}

func (s *ContextService) publishOwnedPlotLabelPDF(ctx context.Context, contextID string, expected ownedPlotLabelPDFPreparation, requested string, hooks plotLabelPDFPublicationHooks) (artifactPublication, error) {
	expected, err := snapshotPlotLabelPDFPreparation(expected)
	if err != nil {
		return artifactPublication{}, err
	}
	if !strings.EqualFold(filepath.Ext(requested), ".pdf") {
		return artifactPublication{}, errors.New("label PDF publication requires an explicit .pdf destination")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result artifactPublication, resultErr error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return result, err
		}
		defer owner.mu.Unlock()
		defer func() {
			if result.Published && resultErr != nil {
				resultErr = fmt.Errorf("owned label PDF published; do not replay publication: %w", resultErr)
			}
		}()
		if expected.Preview.ContextID != contextID || expected.Selection != owner.selection {
			return result, errors.New("owned label PDF context/project/scope/path differs; reload review")
		}
		observe := func(phase string) error {
			if hooks.observe != nil {
				return hooks.observe(phase)
			}
			return nil
		}
		validate := func() error {
			actual, err := withPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (*ownedPlotLabelPDFPreparation, error) {
				return readPlotLabelPDFPreparationSnapshot(ctx, contextID, expected.Preview.PlotNumber, owner, tx, hooks.runtime)
			})
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(*actual, expected) {
				return errors.New("owned label PDF physical source/schema/date/fonts/bytes differs; reload review")
			}
			return nil
		}
		if err := validate(); err != nil {
			return result, err
		}
		if err := observe("validated"); err != nil {
			return result, err
		}
		format := artifactPublicationFormat{
			label: "label PDF", artifact: "label PDF", commitLabel: "owned label PDF",
			stagePattern: ".vpro-label-pdf-*.tmp",
			encode: func(ctx context.Context) ([]byte, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return bytes.Clone(expected.Bytes), nil
			},
			validate: func(ctx context.Context, data []byte) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if !bytes.Equal(data, expected.Bytes) {
					return errors.New("label PDF differs from the internally prepared complete reviewed artifact")
				}
				return nil
			},
		}
		result, resultErr = publishArtifactChecked(ctx, requested, format, func() error {
			if err := observe("precommit"); err != nil {
				return err
			}
			return validate()
		}, hooks.publication)
		if result.Published {
			resultErr = errors.Join(resultErr, observe("published"), validate())
		}
		return result, resultErr
	})
}
