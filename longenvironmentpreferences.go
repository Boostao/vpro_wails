package main

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

func (s *desktopConfig) readLongEnvironmentPreference(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("Long Environment preferences require a context")
	}
	if s == nil {
		return "", errors.New("Long Environment preference configuration is unavailable")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return "", err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return "", err
	}
	title, err := configString(values, "ReportOptions", "LEReportTitle")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return title, nil
}

func (s *desktopConfig) compareAndSetLongEnvironmentPreference(ctx context.Context, expected, proposed string) (configStringPreferenceUpdate, error) {
	if ctx == nil {
		return configStringPreferenceUpdate{}, errors.New("Long Environment preference changes require a context")
	}
	if s == nil {
		return configStringPreferenceUpdate{}, errors.New("Long Environment preference configuration is unavailable")
	}
	if expected != proposed {
		if err := validateLongEnvironmentTitle(proposed); err != nil {
			return configStringPreferenceUpdate{}, err
		}
	}
	return s.compareAndSetReportStrings(ctx, "Long Environment preferences", []reportStringPreferenceChange{
		{Key: "LEReportTitle", Expected: expected, Proposed: proposed},
	})
}
