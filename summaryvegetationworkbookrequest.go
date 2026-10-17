package main

import "errors"

type SummaryVegetationWorkbookOptions struct {
	Title      string `json:"title"`
	Average    int    `json:"average"`
	Display    int    `json:"display"`
	ShowClass  int    `json:"showClass"`
	IncludeEnv bool   `json:"includeEnv"`
}

func (options *SummaryVegetationWorkbookOptions) UnmarshalJSON(data []byte) error {
	var raw struct {
		Title      *string `json:"title"`
		Average    *int    `json:"average"`
		Display    *int    `json:"display"`
		ShowClass  *int    `json:"showClass"`
		IncludeEnv *bool   `json:"includeEnv"`
	}
	if err := decodeStrictRequiredJSON(data, &raw, "Summary draft workbook options",
		"title", "average", "display", "showClass", "includeEnv"); err != nil {
		return err
	}
	if raw.Title == nil || raw.Average == nil || raw.Display == nil || raw.ShowClass == nil || raw.IncludeEnv == nil {
		return errors.New("Summary draft workbook requires explicit title, calculation and environmental options; NULL is not a value")
	}
	*options = SummaryVegetationWorkbookOptions{*raw.Title, *raw.Average, *raw.Display, *raw.ShowClass, *raw.IncludeEnv}
	return nil
}

type summaryVegetationWorkbookExportRequest struct {
	Options      SummaryVegetationWorkbookOptions `json:"options"`
	ApprovalHash string                           `json:"approvalHash"`
	Destination  string                           `json:"destination"`
}

func (request *summaryVegetationWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Options      *SummaryVegetationWorkbookOptions `json:"options"`
		ApprovalHash *string                           `json:"approvalHash"`
		Destination  *string                           `json:"destination"`
	}
	if err := decodeStrictRequiredJSON(data, &raw, "Summary draft workbook publication",
		"options", "approvalHash", "destination"); err != nil {
		return err
	}
	if raw.Options == nil || raw.ApprovalHash == nil || raw.Destination == nil {
		return errors.New("Summary draft workbook publication requires explicit options, approval and destination")
	}
	*request = summaryVegetationWorkbookExportRequest{*raw.Options, *raw.ApprovalHash, *raw.Destination}
	return nil
}
