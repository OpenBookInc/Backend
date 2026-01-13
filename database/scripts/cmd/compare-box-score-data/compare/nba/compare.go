package nba

import (
	"fmt"
	"reflect"
	"strings"

	models_nba "github.com/openbook/shared/models/nba"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Box Score Comparison
// =============================================================================
// Compares NBA box score data between our database and Sportradar.
// Returns detailed error on any discrepancy.
// =============================================================================

// NBADiscrepancy represents a single stat discrepancy
type NBADiscrepancy struct {
	GameID          int
	GameVendorID    string
	PlayerName      string
	PlayerVendorID  string
	Field           string
	DBValue         decimal.Decimal
	SportradarValue decimal.Decimal
}

func (d *NBADiscrepancy) Error() string {
	return fmt.Sprintf("Discrepancy found for game %d (vendor: %s)\n  Player: %s (vendor: %s)\n  Field: %s\n    Database: %s\n    Sportradar: %s",
		d.GameID, d.GameVendorID, d.PlayerName, d.PlayerVendorID, d.Field, d.DBValue.String(), d.SportradarValue.String())
}

// NBAMissingPlayerError represents a player missing from one source
type NBAMissingPlayerError struct {
	GameID         int
	GameVendorID   string
	PlayerName     string
	PlayerVendorID string
	MissingIn      string // "database" or "sportradar"
}

func (e *NBAMissingPlayerError) Error() string {
	return fmt.Sprintf("Player mismatch for game %d (vendor: %s)\n  Player: %s (vendor: %s)\n  Missing in: %s",
		e.GameID, e.GameVendorID, e.PlayerName, e.PlayerVendorID, e.MissingIn)
}

// CompareNBABoxScores compares database box score with Sportradar box score
// Returns nil if all stats match, otherwise returns the first discrepancy found
func CompareNBABoxScores(gameID int, gameVendorID string, dbBoxScore *models_nba.NBABoxScore, sportradarBoxScore *models_nba.NBABoxScore) error {
	// Build map of Sportradar players by vendor ID
	sportradarPlayers := make(map[string]*models_nba.IndividualBoxScore)
	for _, player := range sportradarBoxScore.Players {
		if player.Individual == nil || player.Stats == nil {
			continue
		}
		if shouldExcludeSportradarPlayer(player.Stats) {
			continue
		}
		sportradarPlayers[player.Individual.VendorID] = player
	}

	// Track which Sportradar players we've matched
	matchedSportradarPlayers := make(map[string]bool)

	// Compare each database player against Sportradar
	for _, dbPlayer := range dbBoxScore.Players {
		if dbPlayer.Individual == nil || dbPlayer.Stats == nil {
			continue
		}

		if shouldExcludeDatabasePlayer(dbPlayer.Stats) {
			continue
		}

		vendorID := dbPlayer.Individual.VendorID
		playerName := dbPlayer.Individual.DisplayName

		sportradarPlayer, found := sportradarPlayers[vendorID]
		if !found {
			return &NBAMissingPlayerError{
				GameID:         gameID,
				GameVendorID:   gameVendorID,
				PlayerName:     playerName,
				PlayerVendorID: vendorID,
				MissingIn:      "sportradar",
			}
		}

		matchedSportradarPlayers[vendorID] = true

		if err := compareNBAPlayerStats(gameID, gameVendorID, playerName, vendorID, dbPlayer.Stats, sportradarPlayer.Stats); err != nil {
			return err
		}
	}

	// Check for Sportradar players not in database
	for vendorID, player := range sportradarPlayers {
		if !matchedSportradarPlayers[vendorID] {
			playerName := ""
			if player.Individual != nil {
				playerName = player.Individual.DisplayName
			}
			return &NBAMissingPlayerError{
				GameID:         gameID,
				GameVendorID:   gameVendorID,
				PlayerName:     playerName,
				PlayerVendorID: vendorID,
				MissingIn:      "database",
			}
		}
	}

	return nil
}

// compareNBAPlayerStats compares individual player stats using reflection to iterate
// over all Decimal fields. This makes the comparison future-proof when new statistical
// fields are added to NBAStats.
func compareNBAPlayerStats(gameID int, gameVendorID string, playerName string, playerVendorID string, dbStats *models_nba.NBAStats, srStats *models_nba.NBAStats) error {
	dbVal := reflect.ValueOf(*dbStats)
	srVal := reflect.ValueOf(*srStats)
	dbType := dbVal.Type()
	decimalType := reflect.TypeOf(decimal.Decimal{})

	for i := 0; i < dbVal.NumField(); i++ {
		field := dbType.Field(i)

		if field.Type != decimalType {
			continue
		}

		dbDecimal := dbVal.Field(i).Interface().(decimal.Decimal)
		srDecimal := srVal.Field(i).Interface().(decimal.Decimal)

		if !dbDecimal.Equal(srDecimal) {
			return &NBADiscrepancy{
				GameID:          gameID,
				GameVendorID:    gameVendorID,
				PlayerName:      playerName,
				PlayerVendorID:  playerVendorID,
				Field:           toSnakeCase(field.Name),
				DBValue:         dbDecimal,
				SportradarValue: srDecimal,
			}
		}
	}

	return nil
}

// toSnakeCase converts PascalCase field names to snake_case for error reporting.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
