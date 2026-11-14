package spatial

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type GoogleEarthPreferences struct {
	PlaceName        string
	DescriptionField string
}

type googleEarthPreferenceUpdate = vprocore.ConfigStringPreferenceUpdate

func decodeGoogleEarthPreferences(values vprocore.ConfigValues) (GoogleEarthPreferences, error) {
	var result GoogleEarthPreferences
	var err error
	result.PlaceName, err = vprocore.ConfigString(values, "ReportOptions", "GoogleEarthPlaceName")
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	result.DescriptionField, err = vprocore.ConfigString(values, "ReportOptions", "GoogleEarthDescField")
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	return result, nil
}

func ReadGoogleEarthPreferences(s *vprocore.DesktopConfig, ctx context.Context) (GoogleEarthPreferences, error) {
	if ctx == nil {
		return GoogleEarthPreferences{}, errors.New("Google Earth preferences require a context")
	}
	if s == nil {
		return GoogleEarthPreferences{}, errors.New("Google Earth preference configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return GoogleEarthPreferences{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	result, err := decodeGoogleEarthPreferences(values)
	if err != nil {
		return GoogleEarthPreferences{}, err
	}
	if err := ctx.Err(); err != nil {
		return GoogleEarthPreferences{}, err
	}
	return result, nil
}

func CompareAndSetGoogleEarthPreferences(s *vprocore.DesktopConfig, ctx context.Context, expected, proposed GoogleEarthPreferences) (googleEarthPreferenceUpdate, error) {
	if ctx == nil {
		return googleEarthPreferenceUpdate{}, errors.New("Google Earth preference changes require a context")
	}
	if s == nil {
		return googleEarthPreferenceUpdate{}, errors.New("Google Earth preference configuration is unavailable")
	}
	if proposed.PlaceName != expected.PlaceName {
		if err := validateGoogleEarthXMLText(proposed.PlaceName); err != nil {
			return googleEarthPreferenceUpdate{}, fmt.Errorf("Google Earth Place Name: %w", err)
		}
	}
	if proposed.DescriptionField != expected.DescriptionField &&
		(!utf8.ValidString(proposed.DescriptionField) || proposed.DescriptionField == "" || strings.ContainsRune(proposed.DescriptionField, 0)) {
		return googleEarthPreferenceUpdate{}, errors.New("Google Earth description field requires a literal nonempty Unicode name without NUL")
	}
	return s.CompareAndSetReportStrings(ctx, "Google Earth preferences", []vprocore.ReportStringPreferenceChange{
		{Key: "GoogleEarthPlaceName", Expected: expected.PlaceName, Proposed: proposed.PlaceName},
		{Key: "GoogleEarthDescField", Expected: expected.DescriptionField, Proposed: proposed.DescriptionField},
	})
}

type GoogleEarthPreferenceValues struct {
	Title            string `json:"title"`
	DescriptionField string `json:"descriptionField"`
}

func (v *GoogleEarthPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthPreferenceValues
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Google Earth preferences", "title", "descriptionField"); err != nil {
		return err
	}
	*v = GoogleEarthPreferenceValues(decoded)
	return nil
}

type GoogleEarthPreferencesRequest struct {
	Expected GoogleEarthPreferenceValues `json:"expected"`
	Proposed GoogleEarthPreferenceValues `json:"proposed"`
}

func (v *GoogleEarthPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Google Earth preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = GoogleEarthPreferencesRequest(decoded)
	return nil
}

type GoogleEarthPreferencesReview struct {
	ContextID   string                      `json:"contextId"`
	Project     string                      `json:"project"`
	ProjectPath string                      `json:"projectPath"`
	SU          string                      `json:"su"`
	SUPath      string                      `json:"suPath"`
	Values      GoogleEarthPreferenceValues `json:"values"`
}

type GoogleEarthPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type GoogleEarthPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewGoogleEarthPreferencesService(contexts *vprocore.ContextService) *GoogleEarthPreferencesService {
	return &GoogleEarthPreferencesService{contexts: contexts}
}

func (s *GoogleEarthPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *GoogleEarthPreferencesService) GetGoogleEarthPreferences(ctx context.Context, contextID string) (*GoogleEarthPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Google Earth",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*GoogleEarthPreferencesReview, error) {
			values, err := ReadGoogleEarthPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			if err := vprocore.ValidateGoogleEarthReviewStrings(values.PlaceName, values.DescriptionField); err != nil {
				return nil, err
			}
			return &GoogleEarthPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: GoogleEarthPreferenceValues{Title: values.PlaceName, DescriptionField: values.DescriptionField},
			}, nil
		})
}

func (s *GoogleEarthPreferencesService) SaveGoogleEarthPreferences(ctx context.Context, contextID, requestJSON string) *GoogleEarthPreferencesOutcome {
	out := GoogleEarthPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Google Earth",
		func(owner *vprocore.SqliteContext, tx *sql.Tx, request GoogleEarthPreferencesRequest) error {
			if request.Proposed.DescriptionField == request.Expected.DescriptionField {
				return nil
			}
			table, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Env")
			if err != nil {
				return err
			}
			for _, column := range table.Columns {
				if column.Name == request.Proposed.DescriptionField {
					return nil
				}
			}
			return errors.New("new Google Earth description field is not a current owned physical Env field")
		},
		func(plots *vprocore.PlotService, request GoogleEarthPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetGoogleEarthPreferences(plots.Projects.Preferences, ctx,
				GoogleEarthPreferences{PlaceName: request.Expected.Title, DescriptionField: request.Expected.DescriptionField},
				GoogleEarthPreferences{PlaceName: request.Proposed.Title, DescriptionField: request.Proposed.DescriptionField})
		}))
	return &out
}
