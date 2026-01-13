package nba

import "fmt"

// =============================================================================
// NBA Enum Mapping Functions
// =============================================================================
// These functions map Sportradar NBA API enum values to database enum values.
// Returns an error if an unexpected value is encountered.
// =============================================================================

// MapGameStatusToDB maps Sportradar NBA API game status values to database enum values.
// See: https://developer.sportradar.com/basketball/reference/nba-faq#game--series-statuses
// Returns an error if the value is not recognized.
func MapGameStatusToDB(apiStatus string) (string, error) {
	switch apiStatus {
	// Pre-game states map to "scheduled"
	case "scheduled", "created", "time-tbd", "if-necessary":
		return "scheduled", nil
	// Active game states
	case "inprogress":
		return "in_progress", nil
	case "halftime":
		return "halftime", nil
	// Terminal states - passthrough
	case "complete", "closed", "cancelled", "postponed", "delayed":
		return apiStatus, nil
	// Playoff-specific: series clinched early
	case "unnecessary":
		return "cancelled", nil
	default:
		return "", fmt.Errorf("unexpected game status value from Sportradar NBA API: %q", apiStatus)
	}
}

// MapPeriodTypeToDB maps Sportradar NBA API period type values to database enum values.
// Returns an error if the value is not recognized.
func MapPeriodTypeToDB(apiPeriodType string) (string, error) {
	switch apiPeriodType {
	case "quarter":
		return "quarter", nil
	case "overtime":
		return "overtime", nil
	default:
		return "", fmt.Errorf("unexpected period type value from Sportradar NBA API: %q", apiPeriodType)
	}
}

// MapStatTypeToDB maps Sportradar NBA API statistic type values to database enum values.
// Returns an error if the value is not recognized.
// Note: Some stat types (fouldrawn, technicalfoul, attemptblocked, offensivefoul) are excluded
// in exclusions.go and should not reach this function.
func MapStatTypeToDB(apiStatType string) (string, error) {
	switch apiStatType {
	case "fieldgoal":
		return "field_goal", nil
	case "freethrow":
		return "free_throw", nil
	case "assist":
		return "assist", nil
	case "rebound":
		return "rebound", nil
	case "steal":
		return "steal", nil
	case "block":
		return "block", nil
	case "turnover":
		return "turnover", nil
	case "personalfoul":
		return "personal_foul", nil
	default:
		return "", fmt.Errorf("unexpected stat type value from Sportradar NBA API: %q", apiStatType)
	}
}
