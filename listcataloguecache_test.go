package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func readonlyCatalogueFixture(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := openReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func TestCatalogueCacheWarmAllListsAvoidHashesScansAndCloneMetadata(t *testing.T) {
	service, err := NewParentCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	cache := service.cache
	checksums, validations, bytesRead := cache.checksums, cache.validations, cache.bytesRead
	if checksums != 1 || validations != 1 || bytesRead != int64(len(parentCodeDatabase)) {
		t.Fatal("mount did not perform exactly one full verification")
	}
	for i := 0; i < 100; i++ {
		for _, definition := range listcatalog.ParentProfile().Lists {
			if _, supported := parentCodeListMaximum(definition.Name); !supported {
				continue
			}
			rows, err := service.ListChoices(definition.Name)
			if err != nil || len(rows) != definition.Rows {
				t.Fatalf("warm %s: %d %v", definition.Name, len(rows), err)
			}
			expected, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.Code != nil {
					*row.Code = "modified caller copy"
				}
				if row.ItemOrder != nil {
					*row.ItemOrder = -999
				}
				if row.Flag != nil {
					*row.Flag = !*row.Flag
				}
			}
			again, err := service.ListChoices(definition.Name)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(again)
			if err != nil || !bytes.Equal(actual, expected) {
				t.Fatal("caller mutation changed verified metadata", err)
			}
		}
	}
	if cache.checksums != checksums || cache.validations != validations || cache.bytesRead != bytesRead {
		t.Fatal("warm unchanged lookups hashed or scanned the catalogue again")
	}
}

func TestCatalogueCacheReplacementMissingFailureRestorationAndForcedRetry(t *testing.T) {
	service, err := NewGeologyCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	original, err := service.ListBedrockChoices()
	if err != nil {
		t.Fatal(err)
	}
	checksums := service.cache.checksums
	replacement := filepath.Join(filepath.Dir(service.path), "replacement.db")
	if err := os.WriteFile(replacement, geologyCodeDatabase, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, service.path); err != nil {
		t.Fatal("idle catalogue handle prevented replacement", err)
	}
	rows, err := service.ListBedrockChoices()
	if err != nil || !reflect.DeepEqual(rows, original) || service.cache.checksums != checksums+1 {
		t.Fatal("replacement was not reverified", err)
	}
	if err := os.Remove(service.path); err != nil {
		t.Fatal("idle catalogue handle prevented removal", err)
	}
	if rows, err := service.ListBedrockChoices(); err == nil || rows != nil {
		t.Fatal("missing catalogue returned stale choices")
	}
	if err := os.WriteFile(service.path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if rows, err := service.ListBedrockChoices(); err == nil || rows != nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatal("failed verification returned stale choices", err)
		}
	}
	if err := os.WriteFile(service.path, geologyCodeDatabase, 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.ReloadCatalogue(); err != nil {
		t.Fatal("restoration retry failed", err)
	}
	rows, err = service.ListBedrockChoices()
	if err != nil || !reflect.DeepEqual(rows, original) {
		t.Fatal("restored metadata changed", err)
	}
	checksums = service.cache.checksums
	if err := service.ReloadCatalogue(); err != nil || service.cache.checksums != checksums+1 {
		t.Fatal("explicit Retry did not force verification", err)
	}
}

func TestCatalogueCacheIndistinguishableMetadataNeverReadsUnverifiedRows(t *testing.T) {
	service, err := NewSoilCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	expected, err := service.ListGreatGroupChoices()
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), soilCodeDatabase...)
	data[0] ^= 1
	if err := os.WriteFile(service.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	// Simulate a host capable of preserving every observed metadata component.
	service.cache.stamp, err = catalogueFileStamp(service.path)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := service.ListGreatGroupChoices()
	if err != nil || !reflect.DeepEqual(expected, actual) {
		t.Fatal("warm lookup queried unverified disk data", err)
	}
	if err := service.ReloadCatalogue(); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal("forced verification hid indistinguishable-metadata corruption", err)
	}
	if rows, err := service.ListGreatGroupChoices(); err == nil || rows != nil {
		t.Fatal("failed Retry returned a stale successful snapshot")
	}
}

func TestCatalogueCacheConcurrentLookupReloadAndClose(t *testing.T) {
	service, err := NewRegionCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	errs := make(chan error, 20)
	var warm sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < cap(errs); i++ {
		wait.Add(1)
		warm.Add(1)
		go func() {
			defer wait.Done()
			if _, err := service.ListRegionChoices(); err != nil {
				warm.Done()
				errs <- err
				return
			}
			if err := service.ReloadCatalogue(); err != nil {
				warm.Done()
				errs <- err
				return
			}
			warm.Done()
			<-start
			for i := 0; i < 20; i++ {
				_, err := service.ListRegionChoices()
				if err != nil && !strings.Contains(err.Error(), "closed") {
					errs <- err
					return
				}
				if err := service.ReloadCatalogue(); err != nil && !strings.Contains(err.Error(), "closed") {
					errs <- err
					return
				}
			}
			errs <- nil
		}()
	}
	warm.Wait()
	close(start)
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := service.ReloadCatalogue(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("closed cache reloaded")
	}
}

func BenchmarkCatalogueCacheParentChoices(b *testing.B) {
	service, err := NewParentCodeService(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer service.Close()
	for _, force := range []bool{false, true} {
		name := "warm-verified-snapshot"
		if force {
			name = "forced-file-and-profile-verification"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if force {
					if err := service.ReloadCatalogue(); err != nil {
						b.Fatal(err)
					}
				}
				if _, err := service.ListChoices("HumusForm"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func restoreCatalogueModTime(t *testing.T, path string, original time.Time) {
	t.Helper()
	if err := os.Chtimes(path, original, original); err != nil {
		t.Fatal(err)
	}
}
