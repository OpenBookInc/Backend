package nfl

import (
	"reflect"

	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
	"github.com/shopspring/decimal"
)

// Exclusion rules for filtering data during persistence.
// These exclusions prevent data from being written to the database.
// For fetch-level exclusions, see fetcher/exclusions.go.

// shouldPersistDrive determines whether a drive should be persisted to the database.
// Returns false for "event" type entries (timeouts, end-of-period markers, etc.) and
// standalone penalty plays (which contain no meaningful statistics).
// Returns true for "drive" type entries.
func shouldPersistDrive(drive *fetcher_nfl.Drive) bool {
	if drive.Type == "event" {
		return false
	}
	if drive.Type == "play" && drive.PlayType == "penalty" {
		return false
	}
	return true
}

// shouldPersistPlay determines whether a play (event) should be persisted to the database.
// Returns false for non-play events (timeouts, end-of-period markers, etc.) and unofficial plays.
// Returns true only for official, confirmed plays.
func shouldPersistPlay(event *fetcher_nfl.Event) bool {
	if event.Type != "play" {
		return false
	}

	if !event.Official {
		return false
	}

	return true
}

// shouldPersistPlayStatistic determines whether a play statistic should be persisted to the database.
func shouldPersistPlayStatistic(stat *fetcher_nfl.Statistic) bool {
	// Skip statistics without a player (team-level stats)
	if stat.Player == nil {
		return false
	}

	// Skip ignoreable stat types (stat types that we don't need for our use case)
	ignoreableStatTypes := []string{"return", "first_down", "kick", "punt", "penalty", "block", "conversion" /* refers to two-point conversions */, "defense_conversion" /* indicates a defender was targeted during a two-point conversion */}
	for _, ignoreType := range ignoreableStatTypes {
		if stat.StatType == ignoreType {
			return false
		}
	}

	return true
}

// shouldPersistPlayStatisticForUpsert determines whether a parsed play statistic should be persisted.
// Returns false if all decimal.Decimal fields are zero (no meaningful data to persist).
// Uses reflection to automatically include any new decimal fields added in the future.
func shouldPersistPlayStatisticForUpsert(stat *store_nfl.PlayStatisticForUpsert) bool {
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
