package main

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"sync"

	cloudsyncpkg "github.com/boostao/vpro-wails/internal/cloudsync"
	lifecyclepkg "github.com/boostao/vpro-wails/internal/lifecycle"
	spatialpkg "github.com/boostao/vpro-wails/internal/spatial"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type runtimeShutdownTracker struct {
	mu         sync.Mutex
	projects   *vprocorepkg.ProjectService
	working    *vprocorepkg.WorkingUnitService
	attributor *spatialpkg.PMTilesAttributor
	extraDirs  []string
}

func (t *runtimeShutdownTracker) swap(
	projects *vprocorepkg.ProjectService,
	working *vprocorepkg.WorkingUnitService,
	attributor *spatialpkg.PMTilesAttributor,
	extraDirs []string,
) {
	t.mu.Lock()
	prevProjects, prevWorking, prevAttributor := t.projects, t.working, t.attributor
	t.projects, t.working, t.attributor = projects, working, attributor
	t.extraDirs = append([]string(nil), extraDirs...)
	t.mu.Unlock()
	if prevAttributor != nil && prevAttributor != attributor {
		_ = prevAttributor.Close()
	}
	if prevWorking != nil && prevWorking != working {
		_ = prevWorking.Close()
	}
	if prevProjects != nil && prevProjects != projects {
		_ = prevProjects.CloseSQLiteContext()
	}
}

func (t *runtimeShutdownTracker) currentExtraDirs() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.extraDirs...)
}

func (t *runtimeShutdownTracker) shutdown(refs *referenceCatalogBundle, cloud *cloudsyncpkg.Manager) {
	t.mu.Lock()
	projToClose, wuToClose, attrToClose := t.projects, t.working, t.attributor
	t.projects, t.working, t.attributor = nil, nil, nil
	t.mu.Unlock()

	if wuToClose != nil {
		if err := wuToClose.Close(); err != nil {
			log.Printf("Warning: failed to close WorkingUnitService: %v", err)
		}
	}
	if projToClose != nil {
		if err := projToClose.CloseSQLiteContext(); err != nil {
			log.Printf("Warning: failed to close project context: %v", err)
		}
	}
	refs.Close()
	if attrToClose != nil {
		if err := attrToClose.Close(); err != nil {
			log.Printf("Warning: failed to close PMTilesAttributor: %v", err)
		}
	}
	if cloud != nil {
		cloud.Close()
	}
}

func newStartupServiceWithInitialActivation(dataDir, configDir string, activate func() (StartupState, error)) *lifecyclepkg.StartupService {
	svc := &lifecyclepkg.StartupService{
		DataDir:  dataDir,
		Activate: activate,
		State:    StartupState{ConfigPath: filepath.Join(configDir, "config.yml"), ReleaseChannel: vprocorepkg.ReleaseChannelStable},
	}
	if initialState, initErr := activate(); initErr != nil {
		log.Printf("Saved context remains unavailable (entering startup recovery): %v", initErr)
		recoveryState := StartupState{
			Ready:          false,
			Message:        initErr.Error(),
			ConfigPath:     filepath.Join(configDir, "config.yml"),
			ReleaseChannel: vprocorepkg.ReleaseChannelStable,
		}
		lifecyclepkg.PopulateRecoveryHints(&recoveryState)
		svc.SetState(recoveryState)
	} else {
		svc.SetState(initialState)
	}
	return svc
}

type LifecycleService struct {
	startup *lifecyclepkg.StartupService
	close   *lifecyclepkg.CloseService
	userLog *lifecyclepkg.UserLogService
}

func NewLifecycleService(startup *lifecyclepkg.StartupService, closeSvc *lifecyclepkg.CloseService, userLog *lifecyclepkg.UserLogService) *LifecycleService {
	return &LifecycleService{
		startup: startup,
		close:   closeSvc,
		userLog: userLog,
	}
}

func (s *LifecycleService) GetStartupState() StartupState {
	if s == nil || s.startup == nil {
		return lifecyclepkg.WithBuildMetadata(StartupState{ReleaseChannel: vprocorepkg.ReleaseChannelStable})
	}
	return s.startup.GetState()
}

func (s *LifecycleService) ChooseStartupDatabaseFile(ctx context.Context) (string, error) {
	if s == nil || s.startup == nil {
		return "", errors.New("startup service is unavailable")
	}
	return s.startup.ChooseDatabaseFile(ctx)
}

func (s *LifecycleService) ResetStartupToSample(ctx context.Context) (StartupState, error) {
	if s == nil || s.startup == nil {
		return StartupState{}, errors.New("startup service is unavailable")
	}
	return s.startup.ResetToSample(ctx)
}

func (s *LifecycleService) RecoverStartupWithPaths(ctx context.Context, request StartupRecoveryRequest) (StartupState, error) {
	if s == nil || s.startup == nil {
		return StartupState{}, errors.New("startup service is unavailable")
	}
	return s.startup.RecoverWithPaths(ctx, request)
}

func (s *LifecycleService) GetPendingCloseRequest() string {
	if s == nil || s.close == nil {
		return ""
	}
	return s.close.GetPendingCloseRequest()
}

func (s *LifecycleService) ConfirmClose(requestID string) error {
	if s == nil || s.close == nil {
		return errors.New("close service is unavailable")
	}
	return s.close.ConfirmClose(requestID)
}

func (s *LifecycleService) CancelClose(requestID string) error {
	if s == nil || s.close == nil {
		return errors.New("close service is unavailable")
	}
	return s.close.CancelClose(requestID)
}

func (s *LifecycleService) GetUserLogReview(ctx context.Context, contextID string, mode, offset, limit int) (UserLogReview, error) {
	if s == nil || s.userLog == nil {
		return UserLogReview{}, errors.New("user log service is unavailable")
	}
	return s.userLog.GetReview(ctx, contextID, mode, offset, limit)
}
