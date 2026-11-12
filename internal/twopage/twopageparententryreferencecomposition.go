package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
)

type TwoPageEntryReferenceSnapshot struct {
	Original                     *sourceform.SiviParentProjection
	ProjectSource, WorkingSource int
	Zone, SubZone                vprocore.ProjectMetadataCell
	Fields                       []sourceform.SIVIParentSharedReference
	Policies                     []TwoPageEntryFieldPolicy
	MasterEditingAvailable       bool
	ProjectChoices               *sourceform.SIVIProjectChoices
	ProjectAssignmentAvailable   bool
	ProjectAssignmentDiagnostic  string
}

type TwoPageEntryReferenceSnapshotInternal = TwoPageEntryReferenceSnapshot

func ReadTwoPageEntryReferenceFields(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext,
	provider *TwoPageEntryReferenceProvider, contextID, form string, selection TwoPageEntryReferenceSelection,
	aliases TwoPageEntryReferenceAliases, zone, subZone vprocore.ProjectMetadataCell) ([]sourceform.SIVIParentSharedReference, error) {
	return readTwoPageEntryReferenceFieldsWithHistoricalFilters(ctx, tx, owner, provider, contextID, form, selection,
		aliases, zone, subZone, false)
}

func readTwoPageEntryReferenceFieldsWithHistoricalFilters(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext,
	provider *TwoPageEntryReferenceProvider, contextID, form string, selection TwoPageEntryReferenceSelection,
	aliases TwoPageEntryReferenceAliases, zone, subZone vprocore.ProjectMetadataCell, historical bool) ([]sourceform.SIVIParentSharedReference, error) {
	if tx == nil || owner == nil || provider == nil {
		return nil, errors.New("complete-entry references require the owned transaction and catalogue provider")
	}
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := verifyTwoPageEntryReferenceAliases(ctx, tx, owner, map[string]string{aliases.Project: "project", aliases.Lists: "VLists"}); err != nil {
		return nil, err
	}
	fields, err := twoPageEntryReferenceFields(form)
	if err != nil {
		return nil, err
	}
	if len(provider.Fields) != len(fields) {
		return nil, errors.New("complete-entry reference provider belongs to another source form")
	}
	for _, field := range fields {
		if expected, present := provider.Fields[field.Column]; !present || expected != field {
			return nil, errors.New("complete-entry reference provider changed its exact source policies")
		}
	}
	result := []sourceform.SIVIParentSharedReference{}
	for _, field := range fields {
		var reference sourceform.SIVIParentSharedReference
		var err error
		if historical && (field.Column == "SubZone" || field.Column == "SiteSeries") {
			_, filterError := twoPageEntryBECFilter("Zone", zone, 4)
			if filterError == nil && field.Column == "SiteSeries" {
				_, filterError = twoPageEntryBECFilter("SubZone", subZone, 8)
			}
			if filterError != nil {
				result = append(result, sourceform.SIVIParentSharedReference{Column: field.Column, ListName: field.List,
					Required: field.Required, Source: "owned historical BEC filters",
					Diagnostic:  "Historical BEC filter cannot query choices: " + filterError.Error(),
					Definitions: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
					Choices:     []sourceform.SIVIParentSharedReferenceChoice{}})
				continue
			}
		}
		switch field.Reader {
		case "project", "master-unit", "working-unit":
			reference, err = ReadTwoPageEntryContextReference(ctx, tx, owner, contextID, form, selection, aliases, field)
		default:
			reference, err = provider.Read(ctx, tx, aliases.Lists, field, zone, subZone)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, reference)
	}
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func ReadTwoPageEntryReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string,
	zone, subZone vprocore.ProjectMetadataCell, readers TwoPageEntryReferenceReaders) (*TwoPageEntryReferenceSnapshotInternal, error) {
	return ReadTwoPageEntryReferenceSnapshot(s, ctx, contextID, plot, form, readers,
		&TwoPageEntryReferenceFilters{zone, subZone})
}

func ReadTwoPageEntryReferenceSnapshot(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string,
	readers TwoPageEntryReferenceReaders, filters *TwoPageEntryReferenceFilters) (*TwoPageEntryReferenceSnapshotInternal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if filters != nil {
		if _, err := twoPageEntryBECFilter("Zone", filters.Zone, 4); err != nil {
			return nil, err
		}
		if _, err := twoPageEntryBECFilter("SubZone", filters.SubZone, 8); err != nil {
			return nil, err
		}
	}
	provider, err := NewTwoPageEntryReferenceProvider(form, readers)
	if err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*TwoPageEntryReferenceSnapshotInternal, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*TwoPageEntryReferenceSnapshotInternal, error) {
			values, err := plots.Projects.Preferences.Snapshot()
			if err != nil {
				return nil, err
			}
			selection, err := TwoPageEntryReferenceSelectionFromConfig(values)
			if err != nil {
				return nil, err
			}
			tables, err := sourceform.ReadSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			original, err := ProjectTwoPageParent(ctx, contextID, owner.Selection.Project, plot, form, tables[0], tables[1])
			if err != nil {
				return nil, err
			}
			if len(original.Rows) != 1 {
				return nil, errors.New("complete-entry snapshot requires one owned physical source pair")
			}
			policies, err := TwoPageEntryFieldPolicies(form)
			if err != nil {
				return nil, err
			}
			var zone, subZone vprocore.ProjectMetadataCell
			if filters != nil {
				zone, subZone = filters.Zone, filters.SubZone
			} else {
				for _, binding := range original.Bindings {
					switch binding.Binding {
					case "Zone":
						zone = original.Rows[0].Env.Cells[binding.Column]
					case "SubZone":
						subZone = original.Rows[0].Env.Cells[binding.Column]
					}
				}
			}
			fields, err := readTwoPageEntryReferenceFieldsWithHistoricalFilters(ctx, tx, owner, provider, contextID, form, selection,
				TwoPageEntryReferenceAliases{"project", "VLists", "su"}, zone, subZone, filters == nil)
			if err != nil {
				return nil, err
			}
			choices, err := sourceform.ReadSIVIProjectChoicesAtAlias(ctx, owner, tx, contextID, selection.ProjectSource, "project")
			if err != nil {
				return nil, err
			}
			available, diagnostic, err := sourceform.SiviProjectAssignmentAvailability(ctx, tx, selection.ProjectSource)
			if err != nil {
				return nil, err
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &TwoPageEntryReferenceSnapshotInternal{Original: original, ProjectSource: selection.ProjectSource,
				WorkingSource: selection.WorkingSource, Zone: vprocore.CloneSiteUnitCell(zone), SubZone: vprocore.CloneSiteUnitCell(subZone),
				Fields: fields, Policies: policies, MasterEditingAvailable: vprocore.MasterBECAllowed(plots.CurrentUser),
				ProjectChoices: choices, ProjectAssignmentAvailable: available,
				ProjectAssignmentDiagnostic: diagnostic}, nil
		})
	})
}
