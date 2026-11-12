package lifecycle

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

const UserLogReviewEnvironment = "VPRO_USER_LOG_REVIEW"

type UserLogService struct {
	contexts *vprocore.ContextService
	enabled  bool
}

func NewUserLogService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*UserLogService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("user log review requires owned contexts and explicit feature lookup")
	}
	enabled, err := vprocore.SiviFeature(UserLogReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &UserLogService{contexts, enabled}, nil
}

func (s *UserLogService) GetReview(ctx context.Context, contextID string, mode, offset, limit int) (UserLogReview, error) {
	if ctx == nil {
		return UserLogReview{}, errors.New("user log review requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return UserLogReview{}, err
	}
	if s == nil || !s.enabled || s.contexts == nil {
		return UserLogReview{}, errors.New("saved user log review is independently disabled in this session")
	}
	return ReviewUserLog(s.contexts, ctx, contextID, mode, offset, limit)
}
