package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

type twoPageSoilReference struct {
	Column      string               `json:"column"`
	ListName    string               `json:"listName"`
	Definitions ProjectMetadataTable `json:"definitions"`
}

type twoPageSoilReferences struct {
	ContextID string                 `json:"contextId"`
	Project   string                 `json:"project"`
	Plot      string                 `json:"plot"`
	Form      string                 `json:"form"`
	Source    string                 `json:"source"`
	Fields    []twoPageSoilReference `json:"fields"`
}

func twoPageSoilReferenceFields(form string) (string, []twoPageSoilReference, error) {
	switch form {
	case "SoilHumusXL":
		return "Humus", []twoPageSoilReference{
			{Column: "Horizon", ListName: "HumusHorizon"},
			{Column: "HumusStructureKind", ListName: "HumusStructureKind"},
			{Column: "HumusStructureDegree", ListName: "HumusStructureDegree"},
			{Column: "vonPost", ListName: "vonPost"},
			{Column: "MycelAbundance", ListName: "MycelAbundance"},
			{Column: "FecalAbundance", ListName: "MycelAbundance"},
		}, nil
	case "SoilMineralXL":
		return "Mineral", []twoPageSoilReference{
			{Column: "ASP", ListName: "MinSoilAspect"},
			{Column: "Texture", ListName: "SoilTexture"},
			{Column: "MineralStructureClass", ListName: "MineralStructureClass"},
			{Column: "MineralStructureKind", ListName: "MineralStructureKind"},
		}, nil
	default:
		return "", nil, fmt.Errorf("two-page soil references do not support source form %q", form)
	}
}

func (s *ContextService) readTwoPageSoilReferences(ctx context.Context, contextID, plot, form string) (*twoPageSoilReferences, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("two-page soil references require an owned context")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page soil references require an exact valid literal parent")
	}
	kind, fields, err := twoPageSoilReferenceFields(form)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageSoilReferences, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*twoPageSoilReferences, error) {
			project := owner.selection.Project
			if err := requireTwoPageSoilReadScope(ctx, tx, project, project+"_"+kind, plot); err != nil {
				return nil, fmt.Errorf("two-page soil reference scope: %w", err)
			}
			for i := range fields {
				definitions, err := readSIVISharedFamilyReference(ctx, tx, "VLists", fields[i].ListName)
				if err != nil {
					return nil, fmt.Errorf("two-page soil reference %s.%s: %w", form, fields[i].Column, err)
				}
				fields[i].Definitions = definitions
			}
			return &twoPageSoilReferences{
				ContextID: contextID, Project: project, Plot: plot, Form: form,
				Source: "configured-owned-imported-SQLite VLists.USysTableOfLists; physical rowid; ItemOrder then rowid",
				Fields: fields,
			}, nil
		})
	})
}
