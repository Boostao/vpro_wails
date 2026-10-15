package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var printerSpool = syscall.NewLazyDLL("winspool.drv")
var printerEnum = printerSpool.NewProc("EnumPrintersW")
var printerDefault = printerSpool.NewProc("GetDefaultPrinterW")

type printerReadAPI struct {
	enumerate func([]byte) (uint32, uint32, error)
	def       func([]uint16) (uint32, error)
}

func nativePrinterReadAPI() printerReadAPI {
	return printerReadAPI{
		enumerate: func(buffer []byte) (uint32, uint32, error) {
			var pointer uintptr
			if len(buffer) != 0 {
				pointer = uintptr(unsafe.Pointer(&buffer[0]))
			}
			var needed, count uint32
			ok, _, err := printerEnum.Call(6, 0, 4, pointer, uintptr(len(buffer)),
				uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
			runtime.KeepAlive(buffer)
			if ok == 0 {
				return needed, count, fmt.Errorf("EnumPrintersW: %w", err)
			}
			return needed, count, nil
		},
		def: func(buffer []uint16) (uint32, error) {
			var pointer uintptr
			if len(buffer) != 0 {
				pointer = uintptr(unsafe.Pointer(&buffer[0]))
			}
			count := uint32(len(buffer))
			ok, _, err := printerDefault.Call(pointer, uintptr(unsafe.Pointer(&count)))
			runtime.KeepAlive(buffer)
			if ok == 0 {
				return count, fmt.Errorf("GetDefaultPrinterW: %w", err)
			}
			return count, nil
		},
	}
}

func printerUTF16(units []uint16) (string, error) {
	for i := 0; i < len(units); i++ {
		value := units[i]
		if value == 0 {
			if i == 0 {
				return "", errors.New("printer name is empty")
			}
			return string(utf16.Decode(units[:i])), nil
		}
		if value >= 0xd800 && value <= 0xdbff {
			i++
			if i >= len(units) || units[i] < 0xdc00 || units[i] > 0xdfff {
				return "", errors.New("printer name contains malformed UTF-16")
			}
		} else if value >= 0xdc00 && value <= 0xdfff {
			return "", errors.New("printer name contains malformed UTF-16")
		}
	}
	return "", errors.New("printer name is missing its terminator")
}

func printerBufferNames(ctx context.Context, buffer []byte, count uint32) ([]string, error) {
	result := []string{}
	width := int(unsafe.Sizeof(uintptr(0)))
	stride := width*2 + 4
	if stride%width != 0 {
		stride += width - stride%width
	}
	if uint64(count)*uint64(stride) > uint64(len(buffer)) {
		return nil, errors.New("installed printer result exceeds its native buffer")
	}
	if count == 0 {
		return result, nil
	}
	base := uintptr(unsafe.Pointer(&buffer[0]))
	for i := uint32(0); i < count; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry := buffer[int(i)*stride:]
		pointer := uintptr(binary.LittleEndian.Uint32(entry[:4]))
		if width == 8 {
			pointer = uintptr(binary.LittleEndian.Uint64(entry[:8]))
		}
		if pointer < base || pointer-base >= uintptr(len(buffer)) || (pointer-base)%2 != 0 {
			return nil, errors.New("installed printer name is outside its owned buffer")
		}
		offset := int(pointer - base)
		units := make([]uint16, 0, 64)
		for j := offset; j+1 < len(buffer); j += 2 {
			if (j-offset)%2048 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			unit := binary.LittleEndian.Uint16(buffer[j:])
			units = append(units, unit)
			if unit == 0 {
				break
			}
		}
		name, err := printerUTF16(units)
		if err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	runtime.KeepAlive(buffer)
	return result, nil
}

func observePrintersWithAPI(ctx context.Context, api printerReadAPI) (PrinterObservation, error) {
	fail := func(err error) (PrinterObservation, error) { return PrinterObservation{}, err }
	if ctx == nil || api.enumerate == nil || api.def == nil {
		return fail(errors.New("printer observation requires a context and explicit read-only host API"))
	}
	var buffer []byte
	var names []string
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		needed, count, err := api.enumerate(buffer)
		if err == nil {
			names, err = printerBufferNames(ctx, buffer, count)
			if err != nil {
				return fail(err)
			}
			break
		}
		if !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) || needed == 0 || needed > 16*1024*1024 || attempt >= 2 {
			return fail(fmt.Errorf("installed printers unavailable: %w", err))
		}
		buffer = make([]byte, needed)
	}
	var defaultName *string
	var units []uint16
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		needed, err := api.def(units)
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			break
		}
		if err == nil {
			if needed == 0 || uint64(needed) > uint64(len(units)) {
				return fail(errors.New("default printer result exceeds its native buffer"))
			}
			name, err := printerUTF16(units[:needed])
			if err != nil {
				return fail(err)
			}
			defaultName = &name
			break
		}
		if !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) || needed == 0 || needed > 65536 || attempt >= 2 {
			return fail(fmt.Errorf("default printer unavailable: %w", err))
		}
		units = make([]uint16, needed)
	}
	result := PrinterObservation{Names: names, Default: defaultName, Source: printerObservationSource}
	if err := validatePrinterObservation(ctx, result); err != nil {
		return fail(err)
	}
	return result, nil
}

func observePrinters(ctx context.Context) (PrinterObservation, error) {
	return observePrintersWithAPI(ctx, nativePrinterReadAPI())
}
