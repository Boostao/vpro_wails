package main

import (
	"database/sql"
	"errors"
	"fmt"
)

func soilDrainageHeaderField(h FS882Header) becHeaderField {
	return becHeaderField{"SoilDrainage", "soilDrainage", "Env", 5, h.SoilDrainage}
}

func (s *PlotService) drainageChoices() ([]ParentCodeChoice, error) {
	if s.parentCodesError != nil {
		return nil, s.parentCodesError
	}
	if s.parentCodes != nil {
		return s.parentCodes.ListChoices(s.operationContext(), "SoilDrainage")
	}
	catalogue, err := NewParentCodeService(s.projects.root)
	if err != nil {
		return nil, err
	}
	rows, readErr := catalogue.ListChoices(s.operationContext(), "SoilDrainage")
	return rows, errors.Join(readErr, catalogue.Close())
}

func (s *PlotService) validateSoilDrainageHeaderValues(h FS882Header, old *FS882Header) error {
	if old != nil && sameSiteCode(h.SoilDrainage, old.SoilDrainage) {
		return nil
	}
	field := soilDrainageHeaderField(h)
	if err := validateSiteCodeText(field.name, field.value, field.maximum); err != nil {
		return err
	}
	if field.value == nil {
		return nil
	}
	rows, err := s.drainageChoices()
	if err != nil {
		return fmt.Errorf("SoilDrainage requires a verified SoilDrainage catalogue: %w", err)
	}
	membership := make(map[string]bool)
	for _, row := range rows {
		if row.ListName != nil && *row.ListName == "SoilDrainage" && row.Selectable && row.Code != nil {
			membership[*row.Code] = true
		}
	}
	return canonicalItemError(field.name, "SoilDrainage", *field.value, membership)
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
