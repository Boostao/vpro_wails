package main

import (
	"context"
	"errors"

	picturespkg "github.com/boostao/vpro-wails/internal/pictures"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type PictureService struct {
	*picturespkg.PictureService
}

func unwrapPictureService(s *PictureService) *picturespkg.PictureService {
	if s == nil {
		return nil
	}
	return s.PictureService
}

func newDefaultPictureEnvLookup(base func(string) (string, bool), defaultVPicsPath, defaultImagesDir string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		if base != nil {
			if val, ok := base(key); ok {
				return val, true
			}
		}
		switch key {
		case "VPRO_PICTURE_LIBRARY_PATH":
			if defaultVPicsPath != "" {
				return defaultVPicsPath, true
			}
		case "VPRO_PICTURE_CHILD_DIRECTORY", "VPRO_PICTURE_MANAGER_DIRECTORY":
			if defaultImagesDir != "" {
				return defaultImagesDir, true
			}
		}
		return "", false
	}
}

func NewPictureService(contexts *ContextService, lookup func(string) (string, bool)) (*PictureService, error) {
	core, err := picturespkg.NewPictureService(unwrapContextService(contexts), lookup)
	if err != nil {
		return nil, err
	}
	return &PictureService{PictureService: core}, nil
}

func NewPictureMetadataService(s *PictureService) (*picturespkg.PictureMetadataService, error) {
	return picturespkg.NewPictureMetadataService(unwrapPictureService(s))
}

func NewPictureCreationService(s *PictureService) (*picturespkg.PictureCreationService, error) {
	return picturespkg.NewPictureCreationService(unwrapPictureService(s))
}

func NewPictureDeletionService(s *PictureService) (*picturespkg.PictureDeletionService, error) {
	return picturespkg.NewPictureDeletionService(unwrapPictureService(s))
}

func buildPictureService(contexts *ContextService, dataDir string, baseLookup func(string) (string, bool)) (*PictureService, error) {
	defaultVPicsPath, defaultImagesDir, err := vprocorepkg.EnsureDefaultPictureLibrary(dataDir)
	if err != nil {
		return nil, err
	}
	lookup := newDefaultPictureEnvLookup(baseLookup, defaultVPicsPath, defaultImagesDir)
	svc, err := NewPictureService(contexts, lookup)
	if err != nil {
		return nil, err
	}
	metadata, err := NewPictureMetadataService(svc)
	if err != nil {
		return nil, err
	}
	creation, err := NewPictureCreationService(svc)
	if err != nil {
		return nil, err
	}
	deletion, err := NewPictureDeletionService(svc)
	if err != nil {
		return nil, err
	}
	svc.Metadata = metadata
	svc.Creation = creation
	svc.Deletion = deletion
	return svc, nil
}

func (s *PictureService) GetMetadata(ctx context.Context, contextID, plot string) (*PictureMetadataReview, error) {
	if s == nil || s.PictureService == nil {
		return nil, errors.New("picture service is unavailable")
	}
	return s.PictureService.GetMetadata(ctx, contextID, plot)
}

func (s *PictureService) GetImage(ctx context.Context, contextID, plot, requestJSON string) (*PictureImage, error) {
	if s == nil || s.PictureService == nil {
		return nil, errors.New("picture service is unavailable")
	}
	return s.PictureService.GetImage(ctx, contextID, plot, requestJSON)
}

func (s *PictureService) SaveMetadata(ctx context.Context, contextID, plot, requestJSON string) (*PictureMetadataWriteResult, error) {
	if s == nil || s.PictureService == nil || s.Metadata == nil {
		return nil, errors.New("picture metadata service is unavailable")
	}
	return s.Metadata.Save(ctx, contextID, plot, requestJSON)
}

func (s *PictureService) LookupMetadataReceipt(ctx context.Context, contextID, plot, requestJSON string) (*PictureMetadataWriteResult, error) {
	if s == nil || s.PictureService == nil || s.Metadata == nil {
		return nil, errors.New("picture metadata service is unavailable")
	}
	return s.Metadata.LookupReceipt(ctx, contextID, plot, requestJSON)
}

func (s *PictureService) ChooseCreationImage(ctx context.Context, contextID, project, plot string) (*PictureCreationSelection, error) {
	if s == nil || s.PictureService == nil || s.Creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.Creation.Choose(ctx, contextID, project, plot)
}

func (s *PictureService) GetSelectedCreationImage(ctx context.Context, token, requestJSON string) (*PictureImage, error) {
	if s == nil || s.PictureService == nil || s.Creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.Creation.GetSelectedImage(ctx, token, requestJSON)
}

func (s *PictureService) RecoverCreationSelection(ctx context.Context, contextID, project, plot string) (*PictureCreationSelection, error) {
	if s == nil || s.PictureService == nil || s.Creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.Creation.RecoverSelection(ctx, contextID, project, plot)
}

func (s *PictureService) ReleaseCreationSelection(ctx context.Context, token string) error {
	if s == nil || s.PictureService == nil || s.Creation == nil {
		return errors.New("picture creation service is unavailable")
	}
	return s.Creation.ReleaseSelection(ctx, token)
}

func (s *PictureService) CreatePicture(ctx context.Context, token, requestJSON string) (*PictureCreationResult, error) {
	if s == nil || s.PictureService == nil || s.Creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.Creation.Create(ctx, token, requestJSON)
}

func (s *PictureService) LookupCreationReceipt(ctx context.Context, requestJSON string) (*PictureCreationResult, error) {
	if s == nil || s.PictureService == nil || s.Creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.Creation.LookupReceipt(ctx, requestJSON)
}

func (s *PictureService) AcknowledgeCreationReceipt(ctx context.Context, token string) error {
	if s == nil || s.PictureService == nil || s.Creation == nil {
		return errors.New("picture creation service is unavailable")
	}
	return s.Creation.AcknowledgeReceipt(ctx, token)
}

func (s *PictureService) DeletePicture(ctx context.Context, contextID, requestJSON string) (*PictureDeletionResult, error) {
	if s == nil || s.PictureService == nil || s.Deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.Deletion.Delete(ctx, contextID, requestJSON)
}

func (s *PictureService) GetDeletionHistory(ctx context.Context, contextID, plot string) (*PictureDeletionHistoryList, error) {
	if s == nil || s.PictureService == nil || s.Deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.Deletion.GetHistory(ctx, contextID, plot)
}

func (s *PictureService) LookupDeletionReceipt(ctx context.Context, contextID, requestJSON string) (*PictureDeletionResult, error) {
	if s == nil || s.PictureService == nil || s.Deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.Deletion.LookupReceipt(ctx, contextID, requestJSON)
}

func (s *PictureService) LookupDeletionRestorationReceipt(ctx context.Context, contextID, requestJSON string) (*PictureRestorationResult, error) {
	if s == nil || s.PictureService == nil || s.Deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.Deletion.LookupRestorationReceipt(ctx, contextID, requestJSON)
}

func (s *PictureService) ReviewDeletionRestoration(ctx context.Context, contextID, plot, historyID string) (*PictureRestorationReview, error) {
	if s == nil || s.PictureService == nil || s.Deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.Deletion.ReviewRestoration(ctx, contextID, plot, historyID)
}

func (s *PictureService) RestoreDeletedPicture(ctx context.Context, contextID, requestJSON string) (*PictureRestorationResult, error) {
	if s == nil || s.PictureService == nil || s.Deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.Deletion.Restore(ctx, contextID, requestJSON)
}
