package pictures

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	PictureReadingEnvironment          = "VPRO_PICTURE_READING"
	pictureLibraryEnvironment          = "VPRO_PICTURE_LIBRARY_PATH"
	pictureChildDirectoryEnvironment   = "VPRO_PICTURE_CHILD_DIRECTORY"
	pictureManagerDirectoryEnvironment = "VPRO_PICTURE_MANAGER_DIRECTORY"
)

type PictureMetadataReview = pictureMetadataReview
type PictureImage = pictureImage

type PictureImageRequest struct {
	View     string                      `json:"view"`
	Original vprocore.ProjectMetadataRow `json:"original"`
}

func (request *PictureImageRequest) UnmarshalJSON(data []byte) error {
	const operation = "picture preview"
	properties, err := sourceform.SourceChildJSONObject(data, operation, []string{"view", "original"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, operation, "view", "original"); err != nil {
		return err
	}
	if err := sourceform.SourceChildRowJSON(properties["original"], operation); err != nil {
		return err
	}
	type plain PictureImageRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, operation, "view", "original"); err != nil {
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
	Contexts    *vprocore.ContextService `json:"-" yaml:"-"`
	Enabled     bool                     `json:"-" yaml:"-"`
	source      *ownedPictureLibrary
	directories pictureImageDirectories
	Metadata    *PictureMetadataService `json:"-" yaml:"-"`
	Creation    *PictureCreationService `json:"-" yaml:"-"`
	Deletion    *PictureDeletionService `json:"-" yaml:"-"`
}

func NewPictureService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*PictureService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("picture service requires context ownership and an explicit feature lookup")
	}
	enabled, err := vprocore.SiviFeature(PictureReadingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	service := &PictureService{Contexts: contexts, Enabled: enabled}
	if !enabled {
		return service, nil
	}
	path, present := lookup(pictureLibraryEnvironment)
	if !present {
		if _, readingExplicit := lookup(PictureReadingEnvironment); readingExplicit {
			return nil, errors.New("picture reading requires VPRO_PICTURE_LIBRARY_PATH; no installed library was inferred")
		}
		service.Enabled = false
		return service, nil
	}
	if path == "" {
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
	if s == nil || s.Contexts == nil || !s.Enabled {
		return errors.New("picture reading is disabled in this session")
	}
	return nil
}

func (s *PictureService) GetMetadata(ctx context.Context, contextID, plot string) (*PictureMetadataReview, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	result, err := ReadPictureMetadata(s.Contexts, ctx, contextID, plot, s.source)
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
	result, err := ReadPictureImage(s.Contexts, ctx, contextID, plot, s.source, s.directories, request.View, request.Original)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
