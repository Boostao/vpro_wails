package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func vegetationWorkbookServiceFixture(t *testing.T, external bool) (*vegetationWorkbookService, ProjectState) {
	t.Helper()
	contexts, state := reportServiceFixture(t, external)
	service, err := newVegetationWorkbookService(contexts, func(name string) (string, bool) {
		if name != vegetationWorkbookFeatureEnvironment {
			t.Fatal("vegetation workbook borrowed another workflow gate", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 10, 6, 23, 59, 59, 0, time.Local) }
	return service, state
}

func TestLongVegetationWorkbookPublicationStrictSourceLayoutFlags(t *testing.T) {
	values := longVegetationOptionsFixture(t)
	layout, err := decodeVegetationWorkbookLayout(values)
	if err != nil || !layout.QuickReport || !layout.ReportSummary || layout.SpaceBetweenGroups {
		t.Fatal("source publication defaults changed", layout, err)
	}
	fields := values["ReportOptions"].(map[string]any)
	for _, key := range []string{"LVQuickReport", "LVSpaceBetweenGroups", "LVReportSummary", "LVUseSppCodesOnly"} {
		original := fields[key]
		for _, value := range []any{nil, "false", 1, float64(0)} {
			fields[key] = value
			actual, err := decodeVegetationWorkbookLayout(values)
			if err == nil || !reflect.DeepEqual(actual, vegetationWorkbookLayout{}) {
				t.Fatal("malformed publication flag silently defaulted", key, value, actual, err)
			}
		}
		fields[key] = original
	}
}

func TestLongVegetationWorkbookPublicationOwnedDateBytesCollisionAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := vegetationWorkbookServiceFixture(t, external)
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
		if err != nil || review == nil || review.Preview.ContextID != state.ContextID ||
			review.Preview.ProjectPath != state.ProjectPath || review.Preview.SUPath != state.SUPath ||
			!review.Layout.ReportSummary || review.Layout.Summary == nil ||
			review.CreatedDate != "2026-10-06" || review.Layout.Summary.CreatedDate != review.CreatedDate {
			t.Fatal("owned review lost summary/default/date/scope", review, err)
		}
		service.now = func() time.Time { return time.Date(2026, 10, 7, 0, 0, 1, 0, time.Local) }
		destination := filepath.Join(t.TempDir(), "vegetation.xlsx")
		request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
		outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request)
		if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
			t.Fatal("reviewed date changed at midnight or publication failed", outcome, err)
		}
		data, err := os.ReadFile(destination)
		if err != nil || len(data) != review.Bytes {
			t.Fatal("publication length differs", len(data), err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 {
			t.Fatal("publication differs from reviewed bytes")
		}
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err != nil ||
			outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
			t.Fatal("collision replaced/replayed artifact", outcome, err)
		}
		after, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(data, after) {
			t.Fatal("collision changed existing workbook", err)
		}
		assertProfileSUFiles(t, service.contexts, before)
		afterConfig, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, afterConfig) {
			t.Fatal("publication changed source preferences", err)
		}
	}
}

func TestLongVegetationWorkbookPublicationStrictGateScopeAndRetainedPreferences(t *testing.T) {
	for _, gate := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := newVegetationWorkbookService(nil, func(string) (string, bool) { return gate, gate != "" })
		if gate == "" || gate == "false" || gate == "true" {
			if err != nil || service.enabled != (gate == "true") {
				t.Fatal("literal workbook gate changed", gate, err)
			}
			if _, err := service.GetReview(context.Background(), "owned", `{"scope":"unlumped"}`); err == nil {
				t.Fatal("unavailable gate/context produced review")
			}
		} else if err == nil {
			t.Fatal("nonliteral workbook gate accepted", gate)
		}
	}
	service, state := vegetationWorkbookServiceFixture(t, false)
	for _, raw := range []string{`{}`, `{"scope":null}`, `{"scope":"unlumped","extra":true}`,
		`{"scope":"\ud800"}`, `{"scope":"current"}`, `{"scope":"None_Lump"}`} {
		if review, err := service.GetReview(context.Background(), state.ContextID, raw); err == nil || review != nil {
			t.Fatal("malformed/implicit lump scope accepted", raw, err)
		}
	}
	review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []vegetationWorkbookExportRequest{
		{"unlumped", review.CreatedDate, "BAD", filepath.Join(t.TempDir(), "report.xlsx")},
		{"unlumped", review.CreatedDate, review.ApprovalHash, filepath.Join(t.TempDir(), "report.db")},
		{"unlumped", "2026-10-07", review.ApprovalHash, filepath.Join(t.TempDir(), "report.xlsx")},
	} {
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, workbookRequest(t, request)); err == nil || outcome != nil {
			t.Fatal("invalid hash/date/destination produced outcome", outcome, err)
		}
	}
	for _, raw := range []string{`{"scope":"unlumped","createdDate":"\udfff","approvalHash":"","destination":""}`, `{}`} {
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, raw); err == nil || outcome != nil {
			t.Fatal("raw invalid publication transport repaired", outcome, err)
		}
	}
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUseSppCodesOnly": -1}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`); err == nil {
		t.Fatal("combined variants preference silently ignored")
	}
	after, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("unavailable combined preference was reset", err)
	}
}

func TestLongVegetationWorkbookPublicationSnapshotFailuresCancellationAndRetry(t *testing.T) {
	for _, phase := range []string{"first-commit", "first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel"} {
		t.Run(phase, func(t *testing.T) {
			service, state := vegetationWorkbookServiceFixture(t, false)
			review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			destination := filepath.Join(t.TempDir(), "vegetation.xlsx")
			request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rejected, reads := errors.New("owned vegetation workbook snapshot refused"), 0
			hooks := vegetationWorkbookHooks{
				snapshot: publicationReadSnapshotHooks{
					commitRead: func(*sql.Tx) error {
						if phase == "first-commit" {
							return rejected
						}
						return nil
					},
					rollbackRead: func(*sql.Tx) error {
						reads++
						if phase == "first-cleanup" && reads == 1 || phase == "prelink-cleanup" && reads == 2 ||
							phase == "published-cleanup" && reads == 3 {
							return rejected
						}
						return nil
					},
				},
				publication: artifactPublicationHooks{observe: func(current, _ string) error {
					if phase == "prelink-cancel" && current == "prelink" || phase == "published-cancel" && current == "published" {
						cancel()
					}
					return nil
				}},
			}
			outcome, err := service.exportReviewed(ctx, state.ContextID, request, hooks)
			switch phase {
			case "first-commit", "first-cleanup":
				if !errors.Is(err, rejected) || outcome != nil {
					t.Fatal("initial snapshot failure invented receipt", outcome, err)
				}
			case "prelink-cleanup", "prelink-cancel":
				if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
					t.Fatal("prelink failure lost known no-file outcome", outcome, err)
				}
			default:
				if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
					outcome.SHA256 != review.WorkbookSHA256 || !strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("irreversible failure lost published identity", outcome, err)
				}
			}
			if strings.HasPrefix(phase, "published-") {
				if data, err := os.ReadFile(destination); err != nil || len(data) != review.Bytes {
					t.Fatal("published artifact missing", err)
				}
			} else {
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed publication left artifact", err)
				}
				retry, err := service.ExportReviewed(context.Background(), state.ContextID, request)
				if err != nil || retry == nil || retry.Status != "published" || retry.SHA256 != review.WorkbookSHA256 {
					t.Fatal("rejected read leaked snapshot/lease; explicit retry failed", retry, err)
				}
			}
			assertProfileSUFiles(t, service.contexts, before)
		})
	}
}

func TestLongVegetationWorkbookPublicationRawAndConfigRechecks(t *testing.T) {
	for _, scope := range []string{"env-prelink", "env-published", "config-prelink", "config-published", "config-format"} {
		t.Run(scope, func(t *testing.T) {
			service, state := vegetationWorkbookServiceFixture(t, false)
			review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "vegetation.xlsx")
			request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
			phase := "prelink"
			if strings.HasSuffix(scope, "published") {
				phase = "published"
			}
			hooks := vegetationWorkbookHooks{publication: artifactPublicationHooks{observe: func(current, _ string) error {
				if current != phase {
					return nil
				}
				if strings.HasPrefix(scope, "env-") {
					db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
					if err != nil {
						return err
					}
					_, err = db.Exec(`UPDATE Sample_Env SET SiteNotes='unreported physical drift' WHERE PlotNumber='00337'`)
					return errors.Join(err, db.Close())
				}
				if scope == "config-format" {
					path := service.contexts.projects.preferences.path
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					return os.WriteFile(path, append([]byte("# formatting-only change\n"), data...), 0600)
				}
				return service.contexts.projects.preferences.update("ReportOptions", map[string]any{"LEReportTitle": "unrelated parsed preference drift"})
			}}}
			outcome, err := service.exportReviewed(context.Background(), state.ContextID, request, hooks)
			if scope == "config-format" {
				if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
					t.Fatal("canonical YAML approval unexpectedly depends on comments", outcome, err)
				}
			} else if phase == "prelink" {
				if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
					t.Fatal("raw/config drift published artifact", outcome, err)
				}
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("prelink drift left artifact", err)
				}
			} else if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
				!strings.Contains(outcome.ErrorMessage, "do not replay") {
				t.Fatal("postlink drift lost known publication", outcome, err)
			}
			preview, err := service.contexts.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || !reflect.DeepEqual(preview, review.Preview) {
				t.Fatal("drift test did not isolate otherwise-unreported authority", err)
			}
		})
	}
}

func TestLongVegetationWorkbookPublicationCancelledLeaseAndStaleOwner(t *testing.T) {
	service, state := vegetationWorkbookServiceFixture(t, false)
	owner := service.contexts.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	review, err := service.GetReview(ctx, state.ContextID, `{"scope":"unlumped"}`)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || review != nil {
		t.Fatal("cancelled mutex lease produced review", review, err)
	}

	review, err = service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
	if err != nil {
		t.Fatal("cancellation leaked lease", err)
	}
	request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, filepath.Join(t.TempDir(), "vegetation.xlsx")})
	if _, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state)); err != nil {
		t.Fatal(err)
	}
	if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err == nil || outcome != nil {
		t.Fatal("stale owner published workbook", outcome, err)
	}
	if _, err := service.GetReview(nil, state.ContextID, `{"scope":"unlumped"}`); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestLongVegetationWorkbookPublicationQualitySummaryPreservesFilteredFanout(t *testing.T) {
	service, state := vegetationWorkbookServiceFixture(t, false)
	owner := service.contexts.projects.sqlite
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) SELECT '108050x'
			WHERE NOT EXISTS(SELECT 1 FROM Sample_Env WHERE PlotNumber='108050x');
			INSERT INTO Sample_Admin(Plot) SELECT '108050x'
			WHERE NOT EXISTS(SELECT 1 FROM Sample_Admin WHERE Plot='108050x');
			UPDATE Sample_Admin SET SitePlotQuality='Good',VegPlotQuality='Good',SoilPlotQuality='Good' WHERE Plot='108050';
			UPDATE Sample_Admin SET SitePlotQuality='Fair',VegPlotQuality='Fair',SoilPlotQuality='Fair' WHERE Plot='108050x'`); err != nil {
		t.Fatal(err)
	}
	lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer lists.Close()
	if _, err := lists.Exec(`INSERT INTO USysTableOfLists(Item,ListName,ItemOrder) VALUES('Good','DataQuality',3)`); err != nil {
		t.Fatal(err)
	}
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": -1}); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, owner.attachments)
	review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
	if err != nil || review == nil || review.Preview.Report.Quality == nil || review.Layout.Summary == nil ||
		len(review.Preview.Report.Quality.Occurrences) != 9 || len(review.Layout.Summary.Memberships) != 9 ||
		review.Layout.Summary.SelectedPlotRows != 9 {
		t.Fatal("summary counted raw/distinct SU instead of filtered temporary-SU occurrence weights", review, err)
	}
	members := review.Layout.Summary.Memberships
	counts := map[string]int{}
	for _, member := range members {
		counts[member.RowID]++
		if member.Quality == nil || member.Quality.MembershipID != member.RowID ||
			!reflect.DeepEqual(member.Quality.PlotNumber, member.PlotNumber) {
			t.Fatal("filtered summary invented physical IDs or lost join evidence", member)
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"1": 8, "3": 1}) {
		t.Fatal("summary collapsed original filtered fanout", counts)
	}
	destination := filepath.Join(t.TempDir(), "quality.xlsx")
	request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
	if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err != nil ||
		outcome == nil || outcome.Status != "published" {
		t.Fatal("filtered summary publication failed", outcome, err)
	}
	book, err := excelize.OpenFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if count, err := book.GetCellValue("ReportSummary", "B12"); err != nil || count != "9" {
		t.Fatal("summary worksheet has wrong filtered count", count, err)
	}
	for address, expected := range map[string]string{"D13": "108050x", "E13": ""} {
		value, err := book.GetCellValue("ReportSummary", address)
		if err != nil || value != expected {
			t.Fatal("summary filtered membership/NULL changed", address, value, err)
		}
	}
	assertProfileSUFiles(t, service.contexts, before)
}
