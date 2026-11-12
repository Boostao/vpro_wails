package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"

	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/boostao/vpro-wails/internal/fs882layout"
	"github.com/boostao/vpro-wails/resources"
)

const siviCHARSParentForm = "frmSIVIsite-CHARS"

var siviCHARSParentLayoutJSON = resources.FS1333CHARSSIVILayoutJSON

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

var siviCHARSParentSourceBindings = sync.OnceValues(func() ([]sourceform.SiviParentBinding, error) {
	layout, err := siviCHARSParentSourceLayout()
	if err != nil {
		return nil, err
	}
	return sourceform.SourceSIVIParentBindings(layout.Forms[0].Fields, 71)
})

func projectSIVICHARSParent(ctx context.Context, contextID, project, plot string, env, admin vprocore.ProjectMetadataTable) (*sourceform.SiviParentProjection, error) {
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
	return sourceform.ProjectSourceParent(ctx, contextID, project, plot, siviCHARSParentForm, bindings, env, admin)
}

func ReadSIVICHARSParent(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SiviParentProjection, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SiviParentProjection, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*sourceform.SiviParentProjection, error) {
			tables, err := sourceform.ReadSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			return projectSIVICHARSParent(ctx, contextID, owner.Selection.Project, plot, tables[0], tables[1])
		})
	})
}
