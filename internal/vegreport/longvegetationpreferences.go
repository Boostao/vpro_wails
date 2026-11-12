package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

func ReadLongVegetationPreference(s *vprocore.DesktopConfig, ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("Long Vegetation preferences require a context")
	}
	if s == nil {
		return "", errors.New("Long Vegetation preference configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return "", err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return "", err
	}
	title, err := vprocore.ConfigString(values, "ReportOptions", "LVReportTitle")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return title, nil
}

func CompareAndSetLongVegetationPreference(s *vprocore.DesktopConfig, ctx context.Context, expected, proposed string) (vprocore.ConfigStringPreferenceUpdate, error) {
	if ctx == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Vegetation preference changes require a context")
	}
	if s == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Vegetation preference configuration is unavailable")
	}
	if expected != proposed && (!utf8.ValidString(proposed) || strings.ContainsRune(proposed, 0)) {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Vegetation title requires complete Unicode without NUL; no repair or normalization")
	}
	return s.CompareAndSetReportStrings(ctx, "Long Vegetation preferences", []vprocore.ReportStringPreferenceChange{
		{Key: "LVReportTitle", Expected: expected, Proposed: proposed},
	})
}
