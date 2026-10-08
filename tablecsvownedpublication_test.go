package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func ownedTableCSVPublicationFixture(t *testing.T) (*ContextService, ProjectState, ownedTableCSVReview) {
	t.Helper()
	service, state := contextServiceFixture(t)
	tableCSVProjectWriter(t, service, `ALTER TABLE _table_metadata RENAME TO _owned_publication_metadata_seed;
		CREATE TABLE _table_metadata(table_name TEXT,description TEXT);
		INSERT INTO _table_metadata SELECT table_name,description FROM _owned_publication_metadata_seed WHERE table_name<>'Sample_Other';
		DROP TABLE _owned_publication_metadata_seed;
		INSERT INTO _table_metadata VALUES ('Sample_Other',NULL),('Sample_Other',''),('Sample_Other','  literal  '),('Sample_Other','  literal  ');`)
	review, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
	if err != nil {
		t.Fatal(err)
	}
	return service, state, review
}

func observeOwnedTableCSVPublication(t *testing.T, service *ContextService, review ownedTableCSVReview, result tableCSVPublication) {
	t.Helper()
	data := tableCSVPublicationRead(t, result.Path)
	digest := sha256.Sum256(data)
	if !result.Published || result.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("published state or actual archive hash differs:", result)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) != 2 {
		t.Fatal("actual ZIP members:", err)
	}
	members := map[string][]byte{}
	for _, member := range archive.File {
		reader, err := member.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal(readErr, closeErr)
		}
		members[member.Name] = content
	}
	var manifest TableCSVManifest
	if err := json.Unmarshal(members["manifest.json"], &manifest); err != nil || !reflect.DeepEqual(manifest, review.Document.Manifest) {
		t.Fatal("independently observed manifest differs:", err)
	}
	records, err := csv.NewReader(bytes.NewReader(members["table.csv"])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.selection.ProjectPath, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	physical, err := readSQLiteStorageRows(context.Background(), db, "main", review.Document.Manifest.Table, "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{make([]string, len(physical.Columns))}
	for index, column := range physical.Columns {
		want[0][index] = column.Name
	}
	for _, row := range physical.Rows {
		record := make([]string, len(row.Cells))
		for index, cell := range row.Cells {
			switch cell.Storage {
			case "null":
				record[index] = ""
			case "text":
				value, err := json.Marshal(*cell.Text)
				if err != nil {
					t.Fatal(err)
				}
				record[index] = string(value)
			case "integer":
				record[index] = *cell.Integer
			case "real":
				record[index] = strconv.FormatFloat(*cell.Real, 'g', -1, 64)
			case "blob":
				record[index] = *cell.BlobHex
			default:
				t.Fatal("unexpected physical storage", cell.Storage)
			}
		}
		want = append(want, record)
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatal("actual CSV differs from independent physical source observation")
	}
}

func TestTableCSVOwnedPublicationAllEightPhysicalTablesReadOnly(t *testing.T) {
	service, state, _ := ownedTableCSVPublicationFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config := tableCSVPublicationRead(t, service.projects.preferences.path)
	directory := t.TempDir()
	for _, suffix := range []string{"Admin", "Audit", "Env", "Humus", "Metadata", "Mineral", "Other", "Veg"} {
		t.Run(suffix, func(t *testing.T) {
			review, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_"+suffix)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, suffix+".private")
			result, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, review, path)
			if err != nil {
				t.Fatal(err)
			}
			observeOwnedTableCSVPublication(t, service, review, result)
		})
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) ||
		!bytes.Equal(config, tableCSVPublicationRead(t, service.projects.preferences.path)) {
		t.Fatal("publication changed business databases or preferences")
	}
	for role, data := range before {
		t.Logf("table-csv-owned-publication-source %s sha256=%x unchanged=true", role, sha256.Sum256(data))
	}
}

func TestTableCSVOwnedPublicationDetachesEntireExpectedReview(t *testing.T) {
	service, state, review := ownedTableCSVPublicationFixture(t)
	original := review
	original.Document = snapshotTableCSVBundleDocument(review.Document)
	result, err := service.publishOwnedProjectTableCSVWithHooks(context.Background(), state.ContextID, review,
		filepath.Join(t.TempDir(), "detached"), tableCSVOwnedPublicationHooks{observe: func(phase string) error {
			if phase == "snapshot" {
				review.ContextID, review.Project, review.ProjectPath = "mutated", "mutated", "mutated"
				review.DescriptionMetadataPresent = false
				review.Document.Data[0] = 'X'
				review.Document.Manifest.Columns[0].Name = "mutated"
				review.Document.Manifest.RowIDs[0] = "999999"
				review.Document.Manifest.Storage[0][0] = "blob"
				*review.Document.Manifest.Descriptions[2].Value.Text = "mutated"
			}
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	observeOwnedTableCSVPublication(t, service, original, result)
}

func TestTableCSVOwnedPublicationRejectsExpectedDriftScopeAndStaleContext(t *testing.T) {
	service, state, review := ownedTableCSVPublicationFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, test := range []struct {
		name string
		edit func(*ownedTableCSVReview)
	}{
		{"context", func(r *ownedTableCSVReview) { r.ContextID = "other" }},
		{"project", func(r *ownedTableCSVReview) { r.Project = "Other" }},
		{"path", func(r *ownedTableCSVReview) { r.ProjectPath += ".other" }},
		{"schema", func(r *ownedTableCSVReview) { r.Document.Manifest.Columns[0].DeclaredType = "OTHER" }},
		{"value", func(r *ownedTableCSVReview) { r.Document.Data[0] = 'X' }},
		{"tag", func(r *ownedTableCSVReview) { r.Document.Manifest.Storage[0][0] = "blob" }},
		{"description", func(r *ownedTableCSVReview) { *r.Document.Manifest.Descriptions[2].Value.Text = "other" }},
		{"presence", func(r *ownedTableCSVReview) { r.DescriptionMetadataPresent = false }},
		{"description identity", func(r *ownedTableCSVReview) { r.Document.Manifest.Descriptions[2].RowID = "999" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			expected := review
			expected.Document = snapshotTableCSVBundleDocument(review.Document)
			test.edit(&expected)
			directory := t.TempDir()
			result, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, expected, filepath.Join(directory, "rejected"))
			if err == nil || result.Published {
				t.Fatal("expected drift accepted:", result, err)
			}
			tableCSVPublicationOnly(t, directory)
		})
	}
	for _, table := range []string{"", "Unknown", "sample_Other", "Sample_other", "OtherProject_Other",
		"_table_metadata", "USysEnv", "Sample_Profile", "VLists.USysAllSpecs", "Sample_Other;DELETE FROM Sample_Env"} {
		expected := review
		expected.Document.Manifest.Table = table
		directory := t.TempDir()
		result, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, expected, filepath.Join(directory, "rejected"))
		if err == nil || result.Published {
			t.Fatal("unowned/nonliteral scope accepted:", table, err)
		}
		tableCSVPublicationOnly(t, directory)
	}
	directory := t.TempDir()
	if result, err := service.publishOwnedProjectTableCSV(context.Background(), "stale", review, filepath.Join(directory, "stale")); err == nil || result.Published {
		t.Fatal("stale context accepted:", result, err)
	}
	tableCSVPublicationOnly(t, directory)
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("rejected expectations changed source")
	}
}

func TestTableCSVOwnedPublicationFreshPrelinkSnapshotRejectsSourceDrift(t *testing.T) {
	for _, test := range []struct{ name, sql string }{
		{"schema", `ALTER TABLE Sample_Other ADD COLUMN Extra TEXT`},
		{"rows", `DELETE FROM Sample_Other`},
		{"value", `UPDATE Sample_Other SET PlotNumber='different' WHERE rowid=(SELECT min(rowid) FROM Sample_Other)`},
		{"tagged value", `UPDATE Sample_Other SET PlotNumber=CAST('tagged' AS BLOB) WHERE rowid=(SELECT min(rowid) FROM Sample_Other)`},
		{"description", `UPDATE _table_metadata SET description='different' WHERE table_name='Sample_Other'`},
		{"description candidate", `INSERT INTO _table_metadata VALUES ('Sample_Other','  literal  ')`},
		{"view", `ALTER TABLE Sample_Other RENAME TO Archived_Other; CREATE VIEW Sample_Other AS SELECT * FROM Archived_Other`},
		{"absent metadata", `DROP TABLE _table_metadata`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, state, review := ownedTableCSVPublicationFixture(t)
			directory := t.TempDir()
			commits := 0
			result, err := service.publishOwnedProjectTableCSVWithHooks(context.Background(), state.ContextID, review,
				filepath.Join(directory, "rejected"), tableCSVOwnedPublicationHooks{
					commitRead: func(*sql.Tx) error { commits++; return nil },
					observe: func(phase string) error {
						if phase == "precommit" {
							// Separate connection after the first read transaction closed.
							tableCSVProjectWriter(t, service, test.sql)
						}
						return nil
					},
				})
			wantCommits := 2
			if test.name == "view" {
				wantCommits = 1
			}
			if err == nil || result.Published || commits != wantCommits {
				t.Fatal("fresh transaction failed to observe source drift:", result, err, commits)
			}
			tableCSVPublicationOnly(t, directory)
			fresh, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
			if test.name == "view" {
				if err == nil {
					t.Fatal("view became a physical source")
				}
				return
			}
			if err != nil {
				t.Fatal("rejected drift poisoned next read:", err)
			}
			if result, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, fresh, filepath.Join(directory, "retry")); err != nil || !result.Published {
				t.Fatal("fresh review retry failed:", result, err)
			}
		})
	}
}

func TestTableCSVOwnedPublicationAbsentVersusPresentEmptyMetadata(t *testing.T) {
	for _, startingAbsent := range []bool{false, true} {
		t.Run(fmt.Sprint(startingAbsent), func(t *testing.T) {
			service, state, _ := ownedTableCSVPublicationFixture(t)
			tableCSVProjectWriter(t, service, `DELETE FROM _table_metadata`)
			if startingAbsent {
				tableCSVProjectWriter(t, service, `DROP TABLE _table_metadata`)
			}
			review, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			result, err := service.publishOwnedProjectTableCSVWithHooks(context.Background(), state.ContextID, review, filepath.Join(directory, "rejected"),
				tableCSVOwnedPublicationHooks{observe: func(phase string) error {
					if phase == "precommit" {
						if startingAbsent {
							tableCSVProjectWriter(t, service, `CREATE TABLE _table_metadata(table_name TEXT,description TEXT)`)
						} else {
							tableCSVProjectWriter(t, service, `DROP TABLE _table_metadata`)
						}
					}
					return nil
				}})
			if err == nil || result.Published {
				t.Fatal("metadata presence drift accepted:", result, err)
			}
			tableCSVPublicationOnly(t, directory)
		})
	}
}

func TestTableCSVOwnedPublicationReadCleanupRollbackCancellationAndRetry(t *testing.T) {
	service, state, review := ownedTableCSVPublicationFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	fault := errors.New("read cleanup fault")
	for _, phase := range []string{"initial cancellation", "validated", "staged", "precommit", "commit", "rollback", "prelink cleanup"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			hooks := tableCSVOwnedPublicationHooks{}
			calls := 0
			if phase == "initial cancellation" {
				cancel()
			}
			hooks.observe = func(current string) error {
				if current == phase {
					cancel()
				}
				return nil
			}
			if phase == "staged" {
				hooks.publication.observe = func(current, _ string) error {
					if current == "staged" {
						cancel()
					}
					return nil
				}
			}
			if phase == "commit" {
				hooks.commitRead = func(*sql.Tx) error { return fault }
			}
			if phase == "rollback" || phase == "prelink cleanup" {
				hooks.rollbackRead = func(*sql.Tx) error {
					calls++
					if phase == "rollback" || calls == 2 {
						return fault
					}
					return nil
				}
			}
			directory := t.TempDir()
			result, err := service.publishOwnedProjectTableCSVWithHooks(ctx, state.ContextID, review, filepath.Join(directory, "rejected"), hooks)
			if err == nil || result.Published {
				t.Fatal("failed source cleanup/cancellation published:", result, err)
			}
			if strings.Contains(phase, "cancellation") || phase == "validated" || phase == "staged" || phase == "precommit" {
				if !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation identity lost:", err)
				}
			} else if !errors.Is(err, fault) {
				t.Fatal("read cleanup error identity lost:", err)
			}
			tableCSVPublicationOnly(t, directory)
			if result, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, review, filepath.Join(directory, "retry")); err != nil || !result.Published {
				t.Fatal("read cleanup/rollback damaged pinned owner:", result, err)
			}
		})
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("read cleanup/cancellation changed source")
	}
}

func TestTableCSVOwnedPublicationPostcommitSourceDriftRetainsArtifact(t *testing.T) {
	service, state, review := ownedTableCSVPublicationFixture(t)
	result, err := service.publishOwnedProjectTableCSVWithHooks(context.Background(), state.ContextID, review, filepath.Join(t.TempDir(), "artifact"),
		tableCSVOwnedPublicationHooks{observe: func(phase string) error {
			if phase == "published" {
				tableCSVProjectWriter(t, service, `UPDATE _table_metadata SET description='postcommit drift' WHERE table_name='Sample_Other'`)
			}
			return nil
		}})
	if err == nil || !result.Published || !strings.Contains(err.Error(), "do not replay") || result.Path == "" || result.SHA256 == "" {
		t.Fatal("postcommit source error lost artifact result:", result, err)
	}
	want, encodeErr := encodeTableCSVBundle(context.Background(), review.Document)
	if encodeErr != nil || !bytes.Equal(tableCSVPublicationRead(t, result.Path), want) {
		t.Fatal("postcommit source drift changed or deleted committed artifact:", encodeErr)
	}
}

func TestTableCSVOwnedPublicationSerializesOwnerMutexWriters(t *testing.T) {
	service, state, review := ownedTableCSVPublicationFixture(t)
	owner := service.projects.sqlite
	started := make(chan struct{})
	done := make(chan error, 1)
	hooks := tableCSVOwnedPublicationHooks{observe: func(phase string) error {
		if phase == "validated" {
			go func() {
				close(started)
				done <- owner.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
					var count int
					return conn.QueryRowContext(context.Background(), "SELECT count(*) FROM Sample_Other").Scan(&count)
				})
			}()
			<-started
		}
		if phase == "precommit" || phase == "published" {
			select {
			case err := <-done:
				return fmt.Errorf("owner writer escaped publication mutex: %v", err)
			default:
			}
		}
		return nil
	}}
	result, err := service.publishOwnedProjectTableCSVWithHooks(context.Background(), state.ContextID, review, filepath.Join(t.TempDir(), "artifact"), hooks)
	if err != nil || !result.Published {
		t.Fatal("serialized publication:", result, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("owner writer did not recover after publication:", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("owner writer remained blocked after publication")
	}
}

func TestTableCSVOwnedPublicationOwnershipReplacementAndPublishedErrors(t *testing.T) {
	for _, phase := range []string{"snapshot", "precommit", "published", "postcommit cleanup", "publisher cleanup"} {
		t.Run(phase, func(t *testing.T) {
			service, state, review := ownedTableCSVPublicationFixture(t)
			owner := service.projects.sqlite
			original := owner.attachments["VLists"]
			defer func() { owner.attachments["VLists"] = original }()
			fault := errors.New("postcommit cleanup fault")
			reads := 0
			hooks := tableCSVOwnedPublicationHooks{observe: func(current string) error {
				if current == phase {
					owner.attachments["VLists"] = owner.attachments["project"]
				}
				return nil
			}}
			if phase == "postcommit cleanup" {
				hooks.rollbackRead = func(*sql.Tx) error {
					reads++
					if reads == 3 {
						return fault
					}
					return nil
				}
			}
			if phase == "publisher cleanup" {
				hooks.publication.observe = func(current, _ string) error {
					if current == "cleanup" {
						return fault
					}
					return nil
				}
			}
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			result, err := service.publishOwnedProjectTableCSVWithHooks(context.Background(), state.ContextID, review, path, hooks)
			published := phase == "published" || strings.Contains(phase, "cleanup")
			if err == nil || result.Published != published {
				t.Fatal("wrong published-state error:", result, err)
			}
			owner.attachments["VLists"] = original
			if published {
				if !strings.Contains(err.Error(), "do not replay") || result.Path == "" || result.SHA256 == "" {
					t.Fatal("committed publication result was zeroed:", result, err)
				}
				observeOwnedTableCSVPublication(t, service, review, result)
				if retry, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, review, path); err == nil || retry.Published {
					t.Fatal("published error replay replaced artifact:", retry, err)
				}
			} else {
				tableCSVPublicationOnly(t, directory)
				if retry, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, review, path); err != nil || !retry.Published {
					t.Fatal("ownership rejection poisoned retry:", retry, err)
				}
			}
		})
	}
}

func TestTableCSVOwnedPublicationExistingCollisionAndQueuedContextSwitch(t *testing.T) {
	service, state, review := ownedTableCSVPublicationFixture(t)
	directory := t.TempDir()
	collision := filepath.Join(directory, "collision")
	tableCSVPublicationWrite(t, collision, []byte("existing"))
	if result, err := service.publishOwnedProjectTableCSV(context.Background(), state.ContextID, review, collision); err == nil || result.Published ||
		string(tableCSVPublicationRead(t, collision)) != "existing" {
		t.Fatal("collision replaced existing bytes:", result, err)
	}
	switchDone := make(chan error, 1)
	queued := false
	hooks := tableCSVOwnedPublicationHooks{observe: func(phase string) error {
		if phase == "validated" {
			go func() {
				_, err := service.SwitchContext(state.ContextID, contextSelection(state))
				switchDone <- err
			}()
			// RWMutex.TryRLock fails once the switch's write lease is queued.
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if !service.projects.operationMu.TryRLock() {
					queued = true
					return nil
				}
				service.projects.operationMu.RUnlock()
				time.Sleep(time.Millisecond)
			}
			return errors.New("switch did not queue")
		}
		return nil
	}}
	done := make(chan error, 1)
	go func() {
		result, err := service.publishOwnedProjectTableCSVWithHooks(context.Background(), state.ContextID, review, filepath.Join(directory, "owned"), hooks)
		if err == nil && !result.Published {
			err = errors.New("publication did not commit")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil || !queued {
			t.Fatal("publication with queued switch:", err, queued)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("publication recursively reacquired context/owner lease behind queued switch")
	}
	select {
	case err := <-switchDone:
		if err != nil {
			t.Fatal("queued switch failed:", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued switch did not finish after publication")
	}
}
