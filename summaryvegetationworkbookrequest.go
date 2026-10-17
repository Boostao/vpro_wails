package main

import (
	"encoding/json"
	"errors"
)

type SummaryVegetationWorkbookAncillaryOptions struct {
	CreateSummary bool `json:"createSummary"`
}

func (options *SummaryVegetationReferenceOptions) UnmarshalJSON(data []byte) error {
	var raw struct {
		ShowCommon    *bool `json:"showCommon"`
		UseGuideNames *bool `json:"useGuideNames"`
	}
	if err := decodeStrictRequiredJSON(data, &raw, "Summary effective reference options", "showCommon", "useGuideNames"); err != nil {
		return err
	}
	if raw.ShowCommon == nil || raw.UseGuideNames == nil {
		return errors.New("Summary effective references require explicit non-NULL common/guide selections")
	}
	*options = SummaryVegetationReferenceOptions{ShowCommon: *raw.ShowCommon, UseGuideNames: *raw.UseGuideNames}
	return nil
}

func (options *SummaryVegetationWorkbookAncillaryOptions) UnmarshalJSON(data []byte) error {
	var raw struct {
		CreateSummary *bool `json:"createSummary"`
	}
	if err := decodeStrictRequiredJSON(data, &raw, "Summary ancillary workbook options", "createSummary"); err != nil {
		return err
	}
	if raw.CreateSummary == nil {
		return errors.New("Summary ancillary workbook requires an explicit non-NULL summary selection")
	}
	*options = SummaryVegetationWorkbookAncillaryOptions{CreateSummary: *raw.CreateSummary}
	return nil
}

type SummaryVegetationWorkbookOptions struct {
	Title      string                                     `json:"title"`
	Average    int                                        `json:"average"`
	Display    int                                        `json:"display"`
	ShowClass  int                                        `json:"showClass"`
	IncludeEnv bool                                       `json:"includeEnv"`
	Ancillary  *SummaryVegetationWorkbookAncillaryOptions `json:"ancillary,omitempty"`
	References *SummaryVegetationReferenceOptions         `json:"references,omitempty"`
}

func (options *SummaryVegetationWorkbookOptions) UnmarshalJSON(data []byte) error {
	var raw struct {
		Title      *string                                    `json:"title"`
		Average    *int                                       `json:"average"`
		Display    *int                                       `json:"display"`
		ShowClass  *int                                       `json:"showClass"`
		IncludeEnv *bool                                      `json:"includeEnv"`
		Ancillary  *SummaryVegetationWorkbookAncillaryOptions `json:"ancillary"`
		References *SummaryVegetationReferenceOptions         `json:"references"`
	}
	properties, err := sourceChildJSONObject(data, "Summary draft workbook options",
		[]string{"title", "average", "display", "showClass", "includeEnv"}, []string{"ancillary", "references"})
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "Summary draft workbook options", "ancillary", "references"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Title == nil || raw.Average == nil || raw.Display == nil || raw.ShowClass == nil || raw.IncludeEnv == nil {
		return errors.New("Summary draft workbook requires explicit title, calculation and environmental options; NULL is not a value")
	}
	*options = SummaryVegetationWorkbookOptions{Title: *raw.Title, Average: *raw.Average, Display: *raw.Display,
		ShowClass: *raw.ShowClass, IncludeEnv: *raw.IncludeEnv, Ancillary: raw.Ancillary, References: raw.References}
	return nil
}

type summaryVegetationWorkbookExportRequest struct {
	Options      SummaryVegetationWorkbookOptions `json:"options"`
	ApprovalHash string                           `json:"approvalHash"`
	Destination  string                           `json:"destination"`
	CreatedAt    string                           `json:"createdAt,omitempty"`
}

func (request *summaryVegetationWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Options      *SummaryVegetationWorkbookOptions `json:"options"`
		ApprovalHash *string                           `json:"approvalHash"`
		Destination  *string                           `json:"destination"`
		CreatedAt    *string                           `json:"createdAt"`
	}
	properties, err := sourceChildJSONObject(data, "Summary draft workbook publication",
		[]string{"options", "approvalHash", "destination"}, []string{"createdAt"})
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "Summary draft workbook publication", "createdAt"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Options == nil || raw.ApprovalHash == nil || raw.Destination == nil {
		return errors.New("Summary draft workbook publication requires explicit options, approval and destination")
	}
	createdAt := ""
	if raw.Options.Ancillary != nil {
		if raw.CreatedAt == nil {
			return errors.New("Summary ancillary workbook publication requires its reviewed creation time")
		}
		if err := validateSummaryWorkbookCreatedAt(*raw.CreatedAt); err != nil {
			return err
		}
		createdAt = *raw.CreatedAt
	} else if raw.CreatedAt != nil {
		return errors.New("Summary draft workbook creation time requires explicit ancillary mode")
	}
	*request = summaryVegetationWorkbookExportRequest{Options: *raw.Options, ApprovalHash: *raw.ApprovalHash,
		Destination: *raw.Destination, CreatedAt: createdAt}
	return nil
}
