package main

func validateSourceParentBooleanAssignments(assignments []siviParentScalarAssignment) error {
	for _, assignment := range assignments {
		if err := validateSourceParentBooleanNull(assignment.Column,
			assignment.Before.Storage == "null", assignment.After.Storage == "null"); err != nil {
			return err
		}
	}
	return nil
}
