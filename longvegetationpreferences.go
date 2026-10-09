package main

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

func (s *desktopConfig) readLongVegetationPreference(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("Long Vegetation preferences require a context")
	}
	if s == nil {
		return "", errors.New("Long Vegetation preference configuration is unavailable")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return "", err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return "", err
	}
	title, err := configString(values, "ReportOptions", "LVReportTitle")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return title, nil
}

func (s *desktopConfig) compareAndSetLongVegetationPreference(ctx context.Context, expected, proposed string) (configStringPreferenceUpdate, error) {
	if ctx == nil {
		return configStringPreferenceUpdate{}, errors.New("Long Vegetation preference changes require a context")
	}
	if s == nil {
		return configStringPreferenceUpdate{}, errors.New("Long Vegetation preference configuration is unavailable")
	}
	if expected != proposed && (!utf8.ValidString(proposed) || strings.ContainsRune(proposed, 0)) {
		return configStringPreferenceUpdate{}, errors.New("Long Vegetation title requires complete Unicode without NUL; no repair or normalization")
	}
	return s.compareAndSetReportStrings(ctx, "Long Vegetation preferences", []reportStringPreferenceChange{
		{Key: "LVReportTitle", Expected: expected, Proposed: proposed},
	})
}
