package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type googleEarthPreferences struct {
	PlaceName        string
	DescriptionField string
}

type googleEarthPreferenceUpdate struct {
	Changed   bool
	Committed bool
}

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
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return googleEarthPreferenceUpdate{}, err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return googleEarthPreferenceUpdate{}, err
	}
	current, err := decodeGoogleEarthPreferences(values)
	if err != nil {
		return googleEarthPreferenceUpdate{}, err
	}
	if current != expected {
		return googleEarthPreferenceUpdate{}, errors.New("Google Earth preferences changed; reload the saved values before updating")
	}
	if err := ctx.Err(); err != nil {
		return googleEarthPreferenceUpdate{}, err
	}
	if current == proposed {
		return googleEarthPreferenceUpdate{}, nil
	}
	changes := map[string]string{}
	if proposed.PlaceName != current.PlaceName {
		changes["GoogleEarthPlaceName"] = proposed.PlaceName
	}
	if proposed.DescriptionField != current.DescriptionField {
		changes["GoogleEarthDescField"] = proposed.DescriptionField
	}
	for key, value := range changes {
		if err := setConfigValue(values, "ReportOptions", key, value, false); err != nil {
			return googleEarthPreferenceUpdate{}, err
		}
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return googleEarthPreferenceUpdate{}, err
	}
	committed, err := s.commitWithContext(ctx, data)
	result := googleEarthPreferenceUpdate{Changed: committed, Committed: committed}
	if committed && err != nil {
		return result, fmt.Errorf("Google Earth preferences were committed; do not replay the change: %w", err)
	}
	return result, err
}
