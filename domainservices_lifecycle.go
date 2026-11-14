package main

import (
	"context"
	"errors"

	lifecyclepkg "github.com/boostao/vpro-wails/internal/lifecycle"
)

type LifecycleService struct {
	startup *StartupService
	close   *CloseService
	userLog *UserLogService
}

func NewLifecycleService(startup *StartupService, closeSvc *CloseService, userLog *UserLogService) *LifecycleService {
	return &LifecycleService{
		startup: startup,
		close:   closeSvc,
		userLog: userLog,
	}
}

func (s *LifecycleService) GetStartupState() StartupState {
	if s == nil || s.startup == nil {
		return lifecyclepkg.WithBuildMetadata(StartupState{ReleaseChannel: ReleaseChannelStable})
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
