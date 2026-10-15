package main

import "fmt"

func validateSourceParentBooleanNull(column string, originalNull, nextNull bool) error {
	if !originalNull && nextNull && (column == "SV_FloodPlain" || column == "SpeciesListComplete") {
		return fmt.Errorf("%s source Yes/No converts NULL to No; select No explicitly instead of writing SQLite NULL", column)
	}
	return nil
}

func validateSourceParentBooleanAssignments(assignments []siviParentScalarAssignment) error {
	for _, assignment := range assignments {
		if err := validateSourceParentBooleanNull(assignment.Column,
			assignment.Before.Storage == "null", assignment.After.Storage == "null"); err != nil {
			return err
		}
	}
	return nil
}
