package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type pictureCreationOwnerTestKey struct{}

func pictureCreationFacadeFixture(t *testing.T, format string) (*PictureCreationService, context.Context, pictureCreationRequest) {
	t.Helper()
	contexts, _, source, request := pictureCreationFixture(t, false, pictureCreationTestSchema)
	pictures := &PictureService{contexts: contexts, enabled: true, source: source}
	service, err := NewPictureCreationService(pictures, pictureServiceLookup(map[string]string{pictureCreationWritingEnvironment: "true"}))
	if err != nil {
		t.Fatal(err)
	}
	owner := pictureCreationNativeOwner{&application.App{}, &application.WebviewWindow{}, unsafe.Pointer(new(byte))}
	ctx := context.WithValue(t.Context(), pictureCreationOwnerTestKey{}, owner)
	service.owner = func(ctx context.Context) (pictureCreationNativeOwner, error) {
		if err := ctx.Err(); err != nil {
			return pictureCreationNativeOwner{}, err
		}
		owner, ok := ctx.Value(pictureCreationOwnerTestKey{}).(pictureCreationNativeOwner)
		if !ok {
			return pictureCreationNativeOwner{}, errors.New("unit owner absent")
		}
		return owner, nil
	}
	request.SelectedPath = filepath.Join(pictureSelectionDirectory(t), "  literal #100% 😀."+format)
	pictureSelectionWrite(t, request.SelectedPath, pictureSelectionBytes(t, format))
	service.chooser = func(context.Context) (string, error) { return request.SelectedPath, nil }
	return service, ctx, request
}

func pictureCreationFacadeJSON(t *testing.T, request pictureCreationRequest) string {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func pictureCreationFacadeChoose(t *testing.T, service *PictureCreationService, ctx context.Context, request pictureCreationRequest) *PictureCreationSelection {
	t.Helper()
	selected, err := service.Choose(ctx, request.ContextID, request.Project, request.PlotNumber)
	if err != nil || selected == nil {
		t.Fatal("selection failed", selected, err)
	}
	if len(selected.Token) != 64 || selected.SelectedPath != request.SelectedPath ||
		selected.ContextID != request.ContextID || selected.Project != request.Project || selected.PlotNumber != request.PlotNumber {
		t.Fatal("selection lost literal owner", selected)
	}
	return selected
}

func TestPictureCreationServiceFeatureAndActualOwnerDenials(t *testing.T) {
	service, ctx, request := pictureCreationFacadeFixture(t, "jpeg")
	for _, value := range []string{"", "invalid", "yes", "TRUE "} {
		if _, err := NewPictureCreationService(service.pictures, pictureServiceLookup(map[string]string{pictureCreationWritingEnvironment: value})); err == nil {
			t.Fatal("invalid flag accepted", value)
		}
	}
	for _, flags := range []map[string]string{nil, {pictureCreationWritingEnvironment: "false"}, {pictureMetadataWritingEnvironment: "true"}} {
		disabled, err := NewPictureCreationService(service.pictures, pictureServiceLookup(flags))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := disabled.Choose(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil {
			t.Fatal("creation unexpectedly enabled")
		}
		if _, err := disabled.LookupReceipt(ctx, pictureCreationFacadeJSON(t, request)); err == nil {
			t.Fatal("disabled recovery enabled")
		}
	}
	for _, pictures := range []*PictureService{nil, {}, {contexts: service.pictures.contexts}, {contexts: service.pictures.contexts, enabled: true}} {
		if _, err := NewPictureCreationService(pictures, pictureServiceLookup(map[string]string{pictureCreationWritingEnvironment: "true"})); err == nil {
			t.Fatal("missing owned reading accepted")
		}
	}
	if _, err := NewPictureCreationService(service.pictures, nil); err == nil {
		t.Fatal("nil lookup accepted")
	}
	production, err := NewPictureCreationService(service.pictures, pictureServiceLookup(map[string]string{pictureCreationWritingEnvironment: "true", pictureMetadataWritingEnvironment: "false"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, context := range []context.Context{nil, t.Context(), context.WithValue(t.Context(), application.WindowKey, &application.WebviewWindow{})} {
		if _, err := production.Choose(context, request.ContextID, request.Project, request.PlotNumber); err == nil {
			t.Fatal("production inferred native ownership")
		}
	}
}

func TestPictureCreationServiceChooseCancelErrorsAndChangedAuthority(t *testing.T) {
	for _, scenario := range []string{"cancel", "error", "cancel-context", "owner", "project", "parent", "library", "context", "library-owner"} {
		t.Run(scenario, func(t *testing.T) {
			service, ctx, request := pictureCreationFacadeFixture(t, "png")
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			before := pictureCreationRawRows(t, service.pictures.source)
			owner := ctx.Value(pictureCreationOwnerTestKey{}).(pictureCreationNativeOwner)
			state, err := service.pictures.contexts.projects.GetState(ctx)
			if err != nil {
				t.Fatal(err)
			}
			service.chooser = func(context.Context) (string, error) {
				switch scenario {
				case "cancel":
					return "", nil
				case "error":
					return "", errors.New("test chooser error")
				case "cancel-context":
					cancel()
				case "owner":
					service.owner = func(context.Context) (pictureCreationNativeOwner, error) {
						changed := owner
						changed.handle = unsafe.Pointer(new(byte))
						return changed, nil
					}
				case "parent":
					mutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Env WHERE PlotNumber='108050'`)
				case "library":
					if err := os.Rename(service.pictures.source.path, service.pictures.source.path+".gone"); err != nil {
						t.Fatal(err)
					}
				case "context":
					if _, err := service.pictures.contexts.SwitchContext(state.ContextID, contextSelection(state)); err != nil {
						t.Fatal(err)
					}
				case "library-owner":
					service.pictures.source, _ = pictureLibraryFixture(t, pictureCreationTestSchema)
				}
				return request.SelectedPath, nil
			}
			project := request.Project
			if scenario == "project" {
				project = "Other"
			}
			selected, err := service.Choose(ctx, request.ContextID, project, request.PlotNumber)
			if selected != nil || (scenario == "cancel" && err != nil) || (scenario != "cancel" && err == nil) || len(service.tokens) != 0 {
				t.Fatal("cancel/error published authority", selected, err)
			}
			if scenario != "parent" && scenario != "library" && !reflect.DeepEqual(before, pictureCreationRawRows(t, service.pictures.source)) {
				t.Fatal("chooser changed rows")
			}
		})
	}
}

func TestPictureCreationServiceLiteralCommitPreviewAndReceiptLifecycle(t *testing.T) {
	for _, format := range []string{"jpeg", "png", "gif", "oversized"} {
		t.Run(format, func(t *testing.T) {
			fixtureFormat := format
			if format == "oversized" {
				fixtureFormat = "png"
			}
			service, ctx, request := pictureCreationFacadeFixture(t, fixtureFormat)
			if format == "oversized" {
				file, err := os.OpenFile(request.SelectedPath, os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if err := errors.Join(file.Truncate(pictureImageMaxBytes+1), file.Close()); err != nil {
					t.Fatal(err)
				}
			}
			before := pictureCreationRawRows(t, service.pictures.source)
			selected := pictureCreationFacadeChoose(t, service, ctx, request)
			raw := pictureCreationFacadeJSON(t, request)
			if _, err := service.GetSelectedImage(ctx, selected.Token, raw); err == nil {
				t.Fatal("uncommitted selection preview accepted")
			}
			receipt, err := service.Create(ctx, selected.Token, raw)
			if err != nil || receipt == nil || !receipt.DidCommit {
				t.Fatal(receipt, err)
			}
			dir, name := filepath.Split(request.SelectedPath)
			expected := append(append([][]string{}, before...), []string{"6", "integer", "31",
				"text", fmt.Sprintf("%X", []byte(dir)), "text", fmt.Sprintf("%X", []byte(name)),
				"text", "313038303530", "null", ""})
			if !reflect.DeepEqual(expected, pictureCreationRawRows(t, service.pictures.source)) {
				t.Fatal("literal assignments/NULL comment or untouched peers differ")
			}
			image, err := service.GetSelectedImage(ctx, selected.Token, raw)
			if format == "jpeg" || format == "png" {
				if err != nil || image == nil {
					t.Fatal("regular image refused", err)
				}
			} else if err == nil || image != nil {
				t.Fatal("unsupported/oversized image accepted")
			}
			if format == "jpeg" || format == "png" {
				data := pictureSelectionBytes(t, format)
				hash := sha256.Sum256(data)
				expectedImage := PictureImage{
					ContextID: request.ContextID, Project: "Sample", PlotNumber: "108050", RowID: "6",
					MIME: "image/" + format, Width: 3, Height: 2, SHA256: fmt.Sprintf("%x", hash),
					DataURL: "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(data),
				}
				if *image != expectedImage {
					t.Fatal("complete selected image wire identity/content differs", image, expectedImage)
				}
			}
			sibling := request
			sibling.SelectedPath = filepath.Join(filepath.Dir(request.SelectedPath), "sibling.png")
			pictureSelectionWrite(t, sibling.SelectedPath, pictureSelectionBytes(t, "png"))
			if image, err := service.GetSelectedImage(ctx, selected.Token, pictureCreationFacadeJSON(t, sibling)); err == nil || image != nil {
				t.Fatal("selected capability granted sibling preview")
			}
			if service.pictures.directories.child != nil || service.pictures.directories.manager != nil {
				t.Fatal("selection granted ordinary directory access")
			}
			if pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != 1 {
				t.Fatal("preview changed durable history")
			}
			if _, err := service.Create(ctx, selected.Token, raw); err == nil {
				t.Fatal("attempt replayed through writer")
			}
			if err := service.ReleaseSelection(ctx, selected.Token); err == nil {
				t.Fatal("committed authority silently discarded")
			}
			if _, err := service.Choose(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil {
				t.Fatal("unacknowledged selection replaced")
			}
			if err := os.Remove(request.SelectedPath); err != nil {
				t.Fatal(err)
			}
			recovered, err := service.LookupReceipt(t.Context(), raw)
			if err != nil || recovered == nil || !recovered.DidCommit || !recovered.Replayed {
				t.Fatal("receipt depended on file/native owner", recovered, err)
			}
			if err := service.AcknowledgeReceipt(ctx, selected.Token); err != nil {
				t.Fatal(err)
			}
			if _, err := service.GetSelectedImage(ctx, selected.Token, raw); err == nil {
				t.Fatal("acknowledged capability still read")
			}
			if recovered, err := service.LookupReceipt(t.Context(), raw); err != nil || recovered == nil {
				t.Fatal("acknowledgement erased durable receipt", err)
			}
			restarted, err := NewPictureCreationService(service.pictures, pictureServiceLookup(map[string]string{pictureCreationWritingEnvironment: "true"}))
			if err != nil {
				t.Fatal(err)
			}
			if recovered, err := restarted.LookupReceipt(t.Context(), raw); err != nil || recovered == nil {
				t.Fatal("service recreation/token loss blocked durable recovery", err)
			}
		})
	}
}

func TestPictureCreationServiceStrictRequestAndForgery(t *testing.T) {
	service, ctx, request := pictureCreationFacadeFixture(t, "png")
	selected := pictureCreationFacadeChoose(t, service, ctx, request)
	raw := pictureCreationFacadeJSON(t, request)
	for _, invalid := range []string{
		`{"contextId":"x","contextId":"y"}`,
		strings.TrimSuffix(raw, "}") + `,"requestId":"` + pictureCreationTestUUID2 + `"}`,
		strings.TrimSuffix(raw, "}") + `,"\u0072equestId":"` + pictureCreationTestUUID2 + `"}`,
		strings.Replace(raw, `"project"`, `"Project"`, 1),
		strings.TrimSuffix(raw, "}") + `,"\u0070roject":"Other"}`,
		strings.Replace(raw, `"Sample"`, `"\ud800"`, 1),
		strings.Replace(raw, `"Sample"`, "\"\xff\"", 1),
		strings.TrimSuffix(raw, "}") + `,"token":"forged"}`,
	} {
		if _, err := service.Create(ctx, selected.Token, invalid); err == nil {
			t.Fatal("invalid strict creation request accepted", invalid)
		}
		if _, err := service.LookupReceipt(ctx, invalid); err == nil {
			t.Fatal("invalid strict recovery request accepted", invalid)
		}
	}
	for _, field := range []string{"context", "project", "parent", "path"} {
		changed := request
		switch field {
		case "context":
			changed.ContextID += "x"
		case "project":
			changed.Project = "Other"
		case "parent":
			changed.PlotNumber = "108051"
		case "path":
			changed.SelectedPath = filepath.Join(filepath.Dir(request.SelectedPath), "sibling.png")
			pictureSelectionWrite(t, changed.SelectedPath, pictureSelectionBytes(t, "png"))
		}
		if _, err := service.Create(ctx, selected.Token, pictureCreationFacadeJSON(t, changed)); err == nil {
			t.Fatal("forged capability scope accepted", field)
		}
	}
	if _, err := service.Create(ctx, "forged", raw); err == nil {
		t.Fatal("forged token accepted")
	}
	owner := ctx.Value(pictureCreationOwnerTestKey{}).(pictureCreationNativeOwner)
	for _, dimension := range []string{"app", "window", "handle"} {
		changed := owner
		switch dimension {
		case "app":
			changed.app = &application.App{}
		case "window":
			changed.window = &application.WebviewWindow{}
		case "handle":
			changed.handle = unsafe.Pointer(new(byte))
		}
		foreign := context.WithValue(ctx, pictureCreationOwnerTestKey{}, changed)
		if _, err := service.Create(foreign, selected.Token, raw); err == nil {
			t.Fatal("foreign native owner accepted", dimension)
		}
		if err := service.ReleaseSelection(foreign, selected.Token); err == nil {
			t.Fatal("foreign native owner released token")
		}
		if image, err := service.GetSelectedImage(foreign, selected.Token, raw); err == nil || image != nil {
			t.Fatal("foreign native owner read selected image")
		}
	}
	if err := service.ReleaseSelection(ctx, selected.Token); err != nil {
		t.Fatal("unattempted selection could not be released", err)
	}
	if pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM tblVPics`) != 4 {
		t.Fatal("forgeries wrote rows")
	}
}

func TestPictureCreationServiceReplacementAndCurrentPhysicalRow(t *testing.T) {
	for _, mutation := range []string{"before-create", "after-create", "row"} {
		t.Run(mutation, func(t *testing.T) {
			service, ctx, request := pictureCreationFacadeFixture(t, "jpeg")
			selected := pictureCreationFacadeChoose(t, service, ctx, request)
			raw := pictureCreationFacadeJSON(t, request)
			if mutation == "before-create" {
				if err := os.Rename(request.SelectedPath, request.SelectedPath+".old"); err != nil {
					t.Fatal(err)
				}
				pictureSelectionWrite(t, request.SelectedPath, pictureSelectionBytes(t, "jpeg"))
				if _, err := service.Create(ctx, selected.Token, raw); err == nil {
					t.Fatal("replacement accepted before creation")
				}
				if err := service.ReleaseSelection(ctx, selected.Token); err != nil {
					t.Fatal(err)
				}
				return
			}
			if receipt, err := service.Create(ctx, selected.Token, raw); err != nil || receipt == nil {
				t.Fatal(err)
			}
			if mutation == "after-create" {
				if err := os.Rename(request.SelectedPath, request.SelectedPath+".old"); err != nil {
					t.Fatal(err)
				}
				pictureSelectionWrite(t, request.SelectedPath, pictureSelectionBytes(t, "jpeg"))
			} else {
				pictureMetadataFixtureExec(t, service.pictures.source, `UPDATE tblVPics SET PicComment='changed' WHERE ID=1`)
			}
			if image, err := service.GetSelectedImage(ctx, selected.Token, raw); err == nil || image != nil {
				t.Fatal("changed selection/row preview accepted")
			}
			if pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != 1 {
				t.Fatal("preview pruned history")
			}
			if mutation == "after-create" {
				if receipt, err := service.LookupReceipt(ctx, raw); err != nil || receipt == nil {
					t.Fatal("file replacement blocked metadata recovery", err)
				}
			} else {
				if err := service.AcknowledgeReceipt(ctx, selected.Token); err == nil {
					t.Fatal("unknown changed-row receipt acknowledged")
				}
				if err := service.ReleaseSelection(ctx, selected.Token); err == nil {
					t.Fatal("unknown outcome discarded")
				}
			}
		})
	}
}

func TestPictureCreationServiceImageFinalGuardErrorsReturnNoOutput(t *testing.T) {
	for _, changed := range []string{"library-owner", "native-owner", "cancelled"} {
		t.Run(changed, func(t *testing.T) {
			service, ctx, request := pictureCreationFacadeFixture(t, "png")
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			selected := pictureCreationFacadeChoose(t, service, ctx, request)
			raw := pictureCreationFacadeJSON(t, request)
			if receipt, err := service.Create(ctx, selected.Token, raw); err != nil || receipt == nil {
				t.Fatal(err)
			}
			replacement, _ := pictureLibraryFixture(t, pictureCreationTestSchema)
			originalOwner := service.owner
			var calls atomic.Int32
			service.owner = func(ctx context.Context) (pictureCreationNativeOwner, error) {
				call := calls.Add(1)
				if call == 2 {
					if changed == "library-owner" {
						service.pictures.source = replacement
					}
					if changed == "cancelled" {
						cancel()
					}
				}
				owner, err := originalOwner(ctx)
				if call == 3 && changed == "native-owner" {
					owner.handle = unsafe.Pointer(new(byte))
				}
				return owner, err
			}
			image, err := service.GetSelectedImage(ctx, selected.Token, raw)
			if err == nil || image != nil {
				t.Fatal("failed final image guard returned success-shaped output", image, err)
			}
			if pictureCreationCount(t, service.tokens[selected.Token].source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != 1 {
				t.Fatal("failed image guard erased committed history")
			}
		})
	}
}

func TestPictureCreationServiceRecoverLostSelectionDelivery(t *testing.T) {
	for _, missingFile := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-file-%v", missingFile), func(t *testing.T) {
			service, ctx, request := pictureCreationFacadeFixture(t, "png")
			var chooserCalls int
			service.chooser = func(context.Context) (string, error) {
				chooserCalls++
				return request.SelectedPath, nil
			}
			before := pictureCreationRawRows(t, service.pictures.source)
			if recovered, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber); err != nil || recovered != nil {
				t.Fatal("no selection should recover nil", recovered, err)
			}
			// Ignore delivery entirely: the client never receives this token.
			if _, err := service.Choose(ctx, request.ContextID, request.Project, request.PlotNumber); err != nil {
				t.Fatal(err)
			}
			var expected PictureCreationSelection
			service.mu.Lock()
			for _, capability := range service.tokens {
				expected = capability.dto
			}
			service.mu.Unlock()
			if missingFile {
				if err := os.Remove(request.SelectedPath); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				recovered, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber)
				if err != nil || recovered == nil || *recovered != expected {
					t.Fatal("lost selection was not recovered exactly", recovered, expected, err)
				}
			}
			if chooserCalls != 1 || !reflect.DeepEqual(before, pictureCreationRawRows(t, service.pictures.source)) ||
				pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '__VPRO_PictureCreation%'`) != 0 {
				t.Fatal("selection recovery opened a chooser or wrote creation state")
			}
			if missingFile {
				if image, err := service.GetSelectedImage(ctx, expected.Token, pictureCreationFacadeJSON(t, request)); err == nil || image != nil {
					t.Fatal("stage recovery granted uncommitted file reads")
				}
				if receipt, err := service.Create(ctx, expected.Token, pictureCreationFacadeJSON(t, request)); err == nil || receipt != nil {
					t.Fatal("recovery bypassed selected physical object checks")
				}
			}
			if err := service.ReleaseSelection(ctx, expected.Token); err != nil {
				t.Fatal("recovered unattempted token could not be explicitly released", err)
			}
			if recovered, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber); err != nil || recovered != nil {
				t.Fatal("released selection still recoverable", recovered, err)
			}
			if missingFile {
				pictureSelectionWrite(t, request.SelectedPath, pictureSelectionBytes(t, "png"))
			}
			fresh := pictureCreationFacadeChoose(t, service, ctx, request)
			if fresh.Token == expected.Token || chooserCalls != 2 {
				t.Fatal("explicit release did not permit a genuinely fresh chooser/token")
			}
			raw := pictureCreationFacadeJSON(t, request)
			if receipt, err := service.Create(ctx, fresh.Token, raw); err != nil || receipt == nil {
				t.Fatal(err)
			}
			recovered, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber)
			if err != nil || recovered == nil || *recovered != *fresh {
				t.Fatal("committed selection stage changed during recovery", recovered, err)
			}
			if _, err := service.Create(ctx, recovered.Token, raw); err == nil {
				t.Fatal("stage recovery reset attempted identity or replayed creation")
			}
			if err := service.ReleaseSelection(ctx, recovered.Token); err == nil {
				t.Fatal("stage recovery erased committed outcome")
			}
			if err := service.AcknowledgeReceipt(ctx, recovered.Token); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPictureCreationServiceRecoverSelectionFreshOwnership(t *testing.T) {
	for _, changed := range []string{"app", "window", "handle", "context", "project", "parent", "deleted-parent", "library", "native-postcheck", "cancelled"} {
		t.Run(changed, func(t *testing.T) {
			service, ctx, request := pictureCreationFacadeFixture(t, "png")
			selected := pictureCreationFacadeChoose(t, service, ctx, request)
			before := pictureCreationRawRows(t, service.pictures.source)
			contextID, project, plot := request.ContextID, request.Project, request.PlotNumber
			switch changed {
			case "app", "window", "handle":
				owner := ctx.Value(pictureCreationOwnerTestKey{}).(pictureCreationNativeOwner)
				switch changed {
				case "app":
					owner.app = &application.App{}
				case "window":
					owner.window = &application.WebviewWindow{}
				case "handle":
					owner.handle = unsafe.Pointer(new(byte))
				}
				ctx = context.WithValue(ctx, pictureCreationOwnerTestKey{}, owner)
			case "context":
				contextID += "x"
			case "project":
				project = "Other"
			case "parent":
				plot = "missing"
			case "deleted-parent":
				state, err := service.pictures.contexts.projects.GetState(ctx)
				if err != nil {
					t.Fatal(err)
				}
				mutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Env WHERE PlotNumber='108050'`)
			case "library":
				service.pictures.source, _ = pictureLibraryFixture(t, pictureCreationTestSchema)
			case "native-postcheck":
				originalOwner := service.owner
				var calls int
				service.owner = func(ctx context.Context) (pictureCreationNativeOwner, error) {
					owner, err := originalOwner(ctx)
					calls++
					if calls == 2 {
						owner.handle = unsafe.Pointer(new(byte))
					}
					return owner, err
				}
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			recovered, err := service.RecoverSelection(ctx, contextID, project, plot)
			if err == nil || recovered != nil {
				t.Fatal("selection recovery accepted changed native/scope ownership", recovered, err)
			}
			if len(service.tokens) != 1 || service.tokens[selected.Token].attempted ||
				!reflect.DeepEqual(before, pictureCreationRawRows(t, service.tokens[selected.Token].source)) {
				t.Fatal("refused recovery modified capability or library")
			}
		})
	}
	service, ctx, request := pictureCreationFacadeFixture(t, "png")
	disabled, err := NewPictureCreationService(service.pictures, pictureServiceLookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := disabled.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil || result != nil {
		t.Fatal("disabled selection recovery inferred a production native owner")
	}
	production, err := NewPictureCreationService(service.pictures, pictureServiceLookup(map[string]string{pictureCreationWritingEnvironment: "true"}))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := production.RecoverSelection(t.Context(), request.ContextID, request.Project, request.PlotNumber); err == nil || result != nil {
		t.Fatal("production recovery inferred a native owner")
	}
}

func TestPictureCreationServiceDisabledSelectionRecoveryRequiresOwnedReadingAndNativeScope(t *testing.T) {
	service, ctx, request := pictureCreationFacadeFixture(t, "png")
	before := pictureCreationRawRows(t, service.pictures.source)
	for _, flags := range []map[string]string{nil, {pictureCreationWritingEnvironment: "false"}, {pictureMetadataWritingEnvironment: "true"}} {
		disabled, err := NewPictureCreationService(service.pictures, pictureServiceLookup(flags))
		if err != nil {
			t.Fatal(err)
		}
		disabled.owner = service.owner
		disabled.chooser = func(context.Context) (string, error) {
			t.Fatal("disabled selection recovery opened a chooser")
			return "", nil
		}
		if _, err := disabled.Choose(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil {
			t.Fatal("disabled Choose succeeded")
		}
		recovered, err := disabled.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber)
		if err != nil || recovered != nil {
			t.Fatal("owned readonly recovery failed to prove absence after disabled Choose", recovered, err)
		}
		for _, scope := range [][3]string{
			{request.ContextID + "changed", request.Project, request.PlotNumber},
			{request.ContextID, "Other", request.PlotNumber},
			{request.ContextID, request.Project, "missing"},
		} {
			if result, err := disabled.RecoverSelection(ctx, scope[0], scope[1], scope[2]); err == nil || result != nil {
				t.Fatal("disabled readonly recovery accepted foreign scope", scope, result, err)
			}
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		for _, invalid := range []context.Context{nil, t.Context(), cancelled} {
			if result, err := disabled.RecoverSelection(invalid, request.ContextID, request.Project, request.PlotNumber); err == nil || result != nil {
				t.Fatal("disabled readonly recovery accepted invalid request/native ownership", result, err)
			}
		}
		token := strings.Repeat("a", 64)
		wire := pictureCreationFacadeJSON(t, request)
		if result, err := disabled.Create(ctx, token, wire); err == nil || result != nil {
			t.Fatal("readonly absence enabled Create")
		}
		if result, err := disabled.GetSelectedImage(ctx, token, wire); err == nil || result != nil {
			t.Fatal("readonly absence granted an image")
		}
		if result, err := disabled.LookupReceipt(ctx, wire); err == nil || result != nil {
			t.Fatal("readonly absence enabled unrelated history reading")
		}
		if err := disabled.ReleaseSelection(ctx, token); err == nil {
			t.Fatal("readonly absence enabled capability release")
		}
		if err := disabled.AcknowledgeReceipt(ctx, token); err == nil {
			t.Fatal("readonly absence enabled acknowledgement")
		}
		if len(disabled.tokens) != 0 || len(disabled.requests) != 0 || len(disabled.creating) != 0 ||
			!reflect.DeepEqual(before, pictureCreationRawRows(t, service.pictures.source)) {
			t.Fatal("readonly disabled recovery created authority or changed physical rows")
		}
		disabled.pictures = &PictureService{contexts: service.pictures.contexts, source: service.pictures.source}
		if result, err := disabled.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil || result != nil {
			t.Fatal("readonly disabled recovery bypassed disabled reading")
		}
	}
}

func TestPictureCreationServiceRecoverSelectionRefusesRunningChooser(t *testing.T) {
	service, ctx, request := pictureCreationFacadeFixture(t, "png")
	entered, resume := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-resume:
		default:
			close(resume)
		}
	})
	service.chooser = func(context.Context) (string, error) {
		close(entered)
		<-resume
		return request.SelectedPath, nil
	}
	type outcome struct {
		selection *PictureCreationSelection
		err       error
	}
	done := make(chan outcome, 1)
	go func() {
		selection, err := service.Choose(ctx, request.ContextID, request.Project, request.PlotNumber)
		done <- outcome{selection, err}
	}()
	<-entered
	if result, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil || result != nil ||
		!strings.Contains(err.Error(), "still running") {
		t.Fatal("running modal recovery was not explicitly refused", result, err)
	}
	close(resume)
	result := <-done
	if result.err != nil || result.selection == nil {
		t.Fatal(result)
	}
	recovered, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber)
	if err != nil || recovered == nil || *recovered != *result.selection {
		t.Fatal("chooser completion could not be recovered", recovered, err)
	}
}

func TestPictureCreationServiceInFlightUUIDAndReceiptLookupRefusesUnresolved(t *testing.T) {
	service, ctx, request := pictureCreationFacadeFixture(t, "png")
	selected := pictureCreationFacadeChoose(t, service, ctx, request)
	originalOwner := service.owner
	entered, resume := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	service.owner = func(ctx context.Context) (pictureCreationNativeOwner, error) {
		if calls.Add(1) == 2 {
			close(entered)
			<-resume
		}
		return originalOwner(ctx)
	}
	raw := pictureCreationFacadeJSON(t, request)
	type outcome struct {
		receipt *PictureCreationResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		receipt, err := service.Create(ctx, selected.Token, raw)
		done <- outcome{receipt, err}
	}()
	<-entered
	if result, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil || result != nil ||
		!strings.Contains(err.Error(), "still running") {
		t.Fatal("in-flight writer selection recovery was not explicitly refused", result, err)
	}
	foreignOwner := ctx.Value(pictureCreationOwnerTestKey{}).(pictureCreationNativeOwner)
	foreignOwner.window = &application.WebviewWindow{}
	foreignContext := context.WithValue(ctx, pictureCreationOwnerTestKey{}, foreignOwner)
	foreignSelection := pictureCreationFacadeChoose(t, service, foreignContext, request)
	if _, err := service.Create(foreignContext, foreignSelection.Token, raw); err == nil {
		t.Fatal("in-flight UUID reused by another selected native window")
	}
	for _, uuid := range []string{request.RequestID, pictureCreationTestUUID2} {
		changed := request
		changed.RequestID = uuid
		if _, err := service.Create(ctx, selected.Token, pictureCreationFacadeJSON(t, changed)); err == nil {
			t.Fatal("in-flight token double-submitted/repurposed")
		}
	}
	if receipt, err := service.LookupReceipt(t.Context(), raw); err == nil || receipt != nil || !strings.Contains(err.Error(), "unresolved") {
		t.Fatal("in-flight receipt lookup must refuse a successful null outcome", receipt, err)
	}
	otherRequest := request
	otherRequest.RequestID = pictureCreationTestUUID2
	if receipt, err := service.LookupReceipt(t.Context(), pictureCreationFacadeJSON(t, otherRequest)); err != nil || receipt != nil {
		t.Fatal("unrelated stable receipt lookup was blocked", receipt, err)
	}
	if err := service.ReleaseSelection(ctx, selected.Token); err == nil {
		t.Fatal("premature null lookup cleared running operation")
	}
	if err := service.AcknowledgeReceipt(ctx, selected.Token); err == nil {
		t.Fatal("running operation acknowledged")
	}
	close(resume)
	result := <-done
	if result.err != nil || result.receipt == nil || !result.receipt.DidCommit {
		t.Fatal(result)
	}
	if receipt, err := service.LookupReceipt(t.Context(), raw); err != nil || receipt == nil || !receipt.DidCommit ||
		!receipt.Replayed || receipt.ID != result.receipt.ID || receipt.RowID != result.receipt.RowID {
		t.Fatal("completed writer did not recover its actual committed receipt", receipt, err)
	}
	changed := request
	changed.RequestID = pictureCreationTestUUID2
	if _, err := service.Create(ctx, selected.Token, pictureCreationFacadeJSON(t, changed)); err == nil {
		t.Fatal("completed token repurposed")
	}
	if pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != 1 {
		t.Fatal("concurrent call created duplicate history")
	}
	if err := service.AcknowledgeReceipt(ctx, selected.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(foreignContext, foreignSelection.Token, raw); err == nil {
		t.Fatal("durable UUID replay entered creation through another capability")
	}
	if err := service.ReleaseSelection(foreignContext, foreignSelection.Token); err != nil {
		t.Fatal("unattempted replacement capability could not be relinquished", err)
	}
}

// Hold a planned core context boundary (the final snapshot check by default).
// No facade hook replaces the SQL read or writer; other calls to Err retain
// ordinary cancellation.
type pictureCreationReceiptSnapshotContext struct {
	context.Context
	entered      chan struct{}
	resume       <-chan struct{}
	callerSuffix string
	held         atomic.Bool
}

func (ctx *pictureCreationReceiptSnapshotContext) Err() error {
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		function := runtime.FuncForPC(pc)
		suffix := ctx.callerSuffix
		if suffix == "" {
			suffix = ".readPictureCreationReceipt"
		}
		if function != nil && strings.HasSuffix(function.Name(), suffix) && ctx.held.CompareAndSwap(false, true) {
			close(ctx.entered)
			<-ctx.resume
		}
	}
	return ctx.Context.Err()
}

func TestPictureCreationServiceReceiptLookupRejectsPrewriterNullSnapshot(t *testing.T) {
	service, ctx, request := pictureCreationFacadeFixture(t, "png")
	selected := pictureCreationFacadeChoose(t, service, ctx, request)
	raw := pictureCreationFacadeJSON(t, request)
	resume := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-resume:
		default:
			close(resume)
		}
	})
	lookupContext := &pictureCreationReceiptSnapshotContext{Context: t.Context(), entered: make(chan struct{}), resume: resume}
	type outcome struct {
		receipt *PictureCreationResult
		err     error
	}
	lookupDone := make(chan outcome, 1)
	go func() {
		receipt, err := service.LookupReceipt(lookupContext, raw)
		lookupDone <- outcome{receipt, err}
	}()
	select {
	case <-lookupContext.entered:
	case result := <-lookupDone:
		t.Fatal("actual readonly snapshot did not reach its planned hold", result)
	case <-time.After(5 * time.Second):
		t.Fatal("receipt snapshot did not reach its final context check")
	}
	if pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '__VPRO_PictureCreation%'`) != 0 {
		t.Fatal("held prewriter snapshot was not independently absent")
	}
	createDone := make(chan outcome, 1)
	go func() {
		receipt, err := service.Create(ctx, selected.Token, raw)
		createDone <- outcome{receipt, err}
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		service.mu.Lock()
		bound := service.creating[request.RequestID] != nil
		service.mu.Unlock()
		if bound {
			break
		}
		select {
		case result := <-createDone:
			t.Fatal("writer did not bind during the held lookup", result)
		case <-deadline.C:
			t.Fatal("writer could not bind; lookup must not hold the facade mutex across database callbacks")
		case <-tick.C:
		}
	}
	close(resume)
	lookup := <-lookupDone
	if lookup.err == nil || lookup.receipt != nil || !strings.Contains(lookup.err.Error(), "unresolved") {
		t.Fatal("prewriter snapshot was classified as a successful null after matching Create started", lookup)
	}
	created := <-createDone
	if created.err != nil || created.receipt == nil || !created.receipt.DidCommit {
		t.Fatal("separate internal preflight did not allow the actual writer to commit", created)
	}
	receipt, err := service.LookupReceipt(t.Context(), raw)
	if err != nil || receipt == nil || !receipt.DidCommit || !receipt.Replayed ||
		receipt.ID != created.receipt.ID || receipt.RowID != created.receipt.RowID ||
		pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM "__VPRO_PictureCreationHistory"`) != 1 {
		t.Fatal("completed changed request did not recover its durable receipt", receipt, err)
	}
	service.mu.Lock()
	pending := len(service.reads) + len(service.creating)
	service.mu.Unlock()
	if pending != 0 {
		t.Fatal("completed lookup/writer leaked active request observations", pending)
	}
}

func TestPictureCreationServiceReceiptLookupRemembersFinishedCreate(t *testing.T) {
	service, ctx, request := pictureCreationFacadeFixture(t, "png")
	selected := pictureCreationFacadeChoose(t, service, ctx, request)
	before := pictureCreationRawRows(t, service.pictures.source)
	raw := pictureCreationFacadeJSON(t, request)
	resume := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-resume:
		default:
			close(resume)
		}
	})
	// Hold the core's first context lease check before it takes any ownership
	// mutex, letting a real failed Create start and finish during this read.
	lookupContext := &pictureCreationReceiptSnapshotContext{
		Context: t.Context(), entered: make(chan struct{}), resume: resume, callerSuffix: ".acquireContextLock",
	}
	type outcome struct {
		receipt *PictureCreationResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		receipt, err := service.LookupReceipt(lookupContext, raw)
		done <- outcome{receipt, err}
	}()
	select {
	case <-lookupContext.entered:
	case result := <-done:
		t.Fatal("readonly core did not reach the planned pre-lease hold", result)
	case <-time.After(5 * time.Second):
		t.Fatal("receipt lookup did not reach its first ownership lease")
	}
	if err := os.Remove(request.SelectedPath); err != nil {
		t.Fatal(err)
	}
	if receipt, err := service.Create(ctx, selected.Token, raw); err == nil || receipt != nil {
		t.Fatal("actual selected-file failure was not refused", receipt, err)
	}
	if err := service.ReleaseSelection(ctx, selected.Token); err != nil {
		t.Fatal("unattempted failed Create could not be released", err)
	}
	service.mu.Lock()
	creating, bound := len(service.creating), len(service.requests)
	service.mu.Unlock()
	if creating != 0 || bound != 0 {
		t.Fatal("test did not remove the completed Create reservation before resuming the read")
	}
	close(resume)
	result := <-done
	if result.err == nil || result.receipt != nil || !strings.Contains(result.err.Error(), "unresolved") {
		t.Fatal("finished/released Create hid request activity from an already-running receipt lookup", result)
	}
	if receipt, err := service.LookupReceipt(t.Context(), raw); err != nil || receipt != nil {
		t.Fatal("new completed read could not verify true no-commit", receipt, err)
	}
	if !reflect.DeepEqual(before, pictureCreationRawRows(t, service.pictures.source)) ||
		pictureCreationCount(t, service.pictures.source, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '__VPRO_PictureCreation%'`) != 0 {
		t.Fatal("failed Create/read observation mutated library or technical history")
	}
	service.mu.Lock()
	pending := len(service.reads)
	service.mu.Unlock()
	if pending != 0 {
		t.Fatal("completed read leaked its changed observation", pending)
	}
}

func TestPictureCreationServiceAttemptedNoCommitAndPostCommitError(t *testing.T) {
	t.Run("verified-no-commit", func(t *testing.T) {
		service, ctx, request := pictureCreationFacadeFixture(t, "png")
		selected := pictureCreationFacadeChoose(t, service, ctx, request)
		originalOwner := service.owner
		var calls atomic.Int32
		service.owner = func(ctx context.Context) (pictureCreationNativeOwner, error) {
			if calls.Add(1) == 2 {
				service.pictures.contexts.plots.mu.Lock()
				service.pictures.contexts.plots.currentUser = ""
				service.pictures.contexts.plots.mu.Unlock()
			}
			return originalOwner(ctx)
		}
		raw := pictureCreationFacadeJSON(t, request)
		if receipt, err := service.Create(ctx, selected.Token, raw); err == nil || receipt != nil {
			t.Fatal("invalid actor committed")
		}
		if err := service.ReleaseSelection(ctx, selected.Token); err == nil {
			t.Fatal("lookup error silently discarded unknown request")
		}
		if recovered, err := service.RecoverSelection(ctx, request.ContextID, request.Project, request.PlotNumber); err != nil ||
			recovered == nil || *recovered != *selected {
			t.Fatal("unknown attempted stage could not be recovered without clearing its identity", recovered, err)
		}
		if _, err := service.Choose(ctx, request.ContextID, request.Project, request.PlotNumber); err == nil {
			t.Fatal("unknown attempt replaced")
		}
		if err := service.pictures.contexts.plots.SetCurrentUser(" picture creation actor "); err != nil {
			t.Fatal(err)
		}
		if receipt, err := service.LookupReceipt(ctx, raw); err != nil || receipt != nil {
			t.Fatal("expected independently verified no-commit", receipt, err)
		}
		if _, err := service.Create(ctx, selected.Token, raw); err == nil {
			t.Fatal("unknown attempt implicitly retried")
		}
		if err := service.AcknowledgeReceipt(ctx, selected.Token); err == nil {
			t.Fatal("null receipt acknowledged")
		}
		if err := service.ReleaseSelection(ctx, selected.Token); err != nil {
			t.Fatal(err)
		}
		next := pictureCreationFacadeChoose(t, service, ctx, request)
		if next.Token == selected.Token {
			t.Fatal("explicit release reused a capability token")
		}
		if receipt, err := service.Create(ctx, next.Token, raw); err != nil || receipt == nil || !receipt.DidCommit {
			t.Fatal("verified release did not permit explicit retry with a fresh selection", err)
		}
	})
	t.Run("commit-preserved", func(t *testing.T) {
		service, ctx, request := pictureCreationFacadeFixture(t, "png")
		selected := pictureCreationFacadeChoose(t, service, ctx, request)
		originalOwner := service.owner
		var calls atomic.Int32
		service.owner = func(ctx context.Context) (pictureCreationNativeOwner, error) {
			if calls.Add(1) == 3 {
				return pictureCreationNativeOwner{}, errors.New("postcommit native owner unavailable")
			}
			return originalOwner(ctx)
		}
		raw := pictureCreationFacadeJSON(t, request)
		receipt, err := service.Create(ctx, selected.Token, raw)
		if err == nil || receipt == nil || !receipt.DidCommit {
			t.Fatal("postcommit error erased receipt", receipt, err)
		}
		if recovered, err := service.LookupReceipt(t.Context(), raw); err != nil || recovered == nil || recovered.ID != receipt.ID {
			t.Fatal("postcommit recovery failed", err)
		}
	})
}
