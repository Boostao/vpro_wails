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

func pictureRestorationFixture(t *testing.T, service *ContextService, source *ownedPictureLibrary,
	deletion pictureDeletionResult) pictureRestorationRequest {
	t.Helper()
	review, err := service.readPictureMetadata(t.Context(), deletion.Request.ContextID, deletion.Request.PlotNumber, source)
	if err != nil {
		t.Fatal(err)
	}
	return pictureRestorationRequest{RequestID: pictureCreationTestUUID2, ContextID: deletion.Request.ContextID, HistoryID: deletion.HistoryID,
		Deletion: deletion.Request, Peers: review.Records.Rows}
}

func TestPictureRestorationExactPhysicalRowsRecoveryAndCycles(t *testing.T) {
	for _, external := range []bool{false, true} {
		for strength := 0; strength <= 3; strength++ {
			t.Run(fmt.Sprintf("external-%v/audit-%d", external, strength), func(t *testing.T) {
				service, source, request := pictureDeletionFixture(t, external, pictureDeletionTestSchema)
				if err := service.plots.SetAuditStrength(strength); err != nil {
					t.Fatal(err)
				}
				before := pictureCreationRawRows(t, source)
				deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
				if err != nil {
					t.Fatal(err)
				}
				restoration := pictureRestorationFixture(t, service, source, deleted)
				unknown, err := service.lookupPictureRestorationReceipt(t.Context(), request.ContextID, source, restoration)
				if err != nil || unknown != nil {
					t.Fatal("unperformed restoration reported success", unknown, err)
				}
				restored, err := service.restorePicture(t.Context(), request.ContextID, source, restoration)
				if err != nil || !restored.DidCommit || restored.Replayed || !reflect.DeepEqual(restored.Deletion, deleted) ||
					!reflect.DeepEqual(pictureCreationRawRows(t, source), before) {
					t.Fatal("restoration changed physical historical values or peers", restored, err)
				}
				bytes := databaseBytes(t, map[string]string{"pictures": source.path})
				recovered, err := service.lookupPictureRestorationReceipt(t.Context(), request.ContextID, source, restoration)
				restored.Replayed = true
				if err != nil || recovered == nil || !reflect.DeepEqual(*recovered, restored) {
					t.Fatal("restoration recovery lost durable receipt", recovered, err)
				}
				replayed, err := service.restorePicture(t.Context(), request.ContextID, source, restoration)
				if err != nil || !reflect.DeepEqual(replayed, restored) ||
					!reflect.DeepEqual(bytes, databaseBytes(t, map[string]string{"pictures": source.path})) {
					t.Fatal("restoration replay/recovery wrote again", replayed, err)
				}
				other := restoration
				other.RequestID = "22222222-2222-4222-8222-222222222222"
				if result, err := service.restorePicture(t.Context(), request.ContextID, source, other); err == nil || result.DidCommit {
					t.Fatal("consumed history accepted a second restoration", result, err)
				}
				originalUUID := request.RequestID
				request.RequestID = "33333333-3333-4333-8333-333333333333"
				deleted, err = service.deletePicture(t.Context(), request.ContextID, source, request)
				if err != nil || deleted.HistoryID != "2" {
					t.Fatal("second deletion cycle rejected", deleted, err)
				}
				restoration = pictureRestorationFixture(t, service, source, deleted)
				restoration.RequestID = "44444444-4444-4444-8444-444444444444"
				if result, err := service.restorePicture(t.Context(), request.ContextID, source, restoration); err != nil || !result.DidCommit ||
					!reflect.DeepEqual(pictureCreationRawRows(t, source), before) {
					t.Fatal("second restoration cycle rejected or repaired data", result, err)
				}
				if pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureDeletionHistory"`) != 2 ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureDeletionIdentity" WHERE ID=1 AND RequestID='`+originalUUID+`'`) != 1 ||
					pictureCreationCount(t, source, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '%Audit%'`) != 0 {
					t.Fatal("cycle pruned history, changed first reservation or invented source audits")
				}
				created, err := service.createPicture(t.Context(), request.ContextID, source, pictureCreationRequest{
					RequestID: "55555555-5555-4555-8555-555555555555", ContextID: request.ContextID,
					Project: request.Project, PlotNumber: request.PlotNumber, SelectedPath: `Q:\selected\after.jpg`,
				})
				if err != nil || created.ID != 4 {
					t.Fatal("allocator rejected typed restoration history", created, err)
				}
			})
		}
	}
}

func TestPictureRestorationSignedHistoricalStorage(t *testing.T) {
	for _, id := range []int64{-2147483648, -1, 0, 2147483647} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, fmt.Sprintf(
				`CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
				INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment) VALUES(-7,%d,NULL,'','108050',x'ff00');`, id))
			before := pictureCreationRawRows(t, source)
			deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.restorePicture(t.Context(), request.ContextID, source, pictureRestorationFixture(t, service, source, deleted))
			if err != nil || !result.DidCommit || !reflect.DeepEqual(before, pictureCreationRawRows(t, source)) {
				t.Fatal("restoration repaired signed ID/rowid/NULL/empty/BLOB", result, err)
			}
		})
	}
}

func TestPictureRestorationRefusesCollisionsAndPeerReplacement(t *testing.T) {
	for _, reason := range []string{"rowid", "integer-id", "text-alias", "real-alias", "blob-alias", "peer", "peer-reuse", "trigger", "schema", "context", "history", "uuid", "cancel"} {
		t.Run(reason, func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
				INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment)
				VALUES(0,1,'','old.jpg','108050',NULL),(1,3,'','anchor.jpg','108050',''),(2,2,'','peer.jpg','108050',NULL);`)
			deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
			if err != nil {
				t.Fatal(err)
			}
			restore := pictureRestorationFixture(t, service, source, deleted)
			ctx := t.Context()
			switch reason {
			case "rowid":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber) VALUES(0,9,'','replacement.jpg','outside')`)
			case "integer-id":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics(ID,PlotNumber) VALUES(1,'outside')`)
			case "text-alias":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics(ID,PlotNumber) VALUES('01','outside')`)
			case "real-alias":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics(ID,PlotNumber) VALUES(1.0,'outside')`)
			case "blob-alias":
				pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics(ID,PlotNumber) VALUES(x'3031','outside')`)
			case "peer":
				pictureMetadataFixtureExec(t, source, `UPDATE tblVPics SET PicName='changed.jpg' WHERE rowid=2`)
			case "peer-reuse":
				pictureMetadataFixtureExec(t, source, `DELETE FROM tblVPics WHERE rowid=2;
					INSERT INTO tblVPics(ID,PicDir,PicName,PlotNumber,PicComment) VALUES(2,'','changed.jpg','108050',NULL)`)
				if pictureCreationCount(t, source, `SELECT COUNT(*) FROM tblVPics WHERE rowid=2 AND ID=2 AND PicName='changed.jpg'`) != 1 {
					t.Fatal("test did not reproduce automatic peer rowid reuse")
				}
			case "trigger":
				pictureMetadataFixtureExec(t, source, `CREATE TRIGGER unreviewed AFTER INSERT ON tblVPics BEGIN DELETE FROM tblVPics WHERE ID=3; END`)
			case "schema":
				restore.Deletion.Columns[0].DeclaredType = "OTHER"
			case "context":
				restore.ContextID += "-other"
			case "history":
				restore.HistoryID = "999"
			case "uuid":
				restore.RequestID = restore.Deletion.RequestID
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			result, err := service.restorePicture(ctx, request.ContextID, source, restore)
			if err == nil || result.DidCommit || !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("refused restoration mutated data/history", result, err)
			}
		})
	}
}

func TestPictureRestorationAutomaticTargetRowidReuse(t *testing.T) {
	service, source, request := pictureDeletionFixture(t, false, `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
		INSERT INTO tblVPics VALUES(1,'','old.jpg','108050',NULL);`)
	deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
	if err != nil {
		t.Fatal(err)
	}
	restore := pictureRestorationFixture(t, service, source, deleted)
	pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics VALUES(9,'','replacement.jpg','outside',NULL)`)
	if pictureCreationCount(t, source, `SELECT COUNT(*) FROM tblVPics WHERE rowid=1 AND ID=9`) != 1 {
		t.Fatal("test did not reproduce actual automatic target rowid reuse")
	}
	before := pictureCreationRawRows(t, source)
	if result, err := service.restorePicture(t.Context(), request.ContextID, source, restore); err == nil || result.DidCommit ||
		!reflect.DeepEqual(before, pictureCreationRawRows(t, source)) {
		t.Fatal("restoration replaced the reused rowid", result, err)
	}
}

func TestPictureRestorationRollbackAndPostcommitCleanup(t *testing.T) {
	for _, mode := range []string{"rollback", "cancel-before-commit", "cancel-after-commit", "parent-cleanup"} {
		t.Run(mode, func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
			deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
			if err != nil {
				t.Fatal(err)
			}
			restore := pictureRestorationFixture(t, service, source, deleted)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			var result pictureRestorationResult
			if mode == "rollback" || mode == "cancel-before-commit" {
				calls := 0
				result, err = writePictureRestoration(ctx, source, restore, deleted, func() error {
					calls++
					if calls == 2 {
						if mode == "cancel-before-commit" {
							cancel()
							return ctx.Err()
						}
						return errors.New("planned post-insertion guard failure")
					}
					return nil
				})
				if calls != 2 || err == nil || result.DidCommit ||
					!reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
					t.Fatal("precommit failure lost atomic rollback", result, err, calls)
				}
				return
			}
			result, err = service.restorePictureWithHooks(ctx, request.ContextID, source, restore, pictureDeletionHooks{
				afterLibraryCommit: func(parent *sql.Tx) error {
					if pictureCreationCount(t, source, `SELECT COUNT(*) FROM tblVPics WHERE ID=1`) != 1 {
						t.Fatal("restoration was not independently committed at hook")
					}
					if mode == "parent-cleanup" {
						return parent.Rollback()
					}
					cancel()
					return nil
				},
			})
			var committed *pictureRestorationCommittedError
			if err == nil || !errors.As(err, &committed) || !result.DidCommit {
				t.Fatal("postcommit failure lost restoration receipt", result, err)
			}
			recovered, recoveryErr := service.lookupPictureRestorationReceipt(t.Context(), request.ContextID, source, restore)
			if recoveryErr != nil || recovered == nil || !recovered.DidCommit || !recovered.Replayed {
				t.Fatal("postcommit restoration cannot recover read-only", recovered, recoveryErr)
			}
		})
	}
}

func TestPictureRestorationStrictWireAndCorruptAcknowledgements(t *testing.T) {
	for _, reason := range []string{"duplicate", "escaped-duplicate", "unknown", "null-peers", "bad-ack", "ack-actor", "ack-request", "ack-storage", "missing-ledger"} {
		t.Run(reason, func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
			deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
			if err != nil {
				t.Fatal(err)
			}
			restore := pictureRestorationFixture(t, service, source, deleted)
			wire, err := json.Marshal(restore)
			if err != nil {
				t.Fatal(err)
			}
			if reason == "duplicate" || reason == "escaped-duplicate" || reason == "unknown" || reason == "null-peers" {
				var raw string
				switch reason {
				case "duplicate":
					raw = `{"requestId":"` + restore.RequestID + `",` + string(wire[1:])
				case "escaped-duplicate":
					raw = `{"\u0072equestId":"` + restore.RequestID + `",` + string(wire[1:])
				case "unknown":
					raw = `{"unknown":true,` + string(wire[1:])
				case "null-peers":
					raw = strings.Replace(string(wire), `"peers":[`, `"peers":null,"other":[`, 1)
				}
				var decoded pictureRestorationRequest
				if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
					t.Fatal("strict restoration wire accepted", raw)
				}
				return
			}
			result, err := service.restorePicture(t.Context(), request.ContextID, source, restore)
			if err != nil {
				t.Fatal(err)
			}
			switch reason {
			case "bad-ack":
				pictureMetadataFixtureExec(t, source, `UPDATE "__VPRO_PictureDeletionHistory" SET Restored='{}'`)
			case "ack-actor", "ack-request":
				if reason == "ack-actor" {
					result.Actor = "invalid\x00actor"
				} else {
					result.Request.HistoryID = "999"
				}
				raw, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				pictureMetadataFixtureExec(t, source, `UPDATE "__VPRO_PictureDeletionHistory" SET Restored='`+strings.ReplaceAll(string(raw), "'", "''")+`'`)
			case "ack-storage":
				pictureMetadataFixtureExec(t, source, `UPDATE "__VPRO_PictureDeletionHistory" SET Restored=CAST(Restored AS BLOB)`)
			case "missing-ledger":
				pictureMetadataFixtureExec(t, source, `DELETE FROM "__VPRO_PictureDeletionIdentity"`)
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			if recovered, err := service.lookupPictureRestorationReceipt(t.Context(), request.ContextID, source, restore); err == nil || recovered != nil ||
				!reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("corrupt acknowledgement accepted or rewritten", recovered, err)
			}
		})
	}
}

func TestPictureRestorationAfterActualContextSwitch(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprint(external), func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, external, pictureDeletionTestSchema)
			originalRows := pictureCreationRawRows(t, source)
			deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
			if err != nil {
				t.Fatal(err)
			}
			restore := pictureRestorationFixture(t, service, source, deleted)
			selection := service.projects.sqlite.selection
			contextSelection := ContextSelection{Project: selection.Project, ProjectPath: selection.ProjectPath,
				SU: selection.SU, SUPath: selection.SUPath, Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath}
			next, err := service.SwitchContext(request.ContextID, contextSelection)
			if err != nil || next.ContextID == request.ContextID {
				t.Fatal("test did not rotate the actual owned context", next, err)
			}
			afterDeletion := databaseBytes(t, map[string]string{"pictures": source.path})
			if result, err := service.restorePicture(t.Context(), next.ContextID, source, restore); err == nil || result.DidCommit ||
				!reflect.DeepEqual(afterDeletion, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("stale current restoration review bypassed the operation context", result, err)
			}
			restore.ContextID = next.ContextID
			service.plots.currentUser = "later restoration actor"
			if err := service.plots.SetAuditStrength((deleted.AuditStrength + 1) % 4); err != nil {
				t.Fatal(err)
			}
			result, err := service.restorePicture(t.Context(), next.ContextID, source, restore)
			if err != nil || !result.DidCommit || result.Actor != "later restoration actor" ||
				result.AuditStrength != service.plots.auditStrength || !reflect.DeepEqual(result.Deletion, deleted) ||
				result.Request.Deletion.ContextID != request.ContextID || result.Request.ContextID != next.ContextID ||
				!reflect.DeepEqual(originalRows, pictureCreationRawRows(t, source)) {
				t.Fatal("historical restore rewrote its origin or lost the current actor/context", result, err)
			}
			last, err := service.SwitchContext(next.ContextID, contextSelection)
			if err != nil || last.ContextID == next.ContextID {
				t.Fatal("second real context rotation failed", last, err)
			}
			beforeRecovery := databaseBytes(t, map[string]string{"pictures": source.path})
			recovered, err := service.lookupPictureRestorationReceipt(t.Context(), last.ContextID, source, restore)
			result.Replayed = true
			if err != nil || recovered == nil || !reflect.DeepEqual(*recovered, result) ||
				!reflect.DeepEqual(beforeRecovery, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("read-only historical recovery required an obsolete operation context or wrote", recovered, err)
			}
		})
	}
}

func TestPictureRestorationRejectsReboundFirstReservation(t *testing.T) {
	service, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
	first, err := service.deletePicture(t.Context(), request.ContextID, source, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.restorePicture(t.Context(), request.ContextID, source, pictureRestorationFixture(t, service, source, first)); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "33333333-3333-4333-8333-333333333333"
	second, err := service.deletePicture(t.Context(), request.ContextID, source, request)
	if err != nil {
		t.Fatal(err)
	}
	restore := pictureRestorationFixture(t, service, source, second)
	pictureMetadataFixtureExec(t, source, `UPDATE "__VPRO_PictureDeletionIdentity" SET RequestID='`+request.RequestID+`' WHERE ID=1`)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	if recovered, err := service.lookupPictureDeletionReceipt(t.Context(), request.ContextID, source, request); err == nil || recovered != nil {
		t.Fatal("recovery accepted a rebound permanent reservation", recovered, err)
	}
	if restored, err := service.restorePicture(t.Context(), request.ContextID, source, restore); err == nil || restored.DidCommit {
		t.Fatal("restoration accepted a rebound permanent reservation", restored, err)
	}
	created, err := service.createPicture(t.Context(), request.ContextID, source, pictureCreationRequest{
		RequestID: "55555555-5555-4555-8555-555555555555", ContextID: request.ContextID,
		Project: request.Project, PlotNumber: request.PlotNumber, SelectedPath: `Q:\selected\after.jpg`,
	})
	if err == nil || created.DidCommit || !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("allocation accepted or repaired a rebound permanent reservation", created, err)
	}
}

func TestPictureRestorationConcurrentExactlyOnce(t *testing.T) {
	for _, sameUUID := range []bool{false, true} {
		t.Run(fmt.Sprint(sameUUID), func(t *testing.T) {
			service, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
			deleted, err := service.deletePicture(t.Context(), request.ContextID, source, request)
			if err != nil {
				t.Fatal(err)
			}
			restore := pictureRestorationFixture(t, service, source, deleted)
			other := restore
			if !sameUUID {
				other.RequestID = "66666666-6666-4666-8666-666666666666"
			}
			type outcome struct {
				result pictureRestorationResult
				err    error
			}
			start, outcomes := make(chan struct{}), make(chan outcome, 2)
			for _, operation := range []pictureRestorationRequest{restore, other} {
				go func(operation pictureRestorationRequest) {
					<-start
					result, err := service.restorePicture(t.Context(), request.ContextID, source, operation)
					outcomes <- outcome{result, err}
				}(operation)
			}
			close(start)
			successes, replays := 0, 0
			for range 2 {
				outcome := <-outcomes
				if outcome.err == nil {
					successes++
					if outcome.result.Replayed {
						replays++
					}
				} else if outcome.result.DidCommit {
					t.Fatal("competing request reported unexpected committed error", outcome)
				}
			}
			expectedSuccess := 1
			if sameUUID {
				expectedSuccess = 2
			}
			if successes != expectedSuccess || replays != expectedSuccess-1 ||
				pictureCreationCount(t, source, `SELECT COUNT(*) FROM tblVPics WHERE ID=1`) != 1 ||
				pictureCreationCount(t, source, `SELECT COUNT(*) FROM "__VPRO_PictureDeletionHistory" WHERE Restored IS NOT NULL`) != 1 {
				t.Fatal("concurrent restoration duplicated or lost committed effect", successes, replays)
			}
		})
	}
}
