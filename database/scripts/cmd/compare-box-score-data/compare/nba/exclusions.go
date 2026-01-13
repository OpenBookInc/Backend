package nba

import (
	"reflect"

	models_nba "github.com/openbook/shared/models/nba"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Box Score Comparison Exclusions
// =============================================================================
// Filters out players from box scores that should not be compared.
//
// Philosophy: Players with all-zero statistics should be excluded from
// comparison as they contribute no meaningful data to verify.
// =============================================================================

// shouldExcludeSportradarPlayer determines whether a Sportradar player should be excluded
// from box score comparison. Returns true if the player has all-zero statistics.
func shouldExcludeSportradarPlayer(stats *models_nba.NBAStats) bool {
	return hasAllZeroStats(stats)
}

// shouldExcludeDatabasePlayer determines whether a database player should be excluded
// from box score comparison. Returns true if the player has all-zero statistics.
func shouldExcludeDatabasePlayer(stats *models_nba.NBAStats) bool {
	return hasAllZeroStats(stats)
}

// hasAllZeroStats returns true if all decimal.Decimal fields in the stats struct are zero.
// Uses reflection to automatically check all fields, making it future-proof when new
// statistical fields are added.
func hasAllZeroStats(stats *models_nba.NBAStats) bool {
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
