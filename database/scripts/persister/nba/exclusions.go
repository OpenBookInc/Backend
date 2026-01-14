package nba

import (
	"reflect"

	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	store_nba "github.com/openbook/population-scripts/store/nba"
	"github.com/shopspring/decimal"
)

// Exclusion rules for filtering NBA data during persistence.
// These exclusions prevent data from being written to the database.
// For fetch-level exclusions, see fetcher/exclusions.go.

// excludedEventTypes lists event types that should not be persisted.
// These are game management events that don't represent statistical plays.
var excludedEventTypes = map[string]bool{
	"jumpball":     true,
	"teamtimeout":  true,
	"lineupchange": true,
	"opentip":      true,
	"kickball":     true,
	"review":       true,
	"stoppage":     true,
	"endperiod":    true,
}

// excludedStatTypes lists statistic types that should not be persisted.
// These are excluded to avoid duplicate counting or because they don't represent
// distinct statistical events.
var excludedStatTypes = map[string]bool{
	"fouldrawn":                       true,
	"technicalfoul":                   true,
	"technicalfoulnonunsportsmanlike": true,
	"attemptblocked":                  true,
	// Offensive fouls are excluded because Sportradar already creates a "personalfoul"
	// statistic for the same event. Including both would double-count the foul.
	"offensivefoul": true,
}

// shouldPersistPlay determines whether a play (event) should be persisted to the database.
// Returns false for:
// - Events with excluded event_type (game management events)
// - Events with zero persistable statistics after filtering
func shouldPersistPlay(event *fetcher_nba.Event) bool {
	// Check if event type is excluded
	if excludedEventTypes[event.EventType] {
		return false
	}

	// Check if event has at least one persistable statistic
	persistableStatCount := 0
	for _, stat := range event.Statistics {
		if shouldPersistPlayStatistic(&stat) {
			persistableStatCount++
		}
	}

	if persistableStatCount == 0 {
		return false
	}

	return true
}

// shouldPersistPlayStatistic determines whether a play statistic should be persisted to the database.
// Returns false for:
// - Statistics without a player (team-level stats)
// - Statistics with excluded types (fouldrawn, technicalfoul, etc.)
func shouldPersistPlayStatistic(stat *fetcher_nba.Statistic) bool {
	// Skip statistics without a player (team-level stats)
	if stat.Player == nil {
		return false
	}

	// Skip excluded stat types
	if excludedStatTypes[stat.Type] {
		return false
	}

	return true
}

// shouldPersistPlayStatisticForUpsert determines whether a parsed play statistic should be persisted.
// Returns false if all decimal.Decimal fields are zero (no meaningful data to persist).
// Uses reflection to automatically include any new decimal fields added in the future.
func shouldPersistPlayStatisticForUpsert(stat *store_nba.NBAPlayStatisticForUpsert) bool {
	v := reflect.ValueOf(*stat)
	decimalType := reflect.TypeOf(decimal.Decimal{})

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Type() == decimalType {
			d := field.Interface().(decimal.Decimal)
			if !d.IsZero() {
				return true
			}
		}
	}

	return false
}
