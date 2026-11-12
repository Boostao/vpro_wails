package integration

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageEntryReferenceProviderActualCataloguesAndSourceScope(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		t.Run(form, func(t *testing.T) {
			provider, contexts, state := twoPageEntryProviderFixture(t, form)
			before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
			_, err := vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
				return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(_ *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
					for _, field := range provider.Fields {
						ref, err := provider.Read(context.Background(), tx, "VLists", field, vprotest.MetadataText("BG"), vprotest.MetadataText("xh1"))
						switch field.Reader {
						case "project", "master-unit", "working-unit":
							if err == nil || ref.Available || !strings.Contains(err.Error(), "context-selection provider") {
								t.Fatal("unimplemented context selection looked like available success", field.Column, ref, err)
							}
							continue
						}
						if err != nil || !ref.Available || ref.Column != field.Column || ref.ListName != field.List ||
							ref.Required != field.Required || len(ref.Choices) == 0 {
							t.Fatal("mapped actual reader failed", field.Column, ref, err)
						}
						for _, row := range ref.Definitions.Rows {
							if len(row.Cells) != len(ref.Definitions.Columns) {
								t.Fatal("definition cells/columns lost correspondence", field.Column, row)
							}
							for _, cell := range row.Cells {
								if _, err := vprocore.MetadataCellValue(cell); err != nil {
									t.Fatal("invalid reference definition transport", field.Column, cell, err)
								}
							}
						}
						if field.Column == "Ecosection" {
							selectable := 0
							for _, choice := range ref.Choices {
								if choice.Selectable {
									selectable++
								}
							}
							if len(ref.Choices) != 137 || selectable == 0 || ref.ListName != "ecosection" ||
								!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[0], vprotest.MetadataText("Ecosection")) {
								t.Fatal("source list alias altered raw catalogue metadata or invalidated every choice", ref)
							}
						}
						if field.Reader == "quality" && (len(ref.Choices) != 5 ||
							!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[3], vprotest.MetadataText("NA")) ||
							!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[4], vprotest.MetadataText("null"))) {
							t.Fatal("quality literal NA/null definition or full five-row source changed", ref)
						}
						if field.Column == "SurfaceTopographyType" {
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
			vprotest.AssertProfileSUFiles(t, contexts, before)
			field := provider.Fields["SitePlotQuality"]
			field.Required = true
			if _, err := provider.Read(context.Background(), nil, "", field, vprotest.MetadataText("BG"), vprotest.MetadataText("xh1")); err == nil {
				t.Fatal("caller changed the exact source field policy")
			}
			if form == "FS882-8x6XL-CHARS" {
				if _, exists := provider.Fields["BEC_Use"]; exists {
					t.Fatal("CHARS inherited absent BEC_Use")
				}
			}
		})
	}
	if _, err := twopage.NewTwoPageEntryReferenceProvider("frmSIVIsite", twopage.TwoPageEntryReferenceReaders{}); err == nil {
		t.Fatal("foreign form accepted")
	}
}

type twoPageEntryBECProbe struct {
	zone          *string
	seriesZone    *string
	seriesSubZone *string
	subZones      []references.BECSubZone
	series        []references.BECSiteSeries
	err           error
}

func (s *twoPageEntryBECProbe) ListBECZones(context.Context) ([]references.BECZone, error) {
	return nil, s.err
}

func (s *twoPageEntryBECProbe) ListBECSubZones(_ context.Context, zone *string) ([]references.BECSubZone, error) {
	s.zone = zone
	return s.subZones, s.err
}

func (s *twoPageEntryBECProbe) ListBECSiteSeries(_ context.Context, zone, subZone *string) ([]references.BECSiteSeries, error) {
	s.seriesZone, s.seriesSubZone = zone, subZone
	return s.series, s.err
}

func TestTwoPageEntryReferenceProviderBECFiltersAndCompleteDefinitions(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		probe := &twoPageEntryBECProbe{
			subZones: []references.BECSubZone{{RowID: "1", Zone: vprotest.BecString("BG"), SubZone: vprotest.BecString("xh1")},
				{RowID: "2", Zone: vprotest.BecString("BG"), SubZone: vprotest.BecString("xh1"), Description: vprotest.BecString("")},
				{RowID: "3"}},
		}
		provider, err := twopage.NewTwoPageEntryReferenceProvider(form, twopage.TwoPageEntryReferenceReaders{
			Shared: sourceform.SiviParentSharedReferenceReaders{Bec: probe}, Series: probe,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, zone := range []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("All"), vprotest.MetadataText("BG")} {
			ref, err := provider.Read(context.Background(), nil, "", provider.Fields["SubZone"], zone, vprocore.ProjectMetadataCell{Storage: "null"})
			if err != nil || !ref.Available || len(ref.Choices) != 3 || len(ref.Definitions.Rows) != 3 ||
				!ref.Choices[0].Selectable || !ref.Choices[1].Selectable || ref.Choices[2].Selectable ||
				ref.Definitions.Rows[0].Cells[3].Storage != "null" || !reflect.DeepEqual(ref.Definitions.Rows[1].Cells[3], vprotest.MetadataText("")) {
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
		for _, zone := range []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText("12345"), vprotest.MetadataInteger("1"),
			{Storage: "null", Text: vprotest.BecString("BG")}, vprotest.MetadataText(string([]byte{0xff}))} {
			if _, err := provider.Read(context.Background(), nil, "", provider.Fields["SubZone"], zone,
				vprocore.ProjectMetadataCell{Storage: "null"}); err == nil {
				t.Fatal("invalid typed/filter value silently repaired", zone)
			}
		}
		referenceID, flag := 3.5, true
		probe.series = []references.BECSiteSeries{{RowID: "7", SourceID: vprotest.BecString("source"), SSCode: vprotest.BecString("code"),
			Name: vprotest.BecString("name"), Region: vprotest.BecString("region"), Zone: vprotest.BecString("BG"), SubZone: vprotest.BecString("xh1"),
			BaseSubZone: vprotest.BecString("xh"), Variant: vprotest.BecString("1"), Phase: vprotest.BecString("phase"), SiteSeries: vprotest.BecString("01"),
			SiteSeriesPhase: vprotest.BecString("ssphase"), Variation: vprotest.BecString("variation"), Seral: vprotest.BecString("seral"),
			Description: vprotest.BecString("description"), PlantAssociation: vprotest.BecString("plants"), Comments: vprotest.BecString(""),
			ReferenceID: &referenceID, AddedDate: vprotest.BecString("added"), ExpiredDate: vprotest.BecString("expired"),
			OriginalSourceID: vprotest.BecString("original"), MergedBGC: vprotest.BecString("merged"), Suballiance: vprotest.BecString("suballiance"),
			Alliance: vprotest.BecString("alliance"), MissingNpeNa: vprotest.BecString("missing"), TransferID: vprotest.BecString("transfer"),
			Flag: &flag, Selectable: true},
			{RowID: "8", SiteSeries: vprotest.BecString("01"), Description: vprotest.BecString("")},
			{RowID: "9"}}
		ref, err := provider.Read(context.Background(), nil, "", provider.Fields["SiteSeries"], vprotest.MetadataText("BG"), vprotest.MetadataText("xh1"))
		if err != nil || !ref.Available || len(ref.Choices) != 3 || !ref.Choices[0].Selectable ||
			ref.Choices[1].Selectable || ref.Choices[2].Selectable || len(ref.Definitions.Columns) != 26 ||
			probe.seriesZone == nil || *probe.seriesZone != "BG" || probe.seriesSubZone == nil || *probe.seriesSubZone != "xh1" {
			t.Fatal("SiteSeries filtering/definition boundary changed", ref, err)
		}
		want := []vprocore.ProjectMetadataCell{vprotest.MetadataText("source"), vprotest.MetadataText("code"), vprotest.MetadataText("name"),
			vprotest.MetadataText("region"), vprotest.MetadataText("BG"), vprotest.MetadataText("xh1"), vprotest.MetadataText("xh"), vprotest.MetadataText("1"),
			vprotest.MetadataText("phase"), vprotest.MetadataText("01"), vprotest.MetadataText("ssphase"), vprotest.MetadataText("variation"),
			vprotest.MetadataText("seral"), vprotest.MetadataText("description"), vprotest.MetadataText("plants"), vprotest.MetadataText(""),
			{Storage: "real", Real: &referenceID}, vprotest.MetadataText("added"), vprotest.MetadataText("expired"),
			vprotest.MetadataText("original"), vprotest.MetadataText("merged"), vprotest.MetadataText("suballiance"), vprotest.MetadataText("alliance"),
			vprotest.MetadataText("missing"), vprotest.MetadataText("transfer"), vprotest.MetadataInteger("-1")}
		if !reflect.DeepEqual(ref.Definitions.Rows[0].Cells, want) ||
			ref.Definitions.Columns[16] != (vprocore.ProjectMetadataColumn{Name: "ReferenceID", DeclaredType: "REAL"}) ||
			ref.Definitions.Columns[25] != (vprocore.ProjectMetadataColumn{Name: "Flag", DeclaredType: "BOOLEAN"}) {
			t.Fatal("complete borrowed SiteSeries metadata lost identity/order/types/Access BOOLEAN", ref.Definitions)
		}
		probe.series = nil
		ref, err = provider.Read(context.Background(), nil, "", provider.Fields["SiteSeries"], vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.MetadataText("xh1"))
		if err != nil || !ref.Available || len(ref.Choices) != 0 || probe.seriesZone != nil {
			t.Fatal("NULL Zone inherited stale SiteSeries choices", ref, err)
		}
		probe.err = errors.New("independent catalogue lookup failed")
		if _, err := provider.Read(context.Background(), nil, "", provider.Fields["SubZone"], vprotest.MetadataText("BG"), vprotest.MetadataText("xh1")); !errors.Is(err, probe.err) {
			t.Fatal("lookup error converted to acknowledged availability", err)
		}
		probe.err = nil
		if ref, err := provider.Read(context.Background(), nil, "", provider.Fields["SubZone"], vprotest.MetadataText("BG"), vprotest.MetadataText("xh1")); err != nil || !ref.Available {
			t.Fatal("retry did not recover", ref, err)
		}
	}
}

func TestTwoPageEntryReferenceProviderMissingClosedAndCancelledReaders(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	for _, name := range []string{"Ecosection", "SitePlotQuality", "SoilClassGroup", "SiteSeries", "SubZone"} {
		field := provider.Fields[name]
		empty, err := twopage.NewTwoPageEntryReferenceProvider("FS882-8x6XL", twopage.TwoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		if ref, err := empty.Read(context.Background(), nil, "", field, vprotest.MetadataText("BG"), vprotest.MetadataText("xh1")); err == nil || ref.Available {
			t.Fatal("missing borrowed reader returned success", name, ref, err)
		}
	}
	quality := provider.Readers.Quality.(*references.QualityService)
	if err := quality.Close(); err != nil {
		t.Fatal(err)
	}
	if ref, err := provider.Read(context.Background(), nil, "", provider.Fields["SitePlotQuality"], vprotest.MetadataText("BG"), vprotest.MetadataText("xh1")); err == nil || ref.Available {
		t.Fatal("closed catalogue became acknowledged unavailable", ref, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, field := range provider.Fields {
		if _, err := provider.Read(cancelled, nil, "", field, vprotest.MetadataText("BG"), vprotest.MetadataText("xh1")); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation disappeared", field.Column, err)
		}
	}
	if _, err := provider.Read(context.Background(), nil, "", provider.Fields["PlotType"], vprotest.MetadataText("BG"), vprotest.MetadataText("xh1")); err == nil {
		t.Fatal("family reader accepted missing owned transaction")
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	_, err := vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
		return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(_ *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
			ref, err := provider.Read(context.Background(), tx, "VLists", provider.Fields["LocationAccuracy"], vprotest.MetadataText("BG"), vprotest.MetadataText("xh1"))
			if err != nil {
				return false, err
			}
			for _, choice := range ref.Choices {
				if choice.Selectable {
					if _, err := twopage.TwoPageEntryReferenceCode(provider.Fields["LocationAccuracy"],
						vprocore.ProjectMetadataCell{Storage: "integer", Integer: choice.Code}); err != nil {
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
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestTwoPageEntryReferenceProviderPhysicalExclusionsAndNumericChoices(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(contexts.Projects.Sqlite.Attachments["VLists"], "rw"))
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
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	_, err = vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
		return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(_ *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
			ref, err := provider.Read(context.Background(), tx, "VLists", provider.Fields["SurfaceTopographyType"],
				vprocore.ProjectMetadataCell{Storage: "null"}, vprocore.ProjectMetadataCell{Storage: "null"})
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
			ref, err = provider.Read(context.Background(), tx, "VLists", provider.Fields["LocationAccuracy"],
				vprocore.ProjectMetadataCell{Storage: "null"}, vprocore.ProjectMetadataCell{Storage: "null"})
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
	vprotest.AssertProfileSUFiles(t, contexts, before)
}
