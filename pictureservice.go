package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	pictureReadingEnvironment          = "VPRO_PICTURE_READING"
	pictureLibraryEnvironment          = "VPRO_PICTURE_LIBRARY_PATH"
	pictureChildDirectoryEnvironment   = "VPRO_PICTURE_CHILD_DIRECTORY"
	pictureManagerDirectoryEnvironment = "VPRO_PICTURE_MANAGER_DIRECTORY"
)

type PictureMetadataReview = pictureMetadataReview
type PictureImage = pictureImage

type PictureImageRequest struct {
	View     string             `json:"view"`
	Original ProjectMetadataRow `json:"original"`
}

func (request *PictureImageRequest) UnmarshalJSON(data []byte) error {
	const operation = "picture preview"
	properties, err := sourceChildJSONObject(data, operation, []string{"view", "original"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, "view", "original"); err != nil {
		return err
	}
	if err := sourceChildRowJSON(properties["original"], operation); err != nil {
		return err
	}
	type plain PictureImageRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, operation, "view", "original"); err != nil {
		return err
	}
	if decoded.View != pictureChildView && decoded.View != pictureManagerView {
		return errors.New("picture preview requires a literal child or manager policy")
	}
	if decoded.Original.RowID == "" || len(decoded.Original.Cells) != 5 {
		return errors.New("picture preview requires one complete reviewed five-column physical row")
	}
	*request = PictureImageRequest(decoded)
	return nil
}

type PictureService struct {
	contexts    *ContextService
	enabled     bool
	source      *ownedPictureLibrary
	directories pictureImageDirectories
}

func NewPictureService(contexts *ContextService, lookup func(string) (string, bool)) (*PictureService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("picture service requires context ownership and an explicit feature lookup")
	}
	enabled, err := siviFeature(pictureReadingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	service := &PictureService{contexts: contexts, enabled: enabled}
	if !enabled {
		return service, nil
	}
	path, present := lookup(pictureLibraryEnvironment)
	if !present || path == "" {
		return nil, errors.New("picture reading requires VPRO_PICTURE_LIBRARY_PATH; no installed library was inferred")
	}
	service.source, err = newOwnedPictureLibrary(path)
	if err != nil {
		return nil, err
	}
	for _, entry := range []struct {
		name   string
		target **ownedPictureDirectory
	}{
		{pictureChildDirectoryEnvironment, &service.directories.child},
		{pictureManagerDirectoryEnvironment, &service.directories.manager},
	} {
		path, present := lookup(entry.name)
		if !present {
			continue
		}
		if path == "" {
			return nil, fmt.Errorf("%s must be an explicit directory when configured", entry.name)
		}
		directory, err := newOwnedPictureDirectory(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.name, err)
		}
		*entry.target = directory
	}
	return service, nil
}

func (s *PictureService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("picture service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled {
		return errors.New("picture reading is disabled in this session")
	}
	return nil
}

func (s *PictureService) GetMetadata(ctx context.Context, contextID, plot string) (*PictureMetadataReview, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	result, err := s.contexts.readPictureMetadata(ctx, contextID, plot, s.source)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *PictureService) GetImage(ctx context.Context, contextID, plot, requestJSON string) (*PictureImage, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request PictureImageRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	result, err := s.contexts.readPictureImage(ctx, contextID, plot, s.source, s.directories, request.View, request.Original)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
