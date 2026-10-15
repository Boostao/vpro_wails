package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

const siviCHARSParentForm = "frmSIVIsite-CHARS"

//go:embed resources/fs1333-chars-sivi-layout.json
var siviCHARSParentLayoutJSON []byte

var siviCHARSParentSourceLayout = sync.OnceValues(func() (*fs882layout.Layout, error) {
	var layout fs882layout.Layout
	if err := json.Unmarshal(siviCHARSParentLayoutJSON, &layout); err != nil {
		return nil, fmt.Errorf("CHARS SIVI parent source metadata: %w", err)
	}
	if layout.Root != siviCHARSParentForm || len(layout.Forms) != 4 ||
		layout.Forms[0].Name != siviCHARSParentForm || layout.Forms[0].RecordSource != "USysEnv" ||
		len(layout.Forms[0].Pages) != 3 {
		return nil, errors.New("CHARS SIVI requires its original parent source and closure")
	}
	return &layout, nil
})

var siviCHARSParentSourceBindings = sync.OnceValues(func() ([]siviParentBinding, error) {
	layout, err := siviCHARSParentSourceLayout()
	if err != nil {
		return nil, err
	}
	return sourceSIVIParentBindings(layout.Forms[0].Fields, 71)
})

func projectSIVICHARSParent(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable) (*siviParentProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contextID == "" || project == "" {
		return nil, errors.New("CHARS SIVI parent requires owned context/project identities")
	}
	bindings, err := siviCHARSParentSourceBindings()
	if err != nil {
		return nil, err
	}
	return projectSourceParent(ctx, contextID, project, plot, siviCHARSParentForm, bindings, env, admin)
}

func (s *ContextService) readSIVICHARSParent(ctx context.Context, contextID, plot string) (*siviParentProjection, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviParentProjection, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviParentProjection, error) {
			tables, err := readSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			return projectSIVICHARSParent(ctx, contextID, owner.selection.Project, plot, tables[0], tables[1])
		})
	})
}
