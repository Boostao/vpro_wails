package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type googleEarthPreferences struct {
	PlaceName        string
	DescriptionField string
}

type googleEarthPreferenceUpdate = configStringPreferenceUpdate

func decodeGoogleEarthPreferences(values configValues) (googleEarthPreferences, error) {
	var result googleEarthPreferences
	var err error
	result.PlaceName, err = configString(values, "ReportOptions", "GoogleEarthPlaceName")
	if err != nil {
		return googleEarthPreferences{}, err
	}
	result.DescriptionField, err = configString(values, "ReportOptions", "GoogleEarthDescField")
	if err != nil {
		return googleEarthPreferences{}, err
	}
	return result, nil
}

func (s *desktopConfig) readGoogleEarthPreferences(ctx context.Context) (googleEarthPreferences, error) {
	if ctx == nil {
		return googleEarthPreferences{}, errors.New("Google Earth preferences require a context")
	}
	if s == nil {
		return googleEarthPreferences{}, errors.New("Google Earth preference configuration is unavailable")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return googleEarthPreferences{}, err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return googleEarthPreferences{}, err
	}
	result, err := decodeGoogleEarthPreferences(values)
	if err != nil {
		return googleEarthPreferences{}, err
	}
	if err := ctx.Err(); err != nil {
		return googleEarthPreferences{}, err
	}
	return result, nil
}

func (s *desktopConfig) compareAndSetGoogleEarthPreferences(ctx context.Context, expected, proposed googleEarthPreferences) (googleEarthPreferenceUpdate, error) {
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
	return s.compareAndSetReportStrings(ctx, "Google Earth preferences", []reportStringPreferenceChange{
		{Key: "GoogleEarthPlaceName", Expected: expected.PlaceName, Proposed: proposed.PlaceName},
		{Key: "GoogleEarthDescField", Expected: expected.DescriptionField, Proposed: proposed.DescriptionField},
	})
}
