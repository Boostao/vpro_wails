package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestTwoPageSoilAttemptRejectsRetargetedOwnerAndRetriesAfterExactCorrection(t *testing.T) {
	contexts, state, _, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
	raw := soilAttemptJSON(t, request)
	owner := contexts.Projects.Sqlite
	original := owner.Attachments["VLists"]
	owner.Attachments["VLists"] = owner.Attachments["project"]
	if outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw); err == nil || outcome != nil {
		t.Fatal("retargeted physical owner certified no commit", outcome, err)
	}
	owner.Attachments["VLists"] = original
	outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "notCommitted" {
		t.Fatal("exact physical owner correction could not finish sealed recovery", outcome, err)
	}
	if result, err := service.Save(context.Background(), state.ContextID, raw); err == nil || result != nil {
		t.Fatal("failed ownership observation unsealed future late dispatch", result, err)
	}
}

func TestTwoPageSoilAttemptAttachmentIdentityIsFrozenBeforeRenameReplacement(t *testing.T) {
	contexts, _, db, _ := soilWriteFixture(t, true, "SoilHumusXL", 3)
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	original := owner.AttachmentInfo["project"]
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	moved := path + ".identity-original"
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replacement.Write([]byte("replacement identity witness")); err != nil {
		t.Fatal(errors.Join(err, replacement.Close()))
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(original, current) {
		t.Fatal("planned attachment Stat lazily adopted the replacement path identity")
	}
	if err := vprocore.ProfileOwnedFiles(owner); err == nil || !strings.Contains(err.Error(), "file identity changed") {
		t.Fatal("profile guard did not observe an actual same-path physical replacement", err)
	}
}
