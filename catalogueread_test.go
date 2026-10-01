package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func catalogueReadFixture(t *testing.T) []struct {
	name   string
	mutex  *sync.RWMutex
	cache  *listCatalogueCache
	read   func(context.Context) error
	reload func(context.Context) error
} {
	t.Helper()
	parent, err := NewParentCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { parent.Close() })
	geology, err := NewGeologyCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { geology.Close() })
	soil, err := NewSoilCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { soil.Close() })
	region, err := NewRegionCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { region.Close() })
	site, err := NewSiteCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { site.Close() })
	return []struct {
		name   string
		mutex  *sync.RWMutex
		cache  *listCatalogueCache
		read   func(context.Context) error
		reload func(context.Context) error
	}{
		{"parent", &parent.mu, parent.cache, func(ctx context.Context) error {
			_, err := parent.ListChoices(ctx, "RealmClass")
			return err
		}, parent.ReloadCatalogue},
		{"geology", &geology.mu, geology.cache, func(ctx context.Context) error {
			_, err := geology.ListBedrockChoices(ctx)
			return err
		}, geology.ReloadCatalogue},
		{"soil", &soil.mu, soil.cache, func(ctx context.Context) error {
			_, err := soil.ListGreatGroupChoices(ctx)
			return err
		}, soil.ReloadCatalogue},
		{"region", &region.mu, region.cache, func(ctx context.Context) error {
			_, err := region.ListRegionChoices(ctx)
			return err
		}, region.ReloadCatalogue},
		{"site", &site.mu, site.cache, func(ctx context.Context) error {
			_, err := site.ListExposureChoices(ctx)
			return err
		}, site.ReloadCatalogue},
	}
}

func TestCatalogueReadsCancellationBeforeExecutionAndWhileQueued(t *testing.T) {
	for _, service := range catalogueReadFixture(t) {
		t.Run(service.name, func(t *testing.T) {
			checksums, scans := service.cache.checksums, service.cache.validations
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			for _, read := range []func(context.Context) error{service.read, service.reload} {
				if err := read(ctx); !errors.Is(err, context.Canceled) {
					t.Fatal("cancelled catalogue read executed", err)
				}
				service.mutex.Lock()
				waiting, stop := context.WithTimeout(context.Background(), 50*time.Millisecond)
				err := read(waiting)
				stop()
				service.mutex.Unlock()
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("queued catalogue read did not cancel", err)
				}
			}
			if !service.cache.valid || service.cache.checksums != checksums || service.cache.validations != scans {
				t.Fatal("cancelled queued read changed verified snapshot or scanned bytes")
			}
			if err := service.read(context.Background()); err != nil {
				t.Fatal("valid catalogue read did not recover", err)
			}
			if err := service.reload(context.Background()); err != nil {
				t.Fatal("valid forced verification did not recover", err)
			}
		})
	}
}
