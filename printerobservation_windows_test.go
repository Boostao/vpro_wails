package main

import (
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestPrinterObservationWindowsBufferBoundsUTF16RetryAndNoDefault(t *testing.T) {
	for _, units := range [][]uint16{{}, {0}, {'a'}, {0xd800, 0}, {0xdc00, 0}, {0xd800, 'a', 0}} {
		if _, err := printerUTF16(units); err == nil {
			t.Fatal("invalid native name repaired", units)
		}
	}
	if got, err := printerUTF16([]uint16{' ', 0xd83d, 0xde00, ' ', 0}); err != nil || got != " \U0001f600 " {
		t.Fatal(got, err)
	}
	enumerations, defaults := 0, 0
	api := printerReadAPI{
		enumerate: func(buffer []byte) (uint32, uint32, error) {
			enumerations++
			if len(buffer) == 0 {
				return 256, 0, syscall.ERROR_INSUFFICIENT_BUFFER
			}
			width := int(unsafe.Sizeof(uintptr(0)))
			base := uintptr(unsafe.Pointer(&buffer[0]))
			if width == 8 {
				binary.LittleEndian.PutUint64(buffer, uint64(base+64))
			} else {
				binary.LittleEndian.PutUint32(buffer, uint32(base+64))
			}
			for i, u := range append(utf16.Encode([]rune(" literal \u00e9 ")), 0) {
				binary.LittleEndian.PutUint16(buffer[64+i*2:], u)
			}
			return 256, 1, nil
		},
		def: func(buffer []uint16) (uint32, error) {
			defaults++
			if len(buffer) == 0 {
				return 5, syscall.ERROR_INSUFFICIENT_BUFFER
			}
			copy(buffer, []uint16{'e', 'l', 's', 'e', 0})
			return 5, nil
		},
	}
	got, err := observePrintersWithAPI(context.Background(), api)
	if err != nil || !reflect.DeepEqual(got.Names, []string{" literal \u00e9 "}) || got.Default == nil || *got.Default != "else" || enumerations != 2 || defaults != 2 {
		t.Fatal(got, err, enumerations, defaults)
	}
	api.enumerate = func([]byte) (uint32, uint32, error) { return 0, 0, nil }
	api.def = func([]uint16) (uint32, error) { return 0, syscall.ERROR_FILE_NOT_FOUND }
	got, err = observePrintersWithAPI(context.Background(), api)
	if err != nil || got.Names == nil || len(got.Names) != 0 || got.Default != nil {
		t.Fatal("explicit no installed/default printers lost", got, err)
	}
	for _, buffer := range [][]byte{nil, make([]byte, 256)} {
		if _, err := printerBufferNames(context.Background(), buffer, 1); err == nil {
			t.Fatal("truncated/out-of-owned-buffer native name accepted")
		}
	}
}

func TestPrinterObservationWindowsErrorsCancellationAndCapacity(t *testing.T) {
	success := printerReadAPI{
		enumerate: func([]byte) (uint32, uint32, error) { return 0, 0, nil },
		def:       func([]uint16) (uint32, error) { return 0, syscall.ERROR_FILE_NOT_FOUND },
	}
	for _, api := range []printerReadAPI{
		{},
		{enumerate: func([]byte) (uint32, uint32, error) { return 0, 0, syscall.ERROR_ACCESS_DENIED }, def: success.def},
		{enumerate: func([]byte) (uint32, uint32, error) { return 17 * 1024 * 1024, 0, syscall.ERROR_INSUFFICIENT_BUFFER }, def: success.def},
		{enumerate: func([]byte) (uint32, uint32, error) { return 2, 0, syscall.ERROR_INSUFFICIENT_BUFFER }, def: success.def},
		{enumerate: success.enumerate, def: func([]uint16) (uint32, error) { return 0, syscall.ERROR_ACCESS_DENIED }},
		{enumerate: success.enumerate, def: func([]uint16) (uint32, error) { return 65537, syscall.ERROR_INSUFFICIENT_BUFFER }},
		{enumerate: success.enumerate, def: func([]uint16) (uint32, error) { return 3, syscall.ERROR_INSUFFICIENT_BUFFER }},
		{enumerate: success.enumerate, def: func([]uint16) (uint32, error) { return 0, nil }},
	} {
		if got, err := observePrintersWithAPI(context.Background(), api); err == nil || got.Names != nil || got.Default != nil {
			t.Fatal("failed host reads returned success-shaped observation", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	api := success
	api.enumerate = func([]byte) (uint32, uint32, error) { cancel(); return 0, 0, nil }
	if _, err := observePrintersWithAPI(ctx, api); !errors.Is(err, context.Canceled) {
		t.Fatal("inflight cancellation lost", err)
	}
	if _, err := observePrintersWithAPI(nil, success); err == nil {
		t.Fatal("nil context accepted")
	}
}
