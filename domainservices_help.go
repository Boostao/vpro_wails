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

func buildHelpService(preferences *vprocorepkg.DesktopConfig, contexts *ContextService, dataDir string) (*HelpService, error) {
	svc, err := NewHelpService(preferences)
	if err != nil {
		return nil, err
	}
	svc.setContext(contexts)
	svc.setDataDir(dataDir)
	return svc, nil
}

func (s *HelpService) setContext(contexts *ContextService) {
	if s != nil && s.core != nil {
		s.core.SetContext(unwrapContextService(contexts))
	}
}

func (s *HelpService) setDataDir(dataDir string) {
	if s != nil && s.core != nil {
		s.core.SetDataDir(dataDir)
	}
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

func (s *HelpService) GetHelpBundle(ctx context.Context) (HelpBundle, error) {
	if s == nil || s.core == nil {
		return HelpBundle{}, errors.New("help service is unavailable")
	}
	return s.core.GetHelpBundle(ctx)
}

func (s *HelpService) LookupHelpTopic(ctx context.Context, helpID string) (HelpLookupResult, error) {
	if s == nil || s.core == nil {
		return HelpLookupResult{}, errors.New("help service is unavailable")
	}
	return s.core.LookupHelpTopic(ctx, helpID)
}

func (s *HelpService) SearchHelp(ctx context.Context, query string) ([]HelpSearchHit, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("help service is unavailable")
	}
	return s.core.SearchHelp(ctx, query)
}

func (s *HelpService) GetWhatsNewStatus(ctx context.Context) (WhatsNewStatus, error) {
	if s == nil || s.core == nil {
		return WhatsNewStatus{}, errors.New("help service is unavailable")
	}
	return s.core.GetWhatsNewStatus(ctx)
}

func (s *HelpService) AcknowledgeWhatsNew(ctx context.Context, version string) (WhatsNewStatus, error) {
	if s == nil || s.core == nil {
		return WhatsNewStatus{}, errors.New("help service is unavailable")
	}
	return s.core.AcknowledgeWhatsNew(ctx, version)
}

func (s *HelpService) ResetWorkspaceToSample(ctx context.Context, contextID string) (ProjectState, error) {
	if s == nil || s.core == nil {
		return ProjectState{}, errors.New("help service is unavailable")
	}
	return s.core.ResetWorkspaceToSample(ctx, contextID)
}
