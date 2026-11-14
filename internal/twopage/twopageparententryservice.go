package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type TwoPageEntryReferenceFilters struct {
	Zone    vprocore.ProjectMetadataCell `json:"zone"`
	SubZone vprocore.ProjectMetadataCell `json:"subZone"`
}

func (request *TwoPageEntryReferenceFilters) UnmarshalJSON(data []byte) error {
	type plain TwoPageEntryReferenceFilters
	var decoded plain
	if err := vprocore.ValidateTwoPageEntryJSONProperties(data); err != nil {
		return fmt.Errorf("complete-entry reference filter transport: %w", err)
	}
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "complete-entry reference filters", "zone", "subZone"); err != nil {
		return fmt.Errorf("complete-entry reference filter transport: %w", err)
	}
	*request = TwoPageEntryReferenceFilters(decoded)
	return nil
}

type TwoPageEntryCodeAcknowledgement struct {
	ContextID string                               `json:"contextId"`
	Project   string                               `json:"project"`
	Plot      string                               `json:"plot"`
	Form      string                               `json:"form"`
	Table     string                               `json:"table"`
	RowID     string                               `json:"rowId"`
	Column    string                               `json:"column"`
	Expected  vprocore.ProjectMetadataCell         `json:"expected"`
	Value     vprocore.ProjectMetadataCell         `json:"value"`
	Reference sourceform.SIVIParentSharedReference `json:"reference"`
}

func (request *TwoPageEntryCodeAcknowledgement) UnmarshalJSON(data []byte) error {
	type plain TwoPageEntryCodeAcknowledgement
	var decoded plain
	if err := vprocore.ValidateTwoPageEntryJSONProperties(data); err != nil {
		return fmt.Errorf("complete-entry acknowledgement transport: %w", err)
	}
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "complete-entry acknowledgement", "contextId", "project", "plot", "form", "table", "rowId", "column",
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

func decodeTwoPageEntryAcknowledgementReference(data []byte, reference *sourceform.SIVIParentSharedReference) error {
	if err := vprocore.DecodeStrictRequiredJSON(data, reference, "complete-entry reference", "column", "listName", "required", "source", "available",
		"diagnostic", "definitions", "choices"); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if err := vprocore.DecodeStrictRequiredJSON(properties["definitions"], &reference.Definitions, "complete-entry definitions", "columns", "rows"); err != nil {
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
		if err := vprocore.DecodeStrictRequiredJSON(raw, &reference.Definitions.Columns[index], "complete-entry definition column", "name", "declaredType"); err != nil {
			return err
		}
	}
	for index, raw := range definitions.Rows {
		if err := vprocore.DecodeStrictRequiredJSON(raw, &reference.Definitions.Rows[index], "complete-entry definition row", "rowId", "cells"); err != nil {
			return err
		}
	}
	var choices []json.RawMessage
	if err := json.Unmarshal(properties["choices"], &choices); err != nil {
		return err
	}
	for index, raw := range choices {
		if err := vprocore.DecodeProfileLifecycleJSON(raw, &reference.Choices[index], "rowId", "selectable", "diagnostic"); err != nil {
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

func (request TwoPageEntryCodeAcknowledgement) private() TwoPageEntryCodeAcknowledgementInternal {
	return TwoPageEntryCodeAcknowledgementInternal{request.ContextID, request.Project, request.Plot, request.Form,
		request.Table, request.RowID, request.Column, request.Expected, request.Value, request.Reference}
}

type TwoPageParentEntryWrite struct {
	Original         *sourceform.SIVIParentProjection  `json:"original"`
	Edits            []sourceform.SIVIParentCellEdit   `json:"edits"`
	ProjectSource    int                               `json:"projectSource"`
	WorkingSource    int                               `json:"workingSource"`
	Acknowledgements []TwoPageEntryCodeAcknowledgement `json:"acknowledgements"`
	ProjectSelection *sourceform.SIVIProjectSelection  `json:"projectSelection"`
}

func (request *TwoPageParentEntryWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentEntryWrite
	var decoded plain
	if err := vprocore.ValidateTwoPageEntryJSONProperties(data); err != nil {
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
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "edits", "projectSource", "workingSource", "acknowledgements"); err != nil {
		return fmt.Errorf("complete-entry write transport: %w", err)
	}
	*request = TwoPageParentEntryWrite(decoded)
	return nil
}

type TwoPageParentEntryService struct {
	contexts *vprocore.ContextService
	Readers  TwoPageEntryReferenceReaders `json:"-" yaml:"-"`
}

func NewTwoPageParentEntryService(contexts *vprocore.ContextService, readers TwoPageEntryReferenceReaders) (*TwoPageParentEntryService, error) {
	if contexts == nil {
		return nil, errors.New("complete-entry service requires owned contexts")
	}
	return &TwoPageParentEntryService{contexts: contexts, Readers: readers}, nil
}

func (s *TwoPageParentEntryService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("complete-entry service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("complete-entry service requires owned contexts")
	}
	return nil
}

func (s *TwoPageParentEntryService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageEntryReferenceSnapshot, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageEntryReferenceSnapshot(s.contexts, ctx, contextID, plot, form, s.Readers, nil)
}

func (s *TwoPageParentEntryService) GetReferences(ctx context.Context, contextID, plot, form string,
	filters TwoPageEntryReferenceFilters) (*TwoPageEntryReferenceSnapshot, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageEntryReferenceSnapshot(s.contexts, ctx, contextID, plot, form, s.Readers, &filters)
}

func (s *TwoPageParentEntryService) Save(ctx context.Context, contextID, plot, form string,
	request TwoPageParentEntryWrite) (*sourceform.SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	acknowledgements := make([]TwoPageEntryCodeAcknowledgementInternal, 0, len(request.Acknowledgements))
	for _, acknowledgement := range request.Acknowledgements {
		acknowledgements = append(acknowledgements, acknowledgement.private())
	}
	var selection *sourceform.SiviProjectSelection
	if request.ProjectSelection != nil {
		private := request.ProjectSelection.Private()
		selection = &private
	}
	return WriteTwoPageEntryWithOwnedReferencesAndProject(s.contexts, ctx, contextID, plot, form, request.Original,
		request.Edits, true, s.Readers, TwoPageEntryReferenceSelection{request.ProjectSource, request.WorkingSource}, acknowledgements, selection)
}

func (s *TwoPageParentEntryService) Restore(ctx context.Context, contextID, plot, form, historyID string,
	action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return RestoreTwoPageEntry(s.contexts, ctx, contextID, plot, form, historyID, action, true)
}
