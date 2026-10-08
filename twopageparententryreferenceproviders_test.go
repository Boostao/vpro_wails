package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func twoPageEntryProviderFixture(t *testing.T, form string) (*twoPageEntryReferenceProvider, *ContextService, ProjectState) {
	t.Helper()
	return twoPageEntryProviderSUFixture(t, form, false)
}

func twoPageEntryProviderSUFixture(t *testing.T, form string, external bool) (*twoPageEntryReferenceProvider, *ContextService, ProjectState) {
	t.Helper()
	contexts, state, _, _, _ := twoPageWriteFixture(t, form, external)
	return twoPageEntryCatalogueProviderFixture(t, form), contexts, state
}

func twoPageEntryCatalogueProviderFixture(t *testing.T, form string) *twoPageEntryReferenceProvider {
	t.Helper()
	region, err := NewRegionCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	quality, err := NewQualityService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	soil, err := NewSoilCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bec, err := NewBECService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	geology, err := NewGeologyCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, close := range []func() error{region.Close, quality.Close, soil.Close, bec.Close, geology.Close} {
			if err := close(); err != nil {
				t.Error(err)
			}
		}
	})
	shared := twoPageEntryBorrowedReferenceReaders(t)
	shared.region, shared.bec, shared.geology = region, bec, geology
	provider, err := newTwoPageEntryReferenceProvider(form, twoPageEntryReferenceReaders{
		shared: shared, ecosection: region, quality: quality, soil: soil, series: bec,
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestTwoPageEntryReferenceProviderActualCataloguesAndSourceScope(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		t.Run(form, func(t *testing.T) {
			provider, contexts, state := twoPageEntryProviderFixture(t, form)
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			_, err := withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
				return withOwnedSIVISnapshot(context.Background(), plots, func(_ *sqliteContext, tx *sql.Tx) (bool, error) {
					for _, field := range provider.fields {
						ref, err := provider.read(context.Background(), tx, "VLists", field, metadataText("BG"), metadataText("xh1"))
						switch field.reader {
						case "project", "master-unit", "working-unit":
							if err == nil || ref.Available || !strings.Contains(err.Error(), "context-selection provider") {
								t.Fatal("unimplemented context selection looked like available success", field.column, ref, err)
							}
							continue
						}
						if err != nil || !ref.Available || ref.Column != field.column || ref.ListName != field.list ||
							ref.Required != field.required || len(ref.Choices) == 0 {
							t.Fatal("mapped actual reader failed", field.column, ref, err)
						}
						for _, row := range ref.Definitions.Rows {
							if len(row.Cells) != len(ref.Definitions.Columns) {
								t.Fatal("definition cells/columns lost correspondence", field.column, row)
							}
							for _, cell := range row.Cells {
								if _, err := metadataCellValue(cell); err != nil {
									t.Fatal("invalid reference definition transport", field.column, cell, err)
								}
							}
						}
						if field.column == "Ecosection" {
							selectable := 0
							for _, choice := range ref.Choices {
								if choice.Selectable {
									selectable++
								}
							}
							if len(ref.Choices) != 137 || selectable == 0 || ref.ListName != "ecosection" ||
								!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[0], metadataText("Ecosection")) {
								t.Fatal("source list alias altered raw catalogue metadata or invalidated every choice", ref)
							}
						}
						if field.reader == "quality" && (len(ref.Choices) != 5 ||
							!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[3], metadataText("NA")) ||
							!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[4], metadataText("null"))) {
							t.Fatal("quality literal NA/null definition or full five-row source changed", ref)
						}
						if field.column == "SurfaceTopographyType" {
							excluded := 0
							for _, choice := range ref.Choices {
								if choice.Code != nil && (*choice.Code == "cc" || *choice.Code == "cv" || *choice.Code == "st") {
									excluded++
									if choice.Selectable || choice.Diagnostic == "" {
										t.Fatal("excluded source topography code offered as selectable", choice)
									}
								}
							}
							if excluded != 0 || len(ref.Definitions.Rows) != 14 {
								t.Fatal("source topography exclusions lost complete raw definitions", ref)
							}
						}
					}
					return true, nil
				})
			})
			if err != nil {
				t.Fatal(err)
			}
			assertProfileSUFiles(t, contexts, before)
			field := provider.fields["SitePlotQuality"]
			field.required = true
			if _, err := provider.read(context.Background(), nil, "", field, metadataText("BG"), metadataText("xh1")); err == nil {
				t.Fatal("caller changed the exact source field policy")
			}
			if form == "FS882-8x6XL-CHARS" {
				if _, exists := provider.fields["BEC_Use"]; exists {
					t.Fatal("CHARS inherited absent BEC_Use")
				}
			}
		})
	}
	if _, err := newTwoPageEntryReferenceProvider("frmSIVIsite", twoPageEntryReferenceReaders{}); err == nil {
		t.Fatal("foreign form accepted")
	}
}

type twoPageEntryBECProbe struct {
	zone          *string
	seriesZone    *string
	seriesSubZone *string
	subZones      []BECSubZone
	series        []BECSiteSeries
	err           error
}

func (s *twoPageEntryBECProbe) ListBECZones(context.Context) ([]BECZone, error) {
	return nil, s.err
}

func (s *twoPageEntryBECProbe) ListBECSubZones(_ context.Context, zone *string) ([]BECSubZone, error) {
	s.zone = zone
	return s.subZones, s.err
}

func (s *twoPageEntryBECProbe) ListBECSiteSeries(_ context.Context, zone, subZone *string) ([]BECSiteSeries, error) {
	s.seriesZone, s.seriesSubZone = zone, subZone
	return s.series, s.err
}

func TestTwoPageEntryReferenceProviderBECFiltersAndCompleteDefinitions(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		probe := &twoPageEntryBECProbe{
			subZones: []BECSubZone{{RowID: "1", Zone: becString("BG"), SubZone: becString("xh1")},
				{RowID: "2", Zone: becString("BG"), SubZone: becString("xh1"), Description: becString("")},
				{RowID: "3"}},
		}
		provider, err := newTwoPageEntryReferenceProvider(form, twoPageEntryReferenceReaders{
			shared: siviParentSharedReferenceReaders{bec: probe}, series: probe,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, zone := range []ProjectMetadataCell{{Storage: "null"}, metadataText("All"), metadataText("BG")} {
			ref, err := provider.read(context.Background(), nil, "", provider.fields["SubZone"], zone, ProjectMetadataCell{Storage: "null"})
			if err != nil || !ref.Available || len(ref.Choices) != 3 || len(ref.Definitions.Rows) != 3 ||
				!ref.Choices[0].Selectable || !ref.Choices[1].Selectable || ref.Choices[2].Selectable ||
				ref.Definitions.Rows[0].Cells[3].Storage != "null" || !reflect.DeepEqual(ref.Definitions.Rows[1].Cells[3], metadataText("")) {
				t.Fatal("duplicate/NULL/empty SubZone definition collapsed or became selectable", ref, err)
			}
			if zone.Storage == "null" || *zone.Text == "All" {
				if probe.zone != nil {
					t.Fatal("source NULL/All did not request all SubZones")
				}
			} else if probe.zone == nil || *probe.zone != "BG" {
				t.Fatal("literal source Zone filter changed", probe.zone)
			}
		}
		for _, zone := range []ProjectMetadataCell{metadataText(""), metadataText("12345"), metadataInteger("1"),
			{Storage: "null", Text: becString("BG")}, metadataText(string([]byte{0xff}))} {
			if _, err := provider.read(context.Background(), nil, "", provider.fields["SubZone"], zone,
				ProjectMetadataCell{Storage: "null"}); err == nil {
				t.Fatal("invalid typed/filter value silently repaired", zone)
			}
		}
		referenceID, flag := 3.5, true
		probe.series = []BECSiteSeries{{RowID: "7", SourceID: becString("source"), SSCode: becString("code"),
			Name: becString("name"), Region: becString("region"), Zone: becString("BG"), SubZone: becString("xh1"),
			BaseSubZone: becString("xh"), Variant: becString("1"), Phase: becString("phase"), SiteSeries: becString("01"),
			SiteSeriesPhase: becString("ssphase"), Variation: becString("variation"), Seral: becString("seral"),
			Description: becString("description"), PlantAssociation: becString("plants"), Comments: becString(""),
			ReferenceID: &referenceID, AddedDate: becString("added"), ExpiredDate: becString("expired"),
			OriginalSourceID: becString("original"), MergedBGC: becString("merged"), Suballiance: becString("suballiance"),
			Alliance: becString("alliance"), MissingNpeNa: becString("missing"), TransferID: becString("transfer"),
			Flag: &flag, Selectable: true},
			{RowID: "8", SiteSeries: becString("01"), Description: becString("")},
			{RowID: "9"}}
		ref, err := provider.read(context.Background(), nil, "", provider.fields["SiteSeries"], metadataText("BG"), metadataText("xh1"))
		if err != nil || !ref.Available || len(ref.Choices) != 3 || !ref.Choices[0].Selectable ||
			ref.Choices[1].Selectable || ref.Choices[2].Selectable || len(ref.Definitions.Columns) != 26 ||
			probe.seriesZone == nil || *probe.seriesZone != "BG" || probe.seriesSubZone == nil || *probe.seriesSubZone != "xh1" {
			t.Fatal("SiteSeries filtering/definition boundary changed", ref, err)
		}
		want := []ProjectMetadataCell{metadataText("source"), metadataText("code"), metadataText("name"),
			metadataText("region"), metadataText("BG"), metadataText("xh1"), metadataText("xh"), metadataText("1"),
			metadataText("phase"), metadataText("01"), metadataText("ssphase"), metadataText("variation"),
			metadataText("seral"), metadataText("description"), metadataText("plants"), metadataText(""),
			{Storage: "real", Real: &referenceID}, metadataText("added"), metadataText("expired"),
			metadataText("original"), metadataText("merged"), metadataText("suballiance"), metadataText("alliance"),
			metadataText("missing"), metadataText("transfer"), metadataInteger("-1")}
		if !reflect.DeepEqual(ref.Definitions.Rows[0].Cells, want) ||
			ref.Definitions.Columns[16] != (ProjectMetadataColumn{"ReferenceID", "REAL"}) ||
			ref.Definitions.Columns[25] != (ProjectMetadataColumn{"Flag", "BOOLEAN"}) {
			t.Fatal("complete borrowed SiteSeries metadata lost identity/order/types/Access BOOLEAN", ref.Definitions)
		}
		probe.series = nil
		ref, err = provider.read(context.Background(), nil, "", provider.fields["SiteSeries"], ProjectMetadataCell{Storage: "null"}, metadataText("xh1"))
		if err != nil || !ref.Available || len(ref.Choices) != 0 || probe.seriesZone != nil {
			t.Fatal("NULL Zone inherited stale SiteSeries choices", ref, err)
		}
		probe.err = errors.New("independent catalogue lookup failed")
		if _, err := provider.read(context.Background(), nil, "", provider.fields["SubZone"], metadataText("BG"), metadataText("xh1")); !errors.Is(err, probe.err) {
			t.Fatal("lookup error converted to acknowledged availability", err)
		}
		probe.err = nil
		if ref, err := provider.read(context.Background(), nil, "", provider.fields["SubZone"], metadataText("BG"), metadataText("xh1")); err != nil || !ref.Available {
			t.Fatal("retry did not recover", ref, err)
		}
	}
}

func TestTwoPageEntryReferenceProviderMissingClosedAndCancelledReaders(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	for _, name := range []string{"Ecosection", "SitePlotQuality", "SoilClassGroup", "SiteSeries", "SubZone"} {
		field := provider.fields[name]
		empty, err := newTwoPageEntryReferenceProvider("FS882-8x6XL", twoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		if ref, err := empty.read(context.Background(), nil, "", field, metadataText("BG"), metadataText("xh1")); err == nil || ref.Available {
			t.Fatal("missing borrowed reader returned success", name, ref, err)
		}
	}
	quality := provider.readers.quality.(*QualityService)
	if err := quality.Close(); err != nil {
		t.Fatal(err)
	}
	if ref, err := provider.read(context.Background(), nil, "", provider.fields["SitePlotQuality"], metadataText("BG"), metadataText("xh1")); err == nil || ref.Available {
		t.Fatal("closed catalogue became acknowledged unavailable", ref, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, field := range provider.fields {
		if _, err := provider.read(cancelled, nil, "", field, metadataText("BG"), metadataText("xh1")); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation disappeared", field.column, err)
		}
	}
	if _, err := provider.read(context.Background(), nil, "", provider.fields["PlotType"], metadataText("BG"), metadataText("xh1")); err == nil {
		t.Fatal("family reader accepted missing owned transaction")
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	_, err := withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
		return withOwnedSIVISnapshot(context.Background(), plots, func(_ *sqliteContext, tx *sql.Tx) (bool, error) {
			ref, err := provider.read(context.Background(), tx, "VLists", provider.fields["LocationAccuracy"], metadataText("BG"), metadataText("xh1"))
			if err != nil {
				return false, err
			}
			for _, choice := range ref.Choices {
				if choice.Selectable {
					if _, err := twoPageEntryReferenceCode(provider.fields["LocationAccuracy"],
						ProjectMetadataCell{Storage: "integer", Integer: choice.Code}); err != nil {
						t.Fatal("noncanonical numeric reference looked selectable", choice, err)
					}
				}
			}
			return true, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestTwoPageEntryReferenceProviderPhysicalExclusionsAndNumericChoices(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	db, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, item := range []string{"cc", "CV", "St"} {
		if _, err := db.Exec(`INSERT INTO USysTableOfLists(ListName,Item,ItemDescription) VALUES ('SurfaceTopography',?,NULL)`, item); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []string{"01", "+1", "1.0", "32768", "-32768"} {
		if _, err := db.Exec(`INSERT INTO USysTableOfLists(ListName,Item,ItemDescription) VALUES ('Accuracy',?,NULL)`, item); err != nil {
			t.Fatal(err)
		}
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	_, err = withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
		return withOwnedSIVISnapshot(context.Background(), plots, func(_ *sqliteContext, tx *sql.Tx) (bool, error) {
			ref, err := provider.read(context.Background(), tx, "VLists", provider.fields["SurfaceTopographyType"],
				ProjectMetadataCell{Storage: "null"}, ProjectMetadataCell{Storage: "null"})
			if err != nil {
				return false, err
			}
			if len(ref.Definitions.Rows) != 17 {
				t.Fatal("excluded raw definitions removed", len(ref.Definitions.Rows))
			}
			seen := map[string]bool{}
			for _, choice := range ref.Choices {
				if choice.Code != nil && (*choice.Code == "cc" || *choice.Code == "CV" || *choice.Code == "St") {
					seen[*choice.Code] = true
					if choice.Selectable || !strings.Contains(choice.Diagnostic, "excluded") {
						t.Fatal("source case-insensitive topography exclusion ignored", choice)
					}
				}
			}
			if len(seen) != 3 {
				t.Fatal("excluded choices collapsed", seen)
			}
			ref, err = provider.read(context.Background(), tx, "VLists", provider.fields["LocationAccuracy"],
				ProjectMetadataCell{Storage: "null"}, ProjectMetadataCell{Storage: "null"})
			if err != nil {
				return false, err
			}
			seen = map[string]bool{}
			for _, choice := range ref.Choices {
				if choice.Code == nil {
					continue
				}
				switch *choice.Code {
				case "01", "+1", "1.0", "32768":
					seen[*choice.Code] = true
					if choice.Selectable || choice.Diagnostic == "" {
						t.Fatal("invalid integer choice became selectable", choice)
					}
				case "-32768":
					seen[*choice.Code] = true
					if !choice.Selectable {
						t.Fatal("exact signed short boundary rejected", choice)
					}
				}
			}
			if len(seen) != 5 || len(ref.Definitions.Rows) != 11 {
				t.Fatal("numeric invalid/raw definitions dropped", seen, len(ref.Definitions.Rows))
			}
			return true, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProfileSUFiles(t, contexts, before)
}
