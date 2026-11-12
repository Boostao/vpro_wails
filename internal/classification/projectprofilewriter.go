package classification

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
)

type PlotProfileWriteRequest struct {
	Review    ProjectPlotProfileReview `json:"review"`
	Enabled   bool                     `json:"enabled"`
	Confirmed bool                     `json:"confirmed"`
}

func (request *PlotProfileWriteRequest) UnmarshalJSON(data []byte) error {
	type plain PlotProfileWriteRequest
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "review", "enabled", "confirmed"); err != nil {
		return err
	}
	*request = PlotProfileWriteRequest(decoded)
	return nil
}

func ValidateProfileWriterFiles(c *vprocore.SqliteContext) error {
	if err := vprocore.ProfileOwnedFiles(c); err != nil {
		return err
	}
	role, _, err := c.ProfileLocation()
	if err != nil {
		return err
	}
	target := c.AttachmentInfo[role]
	if target == nil || c.AttachmentInfo["VLists"] == nil {
		return errors.New("profile editing requires identified profile and reference files")
	}
	for owner, info := range c.AttachmentInfo {
		if strings.HasPrefix(owner, "V") && os.SameFile(target, info) {
			return fmt.Errorf("profile file is also owned by %s; support-file writes are unavailable", owner)
		}
	}
	return nil
}

func WithProfileWriter(c *vprocore.SqliteContext, ctx context.Context, operation func(*sql.Conn) error) error {
	role, _, err := c.ProfileLocation()
	if err != nil {
		return err
	}
	return c.WithSelectedFileWriter(ctx, role, func() error {
		if err := ValidateProfileWriterFiles(c); err != nil {
			return err
		}
		info, err := c.ProfileInfo(ctx)
		if err != nil {
			return err
		}
		if !info.Available || !info.Writable {
			return errors.New("selected profile is read-only; explicit current-context write authorization is required")
		}
		return nil
	}, operation)
}

func SetPlotProfileEditing(s *vprocore.ContextService, ctx context.Context, expectedID string, request PlotProfileWriteRequest) (result vprocore.ProjectState, resultErr error) {
	if !request.Confirmed {
		return vprocore.ProjectState{}, errors.New("profile write ownership requires explicit confirmation")
	}
	if err := sqlutil.AcquireContextLock(ctx, s.Projects.OperationMu.TryLock, s.Projects.OperationMu.Lock, s.Projects.OperationMu.Unlock); err != nil {
		return vprocore.ProjectState{}, err
	}
	defer s.Projects.OperationMu.Unlock()
	if err := sqlutil.AcquireContextLock(ctx, s.Projects.Mu.TryLock, s.Projects.Mu.Lock, s.Projects.Mu.Unlock); err != nil {
		return vprocore.ProjectState{}, err
	}
	defer s.Projects.Mu.Unlock()
	owner := s.Projects.Sqlite
	if expectedID == "" || expectedID != s.Projects.ContextID || owner == nil || owner.Conn == nil {
		return vprocore.ProjectState{}, errors.New("profile context changed or closed; review again before authorizing editing")
	}
	review, err := readProjectPlotProfile(ctx, owner)
	if err != nil {
		return vprocore.ProjectState{}, err
	}
	if !reflect.DeepEqual(review, request.Review) {
		return vprocore.ProjectState{}, errors.New("profile source/rules/descriptions changed since review; authorization was not published")
	}
	if owner.Profile == nil || !review.Source.Available ||
		review.Source.Source.Name == owner.Selection.Project &&
			vprocore.SameDesktopPath(review.Source.Source.Path, owner.Selection.ProjectPath) {
		return vprocore.ProjectState{}, errors.New("project-owned profile editing already follows the existing editor policy; separate authorization is for other selected profiles")
	}
	if err := ValidateProfileWriterFiles(owner); err != nil {
		return vprocore.ProjectState{}, err
	}
	candidate, err := vprocore.NewSQLiteContext(ctx, owner.Selection, s.Projects.SupportPaths, owner.Profile)
	if err != nil {
		return vprocore.ProjectState{}, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, candidate.Close())
			if resultErr != nil {
				result = vprocore.ProjectState{}
			}
		}
	}()
	fresh, err := readProjectPlotProfile(ctx, candidate)
	if err != nil {
		return vprocore.ProjectState{}, err
	}
	// A candidate starts read-only even when the previous context held a grant.
	fresh.Source.Writable = review.Source.Writable
	if !reflect.DeepEqual(review, fresh) {
		return vprocore.ProjectState{}, errors.New("profile candidate differs from the reviewed source; authorization was not published")
	}
	if err := ValidateProfileWriterFiles(owner); err != nil {
		return vprocore.ProjectState{}, err
	}
	if err := ValidateProfileWriterFiles(candidate); err != nil {
		return vprocore.ProjectState{}, err
	}
	candidate.ProfileWrite = request.Enabled
	id, err := vprocore.NewContextIdentity()
	if err != nil {
		return vprocore.ProjectState{}, err
	}
	oldID := s.Projects.ContextID
	s.Projects.Sqlite, s.Projects.ContextID = candidate, id
	state, err := s.Projects.SqliteStateContextLocked(ctx)
	s.Projects.Sqlite, s.Projects.ContextID = owner, oldID
	if err != nil {
		return vprocore.ProjectState{}, err
	}
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectState{}, err
	}
	if err := ValidateProfileWriterFiles(owner); err != nil {
		return vprocore.ProjectState{}, err
	}
	s.Projects.PublishSQLiteSelection(candidate, id)
	published = true
	if err := owner.Close(); err != nil {
		log.Printf("Warning: profile authorization published, but previous context could not close: %v", err)
	}
	return state, nil
}
