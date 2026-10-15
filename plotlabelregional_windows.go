package main

import (
	"errors"
	"fmt"
	"strconv"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func plotLabelWindowsText(value []uint16, count uintptr) (string, error) {
	if count == 0 || count > uintptr(len(value)) || value[count-1] != 0 {
		return "", errors.New("native regional date returned incomplete UTF-16 output")
	}
	value = value[:count-1]
	for i := 0; i < len(value); i++ {
		unit := value[i]
		if unit == 0 {
			return "", errors.New("native regional date returned an embedded NUL")
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if i+1 == len(value) || value[i+1] < 0xdc00 || value[i+1] > 0xdfff {
				return "", errors.New("native regional date returned malformed UTF-16")
			}
			i++
		} else if unit >= 0xdc00 && unit <= 0xdfff {
			return "", errors.New("native regional date returned malformed UTF-16")
		}
	}
	return string(utf16.Decode(value)), nil
}

//go:uintptrescapes
func plotLabelWindowsCall(proc *windows.LazyProc, args ...uintptr) (uintptr, error) {
	if err := proc.Find(); err != nil {
		return 0, fmt.Errorf("%s unavailable: %w", proc.Name, err)
	}
	count, _, err := proc.Call(args...)
	if count == 0 {
		if err == syscall.Errno(0) {
			return 0, fmt.Errorf("%s failed without a Windows error code", proc.Name)
		}
		return 0, fmt.Errorf("%s failed: %w", proc.Name, err)
	}
	return count, nil
}

func formatPlotLabelNativeDate(date time.Time) (plotLabelNativeDate, error) {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	return formatPlotLabelNativeDateWithAPI(date, plotLabelWindowsAPI{
		name: func(buffer []uint16) (uintptr, error) {
			return plotLabelWindowsCall(kernel.NewProc("GetUserDefaultLocaleName"),
				uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		},
		info: func(locale *uint16, kind uintptr, buffer []uint16) (uintptr, error) {
			var pointer *uint16
			if len(buffer) != 0 {
				pointer = &buffer[0]
			}
			return plotLabelWindowsCall(kernel.NewProc("GetLocaleInfoEx"),
				uintptr(unsafe.Pointer(locale)), kind, uintptr(unsafe.Pointer(pointer)), uintptr(len(buffer)))
		},
		date: func(locale *uint16, stamp *windows.Systemtime, pattern *uint16, buffer []uint16) (uintptr, error) {
			var pointer *uint16
			if len(buffer) != 0 {
				pointer = &buffer[0]
			}
			return plotLabelWindowsCall(kernel.NewProc("GetDateFormatEx"),
				uintptr(unsafe.Pointer(locale)), 0, uintptr(unsafe.Pointer(stamp)),
				uintptr(unsafe.Pointer(pattern)), uintptr(unsafe.Pointer(pointer)), uintptr(len(buffer)), 0)
		},
	})
}

type plotLabelWindowsAPI struct {
	name func([]uint16) (uintptr, error)
	info func(*uint16, uintptr, []uint16) (uintptr, error)
	date func(*uint16, *windows.Systemtime, *uint16, []uint16) (uintptr, error)
}

func formatPlotLabelNativeDateWithAPI(date time.Time, api plotLabelWindowsAPI) (plotLabelNativeDate, error) {
	if date.Year() < 1601 || date.Year() > 9999 {
		return plotLabelNativeDate{}, errors.New("native label date is outside the supported Windows calendar range")
	}
	name := make([]uint16, 85)
	count, err := api.name(name)
	if err != nil {
		return plotLabelNativeDate{}, err
	}
	locale, err := plotLabelWindowsText(name, count)
	if err != nil || locale == "" {
		return plotLabelNativeDate{}, errors.Join(err, errors.New("native label locale is unavailable"))
	}
	localePointer, err := windows.UTF16PtrFromString(locale)
	if err != nil {
		return plotLabelNativeDate{}, err
	}
	info := func(kind uintptr) (string, error) {
		size, err := api.info(localePointer, kind, nil)
		if err != nil {
			return "", err
		}
		if size == 0 || size > 4096 {
			return "", errors.New("native label locale value exceeds the supported output capacity")
		}
		buffer := make([]uint16, size)
		count, err := api.info(localePointer, kind, buffer)
		if err != nil {
			return "", err
		}
		return plotLabelWindowsText(buffer, count)
	}
	calendar, err := info(0x1009)
	if err != nil {
		return plotLabelNativeDate{}, err
	}
	number, err := strconv.Atoi(calendar)
	if err != nil || number != 1 {
		return plotLabelNativeDate{}, errors.Join(err, errors.New("native label date requires the supported Gregorian user calendar"))
	}
	pattern, err := info(0x20)
	if err != nil || pattern == "" {
		return plotLabelNativeDate{}, errors.Join(err, errors.New("native label Long Date pattern is unavailable"))
	}
	patternPointer, err := windows.UTF16PtrFromString(pattern)
	if err != nil {
		return plotLabelNativeDate{}, err
	}
	stamp := windows.Systemtime{Year: uint16(date.Year()), Month: uint16(date.Month()), Day: uint16(date.Day())}
	size, err := api.date(localePointer, &stamp, patternPointer, nil)
	if err != nil {
		return plotLabelNativeDate{}, err
	}
	if size == 0 || size > 4096 {
		return plotLabelNativeDate{}, errors.New("native label Long Date exceeds the supported output capacity")
	}
	buffer := make([]uint16, size)
	count, err = api.date(localePointer, &stamp, patternPointer, buffer)
	if err != nil {
		return plotLabelNativeDate{}, err
	}
	text, err := plotLabelWindowsText(buffer, count)
	if err != nil || text == "" {
		return plotLabelNativeDate{}, errors.Join(err, errors.New("native label Long Date output is unavailable"))
	}
	return plotLabelNativeDate{text, locale, pattern}, nil
}
