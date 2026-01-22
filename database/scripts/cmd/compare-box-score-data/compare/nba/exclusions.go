package nba

import (
	"reflect"

	models_nba "github.com/openbook/shared/models/nba"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Box Score Comparison Exclusions
// =============================================================================
// Filters out players and stat discrepancies that should not be compared.
//
// Philosophy: Players with all-zero statistics should be excluded from
// comparison as they contribute no meaningful data to verify.
//
// Known Sportradar data inconsistencies are also excluded. These are cases
// where Sportradar's play-by-play data differs from their box score summary
// API, causing our derived box scores to differ from their official totals.
// =============================================================================

// statDiscrepancyKey uniquely identifies a known stat discrepancy
type statDiscrepancyKey struct {
	GameID         int
	PlayerVendorID string
	FieldName      string // PascalCase field name (e.g., "Steals")
}

// knownStatDiscrepancies contains hardcoded exclusions for Sportradar data inconsistencies.
// These are cases where Sportradar's play-by-play data contains statistics that don't
// match their official box score summary.
var knownStatDiscrepancies = map[statDiscrepancyKey]string{
	// OKC @ SAS on 2025-12-23: Sportradar play-by-play shows Cason Wallace with a steal
	// on event f24cf5aa-31da-46ec-b472-dced2d72ab77, but their box score summary shows 0 steals.
	{GameID: 1606, PlayerVendorID: "b3db0b36-344f-4035-b315-9fb18933e535", FieldName: "Steals"}: "Sportradar API inconsistency: play-by-play shows steal, box score shows 0",

	// OKC @ SAS on 2025-12-23: Sportradar box score shows Aaron Wiggins with 1 steal,
	// but their play-by-play API contains no steal events for him.
	{GameID: 1606, PlayerVendorID: "d3b28775-473d-471a-8452-e913f4347f0f", FieldName: "Steals"}: "Sportradar API inconsistency: box score shows 1 steal, play-by-play shows 0",

	// GSW @ PHI on 2025-12-04: Sportradar box score shows Joel Embiid with 8 two-point attempts,
	// but their play-by-play API only contains 7 two-point shot events for him.
	{GameID: 1715, PlayerVendorID: "bf9ad0fd-0cb8-4360-8970-5f1b5cf3fa8d", FieldName: "TwoPointAttempts"}:   "Sportradar API inconsistency: box score shows 8, play-by-play shows 7",
	{GameID: 1715, PlayerVendorID: "bf9ad0fd-0cb8-4360-8970-5f1b5cf3fa8d", FieldName: "ThreePointAttempts"}: "Sportradar API inconsistency: box score shows 5, play-by-play shows 6",

	// MEM @ LAC on 2025-12-15: Sportradar play-by-play shows Jock Landale with 2 steals
	// (events 144b02a1-e15c-48a5-be5e-1096c2e73c7a and dbd20fc7-ff36-4acd-afd4-b0a9b56d946b),
	// but their box score summary shows only 1 steal.
	{GameID: 18073, PlayerVendorID: "02dc6e18-95a4-4919-b4fc-f8a981ccd359", FieldName: "Steals"}: "Sportradar API inconsistency: play-by-play shows 2 steals, box score shows 1",

	// MEM @ LAC on 2025-12-15: Sportradar box score shows Jaren Jackson Jr. with 2 steals,
	// but their play-by-play API only contains 1 steal event (2f31c03d-6ca4-4e2a-a8d6-9342c123428f).
	{GameID: 18073, PlayerVendorID: "3e492a6a-ed3c-499d-b3f5-ff68ca16f6fd", FieldName: "Steals"}: "Sportradar API inconsistency: box score shows 2 steals, play-by-play shows 1",

	// BOS @ POR on 2025-12-28: Sportradar play-by-play shows Derrick White with 4 defensive rebounds,
	// but their box score summary shows only 3.
	{GameID: 2020, PlayerVendorID: "9bcd5cff-4a1c-4454-89b6-a5899b0c6bcc", FieldName: "DefensiveRebounds"}: "Sportradar API inconsistency: play-by-play shows 4 defensive rebounds, box score shows 3",
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
