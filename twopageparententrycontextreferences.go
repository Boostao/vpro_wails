package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"unicode/utf8"
)

type twoPageEntryReferenceSelection struct {
	projectSource, workingSource int
}

type twoPageEntryReferenceAliases struct {
	project, lists, su string
}

func verifyTwoPageEntryReferenceAliases(ctx context.Context, tx *sql.Tx, owner *sqliteContext, expected map[string]string) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA database_list`)
	if err != nil {
		return err
	}
	observed := map[string]string{}
	for rows.Next() {
		var index int
		var alias, path string
		if err := rows.Scan(&index, &alias, &path); err != nil {
			rows.Close()
			return err
		}
		observed[alias] = path
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	for alias, role := range expected {
		path, present := observed[alias]
		if !present || path == "" || owner.attachmentInfo[role] == nil {
			return fmt.Errorf("complete-entry reference alias %s is not owned", alias)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !os.SameFile(info, owner.attachmentInfo[role]) {
			return fmt.Errorf("complete-entry reference alias %s changed its owned %s identity", alias, role)
		}
	}
	return nil
}

func twoPageEntryReferenceSelectionFromConfig(values map[string]any) (twoPageEntryReferenceSelection, error) {
	project, err := configInt(values, "Current", "ProjectIdSource", 1, 2)
	if err != nil {
		return twoPageEntryReferenceSelection{}, err
	}
	working, err := configInt(values, "Current", "AssignedSuSource", 1, 3)
	if err != nil {
		return twoPageEntryReferenceSelection{}, err
	}
	return twoPageEntryReferenceSelection{project, working}, nil
}

func readTwoPageEntryContextReference(ctx context.Context, tx *sql.Tx, owner *sqliteContext, contextID, form string,
	selection twoPageEntryReferenceSelection, aliases twoPageEntryReferenceAliases,
	field twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
	result := SIVIParentSharedReference{Column: field.column, ListName: field.list, Required: field.required,
		Definitions: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}},
		Choices:     []SIVIParentSharedReferenceChoice{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if tx == nil || owner == nil || contextID == "" ||
		(aliases.project != "project" && aliases.project != "main") ||
		(aliases.lists != "VLists" && aliases.lists != "two_page_entry_refs") ||
		(aliases.su != "" && aliases.su != "su" && aliases.su != "sivi_su" && aliases.su != "main") ||
		selection.projectSource < 1 || selection.projectSource > 2 || selection.workingSource < 1 || selection.workingSource > 3 {
		return result, errors.New("complete-entry context references require the owned transaction, source selection and internal aliases")
	}
	if err := profileOwnedFiles(owner); err != nil {
		return result, err
	}
	fields, err := twoPageEntryReferenceFields(form)
	if err != nil {
		return result, err
	}
	valid := false
	for _, expected := range fields {
		valid = valid || (expected == field && (field.reader == "project" || field.reader == "master-unit" || field.reader == "working-unit"))
	}
	if !valid {
		return result, errors.New("complete-entry context reference requires its exact source field policy")
	}
	owned := map[string]string{aliases.project: "project"}
	if field.reader == "project" && selection.projectSource == 2 {
		owned["VMetaData"] = "VMetaData"
	}
	if field.reader == "master-unit" || (field.reader == "working-unit" && selection.workingSource == 2) {
		owned[aliases.lists] = "VLists"
	}
	if field.reader == "working-unit" && selection.workingSource == 3 && owner.selection.SU != "None" {
		if aliases.su == "" {
			return result, errors.New("complete-entry Working Units require the selected owned SU alias")
		}
		owned[aliases.su] = "su"
	}
	if err := verifyTwoPageEntryReferenceAliases(ctx, tx, owner, owned); err != nil {
		return result, err
	}
	switch field.reader {
	case "project":
		choices, err := readSIVIProjectChoicesAtAlias(ctx, owner, tx, contextID, selection.projectSource, aliases.project)
		if err != nil {
			return result, err
		}
		result.Source = fmt.Sprintf("configured-owned-imported-SQLite %s.%s; ProjectIdSource=%d; physical rowid; no metadata loading",
			choices.Alias, choices.Table, selection.projectSource)
		result.Definitions = choices.Choices
		for _, row := range choices.Choices.Rows {
			cell, title := row.Cells[0], row.Cells[1]
			choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: cell.Text, Description: title.Text}
			if cell.Storage != "text" || cell.Text == nil {
				choice.Diagnostic = "ProjectID is not nonempty TEXT"
			} else if err := validateSiteCodeText(field.column, cell.Text, field.maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	case "master-unit", "working-unit":
		if field.reader == "master-unit" || selection.workingSource == 2 {
			result.Source = "configured-owned-imported-SQLite VLists.MasterSiteUnitList; Level=11; physical rowid"
			if err := validateSIVIPhysicalSchema(ctx, tx, aliases.lists, "MasterSiteUnitList", "complete-entry master units"); err != nil {
				return result, err
			}
			level := "11"
			table, err := readSQLiteStorageRows(ctx, tx, aliases.lists, "MasterSiteUnitList", "Level", &level, "SiteSeries")
			if err != nil {
				return result, err
			}
			expected := []ProjectMetadataColumn{{"ID", "INTEGER"}, {"SiteSeries", "TEXT"}, {"SiteSeriesLongName", "TEXT"},
				{"SiteSeriesScientificName", "TEXT"}, {"Level", "INTEGER"}}
			if !reflect.DeepEqual(table.Columns, expected) {
				return result, errors.New("complete-entry master units require the original five-column imported schema")
			}
			result.Definitions = table
			for _, row := range table.Rows {
				code, description := row.Cells[1], row.Cells[2]
				choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: code.Text, Description: description.Text}
				if code.Storage != "text" || code.Text == nil {
					choice.Diagnostic = "SiteSeries is not nonempty TEXT"
				} else if err := validateSiteCodeText(field.column, code.Text, field.maximum); err != nil {
					choice.Diagnostic = err.Error()
				} else {
					choice.Selectable = true
				}
				result.Choices = append(result.Choices, choice)
			}
		} else {
			result.Source = fmt.Sprintf("configured-owned-imported-SQLite current project/SU; AssignedSuSource=%d; distinct literal code identities", selection.workingSource)
			if selection.workingSource == 3 && owner.selection.SU == "None" {
				result.Diagnostic = "Select an authorized SU table before requesting Working Unit SU choices"
				return result, nil
			}
			query := ""
			if selection.workingSource == 1 {
				for _, table := range []string{owner.selection.Project + "_Env", owner.selection.Project + "_Admin"} {
					if err := validateSIVIPhysicalSchema(ctx, tx, aliases.project, table, "complete-entry Env Working Units"); err != nil {
						return result, err
					}
				}
				query = `SELECT DISTINCT a.UserSiteUnit FROM ` + quoteHeaderIdentifier(aliases.project) + "." +
					quoteHeaderIdentifier(owner.selection.Project+"_Admin") + ` AS a INNER JOIN ` +
					quoteHeaderIdentifier(aliases.project) + "." + quoteHeaderIdentifier(owner.selection.Project+"_Env") +
					` AS e ON e.PlotNumber=a.Plot WHERE a.UserSiteUnit IS NOT NULL ORDER BY a.UserSiteUnit COLLATE NOCASE,a.UserSiteUnit`
			} else {
				if aliases.su == "" {
					return result, errors.New("complete-entry Working Units require the selected owned SU alias")
				}
				table := owner.selection.SU + "_SU"
				if err := validateSIVIPhysicalSchema(ctx, tx, aliases.su, table, "complete-entry SU Working Units"); err != nil {
					return result, err
				}
				query = `SELECT DISTINCT SiteUnit FROM ` + quoteHeaderIdentifier(aliases.su) + "." + quoteHeaderIdentifier(table) +
					` WHERE SiteUnit IS NOT NULL ORDER BY SiteUnit COLLATE NOCASE,SiteUnit`
			}
			rows, err := tx.QueryContext(ctx, query)
			if err != nil {
				return result, err
			}
			result.Definitions.Columns = []ProjectMetadataColumn{{"Item", "TEXT"}}
			for rows.Next() {
				var code any
				if err := rows.Scan(&code); err != nil {
					rows.Close()
					return result, err
				}
				value, text := code.(string)
				if !text {
					rows.Close()
					return result, errors.New("complete-entry Working Unit source requires literal TEXT values")
				}
				if !utf8.ValidString(value) {
					rows.Close()
					return result, errors.New("complete-entry Working Unit source contains malformed UTF-8")
				}
				id := strconv.Itoa(len(result.Choices) + 1)
				choice := SIVIParentSharedReferenceChoice{RowID: id, Code: &value}
				if err := validateSiteCodeText(field.column, &value, field.maximum); err != nil {
					choice.Diagnostic = err.Error()
				} else {
					choice.Selectable = true
				}
				result.Choices = append(result.Choices, choice)
				result.Definitions.Rows = append(result.Definitions.Rows, ProjectMetadataRow{RowID: id,
					Cells: []ProjectMetadataCell{siviReferenceText(&value)}})
			}
			if err := errors.Join(rows.Err(), rows.Close()); err != nil {
				return result, err
			}
		}
	}
	result.Available = true
	return result, ctx.Err()
}
