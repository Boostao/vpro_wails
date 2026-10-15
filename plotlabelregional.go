package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const plotLabelRegionalDateEnvironment = "VPRO_PLOT_LABEL_REGIONAL_DATE"

type SavedPlotLabelHeader struct {
	PlotNumber       string  `json:"plotNumber"`
	Zone             *string `json:"zone"`
	SubZone          *string `json:"subZone"`
	SiteSeries       *string `json:"siteSeries"`
	PlotRepresenting *string `json:"plotRepresenting"`
	ProjectID        *string `json:"projectId"`
}

type PlotLabelRegionalPreview struct {
	ContextID  string               `json:"contextId"`
	PlotNumber string               `json:"plotNumber"`
	Header     SavedPlotLabelHeader `json:"header"`
	LabelDate  string               `json:"labelDate"`
	ISODate    string               `json:"isoDate"`
	Locale     string               `json:"locale"`
	Pattern    string               `json:"pattern"`
	Source     string               `json:"source"`
}

type plotLabelNativeDate struct {
	text, locale, pattern string
}

type PlotLabelRegionalService struct {
	contexts *ContextService
	enabled  bool
	now      func() time.Time
	format   func(time.Time) (plotLabelNativeDate, error)
}

func NewPlotLabelRegionalService(contexts *ContextService) (*PlotLabelRegionalService, error) {
	if contexts == nil || contexts.projects == nil || contexts.plots == nil {
		return nil, errors.New("regional plot labels require an owned SQLite context")
	}
	enabled, err := siviFeature(plotLabelRegionalDateEnvironment, os.LookupEnv)
	if err != nil {
		return nil, err
	}
	return &PlotLabelRegionalService{contexts: contexts, enabled: enabled, now: time.Now, format: formatPlotLabelNativeDate}, nil
}

func (s *PlotLabelRegionalService) GetPreview(ctx context.Context, contextID, plot string) (*PlotLabelRegionalPreview, error) {
	if s == nil || !s.enabled || s.contexts == nil {
		return nil, errors.New("native regional plot label preview is unavailable; its independent backend gate is disabled")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := plotLabelText(plot, "PlotNumber1", 7); err != nil || plot == "" {
		return nil, errors.Join(err, errors.New("regional plot label requires a complete literal plot number"))
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*PlotLabelRegionalPreview, error) {
		header, err := readSavedPlotLabelHeader(plots, plot)
		if err != nil {
			return nil, fmt.Errorf("regional plot label read: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		date := s.now().Local()
		formatted, err := s.format(date)
		if err != nil {
			return nil, fmt.Errorf("regional plot label date unavailable: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if date.Year() < 1601 || date.Year() > 9999 {
			return nil, errors.New("regional plot label clock is outside the supported Windows calendar range")
		}
		for _, item := range []struct {
			name, value string
			limit       int
		}{
			{"label date", formatted.text, 4095}, {"locale", formatted.locale, 84}, {"date pattern", formatted.pattern, 4095},
		} {
			if item.value == "" {
				return nil, fmt.Errorf("native regional plot label returned an empty %s", item.name)
			}
			if err := plotLabelText(item.value, item.name, item.limit); err != nil {
				return nil, err
			}
		}
		return &PlotLabelRegionalPreview{
			ContextID: contextID, PlotNumber: plot, Header: *header,
			LabelDate: formatted.text, ISODate: date.Format("2006-01-02"),
			Locale: formatted.locale, Pattern: formatted.pattern, Source: "windows-user-long-date",
		}, nil
	})
}

func plotLabelText(value, name string, limit int) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || len(utf16.Encode([]rune(value))) > limit {
		return fmt.Errorf("%s cannot be assigned to a plot label without repairing source text or exceeding its UTF-16 bound", name)
	}
	return nil
}

func readSavedPlotLabelHeader(plots *PlotService, plot string) (header *SavedPlotLabelHeader, err error) {
	db, project, release, err := plots.getActiveDB()
	if err != nil {
		return nil, err
	}
	defer release()
	env, admin := quoteHeaderIdentifier(project+"_Env"), quoteHeaderIdentifier(project+"_Admin")
	query := `SELECT e.PlotNumber,e.Zone,e.SubZone,e.SiteSeries,e.PlotRepresenting,e.ProjectID
FROM main.` + env + ` AS e INNER JOIN main.` + admin + ` AS a
ON typeof(a.Plot)='text' AND CAST(a.Plot AS BLOB)=CAST(e.PlotNumber AS BLOB)
WHERE typeof(e.PlotNumber)='text' AND CAST(e.PlotNumber AS BLOB)=CAST(? AS BLOB)
AND (SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name COLLATE BINARY IN (?,?))=2`
	rows, err := plots.readDB(db).Query(query, plot, project+"_Env", project+"_Admin")
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, rows.Close())
		if err != nil {
			header = nil
		}
	}()
	count := 0
	var values [6]any
	for rows.Next() {
		count++
		if count > 1 {
			return nil, errors.New("saved plot label has ambiguous physical Env/Admin parents")
		}
		if err := rows.Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5]); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, errors.New("saved plot label has no complete byte-exact text Env/Admin parent")
	}
	names := [6]string{"PlotNumber1", "Zone", "SubZone", "SiteSeries", "Representing1", "ProjectID1"}
	limits := [6]int{7, 4, 50, 50, 255, 50}
	var text [6]*string
	for i, raw := range values {
		if raw == nil {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%s requires original TEXT or NULL storage; no coercion", names[i])
		}
		if err := plotLabelText(value, names[i], limits[i]); err != nil {
			return nil, err
		}
		text[i] = &value
	}
	if text[0] == nil || *text[0] != plot || text[1] == nil {
		return nil, errors.New("saved plot label requires its literal plot and non-NULL Zone for source Len/Space")
	}
	combined := *text[1] + strings.Repeat(" ", 4-len(utf16.Encode([]rune(*text[1]))))
	if text[2] != nil {
		combined += *text[2]
	}
	combined += "/"
	if text[3] != nil {
		combined += *text[3]
	}
	if err := plotLabelText(combined, "Zone1", 50); err != nil {
		return nil, err
	}
	return &SavedPlotLabelHeader{*text[0], text[1], text[2], text[3], text[4], text[5]}, nil
}
