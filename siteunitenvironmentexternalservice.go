package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

const externalSUEnvironmentEditingEnvironment = "VPRO_EXTERNAL_SU_ENVIRONMENT_EDITING"

type ExternalSUEnvironmentService struct {
	contexts                 *ContextService
	enabled                  bool
	sessionID                string
	mu                       sync.Mutex
	reviews                  map[string]*externalSUEnvironmentSource
	attempts                 map[string]*externalSUEnvironmentAttempt
	sourceHash, approvalHash string
}

func NewExternalSUEnvironmentService(contexts *ContextService, enabled bool) *ExternalSUEnvironmentService {
	return &ExternalSUEnvironmentService{contexts: contexts, enabled: enabled, sessionID: rand.Text()}
}

type ExternalSUEnvironmentReview struct {
	Review       SiteUnitEnvironmentReview `json:"review"`
	ProjectPath  string                    `json:"projectPath"`
	SUPath       string                    `json:"suPath"`
	SourceHash   string                    `json:"sourceHash"`
	ApprovalHash string                    `json:"approvalHash"`
	SessionID    string                    `json:"sessionId"`
}

type ExternalSUEnvironmentRequest struct {
	ApprovalHash string `json:"approvalHash"`
	RequestID    string `json:"requestId"`
	SessionID    string `json:"sessionId"`
	Confirmed    bool   `json:"confirmed"`
}

type externalSUEnvironmentAttemptIdentity struct {
	ApprovalHash string `json:"approvalHash"`
	RequestID    string `json:"requestId"`
	SessionID    string `json:"sessionId"`
}

func (identity externalSUEnvironmentAttemptIdentity) validate() error {
	if !siviDeletionUUID.MatchString(identity.RequestID) {
		return errors.New("external SU/environment transfer requires a stable lowercase UUIDv4")
	}
	hash, err := hex.DecodeString(identity.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != identity.ApprovalHash {
		return errors.New("external SU/environment transfer requires an exact lowercase source approval hash")
	}
	if identity.SessionID == "" || len(identity.SessionID) > 128 {
		return errors.New("external SU/environment transfer requires its reviewed service owner")
	}
	return nil
}

func (request *ExternalSUEnvironmentRequest) UnmarshalJSON(data []byte) error {
	type plain ExternalSUEnvironmentRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "external SU/environment transfer",
		"approvalHash", "requestId", "sessionId", "confirmed"); err != nil {
		return err
	}
	if !decoded.Confirmed {
		return errors.New("external SU/environment transfer requires explicit confirmation")
	}
	if err := (externalSUEnvironmentAttemptIdentity{decoded.ApprovalHash, decoded.RequestID, decoded.SessionID}).validate(); err != nil {
		return err
	}
	*request = ExternalSUEnvironmentRequest(decoded)
	return nil
}

type ExternalSUEnvironmentOutcome struct {
	Status       string                     `json:"status"`
	Receipt      *SiteUnitEnvironmentResult `json:"receipt"`
	ErrorMessage string                     `json:"errorMessage"`
}

func (s *ExternalSUEnvironmentService) authorize(ctx context.Context, write bool) error {
	if ctx == nil {
		return errors.New("external SU/environment service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.sessionID == "" {
		return errors.New("external SU/environment service requires its owned SQLite context and service owner")
	}
	if write && !s.enabled {
		return errors.New("external SU/environment editing is disabled in this session")
	}
	return nil
}

func (s *ExternalSUEnvironmentService) GetReview(ctx context.Context, contextID string) (*ExternalSUEnvironmentReview, error) {
	if err := s.authorize(ctx, true); err != nil {
		return nil, err
	}
	source, err := s.contexts.reviewExternalSUEnvironment(ctx, contextID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(struct {
		Review    SiteUnitEnvironmentReview                  `json:"review"`
		Selection desktopSelection                           `json:"selection"`
		Witness   map[string]map[string]ProjectMetadataTable `json:"witness"`
		User      string                                     `json:"user"`
		Paths     map[string]string                          `json:"paths"`
	}{source.Review, source.Selection, source.Witness, source.User, source.Paths})
	if err != nil {
		return nil, fmt.Errorf("external SU/environment source hash: %w", err)
	}
	sourceHash := sha256.Sum256(raw)
	reviewBytes, err := json.Marshal(source.Review)
	if err != nil {
		return nil, err
	}
	var copy SiteUnitEnvironmentReview
	if err := json.Unmarshal(reviewBytes, &copy); err != nil {
		return nil, err
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	sourceKey := hex.EncodeToString(sourceHash[:])
	if s.sourceHash != sourceKey || s.reviews[s.approvalHash] == nil {
		approval := sha256.Sum256(append([]byte("VPRO external SU/environment approval v1\x00"+s.sessionID+"\x00"+rand.Text()+"\x00"), raw...))
		s.sourceHash, s.approvalHash = sourceKey, hex.EncodeToString(approval[:])
		s.reviews = map[string]*externalSUEnvironmentSource{s.approvalHash: source}
	} else if err := compareExternalSUEnvironmentFiles(s.reviews[s.approvalHash].Files, source.Files); err != nil {
		return nil, err
	}
	return &ExternalSUEnvironmentReview{copy, source.Paths["project"], source.Paths["su"],
		sourceKey, s.approvalHash, s.sessionID}, nil
}

func (s *ExternalSUEnvironmentService) TransferReviewed(ctx context.Context, contextID, requestJSON string) (*ExternalSUEnvironmentOutcome, error) {
	return s.transferReviewed(ctx, contextID, requestJSON, externalSUEnvironmentWriteHooks{}, nil)
}

func (s *ExternalSUEnvironmentService) transferReviewed(ctx context.Context, contextID, requestJSON string,
	hooks externalSUEnvironmentWriteHooks, afterWrite func()) (outcome *ExternalSUEnvironmentOutcome, resultErr error) {
	if err := s.authorize(ctx, true); err != nil {
		return nil, err
	}
	var request ExternalSUEnvironmentRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	execute, prior, source, err := s.beginExternalSUEnvironmentAttempt(ctx, contextID, request)
	if err != nil || !execute {
		return prior, err
	}
	defer func() {
		if outcome == nil {
			outcome = &ExternalSUEnvironmentOutcome{Status: "unknown",
				ErrorMessage: "The writer did not return a complete outcome; retain the exact request for readonly recovery."}
		}
		s.finishExternalSUEnvironmentAttempt(request.RequestID, *outcome)
	}()
	identity := externalSUEnvironmentAttemptIdentity{request.ApprovalHash, request.RequestID, request.SessionID}
	result, writeErr := s.contexts.transferExternalSUEnvironmentOwned(ctx, contextID, source, &identity, hooks)
	outcome = externalSUEnvironmentOutcome(result, writeErr)
	if afterWrite != nil {
		afterWrite()
	}
	return outcome, nil
}

func externalSUEnvironmentOutcome(result *SiteUnitEnvironmentResult, err error) *ExternalSUEnvironmentOutcome {
	outcome := &ExternalSUEnvironmentOutcome{Status: "unknown"}
	if result != nil {
		copy := *result
		outcome.Status, outcome.Receipt = "committed", &copy
	} else {
		var rejected *externalSUEnvironmentNotCommittedError
		if errors.As(err, &rejected) {
			outcome.Status = "notCommitted"
		}
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	} else if result == nil {
		outcome.ErrorMessage = "The writer returned no receipt; retain the exact request for readonly recovery."
	}
	return outcome
}
