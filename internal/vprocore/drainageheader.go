package vprocore

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"errors"
	"fmt"
)

func soilDrainageHeaderField(h FS882Header) BecHeaderField {
	return BecHeaderField{"SoilDrainage", "soilDrainage", "Env", 5, h.SoilDrainage}
}

func (s *PlotService) DrainageChoices() ([]references.ParentCodeChoice, error) {
	if s.ParentCodesError != nil {
		return nil, s.ParentCodesError
	}
	if s.ParentCodes != nil {
		return s.ParentCodes.ListChoices(s.OperationContext(), "SoilDrainage")
	}
	catalogue, err := references.NewParentCodeService(s.Projects.Root)
	if err != nil {
		return nil, err
	}
	rows, readErr := catalogue.ListChoices(s.OperationContext(), "SoilDrainage")
	return rows, errors.Join(readErr, catalogue.Close())
}

func (s *PlotService) validateSoilDrainageHeaderValues(h FS882Header, old *FS882Header) error {
	if old != nil && SameSiteCode(h.SoilDrainage, old.SoilDrainage) {
		return nil
	}
	field := soilDrainageHeaderField(h)
	if err := sqlutil.ValidateSiteCodeText(field.Name, field.Value, field.Maximum); err != nil {
		return err
	}
	if field.Value == nil {
		return nil
	}
	rows, err := s.DrainageChoices()
	if err != nil {
		return fmt.Errorf("SoilDrainage requires a verified SoilDrainage catalogue: %w", err)
	}
	membership := make(map[string]bool)
	for _, row := range rows {
		if row.ListName != nil && *row.ListName == "SoilDrainage" && row.Selectable && row.Code != nil {
			membership[*row.Code] = true
		}
	}
	return CanonicalItemError(field.Name, "SoilDrainage", *field.Value, membership)
}

func (s *PlotService) validateSoilDrainageBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, s.validateSoilDrainageHeaderValues)
}

func (s *PlotService) validateSoilDrainageRestoreValue(table, column string, value any) error {
	if table != "Env" || column != "SoilDrainage" || value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return errors.New("SoilDrainage audit restore requires nullable text")
	}
	return s.validateSoilDrainageHeaderValues(FS882Header{SoilDrainage: &text}, nil)
}
