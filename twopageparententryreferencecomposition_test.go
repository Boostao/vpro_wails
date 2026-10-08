package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestTwoPageEntryReferenceCompositionCompleteOwnedSnapshotAndSources(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		provider, contexts, state := twoPageEntryProviderFixture(t, form)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		original, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
		if err != nil {
			t.Fatal(err)
		}
		zone, subZone := metadataText("BG"), metadataText("xh1")
		for _, projectSource := range []int{1, 2} {
			for _, workingSource := range []int{1, 2, 3} {
				if err := contexts.projects.preferences.update("Current", map[string]any{
					"ProjectIdSource": projectSource, "AssignedSuSource": workingSource,
				}); err != nil {
					t.Fatal(err)
				}
				result, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", form, zone, subZone, provider.readers)
				if err != nil || result == nil || result.ProjectSource != projectSource || result.WorkingSource != workingSource ||
					!reflect.DeepEqual(result.Original, original) || !reflect.DeepEqual(result.Zone, zone) || !reflect.DeepEqual(result.SubZone, subZone) {
					t.Fatal("complete owned reference snapshot mixed source/original/draft state", form, projectSource, workingSource, result, err)
				}
				count := 56
				if form == "FS882-8x6XL-CHARS" {
					count = 55
				}
				if len(result.Fields) != count {
					t.Fatal("complete source reference cohort truncated", len(result.Fields))
				}
				seen := map[string]bool{}
				required := 0
				for _, field := range result.Fields {
					if seen[field.Column] || !field.Available {
						t.Fatal("duplicate/unavailable source reference in configured snapshot", field.Column)
					}
					seen[field.Column] = true
					if field.Required {
						required++
					}
					if field.Column == "BECSiteUnit" && len(field.Definitions.Rows) != 3504 {
						t.Fatal("actual configured Master reference did not retain full definitions", len(field.Definitions.Rows))
					}
				}
				if required != 6 || (form == "FS882-8x6XL-CHARS" && seen["BEC_Use"]) {
					t.Fatal("complete composition inherited SIVI/foreign source flags", required, seen["BEC_Use"])
				}
				*result.Zone.Text = "caller-owned change"
				*result.SubZone.Text = "caller-owned change"
				if *zone.Text != "BG" || *subZone.Text != "xh1" {
					t.Fatal("reference result aliased caller's draft filters")
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

type twoPageEntryLateCancellationBEC struct {
	original interface {
		ListBECZones(context.Context) ([]BECZone, error)
		ListBECSubZones(context.Context, *string) ([]BECSubZone, error)
	}
	cancel context.CancelFunc
}

func (s twoPageEntryLateCancellationBEC) ListBECZones(ctx context.Context) ([]BECZone, error) {
	rows, err := s.original.ListBECZones(ctx)
	s.cancel()
	return rows, err
}

func (s twoPageEntryLateCancellationBEC) ListBECSubZones(ctx context.Context, zone *string) ([]BECSubZone, error) {
	return s.original.ListBECSubZones(ctx, zone)
}

func TestTwoPageEntryReferenceCompositionFailuresNoPartialResultAndRetry(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, failure := range []string{"missing", "late cancellation", "foreign form", "bad zone", "closed"} {
		readers := provider.readers
		ctx := context.Background()
		form := "FS882-8x6XL"
		zone := metadataText("BG")
		switch failure {
		case "missing":
			readers.soil = nil
		case "late cancellation":
			cancelled, cancel := context.WithCancel(ctx)
			defer cancel()
			ctx = cancelled
			readers.shared.bec = twoPageEntryLateCancellationBEC{provider.readers.shared.bec, cancel}
		case "foreign form":
			form = "frmSIVIsite"
		case "bad zone":
			zone = metadataInteger("1")
		case "closed":
			quality, err := NewQualityService(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := quality.Close(); err != nil {
				t.Fatal(err)
			}
			readers.quality = quality
		}
		result, err := contexts.readTwoPageEntryReferences(ctx, state.ContextID, "108050", form, zone, metadataText("xh1"), readers)
		if err == nil || result != nil || (failure == "late cancellation" && !errors.Is(err, context.Canceled)) {
			t.Fatal("reference failure returned partial/success-shaped snapshot", failure, result, err)
		}
		assertProfileSUFiles(t, contexts, before)
		result, err = contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
			metadataText("BG"), metadataText("xh1"), provider.readers)
		if err != nil || result == nil || len(result.Fields) != 56 {
			t.Fatal("failed complete read leaked owner/snapshot and prevented retry", failure, result, err)
		}
	}
	_, err := withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
		return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (bool, error) {
			for _, form := range []string{"FS882-8x6XL-CHARS", "frmSIVIsite"} {
				result, err := readTwoPageEntryReferenceFields(context.Background(), tx, owner, provider, state.ContextID, form,
					twoPageEntryReferenceSelection{2, 1}, twoPageEntryReferenceAliases{"project", "VLists", "su"}, metadataText("BG"), metadataText("xh1"))
				if err == nil || result != nil {
					t.Fatal("foreign provider/form composition returned partial fields", form, result, err)
				}
			}
			return true, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	next := contextSelection(state)
	next.SU, next.SUPath = "None", ""
	state, err = contexts.SwitchContext(state.ContextID, next)
	if err != nil {
		t.Fatal(err)
	}
	if err := contexts.projects.preferences.update("Current", map[string]any{"AssignedSuSource": 3}); err != nil {
		t.Fatal(err)
	}
	result, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
		metadataText("All"), ProjectMetadataCell{Storage: "null"}, provider.readers)
	if err != nil || result == nil {
		t.Fatal("known None-SU configuration hid unrelated available references", result, err)
	}
	for _, field := range result.Fields {
		if field.Column == "UserSiteUnit" {
			if field.Available || field.Diagnostic == "" {
				t.Fatal("None-SU working reference silently became available", field)
			}
		} else if !field.Available {
			t.Fatal("None-SU affected an independent catalogue", field.Column)
		}
		if field.Column == "SubZone" && len(field.Choices) != 281 {
			t.Fatal("source All did not select complete current SubZone list", len(field.Choices))
		}
		if field.Column == "SiteSeries" && len(field.Choices) != 0 {
			t.Fatal("NULL filter reused old SiteSeries choices")
		}
	}
}
