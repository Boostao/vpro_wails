package envreport

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

func validateLongEnvironmentTitle(title string) error {
	if !utf8.ValidString(title) || strings.ContainsRune(title, 0) {
		return errors.New("Long Environment title requires complete Unicode without NUL; no repair or normalization")
	}
	return nil
}

func ReadLongEnvironmentPreference(s *vprocore.DesktopConfig, ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("Long Environment preferences require a context")
	}
	if s == nil {
		return "", errors.New("Long Environment preference configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return "", err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return "", err
	}
	title, err := vprocore.ConfigString(values, "ReportOptions", "LEReportTitle")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return title, nil
}

func CompareAndSetLongEnvironmentPreference(s *vprocore.DesktopConfig, ctx context.Context, expected, proposed string) (vprocore.ConfigStringPreferenceUpdate, error) {
	if ctx == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Environment preference changes require a context")
	}
	if s == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Environment preference configuration is unavailable")
	}
	if expected != proposed {
		if err := validateLongEnvironmentTitle(proposed); err != nil {
			return vprocore.ConfigStringPreferenceUpdate{}, err
		}
	}
	return s.CompareAndSetReportStrings(ctx, "Long Environment preferences", []vprocore.ReportStringPreferenceChange{
		{Key: "LEReportTitle", Expected: expected, Proposed: proposed},
	})
}
