package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"errors"
	"strings"
)

func MasterBECAllowed(user string) bool {
	return strings.EqualFold(user, "Will MacKenzie")
}

func (s *PlotService) CanEditMasterBEC() bool {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return MasterBECAllowed(s.CurrentUser)
}

func masterBECHeaderField(h FS882Header) BecHeaderField {
	return BecHeaderField{"BECSiteUnit", "becSiteUnit", "Admin", 100, h.BECSiteUnit}
}

func validateMasterBECHeaderValues(h FS882Header, old *FS882Header, allowed bool) error {
	if old != nil && SameSiteCode(h.BECSiteUnit, old.BECSiteUnit) {
		return nil
	}
	if old == nil && h.BECSiteUnit == nil {
		return nil
	}
	if !allowed {
		return errors.New("BEC Master editing is restricted to the source-authorized user; use Working Unit instead")
	}
	return sqlutil.ValidateSiteCodeText("BECSiteUnit", h.BECSiteUnit, 100)
}

func (s *PlotService) validateMasterBECRestoreValue(table, column string, value any) error {
	if table != "Admin" || column != "BECSiteUnit" {
		return nil
	}
	if !s.CanEditMasterBEC() {
		return errors.New("BEC Master restoration is restricted to the source-authorized user")
	}
	return validateCodeRestoreValue(table, column, value, []BecHeaderField{masterBECHeaderField(FS882Header{})})
}
