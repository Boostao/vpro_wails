package main

import (
	"context"
	"encoding/json"
	"errors"
)

const pictureMetadataWritingEnvironment = "VPRO_PICTURE_METADATA_WRITING"

type PictureMetadataWriteResult = pictureMetadataWriteResult

type PictureMetadataService struct {
	pictures *PictureService
	enabled  bool
}

func NewPictureMetadataService(pictures *PictureService, lookup func(string) (string, bool)) (*PictureMetadataService, error) {
	if pictures == nil || pictures.contexts == nil || lookup == nil {
		return nil, errors.New("picture metadata service requires explicit picture ownership and feature lookup")
	}
	enabled, err := siviFeature(pictureMetadataWritingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	if enabled && (!pictures.enabled || pictures.source == nil) {
		return nil, errors.New("picture metadata writing requires independently enabled owned picture reading")
	}
	return &PictureMetadataService{pictures: pictures, enabled: enabled}, nil
}

func (s *PictureMetadataService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("picture metadata writing requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled || s.pictures == nil {
		return errors.New("picture metadata writing is disabled in this session")
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
	result, err := s.pictures.contexts.editPictureMetadata(ctx, contextID, plot, s.pictures.source, request)
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
	return s.pictures.contexts.lookupPictureMetadataReceipt(ctx, contextID, plot, s.pictures.source, request)
}
