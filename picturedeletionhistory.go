package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func (result *pictureDeletionResult) UnmarshalJSON(data []byte) error {
	const operation = "picture deletion history"
	names := []string{"request", "id", "requestedSource", "source", "ownedFiles", "actor", "auditStrength", "deletedWhen", "historyId", "didCommit", "replayed"}
	properties, err := sourceChildJSONObject(data, operation, names, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, names...); err != nil {
		return err
	}
	var files map[string]json.RawMessage
	if err := json.Unmarshal(properties["ownedFiles"], &files); err != nil {
		return err
	}
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	fileProperties, err := sourceChildJSONObject(properties["ownedFiles"], operation+" owned files", keys, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(fileProperties, operation+" owned files", keys...); err != nil {
		return err
	}
	type plain pictureDeletionResult
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*result = pictureDeletionResult(decoded)
	return nil
}

func readPictureDeletionHistory(ctx context.Context, tx *sql.Tx, source *ownedPictureLibrary) ([]pictureDeletionResult, map[int64]string, error) {
	var tables int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE NOCASE IN (?,?)`,
		pictureDeletionHistoryTable, pictureDeletionIdentityTable).Scan(&tables); err != nil {
		return nil, nil, err
	}
	if tables == 0 {
		return []pictureDeletionResult{}, map[int64]string{}, nil
	}
	if tables != 2 {
		return nil, nil, errors.New("picture deletion history/identity pair is missing or ambiguous")
	}
	for _, definition := range []struct{ table, sql string }{
		{pictureDeletionHistoryTable, pictureDeletionHistorySQL},
		{pictureDeletionIdentityTable, pictureDeletionIdentitySQL},
	} {
		if err := verifyTechnicalProvenance(ctx, tx, definition.table, definition.sql, "picture deletion"); err != nil {
			return nil, nil, err
		}
	}
	reservations := map[int64]string{}
	uuids := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT ID,typeof(RequestID),CAST(RequestID AS BLOB) FROM "__VPRO_PictureDeletionIdentity" ORDER BY ID`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id int64
		var storage string
		var raw []byte
		if err := rows.Scan(&id, &storage, &raw); err != nil {
			return nil, nil, errors.Join(err, rows.Close())
		}
		uuid := string(raw)
		if id < math.MinInt32 || id > math.MaxInt32 || storage != "text" || !siviDeletionUUID.MatchString(uuid) || uuids[uuid] {
			return nil, nil, errors.Join(errors.New("picture deletion permanent identity ledger is corrupt"), rows.Close())
		}
		reservations[id], uuids[uuid] = uuid, true
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT ID,typeof(Created),CAST(Created AS BLOB),typeof(Proposal),CAST(Proposal AS BLOB),typeof(Restored),CAST(Restored AS BLOB)
		FROM "__VPRO_PictureDeletionHistory" ORDER BY ID`)
	if err != nil {
		return nil, nil, err
	}
	events := []pictureDeletionResult{}
	seen, proved := map[string]bool{}, map[int64]bool{}
	restorationIDs, active := map[string]bool{}, map[int64]bool{}
	for rows.Next() {
		var historyID int64
		var createdType, proposalType, restoredType string
		var created, proposal, restored []byte
		if err := rows.Scan(&historyID, &createdType, &created, &proposalType, &proposal, &restoredType, &restored); err != nil {
			return nil, nil, errors.Join(err, rows.Close())
		}
		var event pictureDeletionResult
		decodeErr := json.Unmarshal(proposal, &event)
		_, id, requestErr := preparePictureDeletion(event.Request)
		when, whenErr := time.Parse(time.RFC3339Nano, event.DeletedWhen)
		if decodeErr != nil || requestErr != nil || whenErr != nil || when.UTC().Format(time.RFC3339Nano) != event.DeletedWhen ||
			historyID < 1 || event.HistoryID != strconv.FormatInt(historyID, 10) || event.ID != id ||
			event.DeletedWhen != string(created) || createdType != "text" || proposalType != "text" ||
			!event.DidCommit || event.Replayed || seen[event.Request.RequestID] ||
			event.Source != source.path || event.RequestedSource != source.requestedPath || event.Actor == "" ||
			event.AuditStrength < 0 || event.AuditStrength > 3 || len(event.OwnedFiles) == 0 {
			return nil, nil, errors.Join(fmt.Errorf("picture deletion durable history %d is corrupt", historyID), decodeErr, requestErr, whenErr, rows.Close())
		}
		if err := validateChildPhysicalText("Picture deletion history actor", event.Actor, 255); err != nil {
			return nil, nil, errors.Join(err, rows.Close())
		}
		if strings.ContainsRune(event.Actor, 0) {
			return nil, nil, errors.Join(errors.New("picture deletion history actor contains NUL"), rows.Close())
		}
		for role, path := range event.OwnedFiles {
			if role == "" || path == "" || strings.ContainsRune(role, 0) || strings.ContainsRune(path, 0) {
				return nil, nil, errors.Join(errors.New("picture deletion history has invalid owned file provenance"), rows.Close())
			}
		}
		if _, exists := reservations[event.ID]; !exists {
			return nil, nil, errors.Join(errors.New("picture deletion history is missing its permanent ID reservation"), rows.Close())
		}
		if !proved[event.ID] {
			if reservations[event.ID] != event.Request.RequestID {
				return nil, nil, errors.Join(errors.New("picture deletion permanent reservation differs from its first deletion"), rows.Close())
			}
			proved[event.ID] = true
		}
		if active[event.ID] {
			return nil, nil, errors.Join(errors.New("picture deletion identity was reintroduced without typed restoration"), rows.Close())
		}
		active[event.ID] = true
		switch restoredType {
		case "null":
		case "text":
			receipt, err := decodePictureRestoration(restored, event)
			if err != nil || restorationIDs[receipt.Request.RequestID] {
				return nil, nil, errors.Join(err, errors.New("picture deletion has an invalid or duplicate restoration acknowledgement"), rows.Close())
			}
			restorationIDs[receipt.Request.RequestID] = true
			active[event.ID] = false
		default:
			return nil, nil, errors.Join(errors.New("picture deletion restoration acknowledgement has invalid storage"), rows.Close())
		}
		seen[event.Request.RequestID] = true
		events = append(events, event)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, nil, err
	}
	if len(proved) != len(reservations) {
		return nil, nil, errors.New("picture deletion identity reservation lacks durable history")
	}
	for uuid := range restorationIDs {
		if seen[uuid] {
			return nil, nil, errors.New("picture deletion/restoration UUID collision in durable history")
		}
	}
	return events, reservations, nil
}
