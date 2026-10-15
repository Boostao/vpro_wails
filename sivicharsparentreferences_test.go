package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func charsReferenceFixture(t *testing.T) (*SIVIParentSharedService, ProjectState) {
	t.Helper()
	service, state, _ := sharedReferenceFixture(t)
	mutateContextFixture(t, service.contexts.projects.sqlite.attachments["VLists"], `
		DELETE FROM USysTableOfLists WHERE ListName='SnowCoverRegime';
		INSERT INTO USysTableOfLists(rowid,ListName,ListFilter,ItemOrder,Item,ItemDescription,Validate,Flag) VALUES
		(910001,'SnowCoverRegime',NULL,NULL,NULL,'Null',-1,-1),
		(910002,'SnowCoverRegime','',2,'','',NULL,0),
		(910003,'SnowCoverRegime',NULL,3,'Z','first',1,0),
		(910004,'SnowCoverRegime',NULL,3,'Z','second',1,0),
		(910005,'SnowCoverRegime',NULL,4,'LONG','too long',0,0);`)
	return service, state
}

func charsReferenceField(t *testing.T, snapshot *siviCHARSParentReferenceSnapshot, column string) SIVIParentSharedReference {
	t.Helper()
	for _, field := range snapshot.Fields {
		if field.Column == column {
			return field
		}
	}
	t.Fatal("missing CHARS reference", column)
	return SIVIParentSharedReference{}
}

func TestSIVICHARSReferencePoliciesMatchOriginalComboSources(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "chars-sivi-reference-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		CanonicalInputSHA256 map[string]string
		Forms                map[string]map[string]struct {
			Name, Binding string
			Properties    map[string]string
		}
	}
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	chars, normal := source.Forms["frmSIVIsite-CHARS"], source.Forms["frmSIVIsite"]
	if len(chars) != 32 || len(normal) != 32 || len(source.CanonicalInputSHA256) != 2 {
		t.Fatal("source fixture lacks complete original combos")
	}
	for _, hash := range source.CanonicalInputSHA256 {
		if len(hash) != 64 {
			t.Fatal("source identity missing")
		}
	}
	fields, err := siviCHARSParentReferenceFields()
	if err != nil || len(fields) != 31 {
		t.Fatal("CHARS reference scope changed", fields, err)
	}
	metadata := map[string]charsEntrySourceField{}
	for _, field := range charsEntrySourceFields(t) {
		metadata[field.Binding] = field
	}
	required, shared := 0, 0
	seen := map[string]bool{}
	for _, field := range fields {
		control, present := chars[field.column]
		if !present || control.Binding != field.column || seen[field.column] ||
			metadata[field.column].Type != 10 || metadata[field.column].Size != field.maximum {
			t.Fatal("policy differs from independent source control/DAO width", field)
		}
		seen[field.column] = true
		if field.required != (control.Properties["LimitToList"] == "NotDefault") {
			t.Fatal("membership property changed", field)
		}
		if field.required {
			required++
		}
		if common, present := normal[field.column]; present {
			left, right := map[string]string{}, map[string]string{}
			for key, value := range common.Properties {
				if key != "ColumnWidths" {
					left[key] = value
				}
			}
			for key, value := range control.Properties {
				if key != "ColumnWidths" {
					right[key] = value
				}
			}
			if !reflect.DeepEqual(left, right) {
				t.Fatal("shared source query/events are not equivalent", field.column)
			}
		}
		if _, exists := siviParentSharedReferenceField(field.column); exists {
			shared++
		}
		switch field.column {
		case "Zone":
			if !strings.Contains(control.Properties["RowSource"], "USysZoneList.Zone") {
				t.Fatal("Zone source changed")
			}
		case "SubZone":
			if !strings.Contains(control.Properties["RowSource"], "[forms]![frmSIVIsite]![cmbZone]") {
				t.Fatal("unqualified foreign-form dependency was hidden")
			}
		case "SV_AhorizonType":
			if control.Properties["RowSource"] != `Ah;Ae;""` || control.Properties["RowSourceType"] != "Value List" {
				t.Fatal("literal source choices changed")
			}
		default:
			sourceList := field.list
			switch field.column {
			case "RealmClass":
				sourceList = "realmclass"
			case "SurfaceShape":
				sourceList = "surfaceshape"
			}
			if !strings.Contains(control.Properties["RowSource"], `"`+sourceList+`"`) {
				t.Fatal("reference list name differs from original SQL", field)
			}
		}
	}
	if required != 4 || shared != 28 || seen["ProjectID"] || seen["SV_RootZoneTexture"] ||
		seen["SiteSeries"] || seen["LocationAccuracy"] {
		t.Fatal("CHARS acquired unrelated normal/two-page authority", required, shared, seen)
	}
	fields[0].column = "changed"
	again, err := siviCHARSParentReferenceFields()
	if err != nil || again[0].column == "changed" {
		t.Fatal("source policies share mutable ownership", err)
	}
}

type charsReferenceBECWitness struct {
	base siviParentSharedReferenceReaders
}

func (w charsReferenceBECWitness) ListBECZones(ctx context.Context) ([]BECZone, error) {
	return w.base.bec.ListBECZones(ctx)
}

func (w charsReferenceBECWitness) ListBECSubZones(context.Context, *string) ([]BECSubZone, error) {
	return nil, errors.New("unqualified CHARS SubZone reader must never run")
}

func TestSIVICHARSReferencesOwnedSourceRawDefinitionsAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := charsReferenceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "CHARS project # literal.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := contextSelection(state)
				selection.ProjectPath = path
				state, err = service.contexts.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			readers := service.references
			readers.bec = charsReferenceBECWitness{service.references}
			got, err := service.contexts.readSIVICHARSParentReferences(context.Background(), state.ContextID, "108050", readers)
			if err != nil || got == nil || got.Original.Form != "frmSIVIsite-CHARS" ||
				got.Original.ContextID != state.ContextID || len(got.Original.Rows) != 1 || len(got.Fields) != 31 {
				t.Fatal("owned CHARS references failed", got, err)
			}
			for _, field := range got.Fields {
				if field.Column == "SubZone" {
					if field.Available || field.Diagnostic == "" || len(field.Choices) != 0 || len(field.Definitions.Rows) != 0 {
						t.Fatal("unqualified foreign-form reference became available", field)
					}
				} else if !field.Available || len(field.Choices) == 0 {
					t.Fatal("qualified reference lost availability", field)
				}
			}
			snow := charsReferenceField(t, got, "SnowCoverregime")
			if !reflect.DeepEqual(snow.Definitions.Columns, siviParentReferenceColumns) ||
				len(snow.Definitions.Rows) != 5 || snow.Definitions.Rows[0].RowID != "910001" ||
				snow.Definitions.Rows[0].Cells[1].Storage != "null" ||
				*snow.Definitions.Rows[1].Cells[1].Text != "" || *snow.Definitions.Rows[0].Cells[7].Integer != "-1" ||
				snow.Choices[0].Selectable || snow.Choices[1].Selectable || !snow.Choices[2].Selectable ||
				!snow.Choices[3].Selectable || snow.Choices[4].Selectable ||
				*snow.Choices[2].Description != "first" || *snow.Choices[3].Description != "second" {
				t.Fatal("Snow NULL/empty/duplicates/raw flags/width changed", snow)
			}
			horizon := charsReferenceField(t, got, "SV_AhorizonType")
			if len(horizon.Choices) != 3 || *horizon.Choices[0].Code != "Ah" || *horizon.Choices[1].Code != "Ae" ||
				*horizon.Choices[2].Code != "" || horizon.Choices[2].Selectable || horizon.Choices[2].Diagnostic == "" {
				t.Fatal("Ah/Ae/blank source distinction changed", horizon)
			}
			particle := charsReferenceField(t, got, "RootZoneParticleSize")
			if particle.ListName != "RootZoneParticleSize" || len(particle.Definitions.Rows) != 25 {
				t.Fatal("RootZoneParticleSize aliased normal texture or lost raw definitions", particle)
			}
			for _, reference := range []*SIVIParentSharedReference{&snow, &horizon, &particle} {
				for i := range reference.Choices {
					if reference.Choices[i].Code != nil {
						*reference.Choices[i].Code = "changed"
					}
				}
				for _, row := range reference.Definitions.Rows {
					for _, cell := range row.Cells {
						if cell.Text != nil {
							*cell.Text = "changed"
						}
					}
				}
			}
			*got.Original.Rows[0].Env.Cells[0].Text = "changed"
			again, err := service.contexts.readSIVICHARSParentReferences(context.Background(), state.ContextID, "108050", readers)
			if err != nil || *again.Original.Rows[0].Env.Cells[0].Text != "108050" ||
				*charsReferenceField(t, again, "SnowCoverregime").Choices[2].Code != "Z" ||
				*charsReferenceField(t, again, "SV_AhorizonType").Choices[0].Code != "Ah" ||
				reflect.DeepEqual(charsReferenceField(t, again, "RootZoneParticleSize"), particle) {
				t.Fatal("returned references mutate owned source/catalogue", err)
			}
			assertProfileSUFiles(t, service.contexts, before)
		})
	}
}

type charsReferenceParentStub struct {
	read func(context.Context, string) ([]ParentCodeChoice, error)
}

func (s charsReferenceParentStub) ListChoices(ctx context.Context, list string) ([]ParentCodeChoice, error) {
	return s.read(ctx, list)
}

func TestSIVICHARSReferencesPropagateBorrowedErrorsMalformedTextAndCancellation(t *testing.T) {
	service, state := charsReferenceFixture(t)
	parent := service.references.parent
	failure := errors.New("independent reference failure")
	for _, kind := range []string{"error", "malformed-description", "malformed-code", "cancel-in-flight", "missing-reader"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			readers := service.references
			readers.parent = charsReferenceParentStub{func(ctx context.Context, list string) ([]ParentCodeChoice, error) {
				if list != "RootZoneParticleSize" {
					return parent.ListChoices(ctx, list)
				}
				switch kind {
				case "error":
					return nil, failure
				case "cancel-in-flight":
					cancel()
				}
				rows, err := parent.ListChoices(context.Background(), list)
				if err != nil {
					return nil, err
				}
				bad := string([]byte{0xff})
				if kind == "malformed-description" {
					rows[0].Description = &bad
				}
				if kind == "malformed-code" {
					rows[0].Code = &bad
				}
				return rows, nil
			}}
			if kind == "missing-reader" {
				readers.parent = nil
			}
			got, err := service.contexts.readSIVICHARSParentReferences(ctx, state.ContextID, "108050", readers)
			if err == nil || got != nil || kind == "error" && !errors.Is(err, failure) ||
				kind == "cancel-in-flight" && !errors.Is(err, context.Canceled) {
				t.Fatal("borrowed failure became partial/unavailable success", got, err)
			}
			if retry, err := service.contexts.readSIVICHARSParentReferences(context.Background(), state.ContextID, "108050", service.references); err != nil || retry == nil {
				t.Fatal("failed read discarded coordinator/attachments", err)
			}
		})
	}
}

func TestSIVICHARSReferencesPhysicalFamilyFailureCorrectionAndRetry(t *testing.T) {
	service, state := charsReferenceFixture(t)
	path := service.contexts.projects.sqlite.attachments["VLists"]
	for _, kind := range []string{"malformed-text", "missing-list", "view"} {
		t.Run(kind, func(t *testing.T) {
			switch kind {
			case "malformed-text":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ItemDescription=CAST(X'FF' AS TEXT) WHERE rowid=910003`)
			case "missing-list":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ListName='UnrelatedSnow' WHERE ListName='SnowCoverRegime'`)
			case "view":
				mutateContextFixture(t, path, `ALTER TABLE USysTableOfLists RENAME TO SavedLists; CREATE VIEW USysTableOfLists AS SELECT * FROM SavedLists`)
			}
			got, err := service.contexts.readSIVICHARSParentReferences(context.Background(), state.ContextID, "108050", service.references)
			if err == nil || got != nil {
				t.Fatal("physical family read error suppressed", got, err)
			}
			switch kind {
			case "malformed-text":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ItemDescription='first' WHERE rowid=910003`)
			case "missing-list":
				mutateContextFixture(t, path, `UPDATE USysTableOfLists SET ListName='SnowCoverRegime' WHERE ListName='UnrelatedSnow'`)
			case "view":
				mutateContextFixture(t, path, `DROP VIEW USysTableOfLists; ALTER TABLE SavedLists RENAME TO USysTableOfLists`)
			}
			if retry, err := service.contexts.readSIVICHARSParentReferences(context.Background(), state.ContextID, "108050", service.references); err != nil || retry == nil {
				t.Fatal("corrected physical source cannot retry", err)
			}
		})
	}
}

func TestSIVICHARSReferencesRefuseStaleAbsentAmbiguousAndCancelledRequests(t *testing.T) {
	service, state := charsReferenceFixture(t)
	for _, kind := range []string{"stale", "absent", "cancelled", "ambiguous"} {
		t.Run(kind, func(t *testing.T) {
			contextID, plot := state.ContextID, "108050"
			ctx := context.Background()
			switch kind {
			case "stale":
				contextID = "stale"
			case "absent":
				plot = "108050 "
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			case "ambiguous":
				mutateContextFixture(t, state.ProjectPath, `DROP INDEX uidx_Sample_Admin_PlotNumber; INSERT INTO Sample_Admin(Plot) VALUES('108050')`)
			}
			if got, err := service.contexts.readSIVICHARSParentReferences(ctx, contextID, plot, service.references); err == nil || got != nil {
				t.Fatal("invalid physical/context request returned references", got, err)
			}
		})
	}
}

func TestSIVICHARSReferencesBlockedSnapshotCancellationAndRetry(t *testing.T) {
	service, state := charsReferenceFixture(t)
	before, err := service.contexts.readSIVICHARSParentReferences(context.Background(), state.ContextID, "108050", service.references)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if got, err := service.contexts.readSIVICHARSParentReferences(ctx, state.ContextID, "108050", service.references); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked owned reference snapshot returned partial data", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	if retry, err := service.contexts.readSIVICHARSParentReferences(context.Background(), state.ContextID, "108050", service.references); err != nil || !reflect.DeepEqual(before, retry) {
		t.Fatal("blocked cancellation lost pinned coordinator/attachments", err)
	}
}

func TestSIVICHARSReferenceRejectsForgedPoliciesAndUnownedTransaction(t *testing.T) {
	service, state := charsReferenceFixture(t)
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		t.Fatal(err)
	}
	policy := fields[0]
	if _, err := readSIVICHARSParentReference(context.Background(), nil, service.references, policy); err == nil {
		t.Fatal("unowned transaction accepted")
	}
	_, err = withContextPlotRequest(context.Background(), service.contexts, state.ContextID, func(plots *PlotService) (bool, error) {
		return withOwnedSIVISnapshot(context.Background(), plots, func(_ *sqliteContext, tx *sql.Tx) (bool, error) {
			for _, kind := range []string{"column", "list", "reader", "width", "required"} {
				forged := policy
				switch kind {
				case "column":
					forged.column = "SV_RootZoneTexture"
				case "list":
					forged.list = "Unrelated"
				case "reader":
					forged.reader = "unknown"
				case "width":
					forged.maximum++
				case "required":
					forged.required = !forged.required
				}
				if _, err := readSIVICHARSParentReference(context.Background(), tx, service.references, forged); err == nil {
					t.Fatal("forged source policy accepted", kind)
				}
			}
			return true, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}
