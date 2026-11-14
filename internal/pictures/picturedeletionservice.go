package pictures

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

type PictureDeletionResult = pictureDeletionResult
type PictureRestorationResult = pictureRestorationResult

type pictureLifecycleReceiptObservation struct{ changed bool }

type PictureDeletionService struct {
	pictures  *PictureService
	mu        sync.Mutex
	mutations map[string]bool
	reads     map[string]map[*pictureLifecycleReceiptObservation]struct{}
	afterRead func() error
}

func NewPictureDeletionService(pictures *PictureService) (*PictureDeletionService, error) {
	if pictures == nil || pictures.Contexts == nil || pictures.source == nil {
		return nil, errors.New("picture deletion requires explicit picture ownership")
	}
	return &PictureDeletionService{pictures: pictures,
		mutations: map[string]bool{}, reads: map[string]map[*pictureLifecycleReceiptObservation]struct{}{}}, nil
}

func (s *PictureDeletionService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("picture deletion/restoration requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.pictures == nil {
		return errors.New("picture deletion/restoration requires explicit picture ownership")
	}
	return s.pictures.require(ctx)
}

func (s *PictureDeletionService) beginMutation(requestID string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mutations[requestID] {
		return nil, errors.New("picture lifecycle request is still running; retain the unresolved operation")
	}
	s.mutations[requestID] = true
	for observation := range s.reads[requestID] {
		observation.changed = true
	}
	return func() {
		s.mu.Lock()
		delete(s.mutations, requestID)
		s.mu.Unlock()
	}, nil
}

func withPictureLifecycleReceiptObservation[T any](ctx context.Context, s *PictureDeletionService, requestID string,
	read func() (*T, error)) (*T, error) {
	s.mu.Lock()
	if s.mutations[requestID] {
		s.mu.Unlock()
		return nil, errors.New("picture lifecycle mutation is still running; receipt is unresolved")
	}
	observation := &pictureLifecycleReceiptObservation{}
	if s.reads[requestID] == nil {
		s.reads[requestID] = map[*pictureLifecycleReceiptObservation]struct{}{}
	}
	s.reads[requestID][observation] = struct{}{}
	s.mu.Unlock()
	receipt, err := read()
	if s.afterRead != nil {
		err = errors.Join(err, s.afterRead())
	}
	s.mu.Lock()
	changed := observation.changed || s.mutations[requestID]
	delete(s.reads[requestID], observation)
	if len(s.reads[requestID]) == 0 {
		delete(s.reads, requestID)
	}
	s.mu.Unlock()
	if changed {
		return nil, errors.Join(err, errors.New("picture lifecycle mutation started during receipt lookup; receipt is unresolved"))
	}
	if err := errors.Join(err, ctx.Err()); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *PictureDeletionService) Delete(ctx context.Context, contextID, requestJSON string) (*PictureDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request pictureDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	finish, err := s.beginMutation(request.RequestID)
	if err != nil {
		return nil, err
	}
	defer finish()
	result, err := DeletePicture(s.pictures.Contexts, ctx, contextID, s.pictures.source, request)
	if result.DidCommit {
		return &result, err
	}
	if err != nil {
		return nil, err
	}
	return nil, errors.New("picture deletion returned without a durable commit receipt")
}

func (s *PictureDeletionService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*PictureDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request pictureDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return withPictureLifecycleReceiptObservation(ctx, s, request.RequestID, func() (*PictureDeletionResult, error) {
		return LookupPictureDeletionReceipt(s.pictures.Contexts, ctx, contextID, s.pictures.source, request)
	})
}

func (s *PictureDeletionService) Restore(ctx context.Context, contextID, requestJSON string) (*PictureRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request pictureRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	finish, err := s.beginMutation(request.RequestID)
	if err != nil {
		return nil, err
	}
	defer finish()
	result, err := RestorePicture(s.pictures.Contexts, ctx, contextID, s.pictures.source, request)
	if result.DidCommit {
		return &result, err
	}
	if err != nil {
		return nil, err
	}
	return nil, errors.New("picture restoration returned without a durable commit receipt")
}

func (s *PictureDeletionService) LookupRestorationReceipt(ctx context.Context, contextID, requestJSON string) (*PictureRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request pictureRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return withPictureLifecycleReceiptObservation(ctx, s, request.RequestID, func() (*PictureRestorationResult, error) {
		return LookupPictureRestorationReceipt(s.pictures.Contexts, ctx, contextID, s.pictures.source, request)
	})
}

func (s *PictureDeletionService) GetHistory(ctx context.Context, contextID, plot string) (*PictureDeletionHistoryList, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return ReadPictureDeletionEvents(s.pictures.Contexts, ctx, contextID, s.pictures.source, plot)
}

func (s *PictureDeletionService) ReviewRestoration(ctx context.Context, contextID, plot, historyID string) (*PictureRestorationReview, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return ReviewPictureRestoration(s.pictures.Contexts, ctx, contextID, s.pictures.source, plot, historyID)
}
