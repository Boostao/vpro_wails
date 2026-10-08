package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

func sourceChildJSONObject(data []byte, operation string, required, optional []string) (map[string]json.RawMessage, error) {
	if err := validateMetadataDraftJSON(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("%s authority requires an object", operation)
	}
	allowed := map[string]bool{}
	for _, name := range append(append([]string{}, required...), optional...) {
		allowed[name] = true
	}
	result := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok || !allowed[name] || result[name] != nil {
			return nil, fmt.Errorf("unknown or duplicate %s property %q", operation, token)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		result[name] = value
	}
	for _, name := range required {
		if result[name] == nil {
			return nil, fmt.Errorf("%s requires explicit %s", operation, name)
		}
	}
	return result, nil
}

func sourceChildJSONNonNull(properties map[string]json.RawMessage, operation string, names ...string) error {
	for _, name := range names {
		if raw, present := properties[name]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("%s authority %s must not be NULL", operation, name)
		}
	}
	return nil
}

func sourceChildCellJSON(data []byte, operation string) error {
	properties, err := sourceChildJSONObject(data, operation, []string{"storage", "text", "integer", "real", "blobHex"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, "storage"); err != nil {
		return err
	}
	var cell ProjectMetadataCell
	if err := json.Unmarshal(data, &cell); err != nil {
		return err
	}
	_, err = metadataCellValue(cell)
	return err
}

func sourceChildRowsJSON(data []byte, operation string) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("%s reviewed rows must not be NULL", operation)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		return err
	}
	for _, raw := range rows {
		properties, err := sourceChildJSONObject(raw, operation, []string{"rowId", "cells"}, nil)
		if err != nil {
			return err
		}
		if err := sourceChildJSONNonNull(properties, operation, "rowId", "cells"); err != nil {
			return err
		}
		var cells []json.RawMessage
		if err := json.Unmarshal(properties["cells"], &cells); err != nil {
			return err
		}
		for _, cell := range cells {
			if err := sourceChildCellJSON(cell, operation); err != nil {
				return err
			}
		}
	}
	return nil
}

func sourceChildArrayJSON(data []byte, visit func(json.RawMessage) error) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("source child authority array must not be NULL")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	for _, item := range items {
		if err := visit(item); err != nil {
			return err
		}
	}
	return nil
}
