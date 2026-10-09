package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

type ProjectMetadataOption struct {
	RowID       string  `json:"rowId"`
	Value       *string `json:"value"`
	Description *string `json:"description"`
}

type ProjectMetadataEditorField struct {
	Name            string                  `json:"name"`
	Kind            string                  `json:"kind"`
	Maximum         int                     `json:"maximum"`
	Collection      bool                    `json:"collection"`
	ReferenceList   string                  `json:"referenceList"`
	ReferenceColumn string                  `json:"referenceColumn"`
	LimitToList     bool                    `json:"limitToList"`
	Options         []ProjectMetadataOption `json:"options"`
}

func (s *ContextService) ListProjectMetadataFields(ctx context.Context, contextID string) ([]ProjectMetadataEditorField, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]ProjectMetadataEditorField, error) {
		fields := make([]ProjectMetadataEditorField, 0, len(projectMetadataFields))
		cache := map[string][]ProjectMetadataOption{}
		names := make([]string, 0, len(projectMetadataFields))
		for name := range projectMetadataFields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			policy := projectMetadataFields[name]
			options := []ProjectMetadataOption{}
			if policy.referenceList != "" {
				key := policy.referenceList + ":" + policy.referenceColumn
				if found, present := cache[key]; present {
					options = found
				} else {
					var err error
					options, err = readProjectMetadataOptions(ctx, plots.projects.sqlite, policy.referenceList, policy.referenceColumn)
					if err != nil {
						return nil, fmt.Errorf("metadata %s references unavailable: %w", name, err)
					}
					cache[key] = options
				}
			}
			list, column := policy.referenceList, policy.referenceColumn
			if alias, history := metadataHistoryFields[name]; history {
				table := plots.projects.sqlite.selection.Project + "_Metadata"
				if alias == "VMetaData" {
					table = "ProjectMetaData"
				}
				var err error
				options, err = readProjectMetadataHistory(ctx, plots.projects.sqlite, alias, table, name)
				if err != nil {
					return nil, fmt.Errorf("metadata %s history suggestions unavailable: %w", name, err)
				}
				list, column = alias+"."+table, name
			}
			fields = append(fields, ProjectMetadataEditorField{name, policy.kind, policy.maximum, policy.options,
				list, column, policy.limitToList, options})
		}

		return fields, nil
	})
}

var metadataHistoryFields = map[string]string{
	"ProjectTitle": "VMetaData", "CoordinatingAgency": "project", "FieldLeader": "project",
	"FieldCompanyAgency": "project", "ProponentFunder": "project", "FieldDataCollectionTeam": "project",
	"GeographicStudyArea": "project", "ProjectPurpose": "project", "DataCustodian": "project",
	"StorageLocation": "project",
}

func readProjectMetadataHistory(ctx context.Context, c *sqliteContext, alias, table, column string) ([]ProjectMetadataOption, error) {
	field := quoteHeaderIdentifier(column)
	query := `SELECT rowid,typeof(` + field + `),CAST(` + field + ` AS BLOB) FROM ` +
		quoteHeaderIdentifier(alias) + "." + quoteHeaderIdentifier(table)
	if alias == "VMetaData" {
		query += " WHERE " + field + " IS NOT NULL"
	}
	rows, err := c.conn.QueryContext(ctx, query+" ORDER BY "+field+" COLLATE BINARY,rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := []ProjectMetadataOption{}
	for rows.Next() {
		var rowID int64
		var storage string
		var raw []byte
		if err := rows.Scan(&rowID, &storage, &raw); err != nil {
			return nil, err
		}
		option := ProjectMetadataOption{RowID: strconv.FormatInt(rowID, 10)}
		if storage != "null" {
			if storage != "text" {
				return nil, errors.New("metadata history contains unsupported storage; no suggestion was converted")
			}
			value, err := projectMetadataCell(storage, raw)
			if err != nil {
				return nil, err
			}
			option.Value = value.Text
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

func readProjectMetadataOptions(ctx context.Context, c *sqliteContext, list, column string) ([]ProjectMetadataOption, error) {
	field := quoteHeaderIdentifier(column)
	rows, err := c.conn.QueryContext(ctx, `SELECT rowid,typeof(`+field+`),CAST(`+field+` AS BLOB),
		typeof(ItemDescription),CAST(ItemDescription AS BLOB) FROM VLists.USysTableOfLists
		WHERE ListName COLLATE BINARY=? ORDER BY ItemOrder,rowid`, list)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ProjectMetadataOption{}
	for rows.Next() {
		var id int64
		var valueStorage, descriptionStorage string
		var rawValue, rawDescription []byte
		if err := rows.Scan(&id, &valueStorage, &rawValue, &descriptionStorage, &rawDescription); err != nil {
			return nil, err
		}
		option := ProjectMetadataOption{RowID: strconv.FormatInt(id, 10)}
		for _, entry := range []struct {
			storage string
			raw     []byte
			target  **string
		}{{valueStorage, rawValue, &option.Value}, {descriptionStorage, rawDescription, &option.Description}} {
			if entry.storage == "null" {
				continue
			}
			if entry.storage != "text" {
				return nil, errors.New("metadata reference contains unsupported historical storage; no code was converted")
			}
			cell, err := projectMetadataCell("text", entry.raw)
			if err != nil {
				return nil, err
			}
			*entry.target = cell.Text
		}
		result = append(result, option)
	}
	return result, rows.Err()
}
