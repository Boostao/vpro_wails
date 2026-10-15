package main

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func TestPlotLabelRegionalNativeUTF16Capacity(t *testing.T) {
	cases := []struct {
		value []uint16
		count uintptr
	}{
		{nil, 0}, {[]uint16{0}, 2}, {[]uint16{'a'}, 1},
		{[]uint16{'a', 0, 'b', 0}, 4},
		{[]uint16{0xd800, 0}, 2}, {[]uint16{0xdc00, 0}, 2},
		{[]uint16{0xd800, 'a', 0}, 3},
	}
	for _, item := range cases {
		if _, err := plotLabelWindowsText(item.value, item.count); err == nil {
			t.Fatal("malformed/incomplete native text accepted", item)
		}
	}
	if got, err := plotLabelWindowsText([]uint16{0xd83d, 0xde00, 0}, 3); err != nil || got != "\U0001f600" {
		t.Fatal("valid supplementary native text changed", got, err)
	}
}

func TestPlotLabelRegionalNativeAPIFailures(t *testing.T) {
	stages := []string{"locale", "calendar-size", "calendar-text", "pattern-size", "pattern-text", "date-size", "date-text"}
	date := time.Date(2026, 10, 11, 0, 0, 0, 0, time.Local)
	refusal := errors.New("planned Windows API refusal")
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			api := plotLabelRegionalTestAPI(t, func(current string, buffer []uint16, value string) (uintptr, error) {
				if current == stage {
					return 0, refusal
				}
				return plotLabelRegionalTestText(buffer, value), nil
			})
			got, err := formatPlotLabelNativeDateWithAPI(date, api)
			if !errors.Is(err, refusal) || got != (plotLabelNativeDate{}) {
				t.Fatal("native API error lost or partial preview returned", got, err)
			}
		})
	}
	for _, calendar := range []string{"2", "invalid", ""} {
		t.Run("calendar-"+calendar, func(t *testing.T) {
			api := plotLabelRegionalTestAPI(t, func(stage string, buffer []uint16, value string) (uintptr, error) {
				if strings.HasPrefix(stage, "calendar-") {
					value = calendar
				}
				return plotLabelRegionalTestText(buffer, value), nil
			})
			got, err := formatPlotLabelNativeDateWithAPI(date, api)
			if err == nil || !strings.Contains(err.Error(), "Gregorian") || got != (plotLabelNativeDate{}) {
				t.Fatal("unsupported user calendar accepted", got, err)
			}
		})
	}
	for _, stage := range stages {
		for _, count := range []uintptr{0, 4097} {
			t.Run(stage+"-capacity-"+strconv.FormatUint(uint64(count), 10), func(t *testing.T) {
				api := plotLabelRegionalTestAPI(t, func(current string, buffer []uint16, value string) (uintptr, error) {
					if current == stage {
						return count, nil
					}
					return plotLabelRegionalTestText(buffer, value), nil
				})
				if got, err := formatPlotLabelNativeDateWithAPI(date, api); err == nil || got != (plotLabelNativeDate{}) {
					t.Fatal("invalid API output capacity accepted", got, err)
				}
			})
		}
	}
}

func plotLabelRegionalTestText(buffer []uint16, value string) uintptr {
	encoded := append(utf16.Encode([]rune(value)), 0)
	copy(buffer, encoded)
	return uintptr(len(encoded))
}

func plotLabelRegionalTestAPI(t *testing.T, call func(string, []uint16, string) (uintptr, error)) plotLabelWindowsAPI {
	t.Helper()
	return plotLabelWindowsAPI{
		name: func(buffer []uint16) (uintptr, error) { return call("locale", buffer, "en-US") },
		info: func(locale *uint16, kind uintptr, buffer []uint16) (uintptr, error) {
			if windows.UTF16PtrToString(locale) != "en-US" {
				t.Fatal("captured locale lost")
			}
			stage, value := "pattern", "dddd, MMMM d, yyyy"
			if kind == 0x1009 {
				stage, value = "calendar", "1"
			} else if kind != 0x20 {
				t.Fatal("unexpected native locale property", kind)
			}
			if buffer == nil {
				stage += "-size"
			} else {
				stage += "-text"
			}
			return call(stage, buffer, value)
		},
		date: func(locale *uint16, stamp *windows.Systemtime, pattern *uint16, buffer []uint16) (uintptr, error) {
			if windows.UTF16PtrToString(locale) != "en-US" || windows.UTF16PtrToString(pattern) != "dddd, MMMM d, yyyy" ||
				stamp.Year != 2026 || stamp.Month != 10 || stamp.Day != 11 {
				t.Fatal("captured native format inputs changed", stamp)
			}
			stage := "date-text"
			if buffer == nil {
				stage = "date-size"
			}
			return call(stage, buffer, "Sunday, October 11, 2026")
		},
	}
}

func TestPlotLabelRegionalNativeWindowsCallErrors(t *testing.T) {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	if count, err := plotLabelWindowsCall(kernel.NewProc("VPROPlotLabelNonexistentAPI")); count != 0 || err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatal("missing Windows export not rejected explicitly", count, err)
	}
	if count, err := plotLabelWindowsCall(kernel.NewProc("GetDateFormatEx"), 0, 0xffffffff, 0, 0, 0, 0, 0); count != 0 || err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatal("actual Windows API refusal not surfaced", count, err)
	}
}

func TestPlotLabelRegionalNativeOutputBoundary(t *testing.T) {
	for _, units := range []int{4095, 4096} {
		t.Run(strconv.Itoa(units), func(t *testing.T) {
			text := strings.Repeat("a", units-2) + "\U0001f600"
			api := plotLabelRegionalTestAPI(t, func(stage string, buffer []uint16, value string) (uintptr, error) {
				if strings.HasPrefix(stage, "date-") {
					value = text
				}
				return plotLabelRegionalTestText(buffer, value), nil
			})
			got, err := formatPlotLabelNativeDateWithAPI(time.Date(2026, 10, 11, 0, 0, 0, 0, time.Local), api)
			if units == 4095 {
				if err != nil || got.text != text {
					t.Fatal("maximum complete output changed", got, err)
				}
			} else if err == nil || got != (plotLabelNativeDate{}) {
				t.Fatal("oversized output accepted or truncated", got, err)
			}
		})
	}
}

func TestPlotLabelRegionalNativeOriginalLongDateCases(t *testing.T) {
	data, err := os.ReadFile(`testdata\plot-label-regional-date.json`)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Locale, Pattern, Calendar string
		Cases                     []struct{ ISODate, Actual string }
	}
	if err := json.Unmarshal(data, &source); err != nil || len(source.Cases) != 12 || source.Calendar != "1" {
		t.Fatal("exact twelve original source cases required", err)
	}
	for _, item := range source.Cases {
		date, err := time.Parse("2006-01-02", item.ISODate)
		if err != nil {
			t.Fatal(err)
		}
		got, err := formatPlotLabelNativeDate(date)
		if err != nil {
			t.Fatal(item.ISODate, err)
		}
		if got.locale != source.Locale || got.pattern != source.Pattern {
			t.Skip("source fixture is calibrated only for " + source.Locale + "/" + source.Pattern)
		}
		if "Label Date: "+got.text != item.Actual {
			t.Fatal("native Long Date disagrees with independently observed original Access", item, got)
		}
	}
	for _, year := range []int{1, 1600, 10000} {
		if _, err := formatPlotLabelNativeDate(time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)); err == nil {
			t.Fatal("unsupported native year silently normalized", year)
		}
	}
}
