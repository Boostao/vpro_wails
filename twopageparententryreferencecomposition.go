package main

import (
	"context"
	"database/sql"
	"errors"
)

type TwoPageEntryReferenceSnapshot struct {
	Original                     *siviParentProjection
	ProjectSource, WorkingSource int
	Zone, SubZone                ProjectMetadataCell
	Fields                       []SIVIParentSharedReference
	Policies                     []TwoPageEntryFieldPolicy
	MasterEditingAvailable       bool
	ProjectChoices               *SIVIProjectChoices
	ProjectAssignmentAvailable   bool
	ProjectAssignmentDiagnostic  string
}

type twoPageEntryReferenceSnapshot = TwoPageEntryReferenceSnapshot

func readTwoPageEntryReferenceFields(ctx context.Context, tx *sql.Tx, owner *sqliteContext,
	provider *twoPageEntryReferenceProvider, contextID, form string, selection twoPageEntryReferenceSelection,
	aliases twoPageEntryReferenceAliases, zone, subZone ProjectMetadataCell) ([]SIVIParentSharedReference, error) {
	return readTwoPageEntryReferenceFieldsWithHistoricalFilters(ctx, tx, owner, provider, contextID, form, selection,
		aliases, zone, subZone, false)
}

func readTwoPageEntryReferenceFieldsWithHistoricalFilters(ctx context.Context, tx *sql.Tx, owner *sqliteContext,
	provider *twoPageEntryReferenceProvider, contextID, form string, selection twoPageEntryReferenceSelection,
	aliases twoPageEntryReferenceAliases, zone, subZone ProjectMetadataCell, historical bool) ([]SIVIParentSharedReference, error) {
	if tx == nil || owner == nil || provider == nil {
		return nil, errors.New("complete-entry references require the owned transaction and catalogue provider")
	}
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := verifyTwoPageEntryReferenceAliases(ctx, tx, owner, map[string]string{aliases.project: "project", aliases.lists: "VLists"}); err != nil {
		return nil, err
	}
	fields, err := twoPageEntryReferenceFields(form)
	if err != nil {
		return nil, err
	}
	if len(provider.fields) != len(fields) {
		return nil, errors.New("complete-entry reference provider belongs to another source form")
	}
	for _, field := range fields {
		if expected, present := provider.fields[field.column]; !present || expected != field {
			return nil, errors.New("complete-entry reference provider changed its exact source policies")
		}
	}
	result := []SIVIParentSharedReference{}
	for _, field := range fields {
		var reference SIVIParentSharedReference
		var err error
		if historical && (field.column == "SubZone" || field.column == "SiteSeries") {
			_, filterError := twoPageEntryBECFilter("Zone", zone, 4)
			if filterError == nil && field.column == "SiteSeries" {
				_, filterError = twoPageEntryBECFilter("SubZone", subZone, 8)
			}
			if filterError != nil {
				result = append(result, SIVIParentSharedReference{Column: field.column, ListName: field.list,
					Required: field.required, Source: "owned historical BEC filters",
					Diagnostic:  "Historical BEC filter cannot query choices: " + filterError.Error(),
					Definitions: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}},
					Choices:     []SIVIParentSharedReferenceChoice{}})
				continue
			}
		}
		switch field.reader {
		case "project", "master-unit", "working-unit":
			reference, err = readTwoPageEntryContextReference(ctx, tx, owner, contextID, form, selection, aliases, field)
		default:
			reference, err = provider.read(ctx, tx, aliases.lists, field, zone, subZone)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, reference)
	}
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ContextService) readTwoPageEntryReferences(ctx context.Context, contextID, plot, form string,
	zone, subZone ProjectMetadataCell, readers twoPageEntryReferenceReaders) (*twoPageEntryReferenceSnapshot, error) {
	return s.readTwoPageEntryReferenceSnapshot(ctx, contextID, plot, form, readers,
		&TwoPageEntryReferenceFilters{zone, subZone})
}

func (s *ContextService) readTwoPageEntryReferenceSnapshot(ctx context.Context, contextID, plot, form string,
	readers twoPageEntryReferenceReaders, filters *TwoPageEntryReferenceFilters) (*twoPageEntryReferenceSnapshot, error) {
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
	provider, err := newTwoPageEntryReferenceProvider(form, readers)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageEntryReferenceSnapshot, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*twoPageEntryReferenceSnapshot, error) {
			values, err := plots.projects.preferences.snapshot()
			if err != nil {
				return nil, err
			}
			selection, err := twoPageEntryReferenceSelectionFromConfig(values)
			if err != nil {
				return nil, err
			}
			tables, err := readSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			original, err := projectTwoPageParent(ctx, contextID, owner.selection.Project, plot, form, tables[0], tables[1])
			if err != nil {
				return nil, err
			}
			if len(original.Rows) != 1 {
				return nil, errors.New("complete-entry snapshot requires one owned physical source pair")
			}
			policies, err := twoPageEntryFieldPolicies(form)
			if err != nil {
				return nil, err
			}
			var zone, subZone ProjectMetadataCell
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
				twoPageEntryReferenceAliases{"project", "VLists", "su"}, zone, subZone, filters == nil)
			if err != nil {
				return nil, err
			}
			choices, err := readSIVIProjectChoicesAtAlias(ctx, owner, tx, contextID, selection.projectSource, "project")
			if err != nil {
				return nil, err
			}
			available, diagnostic, err := siviProjectAssignmentAvailability(ctx, tx, selection.projectSource)
			if err != nil {
				return nil, err
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &twoPageEntryReferenceSnapshot{Original: original, ProjectSource: selection.projectSource,
				WorkingSource: selection.workingSource, Zone: cloneSiteUnitCell(zone), SubZone: cloneSiteUnitCell(subZone),
				Fields: fields, Policies: policies, MasterEditingAvailable: masterBECAllowed(plots.currentUser),
				ProjectChoices: choices, ProjectAssignmentAvailable: available,
				ProjectAssignmentDiagnostic: diagnostic}, nil
		})
	})
}
