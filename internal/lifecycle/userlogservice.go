package lifecycle

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

type UserLogService struct {
	contexts *vprocore.ContextService
}

func NewUserLogService(contexts *vprocore.ContextService) (*UserLogService, error) {
	if contexts == nil {
		return nil, errors.New("user log review requires owned contexts")
	}
	return &UserLogService{contexts: contexts}, nil
}

func (s *UserLogService) GetReview(ctx context.Context, contextID string, mode, offset, limit int) (UserLogReview, error) {
	if ctx == nil {
		return UserLogReview{}, errors.New("user log review requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return UserLogReview{}, err
	}
	if s == nil || s.contexts == nil {
		return UserLogReview{}, errors.New("saved user log review requires owned contexts")
	}
	return ReviewUserLog(s.contexts, ctx, contextID, mode, offset, limit)
}
