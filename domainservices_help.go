package main

import (
	"context"
	"errors"

	lifecyclepkg "github.com/boostao/vpro-wails/internal/lifecycle"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type HelpService struct {
	core *lifecyclepkg.HelpService
}

func NewHelpService(preferences *vprocorepkg.DesktopConfig) (*HelpService, error) {
	core, err := lifecyclepkg.NewHelpService(preferences)
	if err != nil {
		return nil, err
	}
	return &HelpService{core: core}, nil
}

func (s *HelpService) GetLibraryStamps(ctx context.Context) (LibraryStamps, error) {
	if s == nil || s.core == nil {
		return LibraryStamps{}, errors.New("help service is unavailable")
	}
	return s.core.GetLibraryStamps(ctx)
}

func (s *HelpService) GetReleaseChannel(ctx context.Context) (string, error) {
	if s == nil || s.core == nil {
		return "", errors.New("help service is unavailable")
	}
	return s.core.GetReleaseChannel(ctx)
}

func (s *HelpService) GetAboutInfo(ctx context.Context) (AboutInfo, error) {
	if s == nil || s.core == nil {
		return AboutInfo{}, errors.New("help service is unavailable")
	}
	return s.core.GetAboutInfo(ctx)
}
