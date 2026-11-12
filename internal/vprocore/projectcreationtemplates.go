package vprocore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

var NewProjectTemplateSources = [...]struct {
	Suffix, Table    string `json:"-" yaml:"-"`
	Columns, indexes int    `json:"-" yaml:"-"`
}{
	{"Env", "USysEnvTable", 112, 3},
	{"Veg", "USysVegTable", 44, 3},
	{"Other", "USysOtherTable", 11, 1},
	{"Humus", "USysSoilHumusTable", 18, 1},
	{"Mineral", "USysSoilMineralTable", 28, 1},
	{"Audit", "USysAudit", 11, 1},
	{"Metadata", "USysMetadataTable", 75, 4},
	{"Admin", "USysAdminTable", 20, 1},
}

type newProjectTemplate struct {
	Suffix       string
	SourceTable  string
	Original     ProjectMetadataTable
	Schema       []SqliteSchemaObject
	Descriptions ProjectMetadataTable
}

type NewProjectTemplateReview struct {
	ContextID                  string
	ProjectPath                string
	TemplatePath               string
	DescriptionMetadataPresent bool
	Templates                  []newProjectTemplate
}

func ReadNewProjectTemplateSnapshot(ctx context.Context, tx ProjectMetadataQueryer) ([]newProjectTemplate, bool, error) {
	descriptionColumns, err := ProfileMetadataColumns(ctx, tx, "VPro64")
	if err != nil {
		return nil, false, err
	}
	if descriptionColumns != nil {
		if reason := ProfileMetadataReason(descriptionColumns); reason != "" {
			return nil, false, fmt.Errorf("new-project template descriptions unavailable: %s", reason)
		}
	}
	templates := make([]newProjectTemplate, 0, len(NewProjectTemplateSources))
	for _, source := range NewProjectTemplateSources {
		schema, err := ReadSQLiteSchemaObjects(ctx, tx, "VPro64", source.Table)
		if err != nil {
			return nil, false, err
		}
		tables, indexes := 0, 0
		for _, object := range schema {
			if object.Table != source.Table || object.SQL != nil && !utf8.ValidString(*object.SQL) {
				return nil, false, fmt.Errorf("template %s has invalid physical schema evidence", source.Table)
			}
			switch object.Type {
			case "table":
				if object.Name != source.Table || object.SQL == nil || *object.SQL == "" {
					return nil, false, fmt.Errorf("template %s lacks its original table definition", source.Table)
				}
				tables++
			case "index":
				indexes++
			case "trigger":
				if object.SQL == nil || *object.SQL == "" {
					return nil, false, fmt.Errorf("template %s has an unavailable trigger definition", source.Table)
				}
			default:
				return nil, false, fmt.Errorf("template %s must be a physical table, not %s", source.Table, object.Type)
			}
		}
		if tables != 1 || indexes != source.indexes {
			return nil, false, fmt.Errorf("template %s original table/index set is unavailable; no inferred repair", source.Table)
		}
		original, err := ReadSQLiteStorageRows(ctx, tx, "VPro64", source.Table, "", nil, "")
		if err != nil {
			return nil, false, err
		}
		if len(original.Columns) != source.Columns || len(original.Rows) != 0 {
			return nil, false, fmt.Errorf("template %s must retain its original %d columns and be empty; no data copied", source.Table, source.Columns)
		}
		descriptions, err := ReadProfileDescriptions(ctx, tx, "VPro64", source.Table)
		if err != nil {
			return nil, false, err
		}
		templates = append(templates, newProjectTemplate{source.Suffix, source.Table, original, schema, descriptions})
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return templates, descriptionColumns != nil, nil
}

func (s *ContextService) ReadNewProjectTemplates(ctx context.Context, contextID string) (*NewProjectTemplateReview, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*NewProjectTemplateReview, error) {
		return WithOwnedSIVISnapshot(ctx, plots, func(owner *SqliteContext, tx *sql.Tx) (*NewProjectTemplateReview, error) {
			path := owner.Attachments["VPro64"]
			if path == "" {
				return nil, errors.New("original VPro64 template file is not owned by this context")
			}
			templates, present, err := ReadNewProjectTemplateSnapshot(ctx, tx)
			if err != nil {
				return nil, err
			}
			return &NewProjectTemplateReview{contextID, owner.Selection.ProjectPath, path, present, templates}, nil
		})
	})
}
