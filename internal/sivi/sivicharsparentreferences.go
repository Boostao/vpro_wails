package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SIVICHARSParentEntryReview struct {
	Original *sourceform.SiviParentProjection
	Fields   []sourceform.SIVIParentSharedReference
	Policies []SIVICHARSParentEntryPolicy
}

type siviCHARSParentReferenceSnapshot = SIVICHARSParentEntryReview

func siviCHARSParentReferenceFields() ([]sourceform.SiviParentSharedReferencePolicy, error) {
	layout, err := siviCHARSParentSourceLayout()
	if err != nil {
		return nil, err
	}
	fields := []sourceform.SiviParentSharedReferencePolicy{}
	seen := map[string]bool{}
	for _, control := range layout.Forms[0].Fields {
		if control.Type != "ComboBox" || control.Binding == "ProjectID" {
			continue
		}
		field, exists := sourceform.SiviParentSharedReferenceField(control.Binding)
		switch control.Binding {
		case "RootZoneParticleSize":
			field, exists = sourceform.SiviParentSharedReferencePolicy{Column: "RootZoneParticleSize", List: "RootZoneParticleSize", Reader: "parent", Maximum: 6, Required: false}, true
		case "SnowCoverregime":
			field, exists = sourceform.SiviParentSharedReferencePolicy{Column: "SnowCoverregime", List: "SnowCoverRegime", Reader: "family", Maximum: 1, Required: false}, true
		case "SV_AhorizonType":
			field, exists = sourceform.SiviParentSharedReferencePolicy{Column: "SV_AhorizonType", List: "SV_AhorizonType", Reader: "chars-ahorizon", Maximum: 5, Required: false}, true
		}
		if !exists || seen[control.Binding] {
			return nil, fmt.Errorf("CHARS reference %s requires one exact source policy", control.Binding)
		}
		limit, present := control.Properties["LimitToList"]
		if present && limit.Value != "NotDefault" || field.Required != present {
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

func readSIVICHARSParentReference(ctx context.Context, tx *sql.Tx, readers sourceform.SiviParentSharedReferenceReaders,
	field sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
	return readSIVICHARSParentReferenceAtAlias(ctx, tx, "VLists", readers, field)
}

func readSIVICHARSParentReferenceAtAlias(ctx context.Context, tx *sql.Tx, alias string, readers sourceform.SiviParentSharedReferenceReaders,
	field sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
	if err := ctx.Err(); err != nil {
		return sourceform.SIVIParentSharedReference{}, err
	}
	if tx == nil {
		return sourceform.SIVIParentSharedReference{}, errors.New("CHARS references require the owned read transaction")
	}
	if alias != "VLists" && alias != "chars_entry_refs" {
		return sourceform.SIVIParentSharedReference{}, errors.New("CHARS references require the exact review or writer attachment alias")
	}
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		return sourceform.SIVIParentSharedReference{}, err
	}
	matched := false
	for _, expected := range fields {
		if expected == field {
			matched = true
			break
		}
	}
	if !matched {
		return sourceform.SIVIParentSharedReference{}, errors.New("CHARS reference requires its exact source field policy")
	}
	result := sourceform.SIVIParentSharedReference{Column: field.Column, ListName: field.List, Required: field.Required,
		Definitions: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
		Choices:     []sourceform.SIVIParentSharedReferenceChoice{}}
	switch field.Column {
	case "SubZone":
		result.Source = "original CHARS SubZone RowSource"
		result.Diagnostic = "CHARS source targets frmSIVIsite.cmbZone; runtime resolution is unqualified, so SubZone choices remain unavailable"
		return result, nil
	case "SV_AhorizonType":
		result.Source = "original CHARS literal Ah/Ae/blank value-list; no physical catalogue rowids"
		result.Definitions.Columns = []vprocore.ProjectMetadataColumn{{Name: "Item", DeclaredType: "TEXT"}}
		for ordinal, code := range []string{"Ah", "Ae", ""} {
			rowID := fmt.Sprintf("source-value-list:%d", ordinal)
			result.Definitions.Rows = append(result.Definitions.Rows, vprocore.ProjectMetadataRow{RowID: rowID,
				Cells: []vprocore.ProjectMetadataCell{sourceform.SiviReferenceText(&code)}})
			choice := sourceform.SIVIParentSharedReferenceChoice{RowID: rowID, Code: sourceform.SiviReferenceText(&code).Text, Selectable: code != ""}
			if code == "" {
				choice.Diagnostic = "The source blank choice does not qualify bound NULL versus empty TEXT assignment"
			}
			result.Choices = append(result.Choices, choice)
		}
		result.Available = true
	default:
		shared := sourceform.SIVIParentSharedService{References: readers}
		var err error
		result, err = shared.ReadReference(ctx, tx, alias, field, vprocore.ProjectMetadataCell{Storage: "null"})
		if err != nil {
			return sourceform.SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference read: %w", field.Column, err)
		}
	}
	return ownSIVICHARSParentReference(ctx, result)
}

func ownSIVICHARSParentReference(ctx context.Context, reference sourceform.SIVIParentSharedReference) (sourceform.SIVIParentSharedReference, error) {
	reference.Definitions.Columns = append([]vprocore.ProjectMetadataColumn{}, reference.Definitions.Columns...)
	rows := make([]vprocore.ProjectMetadataRow, 0, len(reference.Definitions.Rows))
	for _, row := range reference.Definitions.Rows {
		if err := ctx.Err(); err != nil {
			return sourceform.SIVIParentSharedReference{}, err
		}
		if _, err := vprocore.MetadataCellValue(sourceform.SiviReferenceText(&row.RowID)); err != nil {
			return sourceform.SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference row identity: %w", reference.Column, err)
		}
		cells := make([]vprocore.ProjectMetadataCell, 0, len(row.Cells))
		for _, cell := range row.Cells {
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
				return sourceform.SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference definition: %w", reference.Column, err)
			}
			cells = append(cells, vprocore.CloneSiteUnitCell(cell))
		}
		rows = append(rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	reference.Definitions.Rows = rows
	reference.Choices = append([]sourceform.SIVIParentSharedReferenceChoice{}, reference.Choices...)
	for i := range reference.Choices {
		choice := &reference.Choices[i]
		for _, text := range []*string{&choice.RowID, &choice.Diagnostic} {
			if _, err := vprocore.MetadataCellValue(sourceform.SiviReferenceText(text)); err != nil {
				return sourceform.SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference choice: %w", reference.Column, err)
			}
		}
		code, description := sourceform.SiviReferenceText(choice.Code), sourceform.SiviReferenceText(choice.Description)
		for _, cell := range []vprocore.ProjectMetadataCell{code, description} {
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
				return sourceform.SIVIParentSharedReference{}, fmt.Errorf("CHARS %s reference choice: %w", reference.Column, err)
			}
		}
		choice.Code, choice.Description = code.Text, description.Text
	}
	return reference, ctx.Err()
}

func ReadSIVICHARSParentReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot string,
	readers sourceform.SiviParentSharedReferenceReaders) (*siviCHARSParentReferenceSnapshot, error) {
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		return nil, err
	}
	policies, err := siviCHARSParentEntryPolicies()
	if err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviCHARSParentReferenceSnapshot, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*siviCHARSParentReferenceSnapshot, error) {
			tables, err := sourceform.ReadSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			parent, err := projectSIVICHARSParent(ctx, contextID, owner.Selection.Project, plot, tables[0], tables[1])
			if err != nil {
				return nil, err
			}
			if len(parent.Rows) != 1 {
				return nil, errors.New("CHARS entry references require one physical Env/Admin pair")
			}
			result := &siviCHARSParentReferenceSnapshot{Original: parent, Fields: []sourceform.SIVIParentSharedReference{}, Policies: policies}
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
