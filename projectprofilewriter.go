package main

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
	if err := decodeProfileLifecycleJSON(data, &decoded, "review", "enabled", "confirmed"); err != nil {
		return err
	}
	*request = PlotProfileWriteRequest(decoded)
	return nil
}

func (c *sqliteContext) validateProfileWriterFiles() error {
	if err := profileOwnedFiles(c); err != nil {
		return err
	}
	role, _, err := c.profileLocation()
	if err != nil {
		return err
	}
	target := c.attachmentInfo[role]
	if target == nil || c.attachmentInfo["VLists"] == nil {
		return errors.New("profile editing requires identified profile and reference files")
	}
	for owner, info := range c.attachmentInfo {
		if strings.HasPrefix(owner, "V") && os.SameFile(target, info) {
			return fmt.Errorf("profile file is also owned by %s; support-file writes are unavailable", owner)
		}
	}
	return nil
}

func (c *sqliteContext) withProfileWriter(ctx context.Context, operation func(*sql.Conn) error) error {
	role, _, err := c.profileLocation()
	if err != nil {
		return err
	}
	return c.withSelectedFileWriter(ctx, role, func() error {
		if err := c.validateProfileWriterFiles(); err != nil {
			return err
		}
		info, err := c.profileInfo(ctx)
		if err != nil {
			return err
		}
		if !info.Available || !info.Writable {
			return errors.New("selected profile is read-only; explicit current-context write authorization is required")
		}
		return nil
	}, operation)
}

func (s *ContextService) SetPlotProfileEditing(ctx context.Context, expectedID string, request PlotProfileWriteRequest) (result ProjectState, resultErr error) {
	if !request.Confirmed {
		return ProjectState{}, errors.New("profile write ownership requires explicit confirmation")
	}
	if err := acquireContextLock(ctx, s.projects.operationMu.TryLock, s.projects.operationMu.Lock, s.projects.operationMu.Unlock); err != nil {
		return ProjectState{}, err
	}
	defer s.projects.operationMu.Unlock()
	if err := acquireContextLock(ctx, s.projects.mu.TryLock, s.projects.mu.Lock, s.projects.mu.Unlock); err != nil {
		return ProjectState{}, err
	}
	defer s.projects.mu.Unlock()
	owner := s.projects.sqlite
	if expectedID == "" || expectedID != s.projects.contextID || owner == nil || owner.conn == nil {
		return ProjectState{}, errors.New("profile context changed or closed; review again before authorizing editing")
	}
	review, err := readProjectPlotProfile(ctx, owner)
	if err != nil {
		return ProjectState{}, err
	}
	if !reflect.DeepEqual(review, request.Review) {
		return ProjectState{}, errors.New("profile source/rules/descriptions changed since review; authorization was not published")
	}
	if owner.profile == nil || !review.Source.Available ||
		review.Source.Source.Name == owner.selection.Project &&
			sameDesktopPath(review.Source.Source.Path, owner.selection.ProjectPath) {
		return ProjectState{}, errors.New("project-owned profile editing already follows the existing editor policy; separate authorization is for other selected profiles")
	}
	if err := owner.validateProfileWriterFiles(); err != nil {
		return ProjectState{}, err
	}
	candidate, err := newSQLiteContext(ctx, owner.selection, s.projects.supportPaths, owner.profile)
	if err != nil {
		return ProjectState{}, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, candidate.Close())
			if resultErr != nil {
				result = ProjectState{}
			}
		}
	}()
	fresh, err := readProjectPlotProfile(ctx, candidate)
	if err != nil {
		return ProjectState{}, err
	}
	// A candidate starts read-only even when the previous context held a grant.
	fresh.Source.Writable = review.Source.Writable
	if !reflect.DeepEqual(review, fresh) {
		return ProjectState{}, errors.New("profile candidate differs from the reviewed source; authorization was not published")
	}
	if err := owner.validateProfileWriterFiles(); err != nil {
		return ProjectState{}, err
	}
	if err := candidate.validateProfileWriterFiles(); err != nil {
		return ProjectState{}, err
	}
	candidate.profileWrite = request.Enabled
	id, err := newContextIdentity()
	if err != nil {
		return ProjectState{}, err
	}
	oldID := s.projects.contextID
	s.projects.sqlite, s.projects.contextID = candidate, id
	state, err := s.projects.sqliteStateContextLocked(ctx)
	s.projects.sqlite, s.projects.contextID = owner, oldID
	if err != nil {
		return ProjectState{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProjectState{}, err
	}
	if err := owner.validateProfileWriterFiles(); err != nil {
		return ProjectState{}, err
	}
	s.projects.publishSQLiteSelection(candidate, id)
	published = true
	if err := owner.Close(); err != nil {
		log.Printf("Warning: profile authorization published, but previous context could not close: %v", err)
	}
	return state, nil
}
