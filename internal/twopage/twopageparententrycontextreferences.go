package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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

type TwoPageEntryReferenceSelection struct {
	ProjectSource, WorkingSource int `json:"-" yaml:"-"`
}

type TwoPageEntryReferenceAliases struct {
	Project, Lists, Su string `json:"-" yaml:"-"`
}

func verifyTwoPageEntryReferenceAliases(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, expected map[string]string) error {
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
		if !present || path == "" || owner.AttachmentInfo[role] == nil {
			return fmt.Errorf("complete-entry reference alias %s is not owned", alias)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !os.SameFile(info, owner.AttachmentInfo[role]) {
			return fmt.Errorf("complete-entry reference alias %s changed its owned %s identity", alias, role)
		}
	}
	return nil
}

func TwoPageEntryReferenceSelectionFromConfig(values map[string]any) (TwoPageEntryReferenceSelection, error) {
	project, err := vprocore.ConfigInt(values, "Current", "ProjectIdSource", 1, 2)
	if err != nil {
		return TwoPageEntryReferenceSelection{}, err
	}
	working, err := vprocore.ConfigInt(values, "Current", "AssignedSuSource", 1, 3)
	if err != nil {
		return TwoPageEntryReferenceSelection{}, err
	}
	return TwoPageEntryReferenceSelection{project, working}, nil
}

func ReadTwoPageEntryContextReference(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, contextID, form string,
	selection TwoPageEntryReferenceSelection, aliases TwoPageEntryReferenceAliases,
	field TwoPageEntryReferenceField) (sourceform.SIVIParentSharedReference, error) {
	result := sourceform.SIVIParentSharedReference{Column: field.Column, ListName: field.List, Required: field.Required,
		Definitions: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
		Choices:     []sourceform.SIVIParentSharedReferenceChoice{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if tx == nil || owner == nil || contextID == "" ||
		(aliases.Project != "project" && aliases.Project != "main") ||
		(aliases.Lists != "VLists" && aliases.Lists != "two_page_entry_refs") ||
		(aliases.Su != "" && aliases.Su != "su" && aliases.Su != "sivi_su" && aliases.Su != "main") ||
		selection.ProjectSource < 1 || selection.ProjectSource > 2 || selection.WorkingSource < 1 || selection.WorkingSource > 3 {
		return result, errors.New("complete-entry context references require the owned transaction, source selection and internal aliases")
	}
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return result, err
	}
	fields, err := twoPageEntryReferenceFields(form)
	if err != nil {
		return result, err
	}
	valid := false
	for _, expected := range fields {
		valid = valid || (expected == field && (field.Reader == "project" || field.Reader == "master-unit" || field.Reader == "working-unit"))
	}
	if !valid {
		return result, errors.New("complete-entry context reference requires its exact source field policy")
	}
	owned := map[string]string{aliases.Project: "project"}
	if field.Reader == "project" && selection.ProjectSource == 2 {
		owned["VMetaData"] = "VMetaData"
	}
	if field.Reader == "master-unit" || (field.Reader == "working-unit" && selection.WorkingSource == 2) {
		owned[aliases.Lists] = "VLists"
	}
	if field.Reader == "working-unit" && selection.WorkingSource == 3 && owner.Selection.SU != "None" {
		if aliases.Su == "" {
			return result, errors.New("complete-entry Working Units require the selected owned SU alias")
		}
		owned[aliases.Su] = "su"
	}
	if err := verifyTwoPageEntryReferenceAliases(ctx, tx, owner, owned); err != nil {
		return result, err
	}
	switch field.Reader {
	case "project":
		choices, err := sourceform.ReadSIVIProjectChoicesAtAlias(ctx, owner, tx, contextID, selection.ProjectSource, aliases.Project)
		if err != nil {
			return result, err
		}
		result.Source = fmt.Sprintf("configured-owned-imported-SQLite %s.%s; ProjectIdSource=%d; physical rowid; no metadata loading",
			choices.Alias, choices.Table, selection.ProjectSource)
		result.Definitions = choices.Choices
		for _, row := range choices.Choices.Rows {
			cell, title := row.Cells[0], row.Cells[1]
			choice := sourceform.SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: cell.Text, Description: title.Text}
			if cell.Storage != "text" || cell.Text == nil {
				choice.Diagnostic = "ProjectID is not nonempty TEXT"
			} else if err := sqlutil.ValidateSiteCodeText(field.Column, cell.Text, field.Maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	case "master-unit", "working-unit":
		if field.Reader == "master-unit" || selection.WorkingSource == 2 {
			result.Source = "configured-owned-imported-SQLite VLists.MasterSiteUnitList; Level=11; physical rowid"
			if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, aliases.Lists, "MasterSiteUnitList", "complete-entry master units"); err != nil {
				return result, err
			}
			level := "11"
			table, err := vprocore.ReadSQLiteStorageRows(ctx, tx, aliases.Lists, "MasterSiteUnitList", "Level", &level, "SiteSeries")
			if err != nil {
				return result, err
			}
			expected := []vprocore.ProjectMetadataColumn{{Name: "ID", DeclaredType: "INTEGER"}, {Name: "SiteSeries", DeclaredType: "TEXT"}, {Name: "SiteSeriesLongName", DeclaredType: "TEXT"},
				{Name: "SiteSeriesScientificName", DeclaredType: "TEXT"}, {Name: "Level", DeclaredType: "INTEGER"}}
			if !reflect.DeepEqual(table.Columns, expected) {
				return result, errors.New("complete-entry master units require the original five-column imported schema")
			}
			result.Definitions = table
			for _, row := range table.Rows {
				code, description := row.Cells[1], row.Cells[2]
				choice := sourceform.SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: code.Text, Description: description.Text}
				if code.Storage != "text" || code.Text == nil {
					choice.Diagnostic = "SiteSeries is not nonempty TEXT"
				} else if err := sqlutil.ValidateSiteCodeText(field.Column, code.Text, field.Maximum); err != nil {
					choice.Diagnostic = err.Error()
				} else {
					choice.Selectable = true
				}
				result.Choices = append(result.Choices, choice)
			}
		} else {
			result.Source = fmt.Sprintf("configured-owned-imported-SQLite current project/SU; AssignedSuSource=%d; distinct literal code identities", selection.WorkingSource)
			if selection.WorkingSource == 3 && owner.Selection.SU == "None" {
				result.Diagnostic = "Select an authorized SU table before requesting Working Unit SU choices"
				return result, nil
			}
			query := ""
			if selection.WorkingSource == 1 {
				for _, table := range []string{owner.Selection.Project + "_Env", owner.Selection.Project + "_Admin"} {
					if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, aliases.Project, table, "complete-entry Env Working Units"); err != nil {
						return result, err
					}
				}
				query = `SELECT DISTINCT a.UserSiteUnit FROM ` + vprocore.QuoteHeaderIdentifier(aliases.Project) + "." +
					vprocore.QuoteHeaderIdentifier(owner.Selection.Project+"_Admin") + ` AS a INNER JOIN ` +
					vprocore.QuoteHeaderIdentifier(aliases.Project) + "." + vprocore.QuoteHeaderIdentifier(owner.Selection.Project+"_Env") +
					` AS e ON e.PlotNumber=a.Plot WHERE a.UserSiteUnit IS NOT NULL ORDER BY a.UserSiteUnit COLLATE NOCASE,a.UserSiteUnit`
			} else {
				if aliases.Su == "" {
					return result, errors.New("complete-entry Working Units require the selected owned SU alias")
				}
				table := owner.Selection.SU + "_SU"
				if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, aliases.Su, table, "complete-entry SU Working Units"); err != nil {
					return result, err
				}
				query = `SELECT DISTINCT SiteUnit FROM ` + vprocore.QuoteHeaderIdentifier(aliases.Su) + "." + vprocore.QuoteHeaderIdentifier(table) +
					` WHERE SiteUnit IS NOT NULL ORDER BY SiteUnit COLLATE NOCASE,SiteUnit`
			}
			rows, err := tx.QueryContext(ctx, query)
			if err != nil {
				return result, err
			}
			result.Definitions.Columns = []vprocore.ProjectMetadataColumn{{Name: "Item", DeclaredType: "TEXT"}}
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
				choice := sourceform.SIVIParentSharedReferenceChoice{RowID: id, Code: &value}
				if err := sqlutil.ValidateSiteCodeText(field.Column, &value, field.Maximum); err != nil {
					choice.Diagnostic = err.Error()
				} else {
					choice.Selectable = true
				}
				result.Choices = append(result.Choices, choice)
				result.Definitions.Rows = append(result.Definitions.Rows, vprocore.ProjectMetadataRow{RowID: id,
					Cells: []vprocore.ProjectMetadataCell{sourceform.SiviReferenceText(&value)}})
			}
			if err := errors.Join(rows.Err(), rows.Close()); err != nil {
				return result, err
			}
		}
	}
	result.Available = true
	return result, ctx.Err()
}
