package nfl

import (
	"reflect"

	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Box Score Comparison Exclusions
// =============================================================================
// Filters out players and stat discrepancies that should not be compared.
//
// Philosophy: Players with all-zero statistics should be excluded from
// comparison as they contribute no meaningful data to verify.
//
// Known Sportradar data inconsistencies are also excluded. These are cases
// where Sportradar's play-by-play data differs from their box score/statistics
// API, causing our derived box scores to differ from their official totals.
// =============================================================================

// statDiscrepancyKey uniquely identifies a known stat discrepancy
type statDiscrepancyKey struct {
	GameID         int
	PlayerVendorID string
	FieldName      string // PascalCase field name (e.g., "PassingYards")
}

// knownStatDiscrepancies contains hardcoded exclusions for Sportradar data inconsistencies.
// These are cases where Sportradar's play-by-play data contains statistics that don't
// match their official box score/statistics summary.
var knownStatDiscrepancies = map[statDiscrepancyKey]string{
	// Add NFL exclusions here as needed, following this format:
	// {GameID: 123, PlayerVendorID: "uuid-here", FieldName: "field_name"}: "Reason for exclusion",
}

// shouldExcludeStatDiscrepancy returns true if the given stat discrepancy is a known
// Sportradar data inconsistency that should be ignored.
func shouldExcludeStatDiscrepancy(gameID int, playerVendorID string, fieldName string) bool {
	key := statDiscrepancyKey{
		GameID:         gameID,
		PlayerVendorID: playerVendorID,
		FieldName:      fieldName,
	}
	_, exists := knownStatDiscrepancies[key]
	return exists
}

// shouldExcludeSportradarPlayer determines whether a Sportradar player should be excluded
// from box score comparison. Returns true if the player has all-zero statistics.
func shouldExcludeSportradarPlayer(stats *models_nfl.NFLStats) bool {
	return hasAllZeroStats(stats)
}

// shouldExcludeDatabasePlayer determines whether a database player should be excluded
// from box score comparison. Returns true if the player has all-zero statistics.
func shouldExcludeDatabasePlayer(stats *models_nfl.NFLStats) bool {
	return hasAllZeroStats(stats)
}

// hasAllZeroStats returns true if all decimal.Decimal fields in the stats struct are zero.
// Uses reflection to automatically check all fields, making it future-proof when new
// statistical fields are added.
func hasAllZeroStats(stats *models_nfl.NFLStats) bool {
	v := reflect.ValueOf(*stats)
	decimalType := reflect.TypeOf(decimal.Decimal{})

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Type() == decimalType {
			d := field.Interface().(decimal.Decimal)
			if !d.IsZero() {
				return false
			}
		}
	}

	return true
}
