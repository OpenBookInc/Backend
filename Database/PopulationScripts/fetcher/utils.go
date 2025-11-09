package fetcher

import (
	"fmt"
	"strconv"
	"strings"
)

// parseHeight converts height (string or number) to total inches
// Handles formats like "6-7", "6'7\"", or numeric inches
func parseHeight(height interface{}) int {
	switch v := height.(type) {
	case string:
		return parseHeightString(v)
	case float64:
		// API returned numeric inches directly
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// parseHeightString converts height string (e.g., "6-7" or "6'7\"") to total inches
func parseHeightString(heightStr string) int {
	// Handle formats like "6-7" or "6'7\""
	heightStr = strings.ReplaceAll(heightStr, "'", "-")
	heightStr = strings.ReplaceAll(heightStr, "\"", "")
	heightStr = strings.TrimSpace(heightStr)

	parts := strings.Split(heightStr, "-")
	if len(parts) != 2 {
		return 0
	}

	feet, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	inches, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

	if err1 != nil || err2 != nil {
		return 0
	}

	return feet*12 + inches
}

// parseWeight converts weight (string or number) to integer pounds
func parseWeight(weight interface{}) int {
	switch v := weight.(type) {
	case string:
		return parseWeightString(v)
	case float64:
		// API returned numeric pounds directly
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// parseWeightString converts weight string to integer pounds
func parseWeightString(weightStr string) int {
	// Remove "lbs" or any non-numeric characters except the number itself
	weightStr = strings.TrimSpace(weightStr)
	weightStr = strings.ReplaceAll(weightStr, "lbs", "")
	weightStr = strings.ReplaceAll(weightStr, "lb", "")
	weightStr = strings.TrimSpace(weightStr)

	// Handle potential float strings
	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil {
		return 0
	}

	return int(weight)
}

// convertToString converts an interface{} to string for debugging/display
func convertToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
