package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/fs882layout"
	"github.com/boostao/vpro-wails/resources"
)

var siviParentLayoutJSON = resources.FS1333SIVILayoutJSON

type SIVIParentBinding struct {
	ControlID, Binding, Table string
	Column                    int
	Implicit                  bool
}

type SIVIParentRow struct {
	Env, Admin vprocore.ProjectMetadataRow
}

type SIVIParentProjection struct {
	ContextID, Project, Plot, Form, Query, Membership string
	EnvTable, AdminTable                              string
	EnvColumns, AdminColumns                          []vprocore.ProjectMetadataColumn
	Bindings                                          []SIVIParentBinding
	Rows                                              []SIVIParentRow
}

type SiviParentBinding = SIVIParentBinding
type SiviParentRow = SIVIParentRow
type SiviParentProjection = SIVIParentProjection

var SiviParentSourceLayout = sync.OnceValues(func() (*fs882layout.Layout, error) {
	var layout fs882layout.Layout
	if err := json.Unmarshal(siviParentLayoutJSON, &layout); err != nil {
		return nil, fmt.Errorf("SIVI parent source metadata: %w", err)
	}
	if len(layout.Forms) == 0 || layout.Forms[0].Name != "frmSIVIsite" || layout.Forms[0].RecordSource != "USysEnv" {
		return nil, errors.New("SIVI requires the original normal parent source")
	}
	return &layout, nil
})

var SiviParentSourceBindings = sync.OnceValues(func() ([]SiviParentBinding, error) {
	layout, err := SiviParentSourceLayout()
	if err != nil {
		return nil, err
	}
	return SourceSIVIParentBindings(layout.Forms[0].Fields, 77)
})

func SourceSIVIParentBindings(fields []fs882layout.Field, directCount int) ([]SiviParentBinding, error) {
	bindings := []SiviParentBinding{}
	seen := map[string]bool{}
	for _, field := range fields {
		if field.Binding == "" {
			continue
		}
		key := strings.ToLower(field.Binding)
		if seen[key] || field.ControlID == "" || !field.ReadOnly || field.Implementation != "unmapped" {
			return nil, errors.New("SIVI parent requires distinct read-only source bindings")
		}
		seen[key] = true
		bindings = append(bindings, SiviParentBinding{ControlID: field.ControlID, Binding: field.Binding})
	}
	if len(bindings) != directCount || seen["specieslistcomplete"] {
		return nil, fmt.Errorf("SIVI requires%d direct bindings and a distinct implicit species-list target", directCount)
	}
	return append(bindings, SiviParentBinding{Binding: "SpeciesListComplete", Implicit: true}), nil
}

// Literal BINARY membership is a SQLite adaptation, not proof of Access
// Unicode collation/DISTINCTROW behavior. Every matching physical pair survives.
func ProjectSIVIParent(ctx context.Context, contextID, project, plot string, env, admin vprocore.ProjectMetadataTable) (*SiviParentProjection, error) {
	source, err := SiviParentSourceBindings()
	if err != nil {
		return nil, err
	}
	return ProjectSourceParent(ctx, contextID, project, plot, "frmSIVIsite", source, env, admin)
}

func ProjectSourceParent(ctx context.Context, contextID, project, plot, form string, source []SiviParentBinding, env, admin vprocore.ProjectMetadataTable) (*SiviParentProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plot == "" || !utf8.ValidString(plot) || strings.ContainsRune(plot, 0) {
		return nil, errors.New("SIVI parent requires a literal valid nonempty plot identity")
	}
	indexes := make([]map[string]int, 2)
	tables := []vprocore.ProjectMetadataTable{env, admin}
	for i, table := range tables {
		required := "PlotNumber"
		if i == 1 {
			required = "Plot"
		}
		index, err := vprocore.SiteUnitTransferColumns(table, required)
		if err != nil {
			return nil, fmt.Errorf("SIVI parent physical schema: %w", err)
		}
		for name := range index {
			if strings.EqualFold(name, "rowid") || strings.EqualFold(name, "_rowid_") || strings.EqualFold(name, "oid") {
				return nil, errors.New("SIVI parent requires unshadowed physical row identities")
			}
		}
		for _, row := range table.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			for _, cell := range row.Cells {
				if _, err := vprocore.MetadataCellValue(cell); err != nil {
					return nil, fmt.Errorf("SIVI parent physical row %s: %w", row.RowID, err)
				}
			}
		}
		indexes[i] = index
	}
	result := &SiviParentProjection{
		ContextID: contextID, Project: project, Plot: plot, Form: form, Query: "USysEnv",
		Membership: "literal-binary-inner-pairs", EnvTable: project + "_Env", AdminTable: project + "_Admin",
		EnvColumns:   append([]vprocore.ProjectMetadataColumn(nil), env.Columns...),
		AdminColumns: append([]vprocore.ProjectMetadataColumn(nil), admin.Columns...),
		Bindings:     make([]SiviParentBinding, 0, len(source)), Rows: []SiviParentRow{},
	}
	for _, binding := range source {
		matches := 0
		for i, table := range tables {
			for column, definition := range table.Columns {
				if strings.EqualFold(definition.Name, binding.Binding) {
					binding.Table = []string{result.EnvTable, result.AdminTable}[i]
					binding.Column = column
					matches++
				}
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("SIVI parent binding %q requires one physical Env/Admin owner; found%d", binding.Binding, matches)
		}
		result.Bindings = append(result.Bindings, binding)
	}
	matching := make([][]vprocore.ProjectMetadataRow, 2)
	for i, table := range tables {
		key := "PlotNumber"
		if i == 1 {
			key = "Plot"
		}
		for _, row := range table.Rows {
			identity, err := vprocore.VegetationReportIdentity(row.Cells[indexes[i][key]])
			if err != nil {
				return nil, fmt.Errorf("SIVI parent join row %s: %w", row.RowID, err)
			}
			if identity != nil && *identity == plot {
				matching[i] = append(matching[i], row)
			}
		}
	}
	for _, envRow := range matching[0] {
		for _, adminRow := range matching[1] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			pair := SiviParentRow{}
			for i, row := range []vprocore.ProjectMetadataRow{envRow, adminRow} {
				cloned := vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: make([]vprocore.ProjectMetadataCell, len(row.Cells))}
				for j, cell := range row.Cells {
					cloned.Cells[j] = vprocore.CloneSiteUnitCell(cell)
				}
				if i == 0 {
					pair.Env = cloned
				} else {
					pair.Admin = cloned
				}
			}
			result.Rows = append(result.Rows, pair)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
