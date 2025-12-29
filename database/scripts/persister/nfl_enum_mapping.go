package persister

import "fmt"

// =============================================================================
// NFL Enum Mapping Functions
// =============================================================================
// These functions map Sportradar NFL API enum values to database enum values.
// Returns an error if an unexpected value is encountered.
// =============================================================================

// MapPeriodTypeToDB maps Sportradar API period type values to database enum values.
// Returns an error if the value is not recognized.
func MapPeriodTypeToDB(apiPeriodType string) (string, error) {
	switch apiPeriodType {
	case "quarter":
		return "quarter", nil
	case "overtime":
		return "overtime", nil
	default:
		return "", fmt.Errorf("unexpected period type value from Sportradar API: %q", apiPeriodType)
	}
}

// MapStatTypeToDB maps Sportradar API stat type values to database enum values.
// Returns an error if the value is not recognized.
func MapStatTypeToDB(apiStatType string) (string, error) {
	switch apiStatType {
	// Map API variants to canonical DB values
	case "pass":
		return "passing", nil
	case "rush":
		return "rushing", nil
	case "receive":
		return "receiving", nil
	case "defense":
		return "defense", nil
	case "fumble":
		return "fumble", nil
	case "interception":
		return "interception", nil
	case "field_goal":
		return "field_goal", nil
	case "extra_point":
		return "extra_point", nil
	default:
		return "", fmt.Errorf("unexpected stat type value from Sportradar API: %q", apiStatType)
	}
}

// MapGameStatusToDB maps Sportradar API game status values to database enum values.
// Sportradar uses different status strings that we map to our database enum.
// Returns an error if the value is not recognized.
func MapGameStatusToDB(apiStatus string) (string, error) {
	switch apiStatus {
	// Pre-game states all map to "scheduled"
	case "created", "flex-schedule", "time-tbd":
		return "scheduled", nil
	// Active game states map to "in_progress"
	case "inprogress", "halftime":
		return "in_progress", nil
	// Terminal states
	case "complete", "closed", "cancelled", "postponed", "delayed", "suspended":
		return apiStatus, nil
	default:
		return "", fmt.Errorf("unexpected game status value from Sportradar API: %q", apiStatus)
	}
}
