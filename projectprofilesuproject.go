package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
)

type ProfileSUProjectReview struct {
	SU       ProfileSUReview      `json:"su"`
	Project  string               `json:"project"`
	Path     string               `json:"path"`
	Metadata ProjectMetadataTable `json:"metadata"`
}

type ProfileSUProjectCreation struct {
	Review    ProfileSUProjectReview `json:"review"`
	Name      string                 `json:"name"`
	Confirmed bool                   `json:"confirmed"`
}

func (request *ProfileSUProjectCreation) UnmarshalJSON(data []byte) error {
	type plain ProfileSUProjectCreation
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "review", "name", "confirmed"); err != nil {
		return err
	}
	var raw struct {
		Review json.RawMessage `json:"review"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := decodeProfileLifecycleJSON(raw.Review, &decoded.Review, "su", "project", "path", "metadata"); err != nil {
		return err
	}
	var nested struct {
		SU json.RawMessage `json:"su"`
	}
	if err := json.Unmarshal(raw.Review, &nested); err != nil {
		return err
	}
	if err := decodeProfileSUReviewJSON(nested.SU, &decoded.Review.SU); err != nil {
		return err
	}
	*request = ProfileSUProjectCreation(decoded)
	return nil
}

func readProfileSUProjectReview(ctx context.Context, tx *sql.Tx, filter ProjectPlotProfileFilterRequest, result ProjectPlotProfileResult,
	owner *sqliteContext) (ProfileSUProjectReview, error) {
	if err := owner.validateMetadataWriterFiles(); err != nil {
		return ProfileSUProjectReview{}, err
	}
	review, err := readProfileSUReview(ctx, tx, filter, result)
	if err != nil {
		return ProfileSUProjectReview{}, err
	}
	if review.Descriptions != nil {
		return ProfileSUProjectReview{}, errors.New("existing-project SU creation requires the retained template's absent description metadata; use new-file creation for present metadata")
	}
	metadata, err := readSQLiteStorageRows(ctx, tx, "project", "_table_metadata", "", nil, "")
	if err != nil {
		return ProfileSUProjectReview{}, err
	}
	return ProfileSUProjectReview{SU: review, Project: owner.selection.Project, Path: owner.selection.ProjectPath, Metadata: metadata}, nil
}

func (s *ContextService) ReviewProjectPlotProfileSUInProject(ctx context.Context, contextID string, filter ProjectPlotProfileFilterRequest) (ProfileSUProjectReview, error) {
	var review ProfileSUProjectReview
	_, err := s.runProjectPlotProfileAfter(ctx, contextID, filter.Input, func(tx *sql.Tx, result ProjectPlotProfileResult) error {
		owner := s.projects.sqlite
		var err error
		review, err = readProfileSUProjectReview(ctx, tx, filter, result, owner)
		return err
	}, nil)
	if err != nil {
		return ProfileSUProjectReview{}, err
	}
	return review, nil
}

func (s *ContextService) SaveProjectPlotProfileSUInProject(ctx context.Context, contextID string, request ProfileSUProjectCreation) (ProfileSUCreated, error) {
	return s.saveProfileSUInProject(ctx, contextID, request, nil)
}

func (s *ContextService) saveProfileSUInProject(ctx context.Context, contextID string, request ProfileSUProjectCreation, finalCheck func(*sql.Tx) error) (ProfileSUCreated, error) {
	if err := validateNewProfileName(request.Name, request.Confirmed); err != nil {
		return ProfileSUCreated{}, err
	}
	sources := map[string]ProjectMetadataTable{}
	committed := false
	_, err := s.runProjectPlotProfileAfter(ctx, contextID, request.Review.SU.Filter.Input,
		func(tx *sql.Tx, result ProjectPlotProfileResult) error {
			owner := s.projects.sqlite
			fresh, err := readProfileSUProjectReview(ctx, tx, request.Review.SU.Filter, result, owner)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(fresh, request.Review) {
				return errors.New("SU result/template/SiteUnit or project destination metadata changed since review; no table created")
			}
			for _, suffix := range []string{"_Env", "_Veg", "_Lump"} {
				name := fresh.Project + suffix
				table, err := readSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
				if err != nil {
					return err
				}
				sources[name] = table
			}
			return nil
		}, func(owner *sqliteContext) error {
			return owner.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
				tx, err := conn.BeginTx(ctx, nil)
				if err != nil {
					return err
				}
				defer func() {
					if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
						resultErr = errors.Join(resultErr, err)
					}
				}()
				observe := func() error {
					for name, original := range sources {
						current, err := readSQLiteStorageRows(ctx, tx, "main", name, "", nil, "")
						if err != nil {
							return err
						}
						if !reflect.DeepEqual(current, original) {
							return errors.New("project profile source rows changed during SU creation; table/provenance rolled back")
						}
					}
					metadata, err := readSQLiteStorageRows(ctx, tx, "main", "_table_metadata", "", nil, "")
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(metadata, request.Review.Metadata) {
						return errors.New("original project descriptions changed during SU creation; table/provenance rolled back")
					}
					role, table, err := owner.profileLocation()
					if err != nil {
						return err
					}
					var reader projectMetadataQueryer = owner.conn
					if os.SameFile(owner.attachmentInfo[role], owner.attachmentInfo["project"]) {
						reader, role = tx, "main"
					}
					rules, err := readSQLiteStorageRows(ctx, reader, role, table, "", nil, "Order")
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(rules, request.Review.SU.Filter.Input.OriginalRules) {
						return errors.New("selected profile rules changed during SU creation; table/provenance rolled back")
					}
					if request.Review.SU.SourceSU != nil {
						reader, role = owner.conn, "su"
						if os.SameFile(owner.attachmentInfo["su"], owner.attachmentInfo["project"]) {
							reader, role = tx, "main"
						}
						su, err := readSQLiteStorageRows(ctx, reader, role, owner.selection.SU+"_SU", "", nil, "")
						if err != nil {
							return err
						}
						if !reflect.DeepEqual(su, *request.Review.SU.SourceSU) {
							return errors.New("selected SiteUnit records changed during SU creation; table/provenance rolled back")
						}
					}
					if err := validateProfileSUTemplate(ctx, owner.conn); err != nil {
						return err
					}
					template, err := readSQLiteStorageRows(ctx, owner.conn, "VPro64", "USysSuTable", "", nil, "")
					if err != nil {
						return err
					}
					columns, err := profileMetadataColumns(ctx, owner.conn, "VPro64")
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(template, request.Review.SU.Template) || columns != nil {
						return errors.New("SU source template/descriptions changed during creation; table/provenance rolled back")
					}
					return owner.validateMetadataWriterFiles()
				}
				if err := observe(); err != nil {
					return err
				}
				var collision int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name=? COLLATE NOCASE`, request.Name+"_SU").Scan(&collision); err != nil {
					return err
				}
				if collision != 0 {
					return errors.New("SU name already belongs to a physical project object; no replacement")
				}
				proposal, err := json.Marshal(request)
				if err != nil {
					return err
				}
				if err := writeProfileSUContents(ctx, tx, ProfileSUCreation{request.Review.SU, request.Name, request.Review.Path, true}, string(proposal)); err != nil {
					return err
				}
				if finalCheck != nil {
					if err := finalCheck(tx); err != nil {
						return err
					}
				}
				if err := observe(); err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := tx.Commit(); err != nil {
					return err
				}
				committed = true
				return nil
			})
		})
	if err != nil {
		if committed {
			return ProfileSUCreated{}, errors.Join(errors.New("Profile SU table committed, but cleanup failed; do not replay creation"), err)
		}
		return ProfileSUCreated{}, err
	}
	return ProfileSUCreated{request.Name, request.Review.Path, len(request.Review.SU.Plots)}, nil
}
