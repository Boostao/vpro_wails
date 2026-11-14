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
	"strings"
)

const (
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
	source      *ownedPictureLibrary
	directories pictureImageDirectories
	Metadata    *PictureMetadataService `json:"-" yaml:"-"`
	Creation    *PictureCreationService `json:"-" yaml:"-"`
	Deletion    *PictureDeletionService `json:"-" yaml:"-"`
}

func configuredPictureOptions(contexts *vprocore.ContextService) (libraryPath, childDir, managerDir string) {
	if contexts == nil || contexts.Projects == nil || contexts.Projects.Preferences == nil {
		return "", "", ""
	}
	values, err := contexts.Projects.Preferences.Snapshot()
	if err != nil {
		return "", "", ""
	}
	section, err := vprocore.ConfigSection(values, "PictureOptions")
	if err != nil {
		return "", "", ""
	}
	if raw, ok := section["PictureLibraryPath"].(string); ok {
		libraryPath = strings.TrimSpace(raw)
	}
	if raw, ok := section["ChildDirectory"].(string); ok {
		childDir = strings.TrimSpace(raw)
	}
	if raw, ok := section["ManagerDirectory"].(string); ok {
		managerDir = strings.TrimSpace(raw)
	}
	return libraryPath, childDir, managerDir
}

func NewPictureService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*PictureService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("picture service requires context ownership and an explicit environment lookup")
	}
	service := &PictureService{Contexts: contexts}
	cfgLibPath, cfgChildDir, cfgManagerDir := configuredPictureOptions(contexts)
	path, libExplicit := lookup(pictureLibraryEnvironment)
	if !libExplicit {
		if contexts.Projects == nil || strings.TrimSpace(contexts.Projects.Root) == "" {
			return nil, errors.New("picture reading requires VPRO_PICTURE_LIBRARY_PATH; no installed library was inferred")
		}
		if cfgLibPath != "" {
			path = cfgLibPath
		} else {
			installedPath, installErr := vprocore.InstallVPicsDatabase(contexts.Projects.Root)
			if installErr != nil {
				return nil, fmt.Errorf("install canonical VPics.db: %w", installErr)
			}
			path = installedPath
		}
	} else if path == "" {
		return nil, errors.New("picture reading requires VPRO_PICTURE_LIBRARY_PATH; no installed library was inferred")
	}
	var err error
	service.source, err = newOwnedPictureLibrary(path)
	if err != nil {
		return nil, err
	}
	defaultPicDir := ""
	if !libExplicit && contexts.Projects != nil && strings.TrimSpace(contexts.Projects.Root) != "" {
		defaultPicDir, err = vprocore.EnsurePictureDirectory(contexts.Projects.Root)
		if err != nil {
			return nil, fmt.Errorf("ensure default picture directory: %w", err)
		}
	}
	for _, entry := range []struct {
		name       string
		cfgPath    string
		defaultDir string
		target     **ownedPictureDirectory
	}{
		{pictureChildDirectoryEnvironment, cfgChildDir, defaultPicDir, &service.directories.child},
		{pictureManagerDirectoryEnvironment, cfgManagerDir, defaultPicDir, &service.directories.manager},
	} {
		dirPath, present := lookup(entry.name)
		if !present {
			if !libExplicit {
				if entry.cfgPath != "" {
					dirPath = entry.cfgPath
				} else if entry.defaultDir != "" {
					dirPath = entry.defaultDir
				}
			}
			if dirPath == "" {
				continue
			}
		} else if dirPath == "" {
			return nil, fmt.Errorf("%s must be an explicit directory when configured", entry.name)
		}
		directory, err := newOwnedPictureDirectory(dirPath)
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
	if s == nil || s.Contexts == nil || s.source == nil {
		return errors.New("picture service requires owned picture library")
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
