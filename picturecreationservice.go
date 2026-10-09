package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const pictureCreationWritingEnvironment = "VPRO_PICTURE_CREATION_WRITING"

type PictureCreationResult = pictureCreationResult

type PictureCreationSelection struct {
	Token        string `json:"token"`
	SelectedPath string `json:"selectedPath"`
	ContextID    string `json:"contextId"`
	Project      string `json:"project"`
	PlotNumber   string `json:"plotNumber"`
	WindowID     uint   `json:"windowId"`
}

// Request contexts are RPC-owned and must never be retained by a capability.
type pictureCreationNativeOwner struct {
	app    *application.App
	window *application.WebviewWindow
	handle unsafe.Pointer
}

func currentPictureCreationOwner(ctx context.Context) (pictureCreationNativeOwner, error) {
	app, window, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return pictureCreationNativeOwner{}, err
	}
	handle := window.NativeWindow()
	if handle == nil {
		return pictureCreationNativeOwner{}, errors.New("picture creation native request window closed")
	}
	return pictureCreationNativeOwner{app, window, handle}, nil
}

type pictureCreationCapability struct {
	owner     pictureCreationNativeOwner
	dto       PictureCreationSelection
	source    *ownedPictureLibrary
	selection *ownedPictureSelection
	request   *pictureCreationRequest
	busy      bool
	attempted bool
}

type pictureCreationReceiptObservation struct {
	// A matching Create invalidates this read even if it finishes before the
	// database callback returns; removing its in-flight marker cannot hide it.
	changed bool
}

// PictureCreationService is independently default-off and requires owned picture
// reading. Hooks are private unit-test seams; the constructor always installs
// the actual Wails native owner and chooser.
type PictureCreationService struct {
	pictures *PictureService
	enabled  bool
	owner    func(context.Context) (pictureCreationNativeOwner, error)
	chooser  func(context.Context) (string, error)
	mu       sync.Mutex
	tokens   map[string]*pictureCreationCapability
	requests map[string]string
	creating map[string]*pictureCreationCapability
	reads    map[string]map[*pictureCreationReceiptObservation]struct{}
}

func NewPictureCreationService(pictures *PictureService, lookup func(string) (string, bool)) (*PictureCreationService, error) {
	if pictures == nil || pictures.contexts == nil || lookup == nil {
		return nil, errors.New("picture creation service requires explicit picture ownership and feature lookup")
	}
	enabled, err := siviFeature(pictureCreationWritingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	if enabled && (!pictures.enabled || pictures.source == nil) {
		return nil, errors.New("picture creation writing requires independently enabled owned picture reading")
	}
	return &PictureCreationService{
		pictures: pictures, enabled: enabled, owner: currentPictureCreationOwner,
		chooser: chooseNativePictureFile, tokens: make(map[string]*pictureCreationCapability),
		requests: make(map[string]string),
		creating: make(map[string]*pictureCreationCapability),
		reads:    make(map[string]map[*pictureCreationReceiptObservation]struct{}),
	}, nil
}

func (s *PictureCreationService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("picture creation writing requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled || s.pictures == nil {
		return errors.New("picture creation writing is disabled in this session")
	}
	return s.pictures.require(ctx)
}

func (s *PictureCreationService) checkOwner(ctx context.Context, expected pictureCreationNativeOwner) error {
	current, err := s.owner(ctx)
	if err != nil {
		return err
	}
	if current != expected {
		return errors.New("picture creation native owner changed")
	}
	return ctx.Err()
}

func (s *PictureCreationService) checkScope(ctx context.Context, dto PictureCreationSelection, source *ownedPictureLibrary) error {
	if source == nil || s.pictures.source != source {
		return errors.New("picture creation selected library owner changed")
	}
	review, err := s.pictures.GetMetadata(ctx, dto.ContextID, dto.PlotNumber)
	if err != nil {
		return err
	}
	if review.ContextID != dto.ContextID || review.Project != dto.Project || review.PlotNumber != dto.PlotNumber {
		return errors.New("picture creation reviewed context/project/parent changed")
	}
	if s.pictures.source != source {
		return errors.New("picture creation selected library owner changed")
	}
	return nil
}

// Choose returns nil on Cancel, without assigning fields or allocating history.
// One unresolved capability per literal owner/scope prevents silent replacement.
func (s *PictureCreationService) Choose(ctx context.Context, contextID, project, plot string) (result *PictureCreationSelection, resultErr error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	identity := pictureCreationRequest{contextID, project, plot, "00000000-0000-4000-8000-000000000000", `C:\selection.jpg`}
	if _, _, err := preparePictureCreation(identity); err != nil {
		return nil, err
	}
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	dto := PictureCreationSelection{ContextID: contextID, Project: project, PlotNumber: plot}
	if owner.window != nil {
		dto.WindowID = owner.window.ID()
	}
	source := s.pictures.source
	if err := s.checkScope(ctx, dto, source); err != nil {
		return nil, err
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	dto.Token = hex.EncodeToString(random[:])
	capability := &pictureCreationCapability{owner: owner, dto: dto, source: source, busy: true}
	s.mu.Lock()
	for _, existing := range s.tokens {
		if existing.owner == owner && existing.dto.ContextID == contextID &&
			existing.dto.Project == project && existing.dto.PlotNumber == plot {
			s.mu.Unlock()
			return nil, errors.New("picture selection has an unresolved capability; release or acknowledge it explicitly")
		}
	}
	if _, exists := s.tokens[dto.Token]; exists {
		s.mu.Unlock()
		return nil, errors.New("picture selection token collision")
	}
	s.tokens[dto.Token] = capability
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if resultErr != nil || result == nil {
			delete(s.tokens, dto.Token)
		} else {
			capability.busy = false
		}
	}()
	if err := s.checkOwner(ctx, owner); err != nil {
		return nil, err
	}
	path, err := s.chooser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.checkOwner(ctx, owner); err != nil {
		return nil, err
	}
	if err := s.checkScope(ctx, dto, source); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	selection, err := newOwnedPictureSelection(path)
	if err != nil {
		return nil, err
	}
	if err := s.checkOwner(ctx, owner); err != nil {
		return nil, err
	}
	capability.selection = selection
	capability.dto.SelectedPath = path
	copy := capability.dto
	return &copy, nil
}

// RecoverSelection recovers lost Choose delivery, not file or commit authority.
// It never reads the selected object, changes lifecycle state, or opens a dialog.
// Owned reading remains sufficient to verify absence after a disabled Choose.
func (s *PictureCreationService) RecoverSelection(ctx context.Context, contextID, project, plot string) (*PictureCreationSelection, error) {
	if s == nil || s.pictures == nil {
		return nil, errors.New("picture selection recovery requires owned picture reading")
	}
	if err := s.pictures.require(ctx); err != nil {
		return nil, err
	}
	identity := pictureCreationRequest{contextID, project, plot, "00000000-0000-4000-8000-000000000000", `C:\selection.jpg`}
	if _, _, err := preparePictureCreation(identity); err != nil {
		return nil, err
	}
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var capability *pictureCreationCapability
	var foreignOwner bool
	for _, existing := range s.tokens {
		if existing.dto.ContextID != contextID || existing.dto.Project != project || existing.dto.PlotNumber != plot {
			continue
		}
		if existing.owner == owner {
			capability = existing
		} else {
			foreignOwner = true
		}
	}
	if capability != nil && capability.busy {
		return nil, errors.New("picture selection operation is still running; selection recovery is unavailable")
	}
	dto := PictureCreationSelection{ContextID: contextID, Project: project, PlotNumber: plot}
	source := s.pictures.source
	if capability != nil {
		dto = capability.dto
		source = capability.source
	}
	if err := s.checkScope(ctx, dto, source); err != nil {
		return nil, err
	}
	if err := s.checkOwner(ctx, owner); err != nil {
		return nil, err
	}
	if capability == nil {
		if foreignOwner {
			return nil, errors.New("picture selection capability is unavailable for this native owner")
		}
		return nil, nil
	}
	return &dto, nil
}

// acquire serializes all operations on a token, including release/acknowledgement.
func (s *PictureCreationService) acquire(ctx context.Context, token string) (*pictureCreationCapability, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	capability := s.tokens[token]
	if capability == nil || capability.owner != owner {
		return nil, errors.New("picture selection capability is unavailable for this native owner")
	}
	if capability.busy {
		return nil, errors.New("picture selection operation is still running")
	}
	capability.busy = true
	return capability, nil
}

func (s *PictureCreationService) finish(capability *pictureCreationCapability) {
	s.mu.Lock()
	capability.busy = false
	s.mu.Unlock()
}

func pictureCreationCapabilityRequest(capability *pictureCreationCapability, request pictureCreationRequest) error {
	if request.ContextID != capability.dto.ContextID || request.Project != capability.dto.Project ||
		request.PlotNumber != capability.dto.PlotNumber || request.SelectedPath != capability.dto.SelectedPath ||
		(capability.request != nil && *capability.request != request) {
		return errors.New("picture creation request does not match its literal selected capability")
	}
	return nil
}

func (s *PictureCreationService) Create(ctx context.Context, token, requestJSON string) (*PictureCreationResult, error) {
	var request pictureCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	capability, err := s.acquire(ctx, token)
	if err != nil {
		return nil, err
	}
	defer s.finish(capability)
	if err := pictureCreationCapabilityRequest(capability, request); err != nil {
		return nil, err
	}
	if capability.attempted {
		return nil, errors.New("picture creation was already attempted; recover only through LookupReceipt")
	}
	s.mu.Lock()
	if bound, exists := s.requests[request.RequestID]; exists && bound != token {
		s.mu.Unlock()
		return nil, errors.New("picture creation request UUID is bound to another selection")
	}
	s.requests[request.RequestID] = token
	capability.request = &request
	s.creating[request.RequestID] = capability
	for observation := range s.reads[request.RequestID] {
		observation.changed = true
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.creating, request.RequestID)
		s.mu.Unlock()
	}()
	if err := s.checkScope(ctx, capability.dto, capability.source); err != nil {
		return nil, err
	}
	if err := capability.selection.checkFile(); err != nil {
		return nil, err
	}
	// Internal preflight runs inside this Create's reservation; public recovery
	// must refuse that reservation rather than treating it as a completed read.
	receipt, err := s.pictures.contexts.lookupPictureCreationReceipt(ctx, request.ContextID, capability.source, request)
	if err != nil {
		return nil, err
	}
	if receipt != nil {
		return nil, errors.New("picture creation UUID already has a durable receipt; use LookupReceipt")
	}
	if err := s.checkOwner(ctx, capability.owner); err != nil {
		return nil, err
	}
	if err := capability.selection.checkFile(); err != nil {
		return nil, err
	}
	// After entry to the writer, even an error is an unresolved outcome until
	// a separate completed read proves no receipt or acknowledges a receipt.
	capability.attempted = true
	committed, err := s.pictures.contexts.createPicture(ctx, request.ContextID, capability.source, request)
	if committed.DidCommit {
		return &committed, errors.Join(err, s.checkOwner(ctx, capability.owner))
	}
	if err != nil {
		return nil, err
	}
	return nil, errors.New("picture creation returned without a durable commit receipt")
}

// Recovery never requires a capability, a native window, or a surviving image.
// It is strictly read-only and never alters capability lifecycle. A matching
// Create before/during observation is unresolved, never a successful null.
func (s *PictureCreationService) LookupReceipt(ctx context.Context, requestJSON string) (*PictureCreationResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request pictureCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.creating[request.RequestID] != nil {
		s.mu.Unlock()
		return nil, errors.New("picture creation is still running; receipt is unresolved")
	}
	observation := &pictureCreationReceiptObservation{}
	if s.reads[request.RequestID] == nil {
		s.reads[request.RequestID] = make(map[*pictureCreationReceiptObservation]struct{})
	}
	s.reads[request.RequestID][observation] = struct{}{}
	s.mu.Unlock()

	receipt, err := s.pictures.contexts.lookupPictureCreationReceipt(ctx, request.ContextID, s.pictures.source, request)

	s.mu.Lock()
	changed := observation.changed || s.creating[request.RequestID] != nil
	delete(s.reads[request.RequestID], observation)
	if len(s.reads[request.RequestID]) == 0 {
		delete(s.reads, request.RequestID)
	}
	s.mu.Unlock()
	if changed {
		return nil, errors.Join(err, errors.New("picture creation started during receipt lookup; receipt is unresolved"))
	}
	return receipt, err
}

func (s *PictureCreationService) GetSelectedImage(ctx context.Context, token, requestJSON string) (*PictureImage, error) {
	var request pictureCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	capability, err := s.acquire(ctx, token)
	if err != nil {
		return nil, err
	}
	defer s.finish(capability)
	if err := pictureCreationCapabilityRequest(capability, request); err != nil {
		return nil, err
	}
	if !capability.attempted || capability.request == nil {
		return nil, errors.New("selected picture preview requires an attempted creation and durable receipt")
	}
	if err := s.checkScope(ctx, capability.dto, capability.source); err != nil {
		return nil, err
	}
	receipt, err := s.LookupReceipt(ctx, requestJSON)
	if err != nil {
		return nil, err
	}
	if receipt == nil || !receipt.DidCommit {
		return nil, errors.New("selected picture preview requires a durable committed receipt")
	}
	image, err := withOwnedPictureRead(ctx, s.pictures.contexts, request.ContextID, request.PlotNumber, capability.source,
		func(project string, records ProjectMetadataTable) (pictureImage, error) {
			if project != request.Project || !reflect.DeepEqual(records.Columns, receipt.Columns) {
				return pictureImage{}, errors.New("selected picture preview owner/schema changed")
			}
			var matched bool
			for _, row := range records.Rows {
				if reflect.DeepEqual(row, receipt.Committed) {
					matched = true
				}
			}
			if !matched {
				return pictureImage{}, errors.New("selected picture committed physical row changed")
			}
			if err := s.checkOwner(ctx, capability.owner); err != nil {
				return pictureImage{}, err
			}
			return capability.selection.readImage(ctx)
		})
	if err != nil {
		return nil, err
	}
	if err := s.checkScope(ctx, capability.dto, capability.source); err != nil {
		return nil, err
	}
	if err := s.checkOwner(ctx, capability.owner); err != nil {
		return nil, err
	}
	image.ContextID = request.ContextID
	image.Project = request.Project
	image.PlotNumber = request.PlotNumber
	image.RowID = receipt.Committed.RowID
	return &image, nil
}

// ReleaseSelection is permitted before writer entry or after a completed null
// receipt lookup. It refuses committed and unknown/error outcomes.
func (s *PictureCreationService) ReleaseSelection(ctx context.Context, token string) error {
	return s.closeSelection(ctx, token, false)
}

// AcknowledgeReceipt explicitly relinquishes file authority only after verifying
// the bound durable receipt. It cannot acknowledge an in-flight/unknown result.
func (s *PictureCreationService) AcknowledgeReceipt(ctx context.Context, token string) error {
	return s.closeSelection(ctx, token, true)
}

func (s *PictureCreationService) closeSelection(ctx context.Context, token string, acknowledge bool) error {
	capability, err := s.acquire(ctx, token)
	if err != nil {
		return err
	}
	defer s.finish(capability)
	if err := s.checkScope(ctx, capability.dto, capability.source); err != nil {
		return err
	}
	var receipt *pictureCreationResult
	if capability.attempted {
		receipt, err = s.pictures.contexts.lookupPictureCreationReceipt(ctx, capability.dto.ContextID, capability.source, *capability.request)
		if err != nil {
			return err
		}
	}
	if acknowledge != (receipt != nil && receipt.DidCommit) {
		return errors.New("picture selection release requires verified no-commit; acknowledgement requires a durable receipt")
	}
	if err := s.checkOwner(ctx, capability.owner); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.tokens, token)
	if capability.request != nil && s.requests[capability.request.RequestID] == token {
		delete(s.requests, capability.request.RequestID)
	}
	s.mu.Unlock()
	return nil
}
