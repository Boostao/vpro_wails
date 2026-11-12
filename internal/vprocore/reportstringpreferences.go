package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

type ReportPreferenceChange[T comparable] struct {
	Key      string
	Expected T
	Proposed T
}

type ReportStringPreferenceChange = ReportPreferenceChange[string]

type ReportPreferenceUpdate struct {
	Changed   bool
	Committed bool
}

type ConfigStringPreferenceUpdate = ReportPreferenceUpdate

func (s *DesktopConfig) CompareAndSetReportStrings(ctx context.Context, label string, changes []ReportStringPreferenceChange) (ConfigStringPreferenceUpdate, error) {
	return CompareAndSetReportPreferences(s, ctx, label, changes, func(values ConfigValues, key string) (string, error) {
		return ConfigString(values, "ReportOptions", key)
	})
}

func CompareAndSetReportPreferences[T comparable](s *DesktopConfig, ctx context.Context, label string,
	changes []ReportPreferenceChange[T], read func(ConfigValues, string) (T, error)) (ReportPreferenceUpdate, error) {
	if ctx == nil {
		return ConfigStringPreferenceUpdate{}, fmt.Errorf("%s changes require a context", label)
	}
	if s == nil {
		return ConfigStringPreferenceUpdate{}, fmt.Errorf("%s configuration is unavailable", label)
	}
	if label == "" || len(changes) == 0 || read == nil {
		return ReportPreferenceUpdate{}, errors.New("report preference changes require explicit keys, a label and a typed reader")
	}
	seen := make(map[string]bool, len(changes))
	for _, change := range changes {
		if change.Key == "" || seen[change.Key] {
			return ReportPreferenceUpdate{}, errors.New("report preference keys are empty or duplicated")
		}
		seen[change.Key] = true
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return ConfigStringPreferenceUpdate{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return ConfigStringPreferenceUpdate{}, err
	}
	changed := false
	currentValues := make([]T, len(changes))
	for i, change := range changes {
		current, err := read(values, change.Key)
		if err != nil {
			return ConfigStringPreferenceUpdate{}, err
		}
		currentValues[i] = current
	}
	for i, change := range changes {
		current := currentValues[i]
		if current != change.Expected {
			return ConfigStringPreferenceUpdate{}, fmt.Errorf("%s changed; reload the saved values before updating", label)
		}
		changed = changed || current != change.Proposed
	}
	if err := ctx.Err(); err != nil {
		return ConfigStringPreferenceUpdate{}, err
	}
	if !changed {
		return ConfigStringPreferenceUpdate{}, nil
	}
	for _, change := range changes {
		if change.Expected == change.Proposed {
			continue
		}
		if err := setConfigValue(values, "ReportOptions", change.Key, change.Proposed, false); err != nil {
			return ConfigStringPreferenceUpdate{}, err
		}
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return ConfigStringPreferenceUpdate{}, err
	}
	committed, err := s.commitWithContext(ctx, data)
	result := ConfigStringPreferenceUpdate{Changed: committed, Committed: committed}
	if committed && err != nil {
		return result, fmt.Errorf("%s were committed; do not replay the change: %w", label, err)
	}
	return result, err
}
