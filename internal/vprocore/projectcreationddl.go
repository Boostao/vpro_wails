package vprocore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type newProjectSchemaMapping struct {
	Source     SqliteSchemaObject
	TargetName string
	SQL        *string
}

type newProjectDDLTable struct {
	SourceTable, TargetTable string
	Schema                   []newProjectSchemaMapping
}

type newProjectDDLPlan struct {
	ContextID, Project, ProjectPath, TemplatePath string
	Tables                                        []newProjectDDLTable
}

func MapNewProjectSchemaObject(object SqliteSchemaObject, template, target string) (newProjectSchemaMapping, error) {
	mapping := newProjectSchemaMapping{Source: object}
	if object.Table != template || object.Name == "" {
		return newProjectSchemaMapping{}, errors.New("new-project schema object has foreign or incomplete identity")
	}
	if object.Type == "index" && object.SQL == nil {
		if !strings.HasPrefix(object.Name, "sqlite_autoindex_"+template+"_") {
			return newProjectSchemaMapping{}, errors.New("new-project explicit index lacks its original SQL")
		}
		return mapping, nil
	}
	if object.SQL == nil || !utf8.ValidString(*object.SQL) {
		return newProjectSchemaMapping{}, errors.New("new-project schema object lacks valid original SQL")
	}
	original := *object.SQL
	mapping.Source.SQL = &original
	var prefix, replacement string
	switch object.Type {
	case "table":
		if object.Name != template {
			return newProjectSchemaMapping{}, errors.New("new-project table name differs from its source template")
		}
		mapping.TargetName = target
		prefix = "CREATE TABLE " + QuoteHeaderIdentifier(template)
		replacement = "CREATE TABLE " + QuoteHeaderIdentifier(target)
	case "index":
		keyword := "CREATE INDEX "
		if strings.HasPrefix(original, "CREATE UNIQUE INDEX ") {
			keyword = "CREATE UNIQUE INDEX "
		}
		mapping.TargetName = target + "__" + object.Name
		prefix = keyword + QuoteHeaderIdentifier(object.Name) + " ON " + QuoteHeaderIdentifier(template)
		replacement = keyword + QuoteHeaderIdentifier(mapping.TargetName) + " ON " + QuoteHeaderIdentifier(target)
	default:
		return newProjectSchemaMapping{}, fmt.Errorf("new-project %s mapping unavailable; no inferred schema rewrite", object.Type)
	}
	if !strings.HasPrefix(original, prefix) || !strings.HasPrefix(strings.TrimLeft(original[len(prefix):], " \t\r\n"), "(") {
		return newProjectSchemaMapping{}, errors.New("new-project mapping requires the observed literal SQLite CREATE header")
	}
	sql := replacement + original[len(prefix):]
	mapping.SQL = &sql
	return mapping, nil
}

func PlanNewProjectDDL(ctx context.Context, project string, review *NewProjectTemplateReview) (*newProjectDDLPlan, error) {
	if !ProjectNamePattern.MatchString(project) || review == nil || review.ContextID == "" ||
		review.ProjectPath == "" || review.TemplatePath == "" || len(review.Templates) != len(NewProjectTemplateSources) {
		return nil, errors.New("new-project schema plan requires literal project identity and a complete owned template review")
	}
	plan := &newProjectDDLPlan{review.ContextID, project, review.ProjectPath, review.TemplatePath,
		make([]newProjectDDLTable, 0, len(NewProjectTemplateSources))}
	for i, source := range NewProjectTemplateSources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		template := review.Templates[i]
		if template.Suffix != source.Suffix || template.SourceTable != source.Table ||
			len(template.Original.Columns) != source.Columns || template.Original.Rows == nil || len(template.Original.Rows) != 0 {
			return nil, errors.New("new-project schema plan requires original ordered empty templates; no data/schema repair")
		}
		table := newProjectDDLTable{source.Table, project + "_" + source.Suffix, []newProjectSchemaMapping{}}
		tables, indexes := 0, 0
		seen := map[string]bool{}
		for _, object := range template.Schema {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			key := object.Type + "\x00" + object.Name
			if seen[key] {
				return nil, errors.New("new-project schema plan rejects duplicate object definitions")
			}
			seen[key] = true
			mapping, err := MapNewProjectSchemaObject(object, source.Table, table.TargetTable)
			if err != nil {
				return nil, err
			}
			switch object.Type {
			case "table":
				tables++
				table.Schema = append([]newProjectSchemaMapping{mapping}, table.Schema...)
			case "index":
				indexes++
				table.Schema = append(table.Schema, mapping)
			}
		}
		if tables != 1 || indexes != source.indexes {
			return nil, errors.New("new-project schema plan lacks its original table/index set")
		}
		plan.Tables = append(plan.Tables, table)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return plan, nil
}
