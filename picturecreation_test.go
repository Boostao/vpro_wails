package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

const pictureCreationTestUUID = "16fe3d56-d75d-4a74-9507-000000000001"
const pictureCreationTestUUID2 = "16fe3d56-d75d-4a74-9507-000000000002"
const pictureCreationTestSchema = `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
	INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment) VALUES
	(-9223372036854775808,-2147483648,NULL,NULL,'108050',x'ff00'),
	(3,0,'','old.jpg','108050',9223372036854775807),
	(4,9,'Default','foreign.jpg','108050x','retain'),
	(5,10,'Default','numeric.jpg',108050,3.5);`

func pictureCreationFixture(t *testing.T, external bool, schema string) (*ContextService, ProjectState, *ownedPictureLibrary, pictureCreationRequest) {
	t.Helper()
	service, state := reportServiceFixture(t, external)
	source, _ := pictureLibraryFixture(t, schema)
	if err := service.plots.SetCurrentUser(" picture creation actor "); err != nil {
		t.Fatal(err)
	}
	request := pictureCreationRequest{
		ContextID: state.ContextID, Project: "Sample", PlotNumber: "108050",
		RequestID: pictureCreationTestUUID, SelectedPath: `Q:\No image grant\  Case #100% 😀.JpG`,
	}
	return service, state, source, request
}

// Independent SQL projection, never the implementation's tagged-cell reader.
func pictureCreationRawRows(t *testing.T, source *ownedPictureLibrary) [][]string {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro")+"&_query_only=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT CAST(rowid AS TEXT),
		typeof(ID),hex(CAST(ID AS BLOB)),typeof(PicDir),hex(CAST(PicDir AS BLOB)),
		typeof(PicName),hex(CAST(PicName AS BLOB)),typeof(PlotNumber),hex(CAST(PlotNumber AS BLOB)),
		typeof(PicComment),hex(CAST(PicComment AS BLOB)) FROM tblVPics ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := [][]string{}
	for rows.Next() {
		row := make([]string, 11)
		destinations := make([]any, len(row))
		for i := range row {
			destinations[i] = &row[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func pictureCreationCount(t *testing.T, source *ownedPictureLibrary, query string) int64 {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var result int64
	if err := db.QueryRow(query).Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func pictureCreationStoredHistory(t *testing.T, source *ownedPictureLibrary, id string) string {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_PictureCreationHistory" WHERE ID=?`, id).Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPictureCreationSourceCellsNullCommentAndReceipt(t *testing.T) {
	for _, external := range []bool{false, true} {
		for strength := 0; strength <= 3; strength++ {
			t.Run(fmt.Sprintf("external-%v/audit-%d", external, strength), func(t *testing.T) {
				service, state, source, request := pictureCreationFixture(t, external, pictureCreationTestSchema)
				if err := service.plots.SetAuditStrength(strength); err != nil {
					t.Fatal(err)
				}
				parentBytes := databaseBytes(t, service.projects.sqlite.attachments)
				configBytes, err := os.ReadFile(service.projects.preferences.path)
				if err != nil {
					t.Fatal(err)
				}
				before := pictureCreationRawRows(t, source)
				receipt, err := service.createPicture(t.Context(), state.ContextID, source, request)
				if err != nil {
					t.Fatal(err)
				}
				if !receipt.DidCommit || receipt.Replayed || receipt.ID != 1 || receipt.RowID != "6" || receipt.HistoryID != "1" ||
					receipt.ContextID != state.ContextID || receipt.Project != "Sample" || receipt.PlotNumber != "108050" ||
					receipt.RequestID != request.RequestID || receipt.SelectedPath != request.SelectedPath ||
					receipt.Source != source.path || receipt.RequestedSource != source.requestedPath ||
					receipt.Actor != " picture creation actor " || receipt.AuditStrength != strength || len(receipt.OwnedFiles) == 0 ||
					receipt.Committed.RowID != receipt.RowID || receipt.Committed.Cells[0].Storage != "integer" ||
					receipt.Committed.Cells[0].Integer == nil || *receipt.Committed.Cells[0].Integer != "1" ||
					receipt.Committed.Cells[1].Text == nil || *receipt.Committed.Cells[1].Text != `Q:\No image grant\` ||
					receipt.Committed.Cells[2].Text == nil || *receipt.Committed.Cells[2].Text != `  Case #100% 😀.JpG` ||
					receipt.Committed.Cells[3].Text == nil || *receipt.Committed.Cells[3].Text != "108050" ||
					!reflect.DeepEqual(receipt.Committed.Cells[4], ProjectMetadataCell{Storage: "null"}) {
					t.Fatal("literal source assignments/NULL/provenance lost", receipt)
				}
				for i, name := range []string{"ID", "PicDir", "PicName", "PlotNumber", "PicComment"} {
					if receipt.Columns[i].Name != name || receipt.Columns[i].DeclaredType != "" {
						t.Fatal("streamed order repaired", receipt.Columns)
					}
				}
				if _, err := time.Parse(time.RFC3339Nano, receipt.CreatedWhen); err != nil {
					t.Fatal(err)
				}
				after := pictureCreationRawRows(t, source)
				expected := append(append([][]string{}, before...), []string{
					"6", "integer", "31", "text", fmt.Sprintf("%X", []byte(`Q:\No image grant\`)),
					"text", fmt.Sprintf("%X", []byte(`  Case #100% 😀.JpG`)), "text", "313038303530", "null", "",
				})
				if !reflect.DeepEqual(after, expected) {
					t.Fatal("SQL raw append differs, or unrelated library rows changed", after, expected)
				}
				if pictureCreationCount(t, source, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name LIKE '%Audit%'`) != 0 ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != 1 ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationIdentity" WHERE ID=1 AND RequestID='`+request.RequestID+`'`) != 1 {
					t.Fatal("phantom source audit or missing permanent technical provenance")
				}
				var history pictureCreationHistory
				if err := json.Unmarshal([]byte(pictureCreationStoredHistory(t, source, receipt.HistoryID)), &history); err != nil ||
					history.Request != request || !reflect.DeepEqual(history.Result, receipt) {
					t.Fatal("full tagged creation provenance differs", history, err)
				}
				pictureBytes := databaseBytes(t, map[string]string{"pictures": source.path})
				replayed, err := service.createPicture(t.Context(), state.ContextID, source, request)
				receipt.Replayed = true
				if err != nil || !reflect.DeepEqual(replayed, receipt) {
					t.Fatal("exact durable retry failed", replayed, err)
				}
				reopened, err := newOwnedPictureLibrary(source.requestedPath)
				if err != nil {
					t.Fatal(err)
				}
				lookup, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, reopened, request)
				if err != nil || lookup == nil || !reflect.DeepEqual(*lookup, receipt) {
					t.Fatal("read-only reopened receipt differs", lookup, err)
				}
				if !reflect.DeepEqual(pictureBytes, databaseBytes(t, map[string]string{"pictures": source.path})) {
					t.Fatal("receipt replay/lookup wrote data")
				}
				assertProfileSUFiles(t, service, parentBytes)
				afterConfig, err := os.ReadFile(service.projects.preferences.path)
				if err != nil || !bytes.Equal(configBytes, afterConfig) {
					t.Fatal("creation changed image-authority/preferences", err)
				}
			})
		}
	}
}

func TestPictureCreationPostcommitCancellationAndSnapshotCleanupRetainReceipt(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, fault := range []string{"cancel", "read-cleanup"} {
			t.Run(fmt.Sprintf("external-%v/%s", external, fault), func(t *testing.T) {
				service, state, source, request := pictureCreationFixture(t, external, pictureCreationTestSchema)
				parentBytes := databaseBytes(t, service.projects.sqlite.attachments)
				before := pictureCreationRawRows(t, source)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				calls := 0
				hooks := pictureCreationHooks{afterLibraryCommit: func(parent *sql.Tx) error {
					calls++
					if pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != 1 {
						t.Fatal("fault observer ran before the library commit was independently visible")
					}
					if fault == "cancel" {
						cancel()
						return ctx.Err()
					}
					return parent.Rollback()
				}}
				receipt, err := service.createPictureWithHooks(ctx, state.ContextID, source, request, hooks)
				expectedCause := sql.ErrTxDone
				if fault == "cancel" {
					expectedCause = context.Canceled
				}
				var committed *pictureMetadataCommittedError
				if calls != 1 || !errors.Is(err, expectedCause) || !errors.As(err, &committed) ||
					!receipt.DidCommit || receipt.Replayed || receipt.ID != 1 || receipt.RowID != "6" ||
					receipt.HistoryID != "1" || receipt.RequestID != request.RequestID ||
					receipt.SelectedPath != request.SelectedPath || receipt.ContextID != state.ContextID ||
					receipt.Source != source.path || len(receipt.Columns) != 5 || len(receipt.Committed.Cells) != 5 {
					t.Fatal("known irreversible receipt or cleanup cause was lost", receipt, err, calls)
				}
				expected := append(append([][]string{}, before...), []string{
					"6", "integer", "31", "text", fmt.Sprintf("%X", []byte(`Q:\No image grant\`)),
					"text", fmt.Sprintf("%X", []byte(`  Case #100% 😀.JpG`)), "text", "313038303530", "null", "",
				})
				if !reflect.DeepEqual(pictureCreationRawRows(t, source), expected) {
					t.Fatal("postcommit fault changed the committed source append")
				}
				sourceBytes := databaseBytes(t, map[string]string{"pictures": source.path})
				recovered, lookupErr := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request)
				receipt.Replayed = true
				if lookupErr != nil || recovered == nil || !reflect.DeepEqual(*recovered, receipt) {
					t.Fatal("fresh read-only recovery lost full committed provenance", recovered, lookupErr)
				}
				if !reflect.DeepEqual(sourceBytes, databaseBytes(t, map[string]string{"pictures": source.path})) {
					t.Fatal("postcommit recovery replayed or changed the mutation")
				}
				assertProfileSUFiles(t, service, parentBytes)
			})
		}
	}
}

func TestPictureCreationCompletingConcurrentCreators(t *testing.T) {
	for _, independentLibrary := range []bool{false, true} {
		for _, sameRequest := range []bool{true, false} {
			t.Run(fmt.Sprintf("independent-library-%v/same-request-%v", independentLibrary, sameRequest), func(t *testing.T) {
				service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
				before := pictureCreationRawRows(t, source)
				var authority pictureCreationResult
				if independentLibrary {
					// Exercise library transactions without the context owner's serialization.
					var err error
					authority, err = withContextPlotRequest(t.Context(), service, state.ContextID, func(plots *PlotService) (pictureCreationResult, error) {
						return withOwnedSIVISnapshot(t.Context(), plots, func(owner *sqliteContext, parent *sql.Tx) (pictureCreationResult, error) {
							if err := requirePictureParent(t.Context(), owner, parent, request.PlotNumber); err != nil {
								return pictureCreationResult{}, err
							}
							return pictureCreationAuthority(t.Context(), plots, owner, state.ContextID, source, request)
						})
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				start := make(chan struct{})
				type outcome struct {
					request pictureCreationRequest
					receipt pictureCreationResult
					err     error
				}
				results := make(chan outcome, 2)
				for i := 0; i < 2; i++ {
					current := request
					if i == 1 && !sameRequest {
						current.RequestID = pictureCreationTestUUID2
					}
					go func() {
						<-start
						var receipt pictureCreationResult
						var err error
						if independentLibrary {
							planned := authority
							planned.RequestID = current.RequestID
							receipt, err = writePictureCreation(t.Context(), source, current, planned, source.checkFile)
						} else {
							receipt, err = service.createPicture(t.Context(), state.ContextID, source, current)
						}
						results <- outcome{current, receipt, err}
					}()
				}
				close(start)
				ids := map[int64]bool{}
				replays := 0
				for i := 0; i < 2; i++ {
					result := <-results
					if result.err != nil || !result.receipt.DidCommit || result.receipt.RequestID != result.request.RequestID {
						t.Fatal("completing concurrent caller lost its outcome", result)
					}
					ids[result.receipt.ID] = true
					if result.receipt.Replayed {
						replays++
					}
					recovered, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, result.request)
					result.receipt.Replayed = true
					if err != nil || recovered == nil || !reflect.DeepEqual(*recovered, result.receipt) {
						t.Fatal("concurrent result lacks its exact durable receipt", recovered, err)
					}
				}
				additions := 2
				if sameRequest {
					additions = 1
					if replays != 1 || len(ids) != 1 || !ids[1] {
						t.Fatal("same UUID did not establish one append and one exact replay", ids, replays)
					}
				} else if replays != 0 || len(ids) != 2 || !ids[1] || !ids[2] {
					t.Fatal("different UUIDs did not obtain distinct permanent allocations", ids, replays)
				}
				expected := append([][]string{}, before...)
				for i := 1; i <= additions; i++ {
					expected = append(expected, []string{
						strconv.Itoa(5 + i), "integer", fmt.Sprintf("%X", []byte(strconv.Itoa(i))),
						"text", fmt.Sprintf("%X", []byte(`Q:\No image grant\`)),
						"text", fmt.Sprintf("%X", []byte(`  Case #100% 😀.JpG`)), "text", "313038303530", "null", "",
					})
				}
				if !reflect.DeepEqual(pictureCreationRawRows(t, source), expected) ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != int64(additions) ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationIdentity"`) != int64(additions) {
					t.Fatal("concurrent completion retained extra rows or lost history/reservations")
				}
			})
		}
	}
}

func TestPictureCreationLiteralPathsAndUTF16Bounds(t *testing.T) {
	for _, test := range []struct{ path, dir, name string }{
		{`c:\Photo Dir\  literal.JpG`, `c:\Photo Dir\`, `  literal.JpG`},
		{`\\Server\Share\Dir😀\case #%.JPEG`, `\\Server\Share\Dir😀\`, `case #%.JPEG`},
		{`Z:\001`, `Z:\`, `001`},
		{`C:\` + strings.Repeat("d", 251) + `\p`, `C:\` + strings.Repeat("d", 251) + `\`, "p"},
		{`C:\` + strings.Repeat("😀", 127) + "x", `C:\`, strings.Repeat("😀", 127) + "x"},
	} {
		dir, name, err := splitPictureCreationPath(test.path)
		if err != nil || dir != test.dir || name != test.name || dir+name != test.path {
			t.Fatal("selected literal path repaired or refused", test.path, dir, name, err)
		}
	}
	for _, path := range []string{
		"", `photo.jpg`, `C:photo.jpg`, `\photo.jpg`, `C:\`, `\\server\share`, `\\server\share\`,
		`C:/photo.jpg`, `C:\folder\\p.jpg`, `C:\.\p.jpg`, `C:\folder\..\p.jpg`, `\\?\C:\p.jpg`, `\\.\C:\p.jpg`,
		`C:\p.jpg:stream`, `C:\p?.jpg`, `C:\p*.jpg`, `C:\p".jpg`, `C:\p<.jpg`, `C:\p|.jpg`,
		`C:\p.jpg `, `C:\p.jpg.`, `C:\folder \p.jpg`, `C:\NUL.jpg`, `C:\con .jpg`, `C:\COM1`, `C:\LPT³.txt`,
		"C:\\p\x00.jpg", "C:\\p\n.jpg", "C:\\p\xff.jpg",
		`C:\` + strings.Repeat("d", 252) + `\p`,
		`C:\` + strings.Repeat("😀", 128),
	} {
		if dir, name, err := splitPictureCreationPath(path); err == nil || dir != "" || name != "" {
			t.Fatalf("ambiguous/malformed/overlength path accepted: %q -> %q %q %v", path, dir, name, err)
		}
	}
	request := pictureCreationRequest{"context", "Sample", "1234567", pictureCreationTestUUID, `C:\p.jpg`}
	for _, plot := range []string{"1234567", "😀😀😀x"} {
		request.PlotNumber = plot
		if _, _, err := preparePictureCreation(request); err != nil {
			t.Fatal("seven UTF16-unit plot refused", plot, err)
		}
	}
	for _, plot := range []string{"", "12345678", "😀😀😀😀", "bad\x00", "bad\xff"} {
		request.PlotNumber = plot
		if _, _, err := preparePictureCreation(request); err == nil {
			t.Fatal("invalid parent accepted", plot)
		}
	}
}

func TestPictureCreationStrictRawJSON(t *testing.T) {
	request := pictureCreationRequest{"context", "Sample", "108050", pictureCreationTestUUID, `C:\p.jpg`}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded pictureCreationRequest
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded != request {
		t.Fatal("valid exact request refused", decoded, err)
	}
	base := string(raw)
	bad := []string{
		strings.Replace(base, `"requestId":`, `"RequestId":`, 1),
		strings.Replace(base, `"requestId":`, `"requestId":null,"old":`, 1),
		strings.Replace(base, `"project":"Sample"`, `"project":"Sample","project":"Other"`, 1),
		strings.Replace(base, `"project":"Sample"`, `"project":"Sample","\u0070roject":"Other"`, 1),
		strings.Replace(base, `"project":"Sample"`, `"project":null`, 1),
		strings.Replace(base, `"project":"Sample"`, `"project":3`, 1),
		strings.Replace(base, `"project":"Sample",`, "", 1),
		strings.Replace(base, `"context"`, `"\ud800"`, 1),
		strings.Replace(base, `"context"`, `"\udc00"`, 1),
		strings.Replace(base, `"context"`, `"con\u0000text"`, 1),
		strings.Replace(base, `"context"`, "\"con\xfftext\"", 1),
		strings.Replace(base, pictureCreationTestUUID, strings.ToUpper(pictureCreationTestUUID), 1),
		strings.Replace(base, pictureCreationTestUUID, "16fe3d56-d75d-3a74-9507-000000000001", 1),
		strings.Replace(base, pictureCreationTestUUID, "16fe3d56-d75d-4a74-7507-000000000001", 1),
		strings.Replace(base, pictureCreationTestUUID, "request-1", 1),
		base + `{}`, `[]`, `null`,
	}
	for i, raw := range bad {
		original := decoded
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil || decoded != original {
			t.Fatal("strict request accepted repair/unknown/duplicate or changed receiver", i, err)
		}
	}
}

func TestPictureCreationAllLibraryAliasesAndPermanentDeletedIdentities(t *testing.T) {
	schema := `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
		INSERT INTO tblVPics VALUES
		('01','Default','one.jpg','other',NULL),('1','Default','one-b.jpg','other',NULL),
		(2.0,'Default','real.jpg','other',NULL),(x'3033',NULL,NULL,NULL,NULL),
		('04e0',NULL,NULL,NULL,NULL),(NULL,NULL,NULL,NULL,NULL),
		(-1,NULL,NULL,NULL,NULL),(0,NULL,NULL,NULL,NULL),('garbage',NULL,NULL,NULL,NULL);`
	service, state, source, request := pictureCreationFixture(t, false, schema)
	before := pictureCreationRawRows(t, source)
	receipt, err := service.createPicture(t.Context(), state.ContextID, source, request)
	if err != nil || receipt.ID != 5 || receipt.RowID != "10" {
		t.Fatal("global text/real/blob aliases were not reserved exactly once", receipt, err)
	}
	after := pictureCreationRawRows(t, source)
	if !reflect.DeepEqual(before, after[:len(before)]) {
		t.Fatal("numeric aliases were normalized in original storage")
	}
	pictureMetadataFixtureExec(t, source, `DELETE FROM tblVPics WHERE rowid=10`)
	before = pictureCreationRawRows(t, source)
	if lookup, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request); err == nil || lookup != nil {
		t.Fatal("deleted row inferred as replayable from numeric ID/reservation", lookup, err)
	}
	request.RequestID = pictureCreationTestUUID2
	receipt2, err := service.createPicture(t.Context(), state.ContextID, source, request)
	if err != nil || receipt2.ID != 6 || receipt2.RowID != "10" {
		t.Fatal("deleted new identity reused or reused rowid mistaken for ownership", receipt2, err)
	}
	if pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationIdentity" WHERE ID IN (5,6)`) != 2 {
		t.Fatal("permanent reservation removed")
	}
	request.RequestID = pictureCreationTestUUID
	if replay, err := service.createPicture(t.Context(), state.ContextID, source, request); err == nil || replay.DidCommit {
		t.Fatal("automatic rowid reuse incorrectly replayed deleted original", replay, err)
	}
	if !reflect.DeepEqual(before, pictureCreationRawRows(t, source)[:len(before)]) {
		t.Fatal("new append/rejected replay touched old rows")
	}
}

func TestPictureCreationAllocatorExhaustionAndConservativeCoercion(t *testing.T) {
	source, _ := pictureLibraryFixture(t, `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
		INSERT INTO tblVPics(ID) VALUES('01'),('1'),(2.0),(x'3033'),('4e0'),(NULL),(-1),(0);`)
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(pictureCreationIdentitySQL); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int64{0, -1, math.MaxInt32 + 1, 1, 2, 3, 4} {
		if id, err := allocatePictureCreationID(t.Context(), tx, limit, nil); err == nil || id != 0 {
			t.Fatal("bounded allocator accepted invalid/exhausted space", limit, id, err)
		}
	}
	if id, err := allocatePictureCreationID(t.Context(), tx, 5, nil); err != nil || id != 5 {
		t.Fatal("duplicate aliases skipped an available identity", id, err)
	}
	if _, err := tx.Exec(`INSERT INTO "__VPRO_PictureCreationIdentity" VALUES(5,?); DELETE FROM tblVPics WHERE CAST(ID AS NUMERIC)=4`,
		pictureCreationTestUUID); err != nil {
		t.Fatal(err)
	}
	if id, err := allocatePictureCreationID(t.Context(), tx, 5, nil); err != nil || id != 4 {
		t.Fatal("allocator ignored free gap/permanent reservation", id, err)
	}
	if _, err := tx.Exec(`INSERT INTO tblVPics(ID) VALUES('4.9junk')`); err != nil {
		t.Fatal(err)
	}
	if id, err := allocatePictureCreationID(t.Context(), tx, 6, nil); err != nil || id != 6 {
		t.Fatal("conservative unsupported alias was not reserved", id, err)
	}
}

func TestPictureCreationOwnershipLiteralParentAndExternalSU(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, true, pictureCreationTestSchema)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	for _, modify := range []func(*pictureCreationRequest){
		func(r *pictureCreationRequest) { r.ContextID = "" },
		func(r *pictureCreationRequest) { r.ContextID = "stale" },
		func(r *pictureCreationRequest) { r.Project = "Other" },
		func(r *pictureCreationRequest) { r.Project = "sample" },
		func(r *pictureCreationRequest) { r.PlotNumber = "9999999" },
		func(r *pictureCreationRequest) { r.PlotNumber = "10805" },
	} {
		wrong := request
		modify(&wrong)
		if receipt, err := service.createPicture(t.Context(), state.ContextID, source, wrong); err == nil || receipt.DidCommit {
			t.Fatal("unreviewed owner/nonparent reached creation", wrong, receipt, err)
		}
	}
	if _, err := service.createPicture(nil, state.ContextID, source, request); err == nil {
		t.Fatal("nil request context accepted")
	}
	if _, err := service.lookupPictureCreationReceipt(nil, state.ContextID, source, request); err == nil {
		t.Fatal("nil lookup context accepted")
	}
	if _, err := service.createPicture(t.Context(), state.ContextID, nil, request); err == nil {
		t.Fatal("unowned library accepted")
	}
	aliased, err := newOwnedPictureLibrary(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.createPicture(t.Context(), state.ContextID, aliased, request); err == nil {
		t.Fatal("context-owned source alias accepted")
	}
	owner := service.projects.sqlite
	db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["su"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM Report_SU WHERE PlotNumber='108050'; INSERT INTO Report_SU VALUES(108050,'numeric is not literal')`); err != nil {
		t.Fatal(err)
	}
	// Report_SU's TEXT affinity would repair a numeric assignment; use an actual
	// untyped replacement to test independent physical storage, not SQL literals.
	if _, err := db.Exec(`ALTER TABLE Report_SU RENAME TO old_SU;
		CREATE TABLE Report_SU(PlotNumber,SiteUnit); INSERT INTO Report_SU VALUES(108050,'numeric'); DROP TABLE old_SU`); err != nil {
		t.Fatal(err)
	}
	if receipt, err := service.createPicture(t.Context(), state.ContextID, source, request); err == nil || receipt.DidCommit {
		t.Fatal("external numeric SU was mistaken for literal membership", receipt, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("rejected owner wrote picture rows/ledger/history")
	}
}

func TestPictureCreationSourceSchemaAndAffinityRefusals(t *testing.T) {
	for name, schema := range map[string]string{
		"trigger": `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
			CREATE TRIGGER surprise AFTER INSERT ON tblVPics BEGIN DELETE FROM tblVPics; END`,
		"foreign-key": `CREATE TABLE parent(ID PRIMARY KEY);
			CREATE TABLE tblVPics(ID REFERENCES parent(ID),PicDir,PicName,PlotNumber,PicComment)`,
		"view":          `CREATE TABLE originals(ID,PicDir,PicName,PlotNumber,PicComment); CREATE VIEW tblVPics AS SELECT * FROM originals`,
		"order":         `CREATE TABLE tblVPics(ID,PlotNumber,PicDir,PicName,PicComment)`,
		"extra":         `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment,Extra)`,
		"missing":       `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber)`,
		"generated":     `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment AS (PicName))`,
		"without-rowid": `CREATE TABLE tblVPics(ID PRIMARY KEY,PicDir,PicName,PlotNumber,PicComment) WITHOUT ROWID`,
		"default":       `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment DEFAULT '')`,
		"id-affinity":   `CREATE TABLE tblVPics(ID TEXT,PicDir,PicName,PlotNumber,PicComment)`,
		"plot-affinity": `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber NUMERIC,PicComment)`,
		"name-affinity": `CREATE TABLE tblVPics(ID,PicDir,PicName NUMERIC,PlotNumber,PicComment)`,
		"null-comment":  `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment NOT NULL)`,
		"malformed-source": `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
			INSERT INTO tblVPics VALUES(9,NULL,NULL,'other',CAST(x'ff' AS TEXT))`,
	} {
		t.Run(name, func(t *testing.T) {
			service, state, source, request := pictureCreationFixture(t, false, schema)
			request.SelectedPath = `C:\001`
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			if receipt, err := service.createPicture(t.Context(), state.ContextID, source, request); err == nil || receipt.DidCommit {
				t.Fatal("unsupported schema/affinity accepted", receipt, err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("refusal left row/technical tables or repaired original schema")
			}
		})
	}
}

func TestPictureCreationMissingReceiptIsReadOnlyUnknown(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	if receipt, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request); err != nil || receipt != nil {
		t.Fatal("missing durable receipt inferred from values", receipt, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("missing lookup created technical schema")
	}
	if _, err := service.createPicture(t.Context(), state.ContextID, source, request); err != nil {
		t.Fatal(err)
	}
	request.RequestID = pictureCreationTestUUID2
	before = databaseBytes(t, map[string]string{"pictures": source.path})
	if receipt, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request); err != nil || receipt != nil {
		t.Fatal("same values were mistaken for another request's receipt", receipt, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("unknown receipt lookup wrote library")
	}
}

func TestPictureCreationUUIDProvenanceAndPhysicalReplayRefusals(t *testing.T) {
	for _, drift := range []string{"path", "owner", "parent", "actor", "audit", "row-cells", "alias", "reserved-id", "rowid-reuse", "columns"} {
		t.Run(drift, func(t *testing.T) {
			service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
			receipt, err := service.createPicture(t.Context(), state.ContextID, source, request)
			if err != nil {
				t.Fatal(err)
			}
			switch drift {
			case "path":
				request.SelectedPath = `Q:\No image grant\OTHER.jpg`
			case "owner":
				request.Project = "Other"
			case "parent":
				request.PlotNumber = "108050x"
			case "actor":
				if err := service.plots.SetCurrentUser("changed actor"); err != nil {
					t.Fatal(err)
				}
			case "audit":
				if err := service.plots.SetAuditStrength(3); err != nil {
					t.Fatal(err)
				}
			case "row-cells":
				pictureMetadataFixtureExec(t, source, `UPDATE tblVPics SET PicComment='' WHERE rowid=`+receipt.RowID)
			case "alias":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics VALUES('01',NULL,NULL,'elsewhere',NULL)`)
			case "reserved-id":
				pictureMetadataFixtureExec(t, source, `DELETE FROM "__VPRO_PictureCreationIdentity" WHERE ID=1`)
			case "rowid-reuse":
				pictureMetadataFixtureExec(t, source, `DELETE FROM tblVPics WHERE rowid=6;
					INSERT INTO tblVPics(ID,PicDir,PicName,PlotNumber,PicComment) VALUES(1,'different','replacement.jpg','108050',NULL)`)
				if pictureCreationCount(t, source, `SELECT rowid FROM tblVPics WHERE ID=1`) != 6 {
					t.Fatal("fixture did not automatically reuse rowid")
				}
			case "columns":
				pictureMetadataFixtureExec(t, source, `ALTER TABLE tblVPics RENAME TO old_pics;
					CREATE TABLE tblVPics(ID INTEGER,PicDir TEXT,PicName TEXT,PlotNumber TEXT,PicComment);
					INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment)
					SELECT rowid,ID,PicDir,PicName,PlotNumber,PicComment FROM old_pics; DROP TABLE old_pics`)
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			if result, err := service.createPicture(t.Context(), state.ContextID, source, request); err == nil || result.DidCommit {
				t.Fatal("incompatible provenance/physical drift replayed", result, err)
			}
			if result, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request); err == nil || result != nil {
				t.Fatal("incompatible provenance/physical drift looked up", result, err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("refused replay wrote library")
			}
		})
	}
}

func TestPictureCreationGlobalTechnicalCorruptionRefusals(t *testing.T) {
	for name, statement := range map[string]string{
		"history-index":    `CREATE INDEX drift ON "__VPRO_PictureCreationHistory"(Created)`,
		"ledger-index":     `CREATE INDEX drift ON "__VPRO_PictureCreationIdentity"(RequestID)`,
		"ledger-trigger":   `CREATE TRIGGER drift AFTER INSERT ON "__VPRO_PictureCreationIdentity" BEGIN DELETE FROM tblVPics; END`,
		"history-trigger":  `CREATE TRIGGER drift AFTER INSERT ON "__VPRO_PictureCreationHistory" BEGIN DELETE FROM tblVPics; END`,
		"missing-ledger":   `DROP TABLE "__VPRO_PictureCreationIdentity"`,
		"missing-history":  `DROP TABLE "__VPRO_PictureCreationHistory"`,
		"ledger-uuid":      `UPDATE "__VPRO_PictureCreationIdentity" SET RequestID='bogus'`,
		"ledger-blob":      `UPDATE "__VPRO_PictureCreationIdentity" SET RequestID=CAST(RequestID AS BLOB)`,
		"ledger-extra":     `INSERT INTO "__VPRO_PictureCreationIdentity" VALUES(3,'16fe3d56-d75d-4a74-9507-000000000003')`,
		"history-time":     `UPDATE "__VPRO_PictureCreationHistory" SET Created='changed'`,
		"history-restored": `UPDATE "__VPRO_PictureCreationHistory" SET Restored=''`,
		"history-blob":     `UPDATE "__VPRO_PictureCreationHistory" SET Proposal=CAST(Proposal AS BLOB)`,
		"history-unicode":  `UPDATE "__VPRO_PictureCreationHistory" SET Proposal=CAST(x'ff' AS TEXT)`,
		"history-json":     `UPDATE "__VPRO_PictureCreationHistory" SET Proposal='{}'`,
		"history-columns":  `ALTER TABLE "__VPRO_PictureCreationHistory" ADD COLUMN drift`,
		"ledger-columns":   `ALTER TABLE "__VPRO_PictureCreationIdentity" ADD COLUMN drift`,
	} {
		t.Run(name, func(t *testing.T) {
			service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
			if _, err := service.createPicture(t.Context(), state.ContextID, source, request); err != nil {
				t.Fatal(err)
			}
			pictureMetadataFixtureExec(t, source, statement)
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			// A different UUID must still reject corruption elsewhere in history.
			request.RequestID = pictureCreationTestUUID2
			if result, err := service.createPicture(t.Context(), state.ContextID, source, request); err == nil || result.DidCommit {
				t.Fatal("globally corrupt technical history accepted a new write", result, err)
			}
			if result, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request); err == nil || result != nil {
				t.Fatal("globally corrupt history treated as ordinary missing receipt", result, err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("corruption refusal repaired history/ledger/source")
			}
		})
	}
}

func TestPictureCreationStrictHistoryNestedKeysAndImmutableCells(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
	receipt, err := service.createPicture(t.Context(), state.ContextID, source, request)
	if err != nil {
		t.Fatal(err)
	}
	raw := pictureCreationStoredHistory(t, source, receipt.HistoryID)
	for _, corrupt := range []string{
		strings.Replace(raw, `"request":`, `"request":null,"unknown":`, 1),
		strings.Replace(raw, `"actor":`, `"Actor":`, 1),
		strings.Replace(raw, `"id":1`, `"id":1,"\u0069d":2`, 1),
		strings.Replace(raw, `"storage":"null"`, `"storage":"null","storage":"text"`, 1),
		strings.Replace(raw, `"declaredType":""`, `"declaredType":"","type":""`, 1),
		strings.Replace(raw, `"attachment:project":`, `"attachment:project":"collision","\u0061ttachment:project":`, 1),
	} {
		var history pictureCreationHistory
		if err := json.Unmarshal([]byte(corrupt), &history); err == nil {
			t.Fatal("nested strict history accepted unknown/duplicate/escaped collision", corrupt)
		}
	}
	var changed pictureCreationHistory
	if err := json.Unmarshal([]byte(raw), &changed); err != nil {
		t.Fatal(err)
	}
	changed.Result.Committed.Cells[4] = metadataText("")
	corrupt, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_PictureCreationHistory" SET Proposal=?`, string(corrupt)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	request.RequestID = pictureCreationTestUUID2
	if result, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request); err == nil || result != nil {
		t.Fatal("unrelated history with fabricated empty comment was ignored", result, err)
	}
}

func TestPictureCreationCancellationRollbackAndRetry(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := service.createPicture(cancelled, state.ContextID, source, request); !errors.Is(err, context.Canceled) || result.DidCommit {
		t.Fatal("pre-cancelled creation committed", result, err)
	}
	if result, err := service.lookupPictureCreationReceipt(cancelled, state.ContextID, source, request); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("pre-cancelled lookup succeeded", result, err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	waiting, stop := context.WithTimeout(t.Context(), 30*time.Millisecond)
	_, err := service.createPicture(waiting, state.ContextID, source, request)
	stop()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("queued snapshot lease did not cancel", err)
	}
	_, err = withContextPlotRequest(t.Context(), service, state.ContextID, func(plots *PlotService) (pictureCreationResult, error) {
		return withOwnedSIVISnapshot(t.Context(), plots, func(owner *sqliteContext, parent *sql.Tx) (pictureCreationResult, error) {
			authority, err := pictureCreationAuthority(t.Context(), plots, owner, state.ContextID, source, request)
			if err != nil {
				return pictureCreationResult{}, err
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			return writePictureCreation(ctx, source, request, authority, func() error {
				calls++
				if calls == 2 {
					cancel()
				}
				return ctx.Err()
			})
		})
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("in-flight precommit cancellation did not roll back", err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("cancelled creation retained row/reservation/history")
	}
	if result, err := service.createPicture(t.Context(), state.ContextID, source, request); err != nil || !result.DidCommit || result.ID != 1 {
		t.Fatal("cancelled owned snapshot/connection prevented safe retry", result, err)
	}
}

func TestPictureCreationFinalOwnershipAndValidationRollback(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	other, _ := pictureLibraryFixture(t, pictureCreationTestSchema)
	for _, cause := range []string{"source-identity", "parent-validation"} {
		t.Run(cause, func(t *testing.T) {
			info := source.info
			defer func() { source.info = info }()
			calls := 0
			_, err := withContextPlotRequest(t.Context(), service, state.ContextID, func(plots *PlotService) (pictureCreationResult, error) {
				return withOwnedSIVISnapshot(t.Context(), plots, func(owner *sqliteContext, parent *sql.Tx) (pictureCreationResult, error) {
					authority, err := pictureCreationAuthority(t.Context(), plots, owner, state.ContextID, source, request)
					if err != nil {
						return pictureCreationResult{}, err
					}
					return writePictureCreation(t.Context(), source, request, authority, func() error {
						calls++
						if calls == 2 {
							if cause == "source-identity" {
								source.info = other.info
							} else {
								return errors.New("parent left reviewed membership")
							}
						}
						return source.checkFile()
					})
				})
			})
			if err == nil || calls != 2 {
				t.Fatal("fallible final validation did not refuse commit", calls, err)
			}
			source.info = info
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("final validation left rows/ledger/history")
			}
		})
	}
	if _, err := service.createPicture(t.Context(), state.ContextID, source, request); err != nil {
		t.Fatal("rollback prevented retry", err)
	}
}

func TestPictureCreationReplacedSourceAndReadOnlyRefusals(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
	moved := filepath.Join(filepath.Dir(source.path), "original retained.sqlite")
	if err := os.Rename(source.path, moved); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(moved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source.path, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	if result, err := service.createPicture(t.Context(), state.ContextID, source, request); err == nil || result.DidCommit {
		t.Fatal("replacement inherited selected source identity", result, err)
	}
	if result, err := service.lookupPictureCreationReceipt(t.Context(), state.ContextID, source, request); err == nil || result != nil {
		t.Fatal("replacement inherited lookup authority", result, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("replaced source changed")
	}
	reviewed, err := newOwnedPictureLibrary(source.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(reviewed.path, 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(reviewed.path, 0600)
	if result, err := service.createPicture(t.Context(), state.ContextID, reviewed, request); err == nil || result.DidCommit {
		t.Fatal("read-only Windows file accepted creation", result, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("read-only failure changed source")
	}
}

func TestPictureCreationParentReservationsAndFreshWALMembership(t *testing.T) {
	for _, role := range []string{"project", "su"} {
		t.Run("fresh-"+role, func(t *testing.T) {
			service, state, source, request := pictureCreationFixture(t, true, pictureCreationTestSchema)
			owner := service.projects.sqlite
			db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var journal string
			if err := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&journal); err != nil || journal != "wal" {
				t.Fatal(journal, err)
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			_, err = withContextPlotRequest(t.Context(), service, state.ContextID, func(plots *PlotService) (struct{}, error) {
				return withOwnedSIVISnapshot(t.Context(), plots, func(owner *sqliteContext, parent *sql.Tx) (struct{}, error) {
					if err := requirePictureParent(t.Context(), owner, parent, request.PlotNumber); err != nil {
						t.Fatal(err)
					}
					statement := `DELETE FROM Sample_Admin WHERE Plot='108050'`
					if role == "su" {
						statement = `DELETE FROM Report_SU WHERE PlotNumber='108050'`
					}
					if _, err := db.Exec(statement); err != nil {
						t.Fatal(err)
					}
					if err := requirePictureParent(t.Context(), owner, parent, request.PlotNumber); err != nil {
						t.Fatal("fixture did not retain old membership", err)
					}
					return withPictureMetadataParentGuards(t.Context(), owner, request.PlotNumber, func(validate func() error) (struct{}, error) {
						t.Fatal("stale WAL snapshot authorized the creation boundary")
						return struct{}{}, validate()
					})
				})
			})
			if err == nil || !strings.Contains(err.Error(), "independently reserved") ||
				!reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("fresh independent parent membership was not required", err)
			}
		})
	}
	service, state, source, request := pictureCreationFixture(t, true, pictureCreationTestSchema)
	connections := map[string]*sql.DB{}
	for _, role := range []string{"project", "su"} {
		db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments[role], "rw")+"&_busy_timeout=0&_txlock=immediate")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var journal string
		if err := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&journal); err != nil || journal != "wal" {
			t.Fatal(journal, err)
		}
		connections[role] = db
	}
	receipt, err := withContextPlotRequest(t.Context(), service, state.ContextID, func(plots *PlotService) (pictureCreationResult, error) {
		return withOwnedSIVISnapshot(t.Context(), plots, func(owner *sqliteContext, parent *sql.Tx) (pictureCreationResult, error) {
			authority, err := pictureCreationAuthority(t.Context(), plots, owner, state.ContextID, source, request)
			if err != nil {
				return pictureCreationResult{}, err
			}
			return withPictureMetadataParentGuards(t.Context(), owner, request.PlotNumber, func(validate func() error) (pictureCreationResult, error) {
				return writePictureCreation(t.Context(), source, request, authority, func() error {
					for role, db := range connections {
						tx, err := db.BeginTx(t.Context(), nil)
						if err == nil {
							tx.Rollback()
							t.Fatal("independent writer acquired reserved " + role)
						}
						if !strings.Contains(err.Error(), "locked") {
							t.Fatal("unexpected parent reservation failure", role, err)
						}
					}
					return validate()
				})
			})
		})
	})
	if err != nil || !receipt.DidCommit {
		t.Fatal("reserved creation failed", receipt, err)
	}
	for role, db := range connections {
		tx, err := db.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatal("parent reservation leaked after commit", role, err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPictureCreationSignedPhysicalRowIDAndStandardSourceIndexes(t *testing.T) {
	schema := `CREATE TABLE tblVPics(ID INTEGER PRIMARY KEY,PicDir TEXT,PicName TEXT,PlotNumber TEXT,PicComment TEXT);
		CREATE INDEX PlotNumber ON tblVPics(PlotNumber);
		INSERT INTO tblVPics VALUES(-2147483648,'Default','old.jpg','other',NULL);
		INSERT INTO tblVPics VALUES(2147483647,'','max.jpg','other','retain');`
	service, state, source, request := pictureCreationFixture(t, false, schema)
	receipt, err := service.createPicture(t.Context(), state.ContextID, source, request)
	if err != nil || receipt.ID != 1 || receipt.RowID != "1" {
		t.Fatal("standard physical source index/INTEGER primary key refused", receipt, err)
	}
	source, _ = pictureLibraryFixture(t, `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
		INSERT INTO tblVPics(rowid,ID) VALUES(9223372036854775806,2147483647),(-9223372036854775808,-2147483648)`)
	receipt, err = service.createPicture(t.Context(), state.ContextID, source, request)
	if err != nil || receipt.ID != 1 || receipt.RowID != strconv.FormatInt(math.MaxInt64, 10) {
		t.Fatal("signed64 rowid narrowed to application ID", receipt, err)
	}
	request.RequestID = pictureCreationTestUUID2
	receipt2, err := service.createPicture(t.Context(), state.ContextID, source, request)
	if err != nil || receipt2.ID != 2 || receipt2.RowID == receipt.RowID {
		t.Fatal("SQLite rowid overflow/random allocation confused identity", receipt2, err)
	}
	if _, err := strconv.ParseInt(receipt2.RowID, 10, 64); err != nil {
		t.Fatal("physical rowid not signed64 string", err)
	}
}

func TestPictureCreationLibraryCollisionCancellationAndRetry(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, true, pictureCreationTestSchema)
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw")+"&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	lock, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	receipt, err := service.createPicture(ctx, state.ContextID, source, request)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || receipt.DidCommit {
		t.Fatal("cancelled independent library collision claimed commit/lost cancellation", receipt, err)
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if pictureCreationCount(t, source, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '__VPRO_PictureCreation%'`) != 0 {
		t.Fatal("library collision retained ledger/history schema")
	}
	if receipt, err := service.createPicture(t.Context(), state.ContextID, source, request); err != nil || !receipt.DidCommit || receipt.ID != 1 {
		t.Fatal("library collision/cancellation leaked a parent/source lease", receipt, err)
	}
}

func TestPictureCreationHistoryIdentityExhaustionRollsBackAppend(t *testing.T) {
	service, state, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
	receipt, err := service.createPicture(t.Context(), state.ContextID, source, request)
	if err != nil {
		t.Fatal(err)
	}
	var history pictureCreationHistory
	if err := json.Unmarshal([]byte(pictureCreationStoredHistory(t, source, receipt.HistoryID)), &history); err != nil {
		t.Fatal(err)
	}
	history.Result.HistoryID = strconv.FormatInt(math.MaxInt64, 10)
	raw, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_PictureCreationHistory" SET ID=?,Proposal=?`, math.MaxInt64, string(raw)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	request.RequestID = pictureCreationTestUUID2
	if receipt, err := service.createPicture(t.Context(), state.ContextID, source, request); err == nil ||
		!strings.Contains(err.Error(), "history identity space exhausted") || receipt.DidCommit {
		t.Fatal("history identity overflow repaired/wrapped or claimed commit", receipt, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) ||
		pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationIdentity"`) != 1 {
		t.Fatal("history exhaustion retained partial source append/reservation")
	}
}
