package main

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

type reportStringPreferenceChange struct {
	Key      string
	Expected string
	Proposed string
}

type configStringPreferenceUpdate struct {
	Changed   bool
	Committed bool
}

func (s *desktopConfig) compareAndSetReportStrings(ctx context.Context, label string, changes []reportStringPreferenceChange) (configStringPreferenceUpdate, error) {
	if ctx == nil {
		return configStringPreferenceUpdate{}, fmt.Errorf("%s changes require a context", label)
	}
	if s == nil {
		return configStringPreferenceUpdate{}, fmt.Errorf("%s configuration is unavailable", label)
	}
	if label == "" || len(changes) == 0 {
		return configStringPreferenceUpdate{}, errors.New("report string preference changes require explicit keys and a label")
	}
	seen := make(map[string]bool, len(changes))
	for _, change := range changes {
		if change.Key == "" || seen[change.Key] {
			return configStringPreferenceUpdate{}, errors.New("report string preference keys are empty or duplicated")
		}
		seen[change.Key] = true
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return configStringPreferenceUpdate{}, err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return configStringPreferenceUpdate{}, err
	}
	changed := false
	currentValues := make([]string, len(changes))
	for i, change := range changes {
		current, err := configString(values, "ReportOptions", change.Key)
		if err != nil {
			return configStringPreferenceUpdate{}, err
		}
		currentValues[i] = current
	}
	for i, change := range changes {
		current := currentValues[i]
		if current != change.Expected {
			return configStringPreferenceUpdate{}, fmt.Errorf("%s changed; reload the saved values before updating", label)
		}
		changed = changed || current != change.Proposed
	}
	if err := ctx.Err(); err != nil {
		return configStringPreferenceUpdate{}, err
	}
	if !changed {
		return configStringPreferenceUpdate{}, nil
	}
	for _, change := range changes {
		if change.Expected == change.Proposed {
			continue
		}
		if err := setConfigValue(values, "ReportOptions", change.Key, change.Proposed, false); err != nil {
			return configStringPreferenceUpdate{}, err
		}
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return configStringPreferenceUpdate{}, err
	}
	committed, err := s.commitWithContext(ctx, data)
	result := configStringPreferenceUpdate{Changed: committed, Committed: committed}
	if committed && err != nil {
		return result, fmt.Errorf("%s were committed; do not replay the change: %w", label, err)
	}
	return result, err
}
