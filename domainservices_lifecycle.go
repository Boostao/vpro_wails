package main

import (
	"context"
	"errors"
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
		return StartupState{ReleaseChannel: ReleaseChannelStable}
	}
	return s.startup.GetState()
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
