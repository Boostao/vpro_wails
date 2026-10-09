package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type ProfileSUPlot struct {
	PlotNumber string  `json:"plotNumber"`
	SiteUnit   *string `json:"siteUnit"`
}

type ProfileSUReview struct {
	Filter       ProjectPlotProfileFilterRequest `json:"filter"`
	Template     ProjectMetadataTable            `json:"template"`
	SourceSU     *ProjectMetadataTable           `json:"sourceSU"`
	Descriptions *ProjectMetadataTable           `json:"descriptions"`
	Plots        []ProfileSUPlot                 `json:"plots"`
}

type ProfileSUCreation struct {
	Review    ProfileSUReview `json:"review"`
	Name      string          `json:"name"`
	Path      string          `json:"path"`
	Confirmed bool            `json:"confirmed"`
}

type ProfileSUCreated struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	PlotCount int    `json:"plotCount"`
}

const profileSUCreationHistorySQL = `CREATE TABLE "__VPRO_ProfileSUHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`

func (request *ProfileSUCreation) UnmarshalJSON(data []byte) error {
	type plain ProfileSUCreation
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "review", "name", "path", "confirmed"); err != nil {
		return err
	}
	var raw struct {
		Review json.RawMessage `json:"review"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := decodeProfileSUReviewJSON(raw.Review, &decoded.Review); err != nil {
		return err
	}
	*request = ProfileSUCreation(decoded)
	return nil
}

func decodeProfileSUReviewJSON(data []byte, review *ProfileSUReview) error {
	if err := decodeProfileLifecycleJSON(data, review, "filter", "template", "plots"); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, key := range []string{"sourceSU", "descriptions"} {
		if _, present := properties[key]; !present {
			return fmt.Errorf("SU creation requires explicit nullable %s", key)
		}
	}
	return nil
}

func readProfileSUReview(ctx context.Context, tx *sql.Tx, filter ProjectPlotProfileFilterRequest, result ProjectPlotProfileResult) (ProfileSUReview, error) {
	if !reflect.DeepEqual(filter.Preview, result) {
		return ProfileSUReview{}, errors.New("profile results changed since preview; rerun before reviewing or saving an SU")
	}
	if err := validateProfileSUTemplate(ctx, tx); err != nil {
		return ProfileSUReview{}, err
	}
	review := ProfileSUReview{Filter: filter, Plots: []ProfileSUPlot{}}
	var err error
	review.Template, err = readSQLiteStorageRows(ctx, tx, "VPro64", "USysSuTable", "", nil, "")
	if err != nil {
		return review, err
	}
	if len(review.Template.Columns) != 2 || review.Template.Columns[0].Name != "PlotNumber" ||
		review.Template.Columns[1].Name != "SiteUnit" || len(review.Template.Rows) != 0 ||
		!strings.EqualFold(review.Template.Columns[0].DeclaredType, "VARCHAR") ||
		!strings.EqualFold(review.Template.Columns[1].DeclaredType, "VARCHAR") {
		return review, errors.New("original empty two-column SU template is unavailable; no schema repair or inferred defaults")
	}
	var metadata bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM VPro64.sqlite_master WHERE type='table' AND name='_table_metadata')`).Scan(&metadata); err != nil {
		return review, err
	}
	if metadata {
		name := "USysSuTable"
		descriptions, err := readSQLiteStorageRows(ctx, tx, "VPro64", "_table_metadata", "table_name", &name, "")
		if err != nil {
			return review, err
		}
		review.Descriptions = &descriptions
	}
	source := map[string]*string{}
	if result.SU != "None" {
		table, err := readSQLiteStorageRows(ctx, tx, "su", result.SU+"_SU", "", nil, "")
		if err != nil {
			return review, err
		}
		review.SourceSU = &table
		plotIndex, siteIndex := -1, -1
		for index, column := range table.Columns {
			if column.Name == "PlotNumber" {
				plotIndex = index
			}
			if column.Name == "SiteUnit" {
				siteIndex = index
			}
		}
		if plotIndex < 0 || siteIndex < 0 {
			return review, errors.New("selected SU lacks its original plot and SiteUnit fields")
		}
		selected := map[string]bool{}
		for _, plot := range result.PlotNumbers {
			selected[plot] = true
		}
		for _, row := range table.Rows {
			cell := row.Cells[plotIndex]
			if cell.Storage != "text" || cell.Text == nil || !selected[*cell.Text] {
				continue
			}
			if _, duplicate := source[*cell.Text]; duplicate {
				return review, errors.New("selected SU has duplicate matching plot identities; a unique destination cannot preserve them")
			}
			site := row.Cells[siteIndex]
			if _, err := metadataCellValue(site); err != nil || site.Storage != "text" && site.Storage != "null" {
				return review, errors.New("selected SiteUnit has unsupported historical storage; no coercion or repair")
			}
			source[*cell.Text] = site.Text
		}
	}
	for _, plot := range result.PlotNumbers {
		if result.SU != "None" {
			if _, present := source[plot]; !present {
				return review, errors.New("profile plot does not have an independently observed selected SU record")
			}
		}
		site := source[plot]
		if err := validateProfileSUText(plot, 7); err != nil {
			return review, fmt.Errorf("PlotNumber: %w", err)
		}
		if site != nil {
			if err := validateProfileSUText(*site, 255); err != nil {
				return review, fmt.Errorf("SiteUnit: %w", err)
			}
		}
		review.Plots = append(review.Plots, ProfileSUPlot{plot, site})
	}
	return review, nil
}

func validateProfileSUText(value string, maximum int) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || len(utf16.Encode([]rune(value))) > maximum {
		return fmt.Errorf("literal text must contain valid Unicode, no NUL and at most %d UTF-16 units; nothing was trimmed or repaired", maximum)
	}
	return nil
}

func validateProfileSUTemplate(ctx context.Context, tx interface {
	projectMetadataQueryer
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (resultErr error) {
	var physical bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM VPro64.sqlite_master WHERE type='table' AND name='USysSuTable')`).Scan(&physical); err != nil {
		return err
	}
	if !physical {
		return errors.New("SU creation requires the original physical template, not a view")
	}
	var definition string
	if err := tx.QueryRowContext(ctx, `SELECT sql FROM VPro64.sqlite_master WHERE type='table' AND name='USysSuTable'`).Scan(&definition); err != nil {
		return err
	}
	const original = `CREATE TABLE "USysSuTable" ( "PlotNumber" VARCHAR, "SiteUnit" VARCHAR )`
	if strings.Join(strings.Fields(definition), " ") != original {
		return errors.New("SU template table constraints differ from the retained original; no inferred schema adaptation")
	}
	rows, err := tx.QueryContext(ctx, `SELECT "notnull",dflt_value,pk,hidden FROM pragma_table_xinfo('USysSuTable','VPro64')`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var notNull, primary, hidden int
		var defaultValue *string
		if err := rows.Scan(&notNull, &defaultValue, &primary, &hidden); err != nil {
			return errors.Join(err, rows.Close())
		}
		if notNull != 0 || primary != 0 || hidden != 0 || defaultValue != nil {
			return errors.Join(errors.New("altered SU template constraints/defaults/generated fields are unavailable"), rows.Close())
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT indexes."unique",indexes.partial,fields.seqno,fields.name
		FROM pragma_index_list('USysSuTable','VPro64') AS indexes
		JOIN pragma_index_info(indexes.name,'VPro64') AS fields ORDER BY fields.name`)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
	seen := map[string]bool{}
	for rows.Next() {
		var unique, partial, sequence int
		var name string
		if err := rows.Scan(&unique, &partial, &sequence, &name); err != nil {
			return err
		}
		if partial != 0 || sequence != 0 || seen[name] ||
			(name != "PlotNumber" || unique != 1) && (name != "SiteUnit" || unique != 0) {
			return errors.New("SU template requires exactly its original unique plot/nonunique SiteUnit indexes")
		}
		seen[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(seen) != 2 {
		return errors.New("SU template original indexes are missing; no inferred repair")
	}
	return nil
}

func (s *ContextService) ReviewProjectPlotProfileSU(ctx context.Context, contextID string, filter ProjectPlotProfileFilterRequest) (ProfileSUReview, error) {
	var review ProfileSUReview
	_, err := s.runProjectPlotProfile(ctx, contextID, filter.Input, func(tx *sql.Tx, result ProjectPlotProfileResult) error {
		var err error
		review, err = readProfileSUReview(ctx, tx, filter, result)
		return err
	})
	if err != nil {
		return ProfileSUReview{}, err
	}
	return review, nil
}

func profileSUDestination(request ProfileSUCreation) (string, os.FileInfo, error) {
	if !request.Confirmed || !projectNamePattern.MatchString(request.Name) || request.Name == "None" ||
		strings.EqualFold(request.Name, "Sample") || strings.Contains(strings.ToLower(request.Name), "master") {
		return "", nil, errors.New("confirm a literal desktop family name (letter first, ASCII letters/digits/underscore, at most31 characters); None, Sample and master names are reserved")
	}
	return freshSQLiteDestination(request.Path, "SU")
}

func freshSQLiteDestination(requested, kind string) (string, os.FileInfo, error) {
	if !utf8.ValidString(requested) || !filepath.IsAbs(requested) || !strings.EqualFold(filepath.Ext(requested), ".db") {
		return "", nil, fmt.Errorf("%s destination must be an explicit absolute new SQLite .db file", kind)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(requested))
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return "", nil, errors.Join(err, fmt.Errorf("%s destination parent must be an existing directory", kind))
	}
	path := filepath.Join(parent, filepath.Base(requested))
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return "", nil, errors.Join(err, fmt.Errorf("%s destination already exists or cannot be checked; existing files are never replaced", kind))
	}
	return path, info, nil
}

func (s *ContextService) SaveProjectPlotProfileSU(ctx context.Context, contextID string, request ProfileSUCreation) (_ ProfileSUCreated, resultErr error) {
	path, parent, err := profileSUDestination(request)
	if err != nil {
		return ProfileSUCreated{}, err
	}
	temporary := ""
	published := false
	defer func() {
		if temporary != "" {
			resultErr = errors.Join(resultErr, os.Remove(temporary))
		}
		if published && resultErr != nil {
			resultErr = fmt.Errorf("Profile SU file published, but cleanup failed; do not replay creation: %w", resultErr)
		}
	}()
	_, err = s.runProjectPlotProfileAfter(ctx, contextID, request.Review.Filter.Input,
		func(tx *sql.Tx, result ProjectPlotProfileResult) error {
			fresh, err := readProfileSUReview(ctx, tx, request.Review.Filter, result)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(fresh, request.Review) {
				return errors.New("SU template, descriptions, source SiteUnit records or reviewed rows changed; review again before saving")
			}
			file, err := os.CreateTemp(filepath.Dir(path), ".vpro-profile-su-*.db")
			if err != nil {
				return err
			}
			temporary = file.Name()
			if err := file.Close(); err != nil {
				return err
			}
			return writeProfileSUFile(ctx, temporary, request)
		}, func(owner *sqliteContext) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := profileOwnedFiles(owner); err != nil {
				return err
			}
			current, err := os.Stat(filepath.Dir(path))
			if err != nil || !os.SameFile(parent, current) {
				return errors.Join(err, errors.New("SU destination directory identity changed; file was not published"))
			}
			if err := os.Link(temporary, path); err != nil {
				return fmt.Errorf("SU publication requires atomic no-replace linking; destination unchanged: %w", err)
			}
			published = true
			return nil
		})
	if err != nil {
		return ProfileSUCreated{}, err
	}
	return ProfileSUCreated{request.Name, path, len(request.Review.Plots)}, nil
}

func writeProfileSUFile(ctx context.Context, path string, request ProfileSUCreation) (resultErr error) {
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	proposal, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if err := writeProfileSUContents(ctx, tx, request, string(proposal)); err != nil {
		return err
	}
	return tx.Commit()
}

func writeProfileSUContents(ctx context.Context, tx *sql.Tx, request ProfileSUCreation, proposal string) error {
	table := quoteHeaderIdentifier(request.Name + "_SU")
	if _, err := tx.ExecContext(ctx, `CREATE TABLE `+table+`("PlotNumber" VARCHAR,"SiteUnit" VARCHAR);
		CREATE UNIQUE INDEX `+quoteHeaderIdentifier("uidx_"+request.Name+"_SU_PlotNumber")+` ON `+table+`("PlotNumber");
		CREATE INDEX `+quoteHeaderIdentifier("idx_"+request.Name+"_SU_SiteUnit")+` ON `+table+`("SiteUnit")`); err != nil {
		return err
	}
	for _, plot := range request.Review.Plots {
		if _, err := tx.ExecContext(ctx, "INSERT INTO "+table+"(PlotNumber,SiteUnit) VALUES(?,?)", plot.PlotNumber, plot.SiteUnit); err != nil {
			return err
		}
	}
	if request.Review.Descriptions != nil {
		if err := copyProfileSUDescriptions(ctx, tx, *request.Review.Descriptions, request.Name+"_SU"); err != nil {
			return err
		}
		observed, err := readSQLiteStorageRows(ctx, tx, "main", "_table_metadata", "", nil, "")
		if err != nil {
			return err
		}
		expected := *request.Review.Descriptions
		if len(observed.Rows) != len(expected.Rows) {
			return errors.New("new SU description count differs from original metadata")
		}
		expected.Rows = append([]ProjectMetadataRow{}, expected.Rows...)
		for index, row := range expected.Rows {
			row.Cells = append([]ProjectMetadataCell{}, row.Cells...)
			row.RowID = observed.Rows[index].RowID
			for column, definition := range expected.Columns {
				if definition.Name == "table_name" {
					name := request.Name + "_SU"
					row.Cells[column] = ProjectMetadataCell{Storage: "text", Text: &name}
				}
			}
			expected.Rows[index] = row
		}
		if !reflect.DeepEqual(expected, observed) {
			return errors.New("new SU descriptions differ from the independently planned original typed metadata")
		}
	}
	if err := appendCreationProvenance(ctx, tx, "__VPRO_ProfileSUHistory", profileSUCreationHistorySQL, proposal); err != nil {
		return err
	}
	stored, err := readSQLiteStorageRows(ctx, tx, "main", request.Name+"_SU", "", nil, "PlotNumber")
	if err != nil || len(stored.Rows) != len(request.Review.Plots) {
		return errors.Join(err, errors.New("new SU does not have exactly the independently planned rows"))
	}
	for index, row := range stored.Rows {
		expected := request.Review.Plots[index]
		plot, site := row.Cells[0], row.Cells[1]
		if plot.Storage != "text" || plot.Text == nil || *plot.Text != expected.PlotNumber ||
			(expected.SiteUnit == nil && site.Storage != "null") ||
			(expected.SiteUnit != nil && (site.Storage != "text" || site.Text == nil || *site.Text != *expected.SiteUnit)) {
			return errors.New("new SU stored values differ from independently planned literal/NULL assignments")
		}
	}
	return nil
}

func copyProfileSUDescriptions(ctx context.Context, tx *sql.Tx, source ProjectMetadataTable, table string) error {
	fields, definitions := []string{}, []string{}
	nameIndex := -1
	for index, column := range source.Columns {
		fields = append(fields, quoteHeaderIdentifier(column.Name))
		definitions = append(definitions, quoteHeaderIdentifier(column.Name)+" "+quoteHeaderIdentifier(column.DeclaredType))
		if column.Name == "table_name" {
			nameIndex = index
		}
	}
	if nameIndex < 0 {
		return errors.New("original description metadata lacks its literal table identity")
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE "_table_metadata"(`+strings.Join(definitions, ",")+")"); err != nil {
		return err
	}
	for _, row := range source.Rows {
		values, placeholders := []any{}, []string{}
		for index, cell := range row.Cells {
			value, err := metadataCellValue(cell)
			if err != nil {
				return err
			}
			if index == nameIndex {
				value = table
			}
			values, placeholders = append(values, value), append(placeholders, "?")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO "_table_metadata"(`+strings.Join(fields, ",")+") VALUES("+strings.Join(placeholders, ",")+")", values...); err != nil {
			return err
		}
	}
	return nil
}
