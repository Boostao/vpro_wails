package vprocore

import "database/sql"

func SoilHeaderFields(h FS882Header) []BecHeaderField {
	return []BecHeaderField{
		{"SoilClassGroup", "soilClassGroup", "Env", 4, h.SoilClassGroup},
		{"SoilClassSubGroup", "soilClassSubGroup", "Env", 4, h.SoilClassSubGroup},
	}
}

func isSoilProperty(property string) bool {
	return property == "soilClassGroup" || property == "soilClassSubGroup"
}

func validateSoilHeaderValues(h FS882Header, old *FS882Header) error {
	return validateCodeHeaderValues(h, old, SoilHeaderFields, false)
}

func validateSoilHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, validateSoilHeaderValues)
}

func ValidateSoilRestoreValue(table, column string, value any) error {
	return validateCodeRestoreValue(table, column, value, SoilHeaderFields(FS882Header{}))
}
