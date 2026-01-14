package persister

import "fmt"

// =============================================================================
// General Enum Mapping Functions
// =============================================================================
// These functions map Sportradar API enum values to database enum values
// for enums that are shared across sports (not sport-specific).
// Returns an error if an unexpected value is encountered.
// =============================================================================

// MapIndividualStatusToDB maps Sportradar API individual status values to database enum values.
// The API may use Title Case or space-separated values, while the DB uses snake_case.
// Returns an error if the value is not recognized.
func MapIndividualStatusToDB(apiStatus string) (string, error) {
	switch apiStatus {
	//  API variants that may exist
	case "Active":
		return "active", nil
	case "Day To Day", "day to day", "Day to Day":
		return "day_to_day", nil
	case "Doubtful":
		return "doubtful", nil
	case "Out":
		return "out", nil
	case "Out For Season", "out for season", "Out for Season":
		return "out_for_season", nil
	case "Questionable":
		return "questionable", nil
	case "Unknown":
		return "unknown", nil
	default:
		return "", fmt.Errorf("unexpected individual status value from Sportradar API: %q", apiStatus)
	}
}
