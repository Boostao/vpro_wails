package main

import "database/sql"

func regionHeaderFields(h FS882Header) []becHeaderField {
	return []becHeaderField{
		{"FSRegionDistrict", "fsRegionDistrict", "Env", 7, h.FSRegionDistrict},
		{"Ecosection", "ecosection", "Env", 3, h.Ecosection},
	}
}

func isRegionProperty(property string) bool {
	return property == "fsRegionDistrict" || property == "ecosection"
}

func validateRegionHeaderValues(h FS882Header, old *FS882Header) error {
	return validateCodeHeaderValues(h, old, regionHeaderFields, true)
}

func validateRegionHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, validateRegionHeaderValues)
}

func validateRegionRestoreValue(table, column string, value any) error {
	return validateCodeRestoreValue(table, column, value, regionHeaderFields(FS882Header{}))
}
