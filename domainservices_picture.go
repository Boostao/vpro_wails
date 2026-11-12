package main

import (
	"context"
	"errors"
)

func (s *PictureService) SaveMetadata(ctx context.Context, contextID, plot, requestJSON string) (*PictureMetadataWriteResult, error) {
	if s == nil || s.metadata == nil {
		return nil, errors.New("picture metadata service is unavailable")
	}
	return s.metadata.Save(ctx, contextID, plot, requestJSON)
}

func (s *PictureService) LookupMetadataReceipt(ctx context.Context, contextID, plot, requestJSON string) (*PictureMetadataWriteResult, error) {
	if s == nil || s.metadata == nil {
		return nil, errors.New("picture metadata service is unavailable")
	}
	return s.metadata.LookupReceipt(ctx, contextID, plot, requestJSON)
}

func (s *PictureService) ChooseCreationImage(ctx context.Context, contextID, project, plot string) (*PictureCreationSelection, error) {
	if s == nil || s.creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.creation.Choose(ctx, contextID, project, plot)
}

func (s *PictureService) GetSelectedCreationImage(ctx context.Context, token, requestJSON string) (*PictureImage, error) {
	if s == nil || s.creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.creation.GetSelectedImage(ctx, token, requestJSON)
}

func (s *PictureService) RecoverCreationSelection(ctx context.Context, contextID, project, plot string) (*PictureCreationSelection, error) {
	if s == nil || s.creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.creation.RecoverSelection(ctx, contextID, project, plot)
}

func (s *PictureService) ReleaseCreationSelection(ctx context.Context, token string) error {
	if s == nil || s.creation == nil {
		return errors.New("picture creation service is unavailable")
	}
	return s.creation.ReleaseSelection(ctx, token)
}

func (s *PictureService) CreatePicture(ctx context.Context, token, requestJSON string) (*PictureCreationResult, error) {
	if s == nil || s.creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.creation.Create(ctx, token, requestJSON)
}

func (s *PictureService) LookupCreationReceipt(ctx context.Context, requestJSON string) (*PictureCreationResult, error) {
	if s == nil || s.creation == nil {
		return nil, errors.New("picture creation service is unavailable")
	}
	return s.creation.LookupReceipt(ctx, requestJSON)
}

func (s *PictureService) AcknowledgeCreationReceipt(ctx context.Context, token string) error {
	if s == nil || s.creation == nil {
		return errors.New("picture creation service is unavailable")
	}
	return s.creation.AcknowledgeReceipt(ctx, token)
}

func (s *PictureService) DeletePicture(ctx context.Context, contextID, requestJSON string) (*PictureDeletionResult, error) {
	if s == nil || s.deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.deletion.Delete(ctx, contextID, requestJSON)
}

func (s *PictureService) GetDeletionHistory(ctx context.Context, contextID, plot string) (*PictureDeletionHistoryList, error) {
	if s == nil || s.deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.deletion.GetHistory(ctx, contextID, plot)
}

func (s *PictureService) LookupDeletionReceipt(ctx context.Context, contextID, requestJSON string) (*PictureDeletionResult, error) {
	if s == nil || s.deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.deletion.LookupReceipt(ctx, contextID, requestJSON)
}

func (s *PictureService) LookupDeletionRestorationReceipt(ctx context.Context, contextID, requestJSON string) (*PictureRestorationResult, error) {
	if s == nil || s.deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.deletion.LookupRestorationReceipt(ctx, contextID, requestJSON)
}

func (s *PictureService) ReviewDeletionRestoration(ctx context.Context, contextID, plot, historyID string) (*PictureRestorationReview, error) {
	if s == nil || s.deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.deletion.ReviewRestoration(ctx, contextID, plot, historyID)
}

func (s *PictureService) RestoreDeletedPicture(ctx context.Context, contextID, requestJSON string) (*PictureRestorationResult, error) {
	if s == nil || s.deletion == nil {
		return nil, errors.New("picture deletion service is unavailable")
	}
	return s.deletion.Restore(ctx, contextID, requestJSON)
}
