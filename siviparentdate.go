package main

import (
	"fmt"
	"regexp"
	"time"
)

var siviParentTimestampPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?$`)

func validateSIVIParentDateTimestamp(value *string) error {
	if value == nil {
		return nil
	}
	if !siviParentTimestampPattern.MatchString(*value) {
		return fmt.Errorf("Date requires YYYY-MM-DD HH:MM:SS[.fraction] without a timezone")
	}
	parsed, err := time.Parse("2006-01-02 15:04:05.999999999", *value)
	if err != nil {
		return fmt.Errorf("Date must be a valid calendar/time value: %w", err)
	}
	if parsed.Year() < 100 {
		return fmt.Errorf("Date year must be between 0100 and 9999")
	}
	return nil
}
