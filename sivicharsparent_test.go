package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

func TestSIVICHARSParentSourceBindingsAndLinks(t *testing.T) {
	layout, err := siviCHARSParentSourceLayout()
	if err != nil {
		t.Fatal(err)
	}
	if layout.Root != "frmSIVIsite-CHARS" || len(layout.Forms) != 4 || len(layout.Forms[0].Pages) != 3 ||
		layout.Forms[0].SHA256 != "14e5bf440431009fe4eca7c7282ed63263cc05852d9ac43ef301581fa7801df7" {
		t.Fatal("original CHARS parent source changed")
	}
	expectedChildren := []string{"SubVegA-SIVI", "SubVegC-SIVI", "SubVegD-SIVI"}
	expectedHashes := []string{
		"c4012053ac61191b1cb8f28ece6bdd0f95ce40ad07629436cbd3477c8b098026",
		"72769e0be30ebad0a6716f7bba0e866d42c5a370da9074f0ba809e75ccd052e4",
		"3d3e5a81203111db0cd053cdb7c2032cec1c3fb320c0e53fa1f743ea311362dc",
	}
	if len(layout.Forms[0].Embedded) != 3 {
		t.Fatal("source child instances changed")
	}
	for i, link := range layout.Forms[0].Embedded {
		if link.Form != expectedChildren[i] || layout.Forms[i+1].Name != expectedChildren[i] ||
			layout.Forms[i+1].SHA256 != expectedHashes[i] || !link.Resolved || !link.ReadOnly ||
			!reflect.DeepEqual(link.MasterFields, []string{"PlotNumber"}) ||
			!reflect.DeepEqual(link.ChildFields, []string{"PlotNumber"}) {
			t.Fatal("extended A or original C/D PlotNumber child ownership changed", link)
		}
	}
	expected := strings.Fields(`Aspect BedrockGeology1 BedrockGeology2 Date Elevation
		Exposure1 Exposure2 FSRegionDistrict FieldNumber GeoMorProSubSurf GeoMorProSurf
		HumusForm HumusThickness Latitude Location LocationAccuracy Longitude MapUnit
		MesoSlopePosition MoistureRegime NtsMapSheet NutrientRegime PlotNumber
		PlotRepresenting PlotType ProjectID RealmClass RootRestrictingDepth
		RootRestrictingType RootZoneParticleSize RootingDepth SV_AhorizonDepth
		SV_AhorizonType SV_CanopyComposition SV_FloodPlain SV_GleyingMottlingCM
		SV_PercentCoarseFrags SV_PolygonNumber SV_SoilDepth SV_StandAgeEstMeas
		SV_StandHeight SV_StandHeightEstMeas SV_WaterTableCM SiteDisturbance1
		SiteDisturbance2 SiteDisturbance3 SiteSeries SiteSurveyor SlopeGradient
		SnowCoverregime SoilDrainage StandAge StartDate StrataCoverHerb StrataCoverMoss
		StrataCoverShrub StrataCoverTotal StrataCoverTree StructuralStage SubZone
		SuccessionalStatus SurfaceExpSubSurf SurfaceExpSurf SurfaceShape
		SurficialMaterialSubSurf SurficialMaterialSurf TerrainTextureSubSurf
		TerrainTextureSurf VegNotes Zone siteNotes`)
	bindings, err := siviCHARSParentSourceBindings()
	if err != nil || len(bindings) != 72 {
		t.Fatal("CHARS71 direct/one implicit binding scope changed", err)
	}
	names := make([]string, 0, 71)
	for _, binding := range bindings[:71] {
		if binding.Implicit || !strings.HasPrefix(binding.ControlID, "form:frmSIVIsite-CHARS/") {
			t.Fatal("normal or implicit control leaked into CHARS presentation", binding)
		}
		names = append(names, binding.Binding)
	}
	slices.Sort(names)
	if !reflect.DeepEqual(names, expected) || bindings[71].Binding != "SpeciesListComplete" ||
		!bindings[71].Implicit || bindings[71].ControlID != "" {
		t.Fatal("independent original binding set or implicit target changed", names)
	}
	for _, field := range layout.Forms[0].Fields {
		switch field.Binding {
		case "RootZoneParticleSize":
			if field.Caption != "R. Z. Particle Size" || field.Widget != "choice" {
				t.Fatal("source particle-size label/grouping changed", field)
			}
		case "SV_WaterTableCM":
			if field.Caption != "Water Table Depth" || field.Widget != "text" {
				t.Fatal("source water-depth control changed", field)
			}
		case "Latitude", "Longitude":
			if _, exists := field.Properties["Format"]; exists {
				t.Fatal("normal Fixed6 formatting was invented for CHARS", field.Binding)
			}
		}
	}
	normal, err := siviParentSourceBindings()
	if err != nil || len(normal) != 78 {
		t.Fatal("normal77 identity changed", err)
	}
	normalNames := make(map[string]bool)
	for _, binding := range normal[:77] {
		normalNames[binding.Binding] = true
	}
	normalOnly := []string{}
	for name := range normalNames {
		if !slices.Contains(names, name) {
			normalOnly = append(normalOnly, name)
		}
	}
	slices.Sort(normalOnly)
	if !reflect.DeepEqual(normalOnly, []string{"AirPhotoNum", "SV_RootZoneTexture", "SeepageDepth",
		"UTMEasting", "UTMNorthing", "UTMZone", "XCoord", "YCoord"}) {
		t.Fatal("normal-only source controls changed", normalOnly)
	}
	for _, name := range []string{"RootZoneParticleSize", "SV_WaterTableCM"} {
		if normalNames[name] {
			t.Fatal("CHARS distinct source column widened normal bindings", name)
		}
	}
}

func TestSourceSIVIParentBindingsRejectsMalformedCatalogue(t *testing.T) {
	for _, count := range []int{71, 77} {
		for _, kind := range []string{"duplicate-binding", "missing-control", "writable",
			"mapped", "implicit-direct", "count"} {
			t.Run(strings.Join([]string{map[int]string{71: "CHARS", 77: "normal"}[count], kind}, "/"), func(t *testing.T) {
				layout, err := siviParentSourceLayout()
				if count == 71 {
					layout, err = siviCHARSParentSourceLayout()
				}
				if err != nil {
					t.Fatal(err)
				}
				fields := []fs882layout.Field{}
				for _, field := range layout.Forms[0].Fields {
					if field.Binding != "" {
						fields = append(fields, field)
					}
				}
				switch kind {
				case "duplicate-binding":
					fields[1].Binding = strings.ToUpper(fields[0].Binding)
				case "missing-control":
					fields[0].ControlID = ""
				case "writable":
					fields[0].ReadOnly = false
				case "mapped":
					fields[0].Implementation = "implemented"
				case "implicit-direct":
					fields[0].Binding = "SpeciesListComplete"
				case "count":
					fields = fields[1:]
				}
				if got, err := sourceSIVIParentBindings(fields, count); err == nil || got != nil {
					t.Fatal("invalid source catalogue returned success", got, err)
				}
			})
		}
	}
}

func TestSIVICHARSParentProjectionRawPairsAndDistinctFields(t *testing.T) {
	env, admin := siviParentTables(t)
	blob := "00ff"
	values := map[string]ProjectMetadataCell{
		"RootZoneParticleSize": metadataText("  legacy width  "),
		"SV_WaterTableCM":      siviReal(1e100),
		"SV_RootZoneTexture":   metadataText("different texture"),
		"SeepageDepth":         metadataInteger("-9"),
		"VegNotes":             metadataText(""),
		"SiteNotes":            {Storage: "blob", BlobHex: &blob},
		"SpeciesListComplete":  metadataInteger("-1"),
	}
	for i, column := range env.Columns {
		if value, exists := values[column.Name]; exists {
			env.Rows[0].Cells[i] = value
		}
	}
	env.Rows = append(env.Rows, env.Rows[0])
	env.Rows[1].RowID = "-9"
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "9223372036854775807"
	got, err := projectSIVICHARSParent(context.Background(), "owned", "Sample", "108050", env, admin)
	if err != nil || got == nil || len(got.Rows) != 4 || len(got.Bindings) != 72 ||
		got.Form != "frmSIVIsite-CHARS" || got.Query != "USysEnv" ||
		got.Membership != "literal-binary-inner-pairs" {
		t.Fatal("CHARS literal source/duplicate physical pairs changed", got, err)
	}
	for i, column := range got.EnvColumns {
		if value, exists := values[column.Name]; exists && !reflect.DeepEqual(got.Rows[0].Env.Cells[i], value) {
			t.Fatal("historical read normalized, narrowed or aliased storage", column.Name)
		}
	}
	for _, binding := range got.Bindings {
		if binding.Binding == "RootZoneParticleSize" || binding.Binding == "SV_WaterTableCM" {
			if binding.Table != "Sample_Env" || got.EnvColumns[binding.Column].Name != binding.Binding {
				t.Fatal("CHARS-specific field lost its literal physical owner", binding)
			}
		}
	}
	got.Bindings[0].Binding = "changed"
	*got.Rows[0].Env.Cells[0].Text = "changed"
	got.EnvColumns[0].DeclaredType = "changed"
	again, err := projectSIVICHARSParent(context.Background(), "owned", "Sample", "108050", env, admin)
	if err != nil || again.Bindings[0].Binding == "changed" ||
		*again.Rows[0].Env.Cells[0].Text != "108050" || *got.Rows[2].Env.Cells[0].Text != "108050" ||
		again.EnvColumns[0].DeclaredType == "changed" {
		t.Fatal("CHARS projection shares mutable source/cell ownership", err)
	}
}

func TestSIVICHARSParentProjectionRefusesInvalidOwnersAndIdentities(t *testing.T) {
	for _, kind := range []string{"missing-particle", "missing-water", "ambiguous-particle",
		"ambiguous-water", "shadow-rowid", "malformed-text", "empty-context", "empty-project",
		"empty-plot", "invalid-plot"} {
		t.Run(kind, func(t *testing.T) {
			env, admin := siviParentTables(t)
			contextID, project, plot := "owned", "Sample", "108050"
			for i, column := range env.Columns {
				switch {
				case kind == "missing-particle" && column.Name == "RootZoneParticleSize",
					kind == "missing-water" && column.Name == "SV_WaterTableCM":
					env.Columns[i].Name = "Renamed" + column.Name
				case kind == "ambiguous-particle" && column.Name == "RootZoneParticleSize",
					kind == "ambiguous-water" && column.Name == "SV_WaterTableCM":
					admin.Columns = append(admin.Columns, column)
					admin.Rows[0].Cells = append(admin.Rows[0].Cells, env.Rows[0].Cells[i])
				}
			}
			switch kind {
			case "shadow-rowid":
				env.Columns = append(env.Columns, ProjectMetadataColumn{Name: "RoWiD"})
				env.Rows[0].Cells = append(env.Rows[0].Cells, ProjectMetadataCell{Storage: "null"})
			case "malformed-text":
				env.Rows[0].Cells[1] = metadataText(string([]byte{0xff}))
			case "empty-context":
				contextID = ""
			case "empty-project":
				project = ""
			case "empty-plot":
				plot = ""
			case "invalid-plot":
				plot = "bad\x00"
			}
			if got, err := projectSIVICHARSParent(context.Background(), contextID, project, plot, env, admin); err == nil || got != nil {
				t.Fatal("invalid CHARS source/owner returned partial output", got, err)
			}
		})
	}
	env, admin := siviParentTables(t)
	for _, plot := range []string{"108050 ", "108050x", "missing"} {
		got, err := projectSIVICHARSParent(context.Background(), "owned", "Sample", plot, env, admin)
		if err != nil || got == nil || got.Rows == nil || len(got.Rows) != 0 {
			t.Fatal("CHARS repaired a literal source identity", got, err)
		}
	}
}

func TestSIVICHARSParentOwnedRawReadAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # CHARS.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := contextSelection(state)
				selection.ProjectPath = path
				state, err = service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
			}
			mutateContextFixture(t, state.ProjectPath, `DROP INDEX uidx_Sample_Admin_PlotNumber;
				UPDATE Sample_Env SET RootZoneParticleSize=' legacy ',SV_WaterTableCM=1e100,
					SV_RootZoneTexture='not an alias',SeepageDepth=-9,SpeciesListComplete=-1,
					SiteNotes=X'00FF',VegNotes='' WHERE PlotNumber='108050';
				INSERT INTO Sample_Admin(Plot,PlotType) VALUES('108050','')`)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil || got == nil || got.ContextID != state.ContextID ||
				got.Form != "frmSIVIsite-CHARS" || len(got.Rows) != 2 {
				t.Fatal("owned raw CHARS pair read failed", got, err)
			}
			for i, column := range got.EnvColumns {
				cell := got.Rows[0].Env.Cells[i]
				switch column.Name {
				case "RootZoneParticleSize":
					if cell.Text == nil || *cell.Text != " legacy " {
						t.Fatal("historical TEXT6 was normalized", cell)
					}
				case "SV_WaterTableCM":
					if cell.Real == nil || *cell.Real != 1e100 {
						t.Fatal("historical SINGLE-domain read was narrowed", cell)
					}
				case "SiteNotes":
					if cell.BlobHex == nil || *cell.BlobHex != "00ff" {
						t.Fatal("raw blob changed", cell)
					}
				case "VegNotes":
					if cell.Text == nil || *cell.Text != "" {
						t.Fatal("empty text became NULL", cell)
					}
				}
			}
			if stale, err := service.readSIVICHARSParent(context.Background(), "stale", "108050"); err == nil || stale != nil {
				t.Fatal("stale CHARS owner returned data", stale, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if cancelled, err := service.readSIVICHARSParent(ctx, state.ContextID, "108050"); !errors.Is(err, context.Canceled) || cancelled != nil {
				t.Fatal("cancelled CHARS read returned data", cancelled, err)
			}
			retry, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(retry, got) {
				t.Fatal("rejected CHARS read lost its coordinator", retry, err)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("CHARS read changed preferences", err)
			}
		})
	}
}

func TestSIVICHARSParentOwnedBlockedCancellationAndRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	before, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
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
	if got, err := service.readSIVICHARSParent(ctx, state.ContextID, "108050"); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked CHARS snapshot returned partial/success-shaped output", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	again, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(before, again) {
		t.Fatal("in-flight cancellation discarded CHARS coordinator/attachments", again, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVICHARSParentOwnedPhysicalRefusalCorrectionAndRetry(t *testing.T) {
	for _, kind := range []string{"malformed-text", "missing-water", "physical-view"} {
		t.Run(kind, func(t *testing.T) {
			service, state := contextServiceFixture(t)
			original, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			var change, restore string
			switch kind {
			case "malformed-text":
				mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET RootZoneParticleSize='exact' WHERE PlotNumber='108050'`)
				original, err = service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
				if err != nil {
					t.Fatal(err)
				}
				change = `UPDATE Sample_Env SET RootZoneParticleSize=CAST(X'FF' AS TEXT) WHERE PlotNumber='108050'`
				restore = `UPDATE Sample_Env SET RootZoneParticleSize='exact' WHERE PlotNumber='108050'`
			case "missing-water":
				change = `ALTER TABLE Sample_Env RENAME COLUMN SV_WaterTableCM TO RenamedWater`
				restore = `ALTER TABLE Sample_Env RENAME COLUMN RenamedWater TO SV_WaterTableCM`
			case "physical-view":
				change = `ALTER TABLE Sample_Admin RENAME TO Archived_Admin;
					CREATE VIEW Sample_Admin AS SELECT * FROM Archived_Admin`
				restore = `DROP VIEW Sample_Admin; ALTER TABLE Archived_Admin RENAME TO Sample_Admin`
			}
			mutateContextFixture(t, state.ProjectPath, change)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if got, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050"); err == nil || got != nil {
				t.Fatal("invalid physical CHARS source returned success", got, err)
			}
			assertProfileSUFiles(t, service, before)
			mutateContextFixture(t, state.ProjectPath, restore)
			again, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(again, original) {
				t.Fatal("CHARS correction/retry lost original storage or coordinator", again, err)
			}
		})
	}
}

func TestSIVICHARSParentOwnedConcurrentSwitch(t *testing.T) {
	service, initial := contextServiceFixture(t)
	paths := map[string]string{}
	for alias, path := range service.projects.sqlite.attachments {
		paths[alias] = path
	}
	before := databaseBytes(t, paths)
	var stateMu sync.RWMutex
	state := initial
	failures := make(chan error, 3)
	start, finished := make(chan struct{}), make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(3)
	go func() {
		defer workers.Done()
		<-start
		current := initial
		for range 4 {
			next, err := service.SwitchContext(current.ContextID, contextSelection(current))
			if err != nil {
				failures <- fmt.Errorf("CHARS concurrent switch: %w", err)
				return
			}
			current = next
			stateMu.Lock()
			state = next
			stateMu.Unlock()
		}
	}()
	for range 2 {
		go func() {
			defer workers.Done()
			<-start
			for range 50 {
				stateMu.RLock()
				id := state.ContextID
				stateMu.RUnlock()
				got, err := service.readSIVICHARSParent(context.Background(), id, "108050")
				if err != nil {
					if got != nil || err.Error() != "editor context is stale or closed; reload before editing" {
						failures <- fmt.Errorf("CHARS concurrent read: result=%+v err=%w", got, err)
						return
					}
					continue
				}
				if got == nil || got.ContextID != id || got.Form != "frmSIVIsite-CHARS" ||
					got.Project != "Sample" || len(got.Rows) != 1 || len(got.Bindings) != 72 {
					failures <- fmt.Errorf("CHARS concurrent read crossed source/owner: %+v", got)
					return
				}
			}
		}()
	}
	go func() {
		workers.Wait()
		close(finished)
	}()
	close(start)
	select {
	case <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("CHARS reads/context switches failed to release operation leases")
	}
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	got, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
	if err != nil || got == nil || len(got.Rows) != 1 {
		t.Fatal("CHARS final context cannot retry after concurrent switches", got, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, paths)) {
		t.Fatal("CHARS context reads changed physical database family")
	}
}
