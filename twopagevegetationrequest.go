package main

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

type twoPageVegetationRequest struct {
	RequestID string                     `json:"requestId"`
	Original  *twoPageVegetationOriginal `json:"original"`
	Edits     []twoPageVegetationEdit    `json:"edits"`
}

func (request *twoPageVegetationRequest) UnmarshalJSON(data []byte) error {
	properties, err := sourceChildJSONObject(data, "two-page vegetation request", []string{"requestId", "original", "edits"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "two-page vegetation request", "requestId", "original", "edits"); err != nil {
		return err
	}
	required := []string{"contextId", "project", "plot", "form", "recordSource", "table", "records", "sourceRowIds"}
	original, err := sourceChildJSONObject(properties["original"], "two-page vegetation original", required, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(original, "two-page vegetation original", required...); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(original["sourceRowIds"], func(raw json.RawMessage) error {
		var rowID string
		if err := json.Unmarshal(raw, &rowID); err != nil {
			return err
		}
		id, err := strconv.ParseInt(rowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != rowID {
			return errors.New("two-page vegetation source row requires a canonical physical identity")
		}
		return nil
	}); err != nil {
		return err
	}
	records, err := sourceChildJSONObject(original["records"], "two-page vegetation records", []string{"columns", "rows"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildArrayJSON(records["columns"], func(raw json.RawMessage) error {
		column, err := sourceChildJSONObject(raw, "two-page vegetation column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(column, "two-page vegetation column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceChildRowsJSON(records["rows"], "two-page vegetation original"); err != nil {
		return err
	}
	type plain twoPageVegetationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !siviDeletionUUID.MatchString(decoded.RequestID) {
		return errors.New("two-page vegetation request requires a stable lowercase UUIDv4")
	}
	// The wire also carries durable precalibration requests; only a new write requires canonical SINGLE assignments.
	if _, err := planTwoPageVegetationHistoricalEdits(context.Background(), decoded.Original, decoded.Edits); err != nil {
		return err
	}
	*request = twoPageVegetationRequest(decoded)
	return nil
}

func detachTwoPageVegetationRequest(ctx context.Context, service *ContextService, request twoPageVegetationRequest) (twoPageVegetationRequest, error) {
	if ctx == nil || service == nil {
		return twoPageVegetationRequest{}, errors.New("two-page vegetation request requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return twoPageVegetationRequest{}, err
	}
	if !siviDeletionUUID.MatchString(request.RequestID) {
		return twoPageVegetationRequest{}, errors.New("two-page vegetation request requires a stable lowercase UUIDv4")
	}
	if _, err := planTwoPageVegetationHistoricalEdits(ctx, request.Original, request.Edits); err != nil {
		return twoPageVegetationRequest{}, err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return twoPageVegetationRequest{}, err
	}
	var detached twoPageVegetationRequest
	if err := json.Unmarshal(raw, &detached); err != nil {
		return twoPageVegetationRequest{}, err
	}
	return detached, nil
}
