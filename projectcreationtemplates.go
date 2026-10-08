package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

var newProjectTemplateSources = [...]struct {
	suffix, table    string
	columns, indexes int
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
	Schema       []sqliteSchemaObject
	Descriptions ProjectMetadataTable
}

type newProjectTemplateReview struct {
	ContextID                  string
	ProjectPath                string
	TemplatePath               string
	DescriptionMetadataPresent bool
	Templates                  []newProjectTemplate
}

func readNewProjectTemplateSnapshot(ctx context.Context, tx projectMetadataQueryer) ([]newProjectTemplate, bool, error) {
	descriptionColumns, err := profileMetadataColumns(ctx, tx, "VPro64")
	if err != nil {
		return nil, false, err
	}
	if descriptionColumns != nil {
		if reason := profileMetadataReason(descriptionColumns); reason != "" {
			return nil, false, fmt.Errorf("new-project template descriptions unavailable: %s", reason)
		}
	}
	templates := make([]newProjectTemplate, 0, len(newProjectTemplateSources))
	for _, source := range newProjectTemplateSources {
		schema, err := readSQLiteSchemaObjects(ctx, tx, "VPro64", source.table)
		if err != nil {
			return nil, false, err
		}
		tables, indexes := 0, 0
		for _, object := range schema {
			if object.Table != source.table || object.SQL != nil && !utf8.ValidString(*object.SQL) {
				return nil, false, fmt.Errorf("template %s has invalid physical schema evidence", source.table)
			}
			switch object.Type {
			case "table":
				if object.Name != source.table || object.SQL == nil || *object.SQL == "" {
					return nil, false, fmt.Errorf("template %s lacks its original table definition", source.table)
				}
				tables++
			case "index":
				indexes++
			case "trigger":
				if object.SQL == nil || *object.SQL == "" {
					return nil, false, fmt.Errorf("template %s has an unavailable trigger definition", source.table)
				}
			default:
				return nil, false, fmt.Errorf("template %s must be a physical table, not %s", source.table, object.Type)
			}
		}
		if tables != 1 || indexes != source.indexes {
			return nil, false, fmt.Errorf("template %s original table/index set is unavailable; no inferred repair", source.table)
		}
		original, err := readSQLiteStorageRows(ctx, tx, "VPro64", source.table, "", nil, "")
		if err != nil {
			return nil, false, err
		}
		if len(original.Columns) != source.columns || len(original.Rows) != 0 {
			return nil, false, fmt.Errorf("template %s must retain its original %d columns and be empty; no data copied", source.table, source.columns)
		}
		descriptions, err := readProfileDescriptions(ctx, tx, "VPro64", source.table)
		if err != nil {
			return nil, false, err
		}
		templates = append(templates, newProjectTemplate{source.suffix, source.table, original, schema, descriptions})
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return templates, descriptionColumns != nil, nil
}

func (s *ContextService) readNewProjectTemplates(ctx context.Context, contextID string) (*newProjectTemplateReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*newProjectTemplateReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*newProjectTemplateReview, error) {
			path := owner.attachments["VPro64"]
			if path == "" {
				return nil, errors.New("original VPro64 template file is not owned by this context")
			}
			templates, present, err := readNewProjectTemplateSnapshot(ctx, tx)
			if err != nil {
				return nil, err
			}
			return &newProjectTemplateReview{contextID, owner.selection.ProjectPath, path, present, templates}, nil
		})
	})
}
