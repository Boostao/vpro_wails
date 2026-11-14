package pictures

import (
	"context"
	"encoding/json"
	"errors"
)

type PictureMetadataWriteResult = pictureMetadataWriteResult

type PictureMetadataService struct {
	pictures *PictureService
}

func NewPictureMetadataService(pictures *PictureService) (*PictureMetadataService, error) {
	if pictures == nil || pictures.Contexts == nil || pictures.source == nil {
		return nil, errors.New("picture metadata service requires explicit picture ownership")
	}
	return &PictureMetadataService{pictures: pictures}, nil
}

func (s *PictureMetadataService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("picture metadata writing requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.pictures == nil {
		return errors.New("picture metadata writing requires picture ownership")
	}
	return s.pictures.require(ctx)
}

func (s *PictureMetadataService) Save(ctx context.Context, contextID, plot, requestJSON string) (*PictureMetadataWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request pictureMetadataEdit
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	result, err := EditPictureMetadata(s.pictures.Contexts, ctx, contextID, plot, s.pictures.source, request)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *PictureMetadataService) LookupReceipt(ctx context.Context, contextID, plot, requestJSON string) (*PictureMetadataWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request pictureMetadataEdit
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupPictureMetadataReceipt(s.pictures.Contexts, ctx, contextID, plot, s.pictures.source, request)
}
