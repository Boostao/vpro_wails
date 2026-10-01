package main

import "database/sql"

func geologyHeaderFields(h FS882Header) []becHeaderField {
	return []becHeaderField{
		{"BedrockGeology1", "bedrockGeology1", "Env", 4, h.BedrockGeology1},
		{"BedrockGeology2", "bedrockGeology2", "Env", 4, h.BedrockGeology2},
		{"BedrockGeology3", "bedrockGeology3", "Env", 4, h.BedrockGeology3},
	}
}

func isGeologyProperty(property string) bool {
	return property == "bedrockGeology1" || property == "bedrockGeology2" || property == "bedrockGeology3"
}

func validateGeologyHeaderValues(h FS882Header, old *FS882Header) error {
	return validateCodeHeaderValues(h, old, geologyHeaderFields, false)
}

func validateGeologyHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, validateGeologyHeaderValues)
}

func validateGeologyRestoreValue(table, column string, value any) error {
	return validateCodeRestoreValue(table, column, value, geologyHeaderFields(FS882Header{}))
}
