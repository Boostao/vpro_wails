package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

type siviCHARSParentField struct {
	column, binding, owner, kind string
	maximum                      int
	allowEmpty                   bool
}

func siviCHARSParentFields() (map[string]siviCHARSParentField, error) {
	bindings, err := siviCHARSParentSourceBindings()
	if err != nil {
		return nil, err
	}
	fields := make(map[string]siviCHARSParentField)
	ordinaryNumbers := map[string]vprocore.OrdinaryNumberField{}
	for _, field := range vprocore.OrdinaryNumberFields(vprocore.FS882Header{}) {
		ordinaryNumbers[field.Name] = field
	}
	for _, binding := range bindings {
		if binding.Implicit || binding.Binding == "PlotNumber" || binding.Binding == "ProjectID" || binding.Binding == "PlotType" {
			continue
		}
		column := binding.Binding
		if column == "siteNotes" {
			column = "SiteNotes"
		}
		field := siviCHARSParentField{column: column, binding: binding.Binding}
		switch {
		case sourceform.SiviParentScalarOwners[column] != "":
			field.owner, field.kind = sourceform.SiviParentScalarOwners[column], "single"
			if column == "SV_FloodPlain" {
				field.kind = "boolean"
			}
		case column == "SV_StandAgeEstMeas" || column == "SV_StandHeightEstMeas":
			field.owner, field.kind, field.maximum = "Env", "option", 2
		case column == "SV_PolygonNumber" || column == "SV_CanopyComposition":
			field.owner, field.kind, field.maximum = "Env", "text", 25
			if column == "SV_CanopyComposition" {
				field.maximum = 50
			}
		case column == "SnowCoverregime" || column == "SV_AhorizonType":
			field.owner, field.kind, field.maximum, field.allowEmpty = "Env", "text", 1, true
			if column == "SV_AhorizonType" {
				field.maximum = 5
			}
		case sourceform.SiviParentSharedOwners[column] != "":
			field.owner = sourceform.SiviParentSharedOwners[column]
			if kind := sourceform.SiviParentSharedNumberKinds[column]; kind != "" {
				field.kind = kind
				if kind == "latitude" || kind == "longitude" {
					field.kind = "double"
				}
			} else if column == "Date" {
				field.kind = "date"
			} else if maximum, text := sourceform.SiviParentSharedTextBounds[column]; text {
				field.kind, field.maximum = "text", maximum
			} else if column == "VegNotes" {
				field.kind = "text"
			} else if number, exists := ordinaryNumbers[column]; exists && !number.IsInteger && number.Table == field.owner {
				field.kind = "single"
			}
		case column == "RootZoneParticleSize":
			for _, descriptor := range vprocore.ParentCodeDescriptors {
				if descriptor.Name == column {
					field.owner, field.kind, field.maximum = descriptor.Table, "text", descriptor.Maximum
				}
			}
		case column == "SV_WaterTableCM":
			source := sourceform.TwoPageParentExtraFields[column]
			field.owner, field.kind = source.Owner, source.Domain
		default:
			if reference, exists := sourceform.SiviParentSharedReferenceField(column); exists {
				field.owner, field.kind, field.maximum = "Env", "text", reference.Maximum
			}
		}
		if field.owner == "" || field.kind == "" {
			return nil, fmt.Errorf("CHARS SIVI %s has no reviewed editable source domain", column)
		}
		if _, exists := fields[column]; exists {
			return nil, fmt.Errorf("CHARS SIVI field %s repeats a physical owner", column)
		}
		fields[column] = field
	}
	if len(fields) != 68 {
		return nil, errors.New("CHARS SIVI requires68 direct editable domains excluding identity/ProjectID/PlotType")
	}
	return fields, nil
}

func prepareSIVICHARSParentCell(field siviCHARSParentField, value vprocore.ProjectMetadataCell) (vprocore.ProjectMetadataCell, error) {
	if value.Storage == "null" {
		return value, nil
	}
	if field.kind == "single" || field.kind == "double" {
		if value.Storage != "real" || value.Real == nil {
			return vprocore.ProjectMetadataCell{}, fmt.Errorf("CHARS SIVI %s requires nullable REAL storage", field.column)
		}
		number := *value.Real
		if field.kind == "single" {
			number = float64(float32(number))
		}
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return vprocore.ProjectMetadataCell{}, fmt.Errorf("CHARS SIVI %s exceeds its finite source %s domain", field.column, field.kind)
		}
		if number == 0 && math.Signbit(number) {
			return vprocore.ProjectMetadataCell{}, fmt.Errorf("CHARS SIVI %s new negative zero cannot be preserved by SQLite REAL storage", field.column)
		}
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &number}, nil
	}
	return value, nil
}

func validateSIVICHARSParentCell(field siviCHARSParentField, value vprocore.ProjectMetadataCell) error {
	if value.Storage == "null" {
		return nil
	}
	switch field.kind {
	case "single", "double":
		_, err := prepareSIVICHARSParentCell(field, value)
		return err
	case "boolean":
		return sourceform.ValidateSIVIParentScalar("SV_FloodPlain", value)
	case "option":
		return sourceform.ValidateSIVIParentOption(field.column, value)
	case "integer":
		if err := sourceform.ValidateSIVIParentSharedNumber(field.column, "integer", value); err != nil {
			return err
		}
		if field.column == "StartDate" {
			year, err := strconv.Atoi(*value.Integer)
			if err != nil {
				return fmt.Errorf("CHARS SIVI StartDate: %w", err)
			}
			if year < 1900 || year > 2500 {
				return errors.New("CHARS SIVI StartDate requires the source table rule Between1900 And2500 or NULL")
			}
		}
		return nil
	case "text", "date":
		if value.Storage != "text" || value.Text == nil || strings.ContainsRune(*value.Text, 0) {
			return fmt.Errorf("CHARS SIVI %s requires nullable literal TEXT without NUL", field.column)
		}
		if field.kind == "date" {
			return sourceform.ValidateSIVIParentDateTimestamp(value.Text)
		}
		if field.allowEmpty && *value.Text == "" {
			return nil
		}
		return vprocore.ValidateOrdinaryText(vprocore.BecHeaderField{Name: field.column, Table: field.owner, Maximum: field.maximum, Value: value.Text})
	default:
		return fmt.Errorf("CHARS SIVI %s has an unavailable source domain", field.column)
	}
}

func planSIVICHARSParentEntry(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || original.Form != siviCHARSParentForm || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("CHARS SIVI planning requires one original CHARS physical pair and explicit edits")
	}
	parent, err := projectSIVICHARSParent(ctx, original.ContextID, original.Project, original.Plot,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(parent, original) {
		return nil, errors.New("CHARS SIVI planning requires unchanged canonical projection/binding identities")
	}
	fields, err := siviCHARSParentFields()
	if err != nil {
		return nil, err
	}
	owners, byBinding := map[string]string{}, map[string]siviCHARSParentField{}
	for _, field := range fields {
		owners[field.binding], byBinding[field.binding] = field.owner, field
	}
	sourceEdits := append([]sourceform.SiviParentScalarEdit(nil), edits...)
	for i, edit := range sourceEdits {
		field, exists := fields[edit.Column]
		if !exists {
			return nil, fmt.Errorf("CHARS SIVI field %s is readonly or outside its direct source scope", edit.Column)
		}
		sourceEdits[i].Column = field.binding
	}
	result, err := sourceform.PlanPreparedParentCells(ctx, original.ContextID, original.Project, parent, sourceEdits, owners, nil,
		func(column string, value vprocore.ProjectMetadataCell) (vprocore.ProjectMetadataCell, error) {
			return prepareSIVICHARSParentCell(byBinding[column], value)
		}, vprocore.MetadataCellEqual, func(column string, value vprocore.ProjectMetadataCell) error {
			return validateSIVICHARSParentCell(byBinding[column], value)
		})
	if err != nil {
		return nil, fmt.Errorf("CHARS SIVI direct-field planning: %w", err)
	}
	for i := range result {
		result[i].Column = byBinding[result[i].Column].column
	}
	return result, nil
}
