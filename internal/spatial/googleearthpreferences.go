package spatial

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type GoogleEarthPreferences struct {
	PlaceName        string
	DescriptionField string
}

type googleEarthPreferenceUpdate = vprocore.ConfigStringPreferenceUpdate

func decodeGoogleEarthPreferences(values vprocore.ConfigValues) (GoogleEarthPreferences, error) {
	var result GoogleEarthPreferences
	var err error
	result.PlaceName, err = vprocore.ConfigString(values, "ReportOptions", "GoogleEarthPlaceName")
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	result.DescriptionField, err = vprocore.ConfigString(values, "ReportOptions", "GoogleEarthDescField")
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	return result, nil
}

func ReadGoogleEarthPreferences(s *vprocore.DesktopConfig, ctx context.Context) (GoogleEarthPreferences, error) {
	if ctx == nil {
		return GoogleEarthPreferences{}, errors.New("Google Earth preferences require a context")
	}
	if s == nil {
		return GoogleEarthPreferences{}, errors.New("Google Earth preference configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return GoogleEarthPreferences{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	result, err := decodeGoogleEarthPreferences(values)
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	if err := ctx.Err(); err != nil {
		return GoogleEarthPreferences{}, err
	}
	return result, nil
}

func CompareAndSetGoogleEarthPreferences(s *vprocore.DesktopConfig, ctx context.Context, expected, proposed GoogleEarthPreferences) (googleEarthPreferenceUpdate, error) {
	if ctx == nil {
		return googleEarthPreferenceUpdate{}, errors.New("Google Earth preference changes require a context")
	}
	if s == nil {
		return googleEarthPreferenceUpdate{}, errors.New("Google Earth preference configuration is unavailable")
	}
	if proposed.PlaceName != expected.PlaceName {
		if err := validateGoogleEarthXMLText(proposed.PlaceName); err != nil {
			return googleEarthPreferenceUpdate{}, fmt.Errorf("Google Earth Place Name: %w", err)
		}
	}
	if proposed.DescriptionField != expected.DescriptionField &&
		(!utf8.ValidString(proposed.DescriptionField) || proposed.DescriptionField == "" || strings.ContainsRune(proposed.DescriptionField, 0)) {
		return googleEarthPreferenceUpdate{}, errors.New("Google Earth description field requires a literal nonempty Unicode name without NUL")
	}
	return s.CompareAndSetReportStrings(ctx, "Google Earth preferences", []vprocore.ReportStringPreferenceChange{
		{Key: "GoogleEarthPlaceName", Expected: expected.PlaceName, Proposed: proposed.PlaceName},
		{Key: "GoogleEarthDescField", Expected: expected.DescriptionField, Proposed: proposed.DescriptionField},
	})
}
