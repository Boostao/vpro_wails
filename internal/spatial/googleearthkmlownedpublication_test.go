package spatial

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func ownedKMLPublicationFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState, ownedGoogleEarthKMLPreparation) {
	t.Helper()
	service, state := vprotest.PrepareKMLContextFixture(t)
	review, err := ReadGoogleEarthKMLPublicationReview(service, context.Background(), state.ContextID, "Zone", " Literal <&\r\n ")
	if err != nil || review == nil || len(review.Source.Report.Rows) == 0 {
		t.Fatal("owned source preparation:", err)
	}
	return service, state, *review
}

func TestOwnedGoogleEarthKMLPublicationMatchesIndependentSQLAndPreservesDatabases(t *testing.T) {
	service, state, review := ownedKMLPublicationFixture(t)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config := vprotest.TableCSVPublicationRead(t, service.Projects.Preferences.Path)
	path := filepath.Join(t.TempDir(), "explicit output")
	result, err := PublishOwnedGoogleEarthKML(service, context.Background(), state.ContextID, review, path)
	if err != nil || !result.Published {
		t.Fatalf("owned publication: %#v %v", result, err)
	}
	data := vprotest.TableCSVPublicationRead(t, path)
	hash := sha256.Sum256(data)
	if result.SHA256 != hex.EncodeToString(hash[:]) || !bytes.Equal(data, review.KML.Bytes) {
		t.Fatal("actual committed bytes/hash differ")
	}
	var observed observedGoogleEarthKML
	if err := xml.Unmarshal(data, &observed); err != nil || observed.Document.Name != review.KML.Title {
		t.Fatal("literal XML title:", err)
	}
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT PlotNumber,Longitude*-1,Latitude FROM Sample_Env
WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var name string
		var longitude, latitude float64
		if err := rows.Scan(&name, &longitude, &latitude); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if count >= len(observed.Document.Marks) {
			t.Fatal("physical source row omitted")
		}
		mark := observed.Document.Marks[count]
		coordinates := strconv.FormatFloat(longitude, 'f', -1, 64) + "," + strconv.FormatFloat(latitude, 'f', -1, 64) + ",0"
		if mark.Name != name || mark.Point.Coordinates != coordinates || mark.Description != "." {
			t.Fatal("independent source SQL/XML differs", mark, coordinates)
		}
		count++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil || count != review.KML.PlacemarkCount || count != len(observed.Document.Marks) {
		t.Fatal("complete physical source count differs", count, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if !bytes.Equal(config, vprotest.TableCSVPublicationRead(t, service.Projects.Preferences.Path)) {
		t.Fatal("owned file publication changed preferences")
	}
	retry, err := PublishOwnedGoogleEarthKML(service, context.Background(), state.ContextID, review, path)
	if err == nil || retry.Published || !bytes.Equal(data, vprotest.TableCSVPublicationRead(t, path)) {
		t.Fatal("owned artifact overwritten on retry", retry, err)
	}
}

func TestOwnedGoogleEarthKMLPublicationDetachesRawPointersAndBytesBeforeCallbacks(t *testing.T) {
	service, state, review := ownedKMLPublicationFixture(t)
	want := bytes.Clone(review.KML.Bytes)
	path := filepath.Join(t.TempDir(), "output.kml")
	result, err := PublishOwnedGoogleEarthKMLWithHooks(service, context.Background(), state.ContextID, review, path,
		googleEarthKMLOwnedPublicationHooks{observe: func(phase string) error {
			if phase == "snapshot" {
				*review.Source.Report.Rows[0].PlotNumber.Text = "caller change"
				*review.Source.Report.Rows[0].StoredLongitude.Real = 190
				*review.Source.Report.Rows[0].Longitude.Real = 190
				*review.Source.Report.Rows[0].Latitude.Real = 190
				review.Source.Report.Rows[0].Description = vprotest.MetadataText("caller description")
				review.Source.Report.Rows[0].EnvRowID = "different physical row"
				review.KML.Bytes[0] = 'X'
			}
			return nil
		}})
	if err != nil || !result.Published || !bytes.Equal(vprotest.TableCSVPublicationRead(t, path), want) {
		t.Fatal("caller source aliases publisher", result, err)
	}
}

func TestOwnedGoogleEarthKMLPublicationRejectsRawNULLToEmptyEvenWhenXMLMatches(t *testing.T) {
	for _, phase := range []string{"before", "precommit", "published"} {
		t.Run(phase, func(t *testing.T) {
			service, state, review := ownedKMLPublicationFixture(t)
			change := func() { vprotest.MutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Zone=''`) }
			if phase == "before" {
				change()
			}
			path := filepath.Join(t.TempDir(), "output.kml")
			result, err := PublishOwnedGoogleEarthKMLWithHooks(service, context.Background(), state.ContextID, review, path,
				googleEarthKMLOwnedPublicationHooks{observe: func(current string) error {
					if current == phase {
						change()
					}
					return nil
				}})
			if err == nil || !strings.Contains(err.Error(), "raw source") || result.Published != (phase == "published") {
				t.Fatal("raw source drift ignored or irreversible result lost", result, err)
			}
			after, readErr := ReadGoogleEarthKMLPublicationReview(service, context.Background(), state.ContextID, "Zone", review.KML.Title)
			if readErr != nil || !bytes.Equal(after.KML.Bytes, review.KML.Bytes) ||
				reflect.DeepEqual(after.Source, review.Source) {
				t.Fatal("test did not independently prove raw-only drift with identical XML", readErr)
			}
			if result.Published {
				if !bytes.Equal(vprotest.TableCSVPublicationRead(t, path), review.KML.Bytes) || !strings.Contains(err.Error(), "do not replay publication") {
					t.Fatal("published raw-drift result ambiguous", err)
				}
			} else {
				vprotest.TableCSVPublicationOnly(t, filepath.Dir(path))
				retry, retryErr := PublishOwnedGoogleEarthKML(service, context.Background(), state.ContextID, *after, path)
				if retryErr != nil || !retry.Published {
					t.Fatal("fresh raw source review cannot retry", retry, retryErr)
				}
			}
		})
	}
}

func TestOwnedGoogleEarthKMLPublicationScopePhysicalProvenanceAndExpectedBytesReject(t *testing.T) {
	service, state, review := ownedKMLPublicationFixture(t)
	for _, fault := range []string{"context", "project", "path", "su", "su-path", "row", "membership", "bytes", "raw-storage"} {
		t.Run(fault, func(t *testing.T) {
			expected := snapshotGoogleEarthKMLPreparation(review)
			switch fault {
			case "context":
				expected.KML.ContextID = "stale"
			case "project":
				expected.KML.Project = "Other"
			case "path":
				expected.KML.ProjectPath += ".other"
			case "su":
				expected.KML.SU = "Other"
			case "su-path":
				expected.KML.SUPath = "other"
			case "row":
				expected.Source.Report.Rows[0].EnvRowID = "different"
			case "membership":
				expected.Source.Report.Rows[0].MembershipRowID = "phantom"
			case "bytes":
				expected.KML.Bytes[0] = 'X'
			case "raw-storage":
				expected.Source.Report.Rows[0].Description = vprotest.MetadataText("")
			}
			directory := t.TempDir()
			result, err := PublishOwnedGoogleEarthKML(service, context.Background(), state.ContextID, expected, filepath.Join(directory, "output.kml"))
			if err == nil || result.Published {
				t.Fatal("expected review tampering accepted", result, err)
			}
			vprotest.TableCSVPublicationOnly(t, directory)
		})
	}
	directory := t.TempDir()
	if result, err := PublishOwnedGoogleEarthKML(service, context.Background(), "stale", review, filepath.Join(directory, "output.kml")); err == nil || result.Published {
		t.Fatal("stale actual request accepted", result, err)
	}
	vprotest.TableCSVPublicationOnly(t, directory)
}

func TestOwnedGoogleEarthKMLPublicationReadCleanupCancelAndPostcommitErrors(t *testing.T) {
	for _, fault := range []string{"initial-commit", "initial-rollback", "precommit-rollback", "postcommit-rollback", "postcommit-cancel", "initial-cancel", "initial-read-cancel", "precommit-cancel"} {
		t.Run(fault, func(t *testing.T) {
			service, state, review := ownedKMLPublicationFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rejected := errors.New("owned read cleanup fault")
			hooks := googleEarthKMLOwnedPublicationHooks{}
			reads := 0
			hooks.snapshot.RollbackRead = func(*sql.Tx) error {
				reads++
				if fault == "initial-rollback" && reads == 1 ||
					fault == "precommit-rollback" && reads == 2 ||
					fault == "postcommit-rollback" && reads == 3 {
					return rejected
				}
				return nil
			}
			if fault == "initial-commit" {
				hooks.snapshot.CommitRead = func(*sql.Tx) error { return rejected }
			}
			if fault == "initial-read-cancel" || fault == "precommit-cancel" {
				commits := 0
				hooks.snapshot.CommitRead = func(*sql.Tx) error {
					commits++
					if fault == "initial-read-cancel" && commits == 1 || fault == "precommit-cancel" && commits == 2 {
						cancel()
					}
					return nil
				}
			}
			if fault == "initial-cancel" {
				cancel()
			}
			if fault == "postcommit-cancel" {
				hooks.observe = func(phase string) error {
					if phase == "published" {
						cancel()
						return ctx.Err()
					}
					return nil
				}
			}

			directory := t.TempDir()
			path := filepath.Join(directory, "output.kml")
			result, err := PublishOwnedGoogleEarthKMLWithHooks(service, ctx, state.ContextID, review, path, hooks)
			committed := strings.HasPrefix(fault, "postcommit")
			if err == nil || result.Published != committed {
				t.Fatal("read/cancel irreversible result incorrect", result, err)
			}
			if strings.HasSuffix(fault, "cancel") {
				if !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation cause lost", err)
				}
			} else if !errors.Is(err, rejected) {
				t.Fatal("cleanup cause lost", err)
			}
			if committed {
				if !bytes.Equal(vprotest.TableCSVPublicationRead(t, path), review.KML.Bytes) || !strings.Contains(err.Error(), "do not replay publication") {
					t.Fatal("committed read-cleanup result lost", result, err)
				}
			} else {
				vprotest.TableCSVPublicationOnly(t, directory)
				retry, retryErr := PublishOwnedGoogleEarthKML(service, context.Background(), state.ContextID, review, path)
				if retryErr != nil || !retry.Published {
					t.Fatal("fault cleanup did not allow retry", retry, retryErr)
				}
			}
			if _, err := ReadGoogleEarthKML(service, context.Background(), state.ContextID, "Zone", review.KML.Title); err != nil {
				t.Fatal("pinned read coordinator lost after fault", err)
			}
		})
	}
}

func TestOwnedGoogleEarthKMLPublicationQueuedOwnerCancellationReleasesContextLease(t *testing.T) {
	service, state, review := ownedKMLPublicationFixture(t)
	directory := t.TempDir()
	path := filepath.Join(directory, "output.kml")
	owner := service.Projects.Sqlite
	owner.Mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		result, err := PublishOwnedGoogleEarthKML(service, ctx, state.ContextID, review, path)
		if result.Published {
			done <- errors.New("queued request published")
			return
		}
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for service.Projects.OperationMu.TryLock() {
		service.Projects.OperationMu.Unlock()
		if time.Now().After(deadline) {
			cancel()
			owner.Mu.Unlock()
			<-done
			t.Fatal("queued request never acquired its context operation lease")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		owner.Mu.Unlock()
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued cancellation cause lost", err)
		}
	case <-time.After(5 * time.Second):
		owner.Mu.Unlock()
		t.Fatal("queued owner cancellation did not return")
	}
	vprotest.TableCSVPublicationOnly(t, directory)
	result, err := PublishOwnedGoogleEarthKML(service, context.Background(), state.ContextID, review, path)
	if err != nil || !result.Published {
		t.Fatal("queued cancellation leaked lease or owner", result, err)
	}
}

func TestOwnedGoogleEarthKMLPublicationExternalMembershipPhysicalDriftWithSameXML(t *testing.T) {
	service, state := vprotest.PrepareKMLContextFixture(t)
	source, err := ReadGoogleEarthLocations(service, context.Background(), state.ContextID, "Zone")
	if err != nil || len(source.Report.Rows) == 0 {
		t.Fatal(err)
	}
	plot := *source.Report.Rows[0].PlotNumber.Text
	suPath := filepath.Join(t.TempDir(), "Earth SU O'Brien #.db")
	writer, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(suPath, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.Exec(`CREATE TABLE Earth_SU(PlotNumber TEXT,SiteUnit TEXT);
INSERT INTO Earth_SU VALUES(?,NULL),(?,''),(?,'other')`, plot, plot, plot)
	if err := errors.Join(err, writer.Close()); err != nil {
		t.Fatal(err)
	}
	next, err := service.SwitchContext(state.ContextID, vprocore.ContextSelection{
		Project: state.ActiveProject, ProjectPath: state.ProjectPath, SU: "Earth", SUPath: suPath, Hierarchy: "None",
	})
	if err != nil {
		t.Fatal(err)
	}
	review, err := ReadGoogleEarthKMLPublicationReview(service, context.Background(), next.ContextID, "Zone", "")
	if err != nil || review.KML.PlacemarkCount != 3 {
		t.Fatal("external fanout", err)
	}
	path := filepath.Join(t.TempDir(), "fanout.kml")
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	result, err := PublishOwnedGoogleEarthKML(service, context.Background(), next.ContextID, *review, path)
	var observed observedGoogleEarthKML
	if err != nil || !result.Published || xml.Unmarshal(vprotest.TableCSVPublicationRead(t, path), &observed) != nil ||
		len(observed.Document.Marks) != 3 {
		t.Fatal("physical duplicate memberships lost", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	vprotest.MutateContextFixture(t, suPath, `UPDATE Earth_SU SET rowid=rowid+100`)
	after, err := ReadGoogleEarthKMLPublicationReview(service, context.Background(), next.ContextID, "Zone", "")
	if err != nil || !bytes.Equal(review.KML.Bytes, after.KML.Bytes) || reflect.DeepEqual(review.Source, after.Source) {
		t.Fatal("membership-only raw drift not proven", err)
	}
	directory := t.TempDir()
	rejected, err := PublishOwnedGoogleEarthKML(service, context.Background(), next.ContextID, *review, filepath.Join(directory, "old.kml"))
	if err == nil || rejected.Published || !strings.Contains(err.Error(), "raw source") {
		t.Fatal("same rendered XML approved different physical memberships", rejected, err)
	}
	vprotest.TableCSVPublicationOnly(t, directory)
}
