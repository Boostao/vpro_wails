package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const pictureDeletionTestSchema = `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
	INSERT INTO tblVPics VALUES(1,'','old.jpg','108050',NULL),(2,'Default','peer.jpg','108050',''),
		(3,'Default','foreign.jpg','108050x',x'ff00');`

func pictureDeletionFixture(t *testing.T, external bool, schema string) (*ContextService, *ownedPictureLibrary, pictureDeletionRequest) {
	t.Helper()
	service, state, source, _ := pictureCreationFixture(t, external, schema)
	review, err := service.readPictureMetadata(t.Context(), state.ContextID, "108050", source)
	if err != nil || len(review.Records.Rows) == 0 {
		t.Fatal("deletion fixture review failed", review, err)
	}
	return service, source, pictureDeletionRequest{
		RequestID: pictureCreationTestUUID, ContextID: state.ContextID, Project: "Sample", PlotNumber: "108050",
		Columns: review.Records.Columns, Original: review.Records.Rows[0],
	}
}

func TestPictureDeletionExactRowReplayAndPermanentAllocation(t *testing.T) {
	for _, external := range []bool{false, true} {
		for strength := 0; strength <= 3; strength++ {
			t.Run(fmt.Sprintf("external-%v/audit-%d", external, strength), func(t *testing.T) {
				service, source, request := pictureDeletionFixture(t, external, pictureDeletionTestSchema)
				if err := service.plots.SetAuditStrength(strength); err != nil {
					t.Fatal(err)
				}
				before := pictureCreationRawRows(t, source)
				parents := databaseBytes(t, service.projects.sqlite.attachments)
				receipt, err := service.deletePicture(t.Context(), request.ContextID, source, request)
				if err != nil || !receipt.DidCommit || receipt.Replayed || receipt.ID != 1 || receipt.HistoryID != "1" ||
					!reflect.DeepEqual(receipt.Request, request) || receipt.AuditStrength != strength {
					t.Fatal("exact deletion receipt differs", receipt, err)
				}
				after := pictureCreationRawRows(t, source)
				if !reflect.DeepEqual(after, before[1:]) {
					t.Fatal("deleted more than the reviewed row", after, before)
				}
				if pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureDeletionIdentity" WHERE ID=1`) != 1 ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureDeletionHistory"`) != 1 ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '%Audit%'`) != 0 {
					t.Fatal("missing permanent history/reservation or phantom audit")
				}
				library := databaseBytes(t, map[string]string{"pictures": source.path})
				recovered, err := service.lookupPictureDeletionReceipt(t.Context(), request.ContextID, source, request)
				if err != nil || recovered == nil || !recovered.Replayed || !reflect.DeepEqual(recovered.Request, request) ||
					!reflect.DeepEqual(databaseBytes(t, map[string]string{"pictures": source.path}), library) {
					t.Fatal("read-only recovery lost durable deletion or wrote", recovered, err)
				}
				replayed, err := service.deletePicture(t.Context(), request.ContextID, source, request)
				receipt.Replayed = true
				if err != nil || !reflect.DeepEqual(replayed, receipt) || !reflect.DeepEqual(databaseBytes(t, map[string]string{"pictures": source.path}), library) {
					t.Fatal("exact retry wrote again or lost receipt", replayed, err)
				}
				if !reflect.DeepEqual(databaseBytes(t, service.projects.sqlite.attachments), parents) {
					t.Fatal("deletion changed parent/support data")
				}
				created, err := service.createPicture(t.Context(), request.ContextID, source, pictureCreationRequest{
					RequestID: pictureCreationTestUUID2, ContextID: request.ContextID, Project: "Sample", PlotNumber: "108050",
					SelectedPath: `Q:\selected\new.jpg`,
				})
				if err != nil || created.ID != 4 {
					t.Fatal("creation recycled the deleted permanent ID", created, err)
				}
			})
		}
	}
}

func TestPictureDeletionPostcommitCancellationRetainsRecoverableReceipt(t *testing.T) {
	for _, cleanupFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(cleanupFailure), func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			receipt, err := service.deletePictureWithHooks(ctx, request.ContextID, source, request, pictureDeletionHooks{
				afterLibraryCommit: func(parent *sql.Tx) error {
					if pictureCreationCount(t, source, `SELECT COUNT(*) FROM tblVPics WHERE ID=1`) != 0 {
						t.Fatal("hook did not observe a durable independent commit")
					}
					if cleanupFailure {
						return parent.Rollback()
					}
					cancel()
					return nil
				},
			})
			var committed *pictureDeletionCommittedError
			if err == nil || !errors.As(err, &committed) || !receipt.DidCommit ||
				pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureDeletionHistory"`) != 1 {
				t.Fatal("postcommit failure erased or repeated deletion", receipt, err)
			}
			recovered, recoveryErr := service.lookupPictureDeletionReceipt(t.Context(), request.ContextID, source, request)
			if recoveryErr != nil || recovered == nil || recovered.ID != 1 || !recovered.DidCommit || !recovered.Replayed {
				t.Fatal("postcommit result cannot recover read-only", recovered, recoveryErr)
			}
		})
	}
}

func TestPictureDeletionRefusesIncomingCascade(t *testing.T) {
	service, source, request := pictureDeletionFixture(t, false, `CREATE TABLE tblVPics(ID INTEGER PRIMARY KEY,PicDir,PicName,PlotNumber,PicComment);
		INSERT INTO tblVPics VALUES(1,'','original.jpg','108050',NULL);
		CREATE TABLE unreviewed(ID INTEGER REFERENCES tblVPics(ID) ON DELETE CASCADE);
		INSERT INTO unreviewed VALUES(1);`)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	receipt, err := service.deletePicture(t.Context(), request.ContextID, source, request)
	if err == nil || receipt.DidCommit || !strings.Contains(err.Error(), "incoming foreign keys") ||
		!reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("deletion cascaded outside the source row", receipt, err)
	}
}

func TestPictureDeletionPreservesSignedZeroAndHistoricalCells(t *testing.T) {
	for _, id := range []int64{-2147483648, -1, 0, 2147483647} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, fmt.Sprintf(
				`CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
				INSERT INTO tblVPics VALUES(%d,NULL,'','108050',x'ff00');`, id))
			receipt, err := service.deletePicture(t.Context(), request.ContextID, source, request)
			if err != nil || receipt.ID != id || !reflect.DeepEqual(receipt.Request.Original, request.Original) ||
				len(pictureCreationRawRows(t, source)) != 0 {
				t.Fatal("historical cells/identity were repaired", receipt, err)
			}
		})
	}
}

func TestPictureDeletionRejectsDriftCollisionAndInvalidRequests(t *testing.T) {
	for _, reason := range []string{"context", "project", "parent", "request-id", "rowid", "id", "cell", "schema", "cancel", "duplicate-id", "numeric-alias", "trigger", "history-case"} {
		t.Run(reason, func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
			ctx := t.Context()
			switch reason {
			case "context":
				request.ContextID += "-other"
			case "project":
				request.Project = "Other"
			case "parent":
				request.PlotNumber += "x"
			case "request-id":
				request.RequestID = "not a UUID"
			case "rowid":
				request.Original.RowID = "01"
			case "id":
				request.Original.Cells[0] = ProjectMetadataCell{Storage: "null"}
			case "cell":
				value := "other.jpg"
				request.Original.Cells[2].Text = &value
			case "schema":
				request.Columns[0].DeclaredType = "OTHER"
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "duplicate-id":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics VALUES(1,'','outside.jpg','other',NULL)`)
			case "numeric-alias":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics VALUES('01','','outside.jpg','other',NULL)`)
			case "trigger":
				pictureMetadataFixtureExec(t, source, `CREATE TRIGGER unreviewed AFTER DELETE ON tblVPics BEGIN DELETE FROM tblVPics; END`)
			case "history-case":
				pictureMetadataFixtureExec(t, source, `CREATE TABLE "__vpro_picturedeletionhistory"(ID)`)
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			receipt, err := service.deletePicture(ctx, request.ContextID, source, request)
			if err == nil || receipt.DidCommit || !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("rejection committed or modified library", receipt, err)
			}
			if reason == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("lost cancellation", err)
			}
		})
	}
}

func TestPictureDeletionStrictJSONAndHistoryCorruption(t *testing.T) {
	service, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Replace(string(raw), `"requestId":`, `"unknown":0,"requestId":`, 1),
		strings.Replace(string(raw), `"requestId":`, `"requestId":"`+pictureCreationTestUUID2+`","requestId":`, 1),
		strings.Replace(string(raw), `"declaredType":`, `"type":"TEXT","declaredType":`, 1),
	} {
		var decoded pictureDeletionRequest
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatal("strict deletion repaired an invalid wire", invalid)
		}
	}
	if _, err := service.deletePicture(t.Context(), request.ContextID, source, request); err != nil {
		t.Fatal(err)
	}
	pictureMetadataFixtureExec(t, source, `UPDATE "__VPRO_PictureDeletionHistory" SET Proposal=replace(Proposal,'"didCommit":true','"didCommit":false')`)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	receipt, err := service.deletePicture(t.Context(), request.ContextID, source, request)
	if err == nil || receipt.DidCommit || !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("corrupt history was replayed or rewritten", receipt, err)
	}
}
