package main

import "testing"

func TestSIVIParentDateTimestamp(t *testing.T) {
	if err := validateSIVIParentDateTimestamp(nil); err != nil {
		t.Fatalf("explicit NULL rejected: %v", err)
	}
	for _, value := range []string{
		"0100-01-01 00:00:00", "9999-12-31 23:59:59.999999999",
		"2000-02-29 12:34:56", "2024-02-29 00:00:00.000",
		"2026-11-01 01:30:00.123456789", "2026-03-08 02:30:00.100000000",
	} {
		original := value
		if err := validateSIVIParentDateTimestamp(&value); err != nil {
			t.Errorf("valid timestamp %q rejected: %v", value, err)
		}
		if value != original {
			t.Errorf("wallclock/fraction was normalized: %q -> %q", original, value)
		}
	}
	for _, value := range []string{
		"", "2026-01-01", "2026-01-01T00:00:00", " 2026-01-01 00:00:00",
		"2026-01-01 00:00:00 ", "2026-01-01 00:00:00\n", "2026-01-01 00:00:00\r\n",
		"2026-01-01 00:00:00Z", "2026-01-01 00:00:00+01:00",
		"0099-12-31 23:59:59", "0000-01-01 00:00:00", "10000-01-01 00:00:00",
		"1900-02-29 00:00:00", "2025-02-29 00:00:00", "2026-04-31 00:00:00",
		"2026-00-01 00:00:00", "2026-13-01 00:00:00", "2026-01-00 00:00:00",
		"2026-01-32 00:00:00", "2026-01-01 24:00:00", "2026-01-01 00:60:00",
		"2026-01-01 00:00:60", "2026-01-01 00:00:00.",
		"2026-01-01 00:00:00.1234567890", "2026-01-01 00:00:00,1",
		"2026-01-01 00:00:00\x00", "2026-01-01 00:00:00\xed\xa0\x80",
	} {
		if err := validateSIVIParentDateTimestamp(&value); err == nil {
			t.Errorf("malformed timestamp %q accepted", value)
		}
	}
}
