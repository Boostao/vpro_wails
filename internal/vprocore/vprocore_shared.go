package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"bytes"
	"context"

	"database/sql"

	"encoding/json"
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"

	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"unicode/utf8"
)

const BreakReportReviewEnvironment = "VPRO_BREAK_REPORT_REVIEW"

type EnvironmentReportField struct {
	Source  string `json:"source"`
	Key     string `json:"key"`
	Label   string `json:"label"`
	Heading bool   `json:"heading"`
}

type EnvironmentReportName struct {
	RowID string              `json:"rowId"`
	Value ProjectMetadataCell `json:"value"`
}

func SortedEnvironmentReportKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type artifactPublicationReceipt struct {
	Status, RequestedDestination, Path, SHA256, ErrorMessage string
}

func ArtifactPublicationOutcome(requested string, result ArtifactPublication, err error, label string) artifactPublicationReceipt {
	outcome := artifactPublicationReceipt{RequestedDestination: requested, Path: result.Path, SHA256: result.SHA256}
	switch {
	case result.Published && err == nil:
		outcome.Status = "published"
	case result.Published:
		outcome.Status = "published-with-errors"
	case err != nil:
		outcome.Status = "not-published"
	default:
		outcome.Status = "not-published"
		err = fmt.Errorf("%s publication returned no committed artifact", label)
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}

func DecodeGoogleEarthKMLJSON(data []byte, target any, operation string, properties ...string) error {
	return DecodeStrictRequiredJSON(data, target, operation, properties...)
}

func DecodeStrictRequiredJSON(data []byte, target any, operation string, properties ...string) error {
	if err := DecodeProfileLifecycleJSON(data, target, properties...); err != nil {
		return fmt.Errorf("%s request: %w", operation, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.Token(); err != nil {
		return err
	}
	seen := map[string]bool{}
	allowed := map[string]bool{}
	for _, property := range properties {
		allowed[property] = true
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok || !allowed[key] || seen[key] {
			return fmt.Errorf("%s has an unknown or duplicate property %q", operation, key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	return nil
}

func WithOwnedPreferenceSnapshot[T any](ctx context.Context, plots *PlotService, hooks PublicationReadSnapshotHooks,
	read func(*SqliteContext, *sql.Tx) (T, error)) (T, error) {
	owner := plots.Projects.Sqlite
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		var zero T
		return zero, err
	}
	defer owner.Mu.Unlock()
	return WithPublicationReadSnapshot(ctx, owner, hooks, func(tx *sql.Tx) (T, error) { return read(owner, tx) })
}

func ValidateGoogleEarthReviewStrings(values ...string) error {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return errors.New("Google Earth review requires valid UTF-8; no transport text was repaired")
		}
	}
	return nil
}

func PictureNativeChooserOwner(ctx context.Context) (*application.App, *application.WebviewWindow, error) {
	if ctx == nil {
		return nil, nil, errors.New("picture selection requires a native request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	window, ok := ctx.Value(application.WindowKey).(*application.WebviewWindow)
	if !ok || window == nil || window.NativeWindow() == nil {
		return nil, nil, errors.New("picture selection requires its live native request window")
	}
	app := application.Get()
	if app == nil || app.Window == nil || app.Dialog == nil {
		return nil, nil, errors.New("picture selection requires the active native application")
	}
	registered, present := app.Window.GetByID(window.ID())
	native, ok := registered.(*application.WebviewWindow)
	if !present || !ok || native != window {
		return nil, nil, errors.New("picture selection request window is no longer owned by this application")
	}
	return app, window, nil
}

func AppendCreationProvenance(ctx context.Context, tx *sql.Tx, table, expectedDefinition, proposal string) error {
	return AppendTechnicalProvenance(ctx, tx, table, expectedDefinition, proposal, "profile creation")
}

func AppendTechnicalProvenance(ctx context.Context, tx *sql.Tx, table, expectedDefinition, proposal, purpose string) error {
	if _, err := tx.ExecContext(ctx, strings.Replace(expectedDefinition, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)); err != nil {
		return err
	}
	if err := VerifyTechnicalProvenance(ctx, tx, table, expectedDefinition, purpose); err != nil {
		return err
	}
	when := time.Now().UTC().Format(time.RFC3339Nano)
	inserted, err := tx.ExecContext(ctx, `INSERT INTO `+QuoteHeaderIdentifier(table)+`(Created,Proposal) VALUES(?,?)`, when, proposal)
	if err != nil {
		return err
	}
	if count, err := inserted.RowsAffected(); err != nil || count != 1 {
		return errors.Join(err, fmt.Errorf("%s provenance did not insert exactly one event", purpose))
	}
	id, err := inserted.LastInsertId()
	if err != nil {
		return err
	}
	var observed, observedWhen string
	if err := tx.QueryRowContext(ctx, `SELECT Created,Proposal FROM `+QuoteHeaderIdentifier(table)+` WHERE ID=?`, id).
		Scan(&observedWhen, &observed); err != nil || observed != proposal || observedWhen != when {
		return errors.Join(err, fmt.Errorf("new %s technical history differs from its complete proposal", purpose))
	}
	return nil
}

func VerifyTechnicalProvenance(ctx context.Context, tx *sql.Tx, table, expectedDefinition, purpose string) error {
	return VerifyTechnicalProvenanceIn(ctx, tx, "main", table, expectedDefinition, purpose)
}

func VerifyTechnicalProvenanceIn(ctx context.Context, tx *sql.Tx, alias, table, expectedDefinition, purpose string) error {
	var definition string
	if err := tx.QueryRowContext(ctx, `SELECT sql FROM `+QuoteHeaderIdentifier(alias)+`.sqlite_master
		WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&definition); err != nil {
		return fmt.Errorf("physical %s provenance unavailable: %w", purpose, err)
	}
	if strings.Join(strings.Fields(definition), " ") != strings.Join(strings.Fields(expectedDefinition), " ") {
		return fmt.Errorf("%s provenance schema differs; no existing history overwritten", purpose)
	}
	var extras int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+QuoteHeaderIdentifier(alias)+`.sqlite_master
		WHERE tbl_name COLLATE NOCASE=? AND type<>'table'`, table).Scan(&extras); err != nil {
		return err
	}
	if extras != 0 {
		return fmt.Errorf("%s provenance has unreviewed indexes/triggers; no history appended", purpose)
	}
	return nil
}

func DecodeProfileLifecycleJSON(data []byte, target any, required ...string) error {
	if err := ValidateMetadataDraftJSON(data); err != nil {
		return fmt.Errorf("profile lifecycle JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, name := range required {
		value, present := properties[name]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("profile lifecycle requires explicit non-NULL %s", name)
		}
	}
	return nil
}

func FreshSQLiteDestination(requested, kind string) (string, os.FileInfo, error) {
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

func ProfileOwnedFiles(c *SqliteContext) error {
	for role, path := range c.Attachments {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !os.SameFile(info, c.AttachmentInfo[role]) {
			return fmt.Errorf("profile %s file identity changed; reload the context", role)
		}
	}
	return nil
}

func ProfileDriverNull(value any) bool {

	blob, isBlob := value.([]byte)
	return value == nil || isBlob && blob == nil
}

func ProfileNumeric(value any) (any, error) {
	if ProfileDriverNull(value) {
		return nil, nil
	}
	var number float64
	switch value := value.(type) {
	case int64:
		number = float64(value)
	case float64:
		number = value
	default:
		return nil, errors.New("profile numeric storage is not numeric; historical values were not coerced")
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, errors.New("profile numeric value is not finite")
	}
	return number, nil
}

var ProfileDecimal = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func SiteUnitTransferIndex(table ProjectMetadataTable, column int) (map[string][]ProjectMetadataRow, error) {
	result := map[string][]ProjectMetadataRow{}
	for _, row := range table.Rows {
		cell := row.Cells[column]
		if _, err := MetadataCellValue(cell); err != nil {
			return nil, err
		}
		if cell.Storage == "null" {
			continue
		}
		if cell.Storage != "text" {
			return nil, errors.New("transfer identities require original text storage; no coercion")
		}
		result[*cell.Text] = append(result[*cell.Text], row)
	}
	return result, nil
}

func CloneSiteUnitCell(cell ProjectMetadataCell) ProjectMetadataCell {
	if cell.Text != nil {
		text := *cell.Text
		cell.Text = &text
	}
	if cell.Integer != nil {
		integer := *cell.Integer
		cell.Integer = &integer
	}
	if cell.Real != nil {
		real := *cell.Real
		cell.Real = &real
	}
	if cell.BlobHex != nil {
		blob := *cell.BlobHex
		cell.BlobHex = &blob
	}
	return cell
}

func EnvironmentSiteUnitTables(ctx context.Context, tx *sql.Tx) (map[string]ProjectMetadataTable, error) {
	schema, err := ReadSQLiteStorageRows(ctx, tx, "main", "sqlite_master", "", nil, "")
	if err != nil {
		return nil, err
	}
	tables := map[string]ProjectMetadataTable{"sqlite_master": schema}
	for _, row := range schema.Rows {
		if row.Cells[0].Text == nil || *row.Cells[0].Text != "table" {
			continue
		}
		name := *row.Cells[1].Text
		table, err := ReadSQLiteStorageRows(ctx, tx, "main", name, "", nil, "")
		if err != nil {
			return nil, err
		}
		tables[name] = table
	}
	return tables, nil
}

func SiteUnitTransferColumns(table ProjectMetadataTable, required ...string) (map[string]int, error) {
	index := map[string]int{}
	for i, column := range table.Columns {
		if _, duplicate := index[column.Name]; duplicate || column.Name == "" {
			return nil, errors.New("transfer schema has missing or ambiguous column identities")
		}
		index[column.Name] = i
	}
	for _, name := range required {
		if _, present := index[name]; !present {
			return nil, fmt.Errorf("transfer requires original %s column", name)
		}
	}
	seen := map[string]bool{}
	for _, row := range table.Rows {
		id, err := strconv.ParseInt(row.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != row.RowID || seen[row.RowID] || len(row.Cells) != len(table.Columns) {
			return nil, errors.New("transfer requires complete rows with distinct exact signed64 physical identities")
		}
		seen[row.RowID] = true
	}
	return index, nil
}

func VerifySiteUnitTransferTables(original, observed, expected map[string]ProjectMetadataTable, history string) error {
	for name, before := range original {
		if planned, changed := expected[name]; changed {
			before = planned
		}
		switch name {
		case history:
			after := observed[name]
			if !reflect.DeepEqual(after.Columns, before.Columns) || len(after.Rows) != len(before.Rows)+1 ||
				!reflect.DeepEqual(after.Rows[:len(before.Rows)], before.Rows) {
				return errors.New("original SU transfer history changed; all changes rolled back")
			}
			continue
		case "sqlite_master":
			filter := func(schema ProjectMetadataTable) ProjectMetadataTable {
				schema.Rows = append([]ProjectMetadataRow(nil), schema.Rows...)
				rows := schema.Rows[:0]
				for _, row := range schema.Rows {
					if row.Cells[2].Text == nil || *row.Cells[2].Text != history {
						rows = append(rows, row)
					}
				}
				schema.Rows = rows
				return schema
			}
			if !reflect.DeepEqual(filter(observed[name]), filter(before)) {
				return errors.New("original project schema changed during SU transfer; all changes rolled back")
			}
			continue
		}
		if !reflect.DeepEqual(observed[name], before) {
			return fmt.Errorf("project table %s differs from the reviewed SU transfer plan; all changes rolled back", name)
		}
	}
	if len(observed) != len(original) && !(len(observed) == len(original)+1 && original[history].Columns == nil) {
		return errors.New("SU transfer introduced unexpected project tables; all changes rolled back")
	}
	return nil
}

func ValidateSIVIPhysicalSchema(ctx context.Context, tx *sql.Tx, alias, table, scope string) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA "+QuoteHeaderIdentifier(alias)+".table_xinfo("+QuoteHeaderIdentifier(table)+")")
	if err != nil {
		return err
	}
	for rows.Next() {
		var position, required, primary, hidden int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&position, &name, &kind, &required, &defaultValue, &primary, &hidden); err != nil {
			return errors.Join(err, rows.Close())
		}
		if hidden != 0 || strings.EqualFold(name, "rowid") || strings.EqualFold(name, "_rowid_") || strings.EqualFold(name, "oid") {
			return errors.Join(fmt.Errorf("SIVI %s require unshadowed physical identities and ordinary columns", scope), rows.Close())
		}
	}
	return errors.Join(rows.Err(), rows.Close())
}

var SiviDeletionUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func SiviFeature(name string, lookup func(string) (string, bool)) (bool, error) {
	value, present := lookup(name)
	if !present {
		if name == BreakReportReviewEnvironment || name == "VPRO_SIVI_CHARS_PARENT_ENTRY" {
			return false, nil
		}
		return true, nil
	}
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s requires literal true or false", name)
	}
}

func WithOwnedSIVISnapshot[T any](ctx context.Context, plots *PlotService, read func(*SqliteContext, *sql.Tx) (T, error)) (result T, resultErr error) {
	var zero T
	owner := plots.Projects.Sqlite
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return zero, err
	}
	defer owner.Mu.Unlock()
	if err := ProfileOwnedFiles(owner); err != nil {
		return zero, err
	}
	tx, err := owner.BeginReadSnapshot(ctx)
	if err != nil {
		return zero, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = zero
		}
	}()
	result, err = read(owner, tx)
	if err != nil {
		return zero, err
	}
	if err := ProfileOwnedFiles(owner); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}

type ProfileSQLNumber struct {
	Value   float64 `json:"-" yaml:"-"`
	Decimal string  `json:"-" yaml:"-"`
}

func ValidateSourceParentBooleanNull(column string, originalNull, nextNull bool) error {
	if !originalNull && nextNull && (column == "SV_FloodPlain" || column == "SpeciesListComplete") {
		return fmt.Errorf("%s source Yes/No converts NULL to No; select No explicitly instead of writing SQLite NULL", column)
	}
	return nil
}

func ValidateSourceParentEntryJSONProperties(data []byte, operation string) error {
	if err := ValidateMetadataDraftJSON(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token {
		case json.Delim('{'):
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate %s JSON property %q", operation, key)
				}
				seen[name] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
		case json.Delim('['):
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
		}
		return err
	}
	return walk()
}

func ValidateTwoPageEntryJSONProperties(data []byte) error {
	return ValidateSourceParentEntryJSONProperties(data, "complete-entry")
}

type twoPageSoilAttempt struct {
	request any
	Active  int  `json:"-" yaml:"-"`
	Sealed  bool `json:"-" yaml:"-"`
}

type twoPageSoilAttemptLedger struct {
	Mu       sync.Mutex                     `json:"-" yaml:"-"`
	Requests map[string]*twoPageSoilAttempt `json:"-" yaml:"-"`
}

func (ledger *twoPageSoilAttemptLedger) Entry(requestID string, request any) (*twoPageSoilAttempt, error) {
	if ledger.Requests == nil {
		ledger.Requests = map[string]*twoPageSoilAttempt{}
	}
	existing := ledger.Requests[requestID]
	if existing != nil {
		if !reflect.DeepEqual(existing.request, request) {
			return nil, errors.New("two-page soil UUID already belongs to another exact request payload")
		}
		return existing, nil
	}
	entry := &twoPageSoilAttempt{request: request}
	ledger.Requests[requestID] = entry
	return entry, nil
}

type twoPageVegetationAttempt struct {
	payload string
	Active  int  `json:"-" yaml:"-"`
	Sealed  bool `json:"-" yaml:"-"`
}

type twoPageVegetationAttemptLedger struct {
	Mu       sync.Mutex                           `json:"-" yaml:"-"`
	Requests map[string]*twoPageVegetationAttempt `json:"-" yaml:"-"`
}

func (ledger *twoPageVegetationAttemptLedger) Entry(requestID string, request any) (*twoPageVegetationAttempt, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	payload := string(raw)
	if ledger.Requests == nil {
		ledger.Requests = map[string]*twoPageVegetationAttempt{}
	}
	if existing := ledger.Requests[requestID]; existing != nil {
		if existing.payload != payload {
			return nil, errors.New("two-page vegetation UUID already belongs to another exact request payload")
		}
		return existing, nil
	}
	entry := &twoPageVegetationAttempt{payload: payload}
	ledger.Requests[requestID] = entry
	return entry, nil
}

func VegetationLifeformInteger(cell ProjectMetadataCell) (*int, error) {
	if _, err := MetadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage == "null" {
		return nil, nil
	}
	if cell.Storage != "integer" {
		return nil, errors.New("lifeform requires exact nullable INTEGER storage; no coercion")
	}
	value, err := strconv.ParseInt(*cell.Integer, 10, 16)
	if err != nil {
		return nil, errors.New("lifeform exceeds source INTEGER range")
	}
	result := int(value)
	return &result, nil
}

const LongVegetationProjectScopeFeatureEnvironment = "VPRO_LONG_VEGETATION_PROJECT_SCOPE"

const VegetationSelectedListScope = "selected-list"

const VegetationProjectFieldsScope = "project-selected-fields"

const VegetationProjectAllScope = "project-all-plots"

type LongVegetationProjectUnit struct {
	EnvRowID   string              `json:"envRowId"`
	AdminRowID *string             `json:"adminRowId"`
	PlotNumber ProjectMetadataCell `json:"plotNumber"`
	SiteUnit   ProjectMetadataCell `json:"siteUnit"`
}

type LongVegetationProjectBuild struct {
	ApprovalHash  string                      `json:"approvalHash"`
	ProjectTuples []LongVegetationProjectUnit `json:"projectTuples"`
}

type VegetationProjectScopeEntry struct {
	Revision   int64                       `json:"-" yaml:"-"`
	SourceKind string                      `json:"-" yaml:"-"`
	Fields     []string                    `json:"-" yaml:"-"`
	Build      *LongVegetationProjectBuild `json:"-" yaml:"-"`
}

type vegetationProjectScopeLedger struct {
	Mu      sync.Mutex                             `json:"-" yaml:"-"`
	Entries map[string]VegetationProjectScopeEntry `json:"-" yaml:"-"`
}

func CloneProjectScopeEntry(entry VegetationProjectScopeEntry) VegetationProjectScopeEntry {
	if entry.SourceKind == "" {
		entry.SourceKind = VegetationSelectedListScope
	}
	entry.Fields = append([]string{}, entry.Fields...)
	if entry.Build != nil {
		build := &LongVegetationProjectBuild{ApprovalHash: entry.Build.ApprovalHash,
			ProjectTuples: make([]LongVegetationProjectUnit, len(entry.Build.ProjectTuples))}
		for i, row := range entry.Build.ProjectTuples {
			row.AdminRowID = sqlutil.CloneCatalogueValue(row.AdminRowID)
			row.PlotNumber, row.SiteUnit = CloneSiteUnitCell(row.PlotNumber), CloneSiteUnitCell(row.SiteUnit)
			build.ProjectTuples[i] = row
		}
		entry.Build = build
	}
	return entry
}

func (ledger *vegetationProjectScopeLedger) Read(contextID string) VegetationProjectScopeEntry {
	ledger.Mu.Lock()
	defer ledger.Mu.Unlock()
	return CloneProjectScopeEntry(ledger.Entries[contextID])
}

func (s *ContextService) RequireSelectedListReportScope(contextID string) error {
	s.ProjectReportScopes.Mu.Lock()
	kind := s.ProjectReportScopes.Entries[contextID].SourceKind
	s.ProjectReportScopes.Mu.Unlock()
	if kind != "" && kind != VegetationSelectedListScope {
		return errors.New("Long Vegetation has an explicit project report scope; this selected-list consumer is unavailable")
	}
	return nil
}

func VegetationReportIdentity(cell ProjectMetadataCell) (*string, error) {
	if _, err := MetadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage != "text" && cell.Storage != "null" {
		return nil, errors.New("report identities require original text/NULL without coercion")
	}
	return cell.Text, nil
}

func VegetationReportNumber(cell ProjectMetadataCell) (*big.Rat, error) {
	value, err := MetadataCellValue(cell)
	if err != nil {
		return nil, err
	}
	switch value := value.(type) {
	case nil:
		return nil, nil
	case int64:
		return new(big.Rat).SetInt64(value), nil
	case float64:
		return new(big.Rat).SetFloat64(value), nil
	default:
		return nil, errors.New("report cover requires original finite numeric/NULL storage; no historical text/blob was converted")
	}
}

type LongVegetationSelectedUnitOccurrence struct {
	MembershipID string              `json:"membershipId"`
	EnvRowID     *string             `json:"envRowId"`
	AdminRowID   *string             `json:"adminRowId"`
	PlotNumber   ProjectMetadataCell `json:"plotNumber"`
	SiteUnit     ProjectMetadataCell `json:"siteUnit"`
}

type LongVegetationSelectedUnitBuild struct {
	ApprovalHash string                                 `json:"approvalHash"`
	Occurrences  []LongVegetationSelectedUnitOccurrence `json:"occurrences"`
}

type SelectedUnitSelectionEntry struct {
	Revision int64                            `json:"-" yaml:"-"`
	Fields   []string                         `json:"-" yaml:"-"`
	Build    *LongVegetationSelectedUnitBuild `json:"-" yaml:"-"`
}

type selectedUnitSelectionLedger struct {
	Mu      sync.Mutex                            `json:"-" yaml:"-"`
	Entries map[string]SelectedUnitSelectionEntry `json:"-" yaml:"-"`
}

func CloneSelectedUnitBuild(build *LongVegetationSelectedUnitBuild) *LongVegetationSelectedUnitBuild {
	if build == nil {
		return nil
	}
	result := &LongVegetationSelectedUnitBuild{ApprovalHash: build.ApprovalHash,
		Occurrences: make([]LongVegetationSelectedUnitOccurrence, len(build.Occurrences))}
	for i, row := range build.Occurrences {
		row.EnvRowID, row.AdminRowID = sqlutil.CloneCatalogueValue(row.EnvRowID), sqlutil.CloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = CloneSiteUnitCell(row.PlotNumber), CloneSiteUnitCell(row.SiteUnit)
		result.Occurrences[i] = row
	}
	return result
}

func (ledger *selectedUnitSelectionLedger) Read(contextID string) SelectedUnitSelectionEntry {
	ledger.Mu.Lock()
	defer ledger.Mu.Unlock()
	entry := ledger.Entries[contextID]
	entry.Fields = append([]string{}, entry.Fields...)
	entry.Build = CloneSelectedUnitBuild(entry.Build)
	return entry
}
