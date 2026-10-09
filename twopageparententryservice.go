package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const twoPageParentEntryEditingEnvironment = "VPRO_TWO_PAGE_PARENT_ENTRY_EDITING"

type TwoPageEntryReferenceFilters struct {
	Zone    ProjectMetadataCell `json:"zone"`
	SubZone ProjectMetadataCell `json:"subZone"`
}

func (request *TwoPageEntryReferenceFilters) UnmarshalJSON(data []byte) error {
	type plain TwoPageEntryReferenceFilters
	var decoded plain
	if err := validateTwoPageEntryJSONProperties(data); err != nil {
		return fmt.Errorf("complete-entry reference filter transport: %w", err)
	}
	if err := decodeStrictRequiredJSON(data, &decoded, "complete-entry reference filters", "zone", "subZone"); err != nil {
		return fmt.Errorf("complete-entry reference filter transport: %w", err)
	}
	*request = TwoPageEntryReferenceFilters(decoded)
	return nil
}

type TwoPageEntryCodeAcknowledgement struct {
	ContextID string                    `json:"contextId"`
	Project   string                    `json:"project"`
	Plot      string                    `json:"plot"`
	Form      string                    `json:"form"`
	Table     string                    `json:"table"`
	RowID     string                    `json:"rowId"`
	Column    string                    `json:"column"`
	Expected  ProjectMetadataCell       `json:"expected"`
	Value     ProjectMetadataCell       `json:"value"`
	Reference SIVIParentSharedReference `json:"reference"`
}

func (request *TwoPageEntryCodeAcknowledgement) UnmarshalJSON(data []byte) error {
	type plain TwoPageEntryCodeAcknowledgement
	var decoded plain
	if err := validateTwoPageEntryJSONProperties(data); err != nil {
		return fmt.Errorf("complete-entry acknowledgement transport: %w", err)
	}
	if err := decodeStrictRequiredJSON(data, &decoded, "complete-entry acknowledgement", "contextId", "project", "plot", "form", "table", "rowId", "column",
		"expected", "value", "reference"); err != nil {
		return fmt.Errorf("complete-entry acknowledgement transport: %w", err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if err := decodeTwoPageEntryAcknowledgementReference(properties["reference"], &decoded.Reference); err != nil {
		return fmt.Errorf("complete-entry acknowledgement reference transport: %w", err)
	}
	*request = TwoPageEntryCodeAcknowledgement(decoded)
	return nil
}

func decodeTwoPageEntryAcknowledgementReference(data []byte, reference *SIVIParentSharedReference) error {
	if err := decodeStrictRequiredJSON(data, reference, "complete-entry reference", "column", "listName", "required", "source", "available",
		"diagnostic", "definitions", "choices"); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if err := decodeStrictRequiredJSON(properties["definitions"], &reference.Definitions, "complete-entry definitions", "columns", "rows"); err != nil {
		return err
	}
	var definitions struct {
		Columns []json.RawMessage `json:"columns"`
		Rows    []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(properties["definitions"], &definitions); err != nil {
		return err
	}
	for index, raw := range definitions.Columns {
		if err := decodeStrictRequiredJSON(raw, &reference.Definitions.Columns[index], "complete-entry definition column", "name", "declaredType"); err != nil {
			return err
		}
	}
	for index, raw := range definitions.Rows {
		if err := decodeStrictRequiredJSON(raw, &reference.Definitions.Rows[index], "complete-entry definition row", "rowId", "cells"); err != nil {
			return err
		}
	}
	var choices []json.RawMessage
	if err := json.Unmarshal(properties["choices"], &choices); err != nil {
		return err
	}
	for index, raw := range choices {
		if err := decodeProfileLifecycleJSON(raw, &reference.Choices[index], "rowId", "selectable", "diagnostic"); err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		for name := range fields {
			switch name {
			case "rowId", "code", "description", "selectable", "diagnostic":
			default:
				return fmt.Errorf("unknown complete-entry choice property %q", name)
			}
		}
		for _, name := range []string{"code", "description"} {
			if _, present := fields[name]; !present {
				return fmt.Errorf("choice requires explicit nullable %s", name)
			}
		}
	}
	return nil
}

func (request TwoPageEntryCodeAcknowledgement) private() twoPageEntryCodeAcknowledgement {
	return twoPageEntryCodeAcknowledgement{request.ContextID, request.Project, request.Plot, request.Form,
		request.Table, request.RowID, request.Column, request.Expected, request.Value, request.Reference}
}

type TwoPageParentEntryWrite struct {
	Original         *SIVIParentProjection             `json:"original"`
	Edits            []SIVIParentCellEdit              `json:"edits"`
	ProjectSource    int                               `json:"projectSource"`
	WorkingSource    int                               `json:"workingSource"`
	Acknowledgements []TwoPageEntryCodeAcknowledgement `json:"acknowledgements"`
	ProjectSelection *SIVIProjectSelection             `json:"projectSelection"`
}

func (request *TwoPageParentEntryWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentEntryWrite
	var decoded plain
	if err := validateTwoPageEntryJSONProperties(data); err != nil {
		return fmt.Errorf("complete-entry write transport: %w", err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for name := range properties {
		switch name {
		case "original", "edits", "projectSource", "workingSource", "acknowledgements", "projectSelection":
		default:
			return fmt.Errorf("unknown complete-entry write property %q", name)
		}
	}
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "edits", "projectSource", "workingSource", "acknowledgements"); err != nil {
		return fmt.Errorf("complete-entry write transport: %w", err)
	}
	*request = TwoPageParentEntryWrite(decoded)
	return nil
}

func validateTwoPageEntryJSONProperties(data []byte) error {
	if err := validateMetadataDraftJSON(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token {
		case json.Delim('{'):
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate complete-entry JSON property %q", key)
				}
				seen[name] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
		case json.Delim('['):
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
		}
		return err
	}
	return walk()
}

type TwoPageParentEntryService struct {
	contexts                    *ContextService
	readers                     twoPageEntryReferenceReaders
	reviewEnabled, editsEnabled bool
}

func NewTwoPageParentEntryService(contexts *ContextService, readers twoPageEntryReferenceReaders,
	lookup func(string) (string, bool)) (*TwoPageParentEntryService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("complete-entry service requires owned contexts and an explicit feature lookup")
	}
	review, err := siviFeature(twoPageParentReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	editing, err := siviFeature(twoPageParentEntryEditingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageParentEntryService{contexts: contexts, readers: readers, reviewEnabled: review, editsEnabled: editing}, nil
}

func (s *TwoPageParentEntryService) require(ctx context.Context, editing bool) error {
	if ctx == nil {
		return errors.New("complete-entry service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.reviewEnabled {
		return errors.New("complete-entry reads require independently enabled two-page source review")
	}
	if editing && !s.editsEnabled {
		return errors.New("complete-entry writes require independently enabled complete-form editing")
	}
	return nil
}

func (s *TwoPageParentEntryService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageEntryReferenceSnapshot, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageEntryReferenceSnapshot(ctx, contextID, plot, form, s.readers, nil)
}

func (s *TwoPageParentEntryService) GetReferences(ctx context.Context, contextID, plot, form string,
	filters TwoPageEntryReferenceFilters) (*TwoPageEntryReferenceSnapshot, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageEntryReferenceSnapshot(ctx, contextID, plot, form, s.readers, &filters)
}

func (s *TwoPageParentEntryService) Save(ctx context.Context, contextID, plot, form string,
	request TwoPageParentEntryWrite) (*SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	acknowledgements := make([]twoPageEntryCodeAcknowledgement, 0, len(request.Acknowledgements))
	for _, acknowledgement := range request.Acknowledgements {
		acknowledgements = append(acknowledgements, acknowledgement.private())
	}
	var selection *siviProjectSelection
	if request.ProjectSelection != nil {
		private := request.ProjectSelection.private()
		selection = &private
	}
	return s.contexts.writeTwoPageEntryWithOwnedReferencesAndProject(ctx, contextID, plot, form, request.Original,
		request.Edits, true, s.readers, twoPageEntryReferenceSelection{request.ProjectSource, request.WorkingSource}, acknowledgements, selection)
}

func (s *TwoPageParentEntryService) Restore(ctx context.Context, contextID, plot, form, historyID string,
	action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return s.contexts.restoreTwoPageEntry(ctx, contextID, plot, form, historyID, action, true)
}
