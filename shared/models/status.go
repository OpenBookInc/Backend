package models

import "fmt"

// IndividualStatusType represents the valid individual status enum values
type IndividualStatusType string

const (
	StatusActive        IndividualStatusType = "Active"
	StatusDayToDay      IndividualStatusType = "Day To Day"
	StatusDoubtful      IndividualStatusType = "Doubtful"
	StatusOut           IndividualStatusType = "Out"
	StatusOutForSeason  IndividualStatusType = "Out For Season"
	StatusQuestionable  IndividualStatusType = "Questionable"
)

// ValidateIndividualStatusType checks if a status string is a valid IndividualStatusType enum value
// Returns the validated IndividualStatusType or an error if invalid
func ValidateIndividualStatusType(status string) (IndividualStatusType, error) {
	switch IndividualStatusType(status) {
	case StatusActive, StatusDayToDay, StatusDoubtful, StatusOut, StatusOutForSeason, StatusQuestionable:
		return IndividualStatusType(status), nil
	default:
		return "", fmt.Errorf("invalid individual status: '%s' (valid values: Active, Day To Day, Doubtful, Out, Out For Season, Questionable)", status)
	}
}

// AllIndividualStatusTypes returns a slice of all valid individual status values
func AllIndividualStatusTypes() []IndividualStatusType {
	return []IndividualStatusType{
		StatusActive,
		StatusDayToDay,
		StatusDoubtful,
		StatusOut,
		StatusOutForSeason,
		StatusQuestionable,
	}
}
