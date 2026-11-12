package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Environment preferences", "method", "siteUnitType"); err != nil {
		return err
	}
	*values = SiteUnitSummaryPreferenceValues(decoded)
	return nil
}

func CompareAndSetSiteUnitSummaryPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed SiteUnitSummaryPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if proposed.Method != 1 && proposed.Method != 2 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if proposed.SiteUnitType != 1 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Environment preference changes support only normal SU; hierarchy/field workflows are unavailable")
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Summary Environment preferences", []vprocore.ReportPreferenceChange[int]{
		{Key: "SEOptValueMethod", Expected: expected.Method, Proposed: proposed.Method},
		{Key: "SESuType", Expected: expected.SiteUnitType, Proposed: proposed.SiteUnitType},
	}, func(values vprocore.ConfigValues, key string) (int, error) {
		switch key {
		case "SEOptValueMethod":
			return vprocore.ConfigInt(values, "ReportOptions", key, 1, 2)
		case "SESuType":
			return vprocore.ConfigInt(values, "ReportOptions", key, 1, 3)
		default:
			return 0, fmt.Errorf("unsupported Summary Environment preference %s", key)
		}
	})
}
