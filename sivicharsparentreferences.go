package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type siviCHARSParentReferenceSnapshot struct {
	Original *siviParentProjection
	Fields   []SIVIParentSharedReference
}

func siviCHARSParentReferenceFields() ([]siviParentSharedReferencePolicy, error) {
	layout, err := siviCHARSParentSourceLayout()
	if err != nil {
		return nil, err
	}
	fields := []siviParentSharedReferencePolicy{}
	seen := map[string]bool{}
	for _, control := range layout.Forms[0].Fields {
		if control.Type != "ComboBox" || control.Binding == "ProjectID" {
			continue
		}
		field, exists := siviParentSharedReferenceField(control.Binding)
		switch control.Binding {
		case "RootZoneParticleSize":
			field, exists = siviParentSharedReferencePolicy{"RootZoneParticleSize", "RootZoneParticleSize", "parent", 6, false}, true
		case "SnowCoverregime":
			field, exists = siviParentSharedReferencePolicy{"SnowCoverregime", "SnowCoverRegime", "family", 1, false}, true
		case "SV_AhorizonType":
			field, exists = siviParentSharedReferencePolicy{"SV_AhorizonType", "SV_AhorizonType", "chars-ahorizon", 5, false}, true
		}
		if !exists || seen[control.Binding] {
			return nil, fmt.Errorf("CHARS reference %s requires one exact source policy", control.Binding)
		}
		limit, present := control.Properties["LimitToList"]
		if present && limit.Value != "NotDefault" || field.required != present {
			return nil, fmt.Errorf("CHARS reference %s changed its source membership property", control.Binding)
		}
		seen[control.Binding] = true
		fields = append(fields, field)
	}
	if len(fields) != 31 {
		return nil, errors.New("CHARS parent requires exactly31 non-ProjectID ComboBox references")
	}
	return fields, nil
}

func readSIVICHARSParentReference(ctx context.Context, tx *sql.Tx, readers siviParentSharedReferenceReaders,
	field siviParentSharedReferencePolicy) (SIVIParentSharedReference, error) {
	if err := ctx.Err(); err != nil {
		return SIVIParentSharedReference{}, err
	}
	if tx == nil {
		return SIVIParentSharedReference{}, errors.New("CHARS references require the owned read transaction")
	}
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		return SIVIParentSharedReference{}, err
	}
	matched := false
	for _, expected := range fields {
		if expected == field {
			matched = true
			break
		}
	}
	if !matched {
		return SIVIParentSharedReference{}, errors.New("CHARS reference requires its exact source field policy")
	}
	result := SIVIParentSharedReference{Column: field.column, ListName: field.list, Required: field.required,
		Definitions: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}},
		Choices:     []SIVIParentSharedReferenceChoice{}}
	switch field.column {
	case "SubZone":
		result.Source = "original CHARS SubZone RowSource"
		result.Diagnostic = "CHARS source targets frmSIVIsite.cmbZone; runtime resolution is unqualified, so SubZone choices remain unavailable"
		return result, nil
	case "SV_AhorizonType":
		result.Source = "original CHARS literal Ah/Ae/blank value-list; no physical catalogue rowids"
		result.Definitions.Columns = []ProjectMetadataColumn{{"Item", "TEXT"}}
		for ordinal, code := range []string{"Ah", "Ae", ""} {
			rowID := fmt.Sprintf("source-value-list:%d", ordinal)
			result.Definitions.Rows = append(result.Definitions.Rows, ProjectMetadataRow{RowID: rowID,
				Cells: []ProjectMetadataCell{siviReferenceText(&code)}})
			choice := SIVIParentSharedReferenceChoice{RowID: rowID, Code: siviReferenceText(&code).Text, Selectable: code != ""}
			if code == "" {
				choice.Diagnostic = "The source blank choice does not qualify bound NULL versus empty TEXT assignment"
			}
			result.Choices = append(result.Choices, choice)
		}
		result.Available = true
	default:
		shared := SIVIParentSharedService{references: readers}
		var err error
		result, err = shared.readReference(ctx, tx, "VLists", field, ProjectMetadataCell{Storage: "null"})
		if err != nil {
			return SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference read: %w", field.column, err)
		}
	}
	return ownSIVICHARSParentReference(ctx, result)
}

func ownSIVICHARSParentReference(ctx context.Context, reference SIVIParentSharedReference) (SIVIParentSharedReference, error) {
	reference.Definitions.Columns = append([]ProjectMetadataColumn{}, reference.Definitions.Columns...)
	rows := make([]ProjectMetadataRow, 0, len(reference.Definitions.Rows))
	for _, row := range reference.Definitions.Rows {
		if err := ctx.Err(); err != nil {
			return SIVIParentSharedReference{}, err
		}
		if _, err := metadataCellValue(siviReferenceText(&row.RowID)); err != nil {
			return SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference row identity: %w", reference.Column, err)
		}
		cells := make([]ProjectMetadataCell, 0, len(row.Cells))
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil {
				return SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference definition: %w", reference.Column, err)
			}
			cells = append(cells, cloneSiteUnitCell(cell))
		}
		rows = append(rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	reference.Definitions.Rows = rows
	reference.Choices = append([]SIVIParentSharedReferenceChoice{}, reference.Choices...)
	for i := range reference.Choices {
		choice := &reference.Choices[i]
		for _, text := range []*string{&choice.RowID, &choice.Diagnostic} {
			if _, err := metadataCellValue(siviReferenceText(text)); err != nil {
				return SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference choice: %w", reference.Column, err)
			}
		}
		code, description := siviReferenceText(choice.Code), siviReferenceText(choice.Description)
		for _, cell := range []ProjectMetadataCell{code, description} {
			if _, err := metadataCellValue(cell); err != nil {
				return SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference choice: %w", reference.Column, err)
			}
		}
		choice.Code, choice.Description = code.Text, description.Text
	}
	return reference, ctx.Err()
}

func (s *ContextService) readSIVICHARSParentReferences(ctx context.Context, contextID, plot string,
	readers siviParentSharedReferenceReaders) (*siviCHARSParentReferenceSnapshot, error) {
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviCHARSParentReferenceSnapshot, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviCHARSParentReferenceSnapshot, error) {
			tables, err := readSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			parent, err := projectSIVICHARSParent(ctx, contextID, owner.selection.Project, plot, tables[0], tables[1])
			if err != nil {
				return nil, err
			}
			if len(parent.Rows) != 1 {
				return nil, errors.New("CHARS entry references require one physical Env/Admin pair")
			}
			result := &siviCHARSParentReferenceSnapshot{Original: parent, Fields: []SIVIParentSharedReference{}}
			for _, field := range fields {
				reference, err := readSIVICHARSParentReference(ctx, tx, readers, field)
				if err != nil {
					return nil, err
				}
				result.Fields = append(result.Fields, reference)
			}
			return result, nil
		})
	})
}
