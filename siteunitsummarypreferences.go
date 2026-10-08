package main

import (
	"context"
	"errors"
	"fmt"
)

type SiteUnitSummaryPreferenceValues struct {
	Method       int `json:"method"`
	SiteUnitType int `json:"siteUnitType"`
}

func (values *SiteUnitSummaryPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Summary Environment preferences", "method", "siteUnitType"); err != nil {
		return err
	}
	*values = SiteUnitSummaryPreferenceValues(decoded)
	return nil
}

func (s *desktopConfig) compareAndSetSiteUnitSummaryPreferences(ctx context.Context,
	expected, proposed SiteUnitSummaryPreferenceValues) (reportPreferenceUpdate, error) {
	if proposed.Method != 1 && proposed.Method != 2 {
		return reportPreferenceUpdate{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if proposed.SiteUnitType != 1 {
		return reportPreferenceUpdate{}, errors.New("Summary Environment preference changes support only normal SU; hierarchy/field workflows are unavailable")
	}
	return compareAndSetReportPreferences(s, ctx, "Summary Environment preferences", []reportPreferenceChange[int]{
		{Key: "SEOptValueMethod", Expected: expected.Method, Proposed: proposed.Method},
		{Key: "SESuType", Expected: expected.SiteUnitType, Proposed: proposed.SiteUnitType},
	}, func(values configValues, key string) (int, error) {
		switch key {
		case "SEOptValueMethod":
			return configInt(values, "ReportOptions", key, 1, 2)
		case "SESuType":
			return configInt(values, "ReportOptions", key, 1, 3)
		default:
			return 0, fmt.Errorf("unsupported Summary Environment preference %s", key)
		}
	})
}
