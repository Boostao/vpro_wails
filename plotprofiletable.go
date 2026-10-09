package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
)

type PlotProfileTableReview struct {
	Template PlotProfileFileReview    `json:"template"`
	Profile  ProjectPlotProfileReview `json:"profile"`
}

type PlotProfileTableCreation struct {
	Review    PlotProfileTableReview `json:"review"`
	Name      string                 `json:"name"`
	Confirmed bool                   `json:"confirmed"`
}

func (review *PlotProfileTableReview) UnmarshalJSON(data []byte) error {
	type plain PlotProfileTableReview
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "template", "profile"); err != nil {
		return err
	}
	*review = PlotProfileTableReview(decoded)
	return nil
}

func (request *PlotProfileTableCreation) UnmarshalJSON(data []byte) error {
	type plain PlotProfileTableCreation
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "review", "name", "confirmed"); err != nil {
		return err
	}
	*request = PlotProfileTableCreation(decoded)
	return nil
}

func readPlotProfileTableReview(ctx context.Context, c *sqliteContext) (PlotProfileTableReview, error) {
	if err := c.validateProfileWriterFiles(); err != nil {
		return PlotProfileTableReview{}, err
	}
	profile, err := readProjectPlotProfile(ctx, c)
	if err != nil {
		return PlotProfileTableReview{}, err
	}
	if !profile.Source.Available || !profile.Source.Writable {
		return PlotProfileTableReview{}, errors.New("existing-file profile creation requires current selected-profile write ownership")
	}
	template, err := readPlotProfileFileReview(ctx, c.conn)
	if err != nil {
		return PlotProfileTableReview{}, err
	}
	return PlotProfileTableReview{Template: template, Profile: profile}, c.validateProfileWriterFiles()
}

func (s *ContextService) ReviewPlotProfileTableCreation(ctx context.Context, contextID string) (PlotProfileTableReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (PlotProfileTableReview, error) {
		return readPlotProfileTableReview(ctx, plots.projects.sqlite)
	})
}

func (s *ContextService) CreatePlotProfileTable(ctx context.Context, contextID string, request PlotProfileTableCreation) (PlotProfileFileCreated, error) {
	if err := validateNewProfileName(request.Name, request.Confirmed); err != nil {
		return PlotProfileFileCreated{}, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (PlotProfileFileCreated, error) {
		return plots.projects.sqlite.createProfileTable(ctx, request, nil)
	})
}

func (c *sqliteContext) createProfileTable(ctx context.Context, request PlotProfileTableCreation, finalCheck func(*sql.Tx) error) (PlotProfileFileCreated, error) {
	fresh, err := readPlotProfileTableReview(ctx, c)
	if err != nil {
		return PlotProfileFileCreated{}, err
	}
	if !reflect.DeepEqual(fresh, request.Review) {
		return PlotProfileFileCreated{}, errors.New("selected profile ownership/rules/descriptions/template changed since review; no table created")
	}
	proposal, err := json.Marshal(request)
	if err != nil {
		return PlotProfileFileCreated{}, err
	}
	committed := false
	err = c.withProfileWriter(ctx, func(conn *sql.Conn) (resultErr error) {
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
			rules, err := readSQLiteStorageRows(ctx, tx, "main", fresh.Profile.Table, "", nil, "Order")
			if err != nil {
				return err
			}
			descriptions, err := readProfileDescriptions(ctx, tx, "main", fresh.Profile.Table)
			if err != nil {
				return err
			}
			template, err := readPlotProfileFileReview(ctx, c.conn)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(rules, fresh.Profile.Rules) || !reflect.DeepEqual(descriptions, fresh.Profile.Descriptions) ||
				!reflect.DeepEqual(template, fresh.Template) {
				return errors.New("selected profile/template changed during creation; table and provenance rolled back")
			}
			return c.validateProfileWriterFiles()
		}
		if err := observe(); err != nil {
			return err
		}
		if err := createBlankProfileTable(ctx, tx, request.Name, fresh.Template.Template, string(proposal)); err != nil {
			return err
		}
		if err := observe(); err != nil {
			return err
		}
		if finalCheck != nil {
			if err := finalCheck(tx); err != nil {
				return err
			}
		}
		if err := c.validateProfileWriterFiles(); err != nil {
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
	if err != nil {
		if committed {
			return PlotProfileFileCreated{}, errors.Join(errors.New("Profile table committed, but cleanup failed; do not replay creation"), err)
		}
		return PlotProfileFileCreated{}, err
	}
	return PlotProfileFileCreated{Source: PlotProfileSource{request.Name, fresh.Profile.Source.Source.Path},
		Table: request.Name + "_Profile", RuleCount: 0}, nil
}
