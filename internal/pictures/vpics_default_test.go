package pictures

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestPictureServiceDefaultsToCanonicalVPicsWithoutEnvVars(t *testing.T) {
	contexts, state := vprotest.ContextServiceFixture(t)
	emptyLookup := func(string) (string, bool) { return "", false }

	if len(contexts.Projects.SupportPaths) != len(vprocore.DatabaseFamilySeeds) {
		t.Fatalf("SupportPaths len = %d, want %d", len(contexts.Projects.SupportPaths), len(vprocore.DatabaseFamilySeeds))
	}

	svc, err := NewPictureService(contexts, emptyLookup)
	if err != nil {
		t.Fatalf("NewPictureService with zero env vars failed: %v", err)
	}
	if !svc.Enabled || svc.source == nil || svc.directories.child == nil || svc.directories.manager == nil {
		t.Fatalf("expected PictureService + child/manager dirs enabled by default, got %+v", svc)
	}
	wantVPics := filepath.Join(contexts.Projects.Root, "database-family", "VPics.db")
	if info, err := os.Stat(wantVPics); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("expected canonical VPics.db at %s: %v", wantVPics, err)
	}

	metaSvc, err := NewPictureMetadataService(svc, emptyLookup)
	if err != nil || !metaSvc.enabled {
		t.Fatalf("expected PictureMetadataService enabled by default: %+v (%v)", metaSvc, err)
	}
	createSvc, err := NewPictureCreationService(svc, emptyLookup)
	if err != nil || !createSvc.enabled {
		t.Fatalf("expected PictureCreationService enabled by default: %+v (%v)", createSvc, err)
	}
	delSvc, err := NewPictureDeletionService(svc, emptyLookup)
	if err != nil || !delSvc.writing || !delSvc.restoring {
		t.Fatalf("expected PictureDeletionService enabled by default: %+v (%v)", delSvc, err)
	}

	review, err := svc.GetMetadata(context.Background(), state.ContextID, "00337")
	if err != nil || review == nil || len(review.Records.Columns) != 5 {
		t.Fatalf("GetMetadata against canonical VPics.db failed: %+v (%v)", review, err)
	}
}
