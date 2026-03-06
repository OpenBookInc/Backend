package nba

import (
	"fmt"
	"reflect"

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
	GameID          string
	GameSportradarID    string
	PlayerName      string
	PlayerSportradarID  string
	Field           string
	DBValue         decimal.Decimal
	SportradarValue decimal.Decimal
}

func (d *NBADiscrepancy) Error() string {
	return fmt.Sprintf("Discrepancy found for game %s (vendor: %s)\n  Player: %s (vendor: %s)\n  Field: %s\n    Database: %s\n    Sportradar: %s",
		d.GameID, d.GameSportradarID, d.PlayerName, d.PlayerSportradarID, d.Field, d.DBValue.String(), d.SportradarValue.String())
}

// NBAMissingPlayerError represents a player missing from one source
type NBAMissingPlayerError struct {
	GameID         string
	GameSportradarID   string
	PlayerName     string
	PlayerSportradarID string
	MissingIn      string // "database" or "sportradar"
}

func (e *NBAMissingPlayerError) Error() string {
	return fmt.Sprintf("Player mismatch for game %s (vendor: %s)\n  Player: %s (vendor: %s)\n  Missing in: %s",
		e.GameID, e.GameSportradarID, e.PlayerName, e.PlayerSportradarID, e.MissingIn)
}

// CompareNBABoxScores compares database box score with Sportradar box score
// Returns nil if all stats match, otherwise returns the first discrepancy found
func CompareNBABoxScores(gameID string, gameSportradarID string, dbBoxScore *models_nba.NBABoxScore, sportradarBoxScore *models_nba.NBABoxScore) error {
	// Build map of Sportradar players by vendor ID
	sportradarPlayers := make(map[string]*models_nba.IndividualBoxScore)
	for _, player := range sportradarBoxScore.Players {
		if player.Individual == nil || player.Stats == nil {
			continue
		}
		if shouldExcludeSportradarPlayer(player.Stats) {
			continue
		}
		sportradarPlayers[player.Individual.SportradarID] = player
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

		sportradarID := dbPlayer.Individual.SportradarID
		playerName := dbPlayer.Individual.DisplayName

		sportradarPlayer, found := sportradarPlayers[sportradarID]
		if !found {
			return &NBAMissingPlayerError{
				GameID:             gameID,
				GameSportradarID:   gameSportradarID,
				PlayerName:         playerName,
				PlayerSportradarID: sportradarID,
				MissingIn:          "sportradar",
			}
		}

		matchedSportradarPlayers[sportradarID] = true

		if err := compareNBAPlayerStats(gameID, gameSportradarID, playerName, sportradarID, dbPlayer.Stats, sportradarPlayer.Stats); err != nil {
			return err
		}
	}

	// Check for Sportradar players not in database
	for sportradarID, player := range sportradarPlayers {
		if !matchedSportradarPlayers[sportradarID] {
			playerName := ""
			if player.Individual != nil {
				playerName = player.Individual.DisplayName
			}
			return &NBAMissingPlayerError{
				GameID:             gameID,
				GameSportradarID:   gameSportradarID,
				PlayerName:         playerName,
				PlayerSportradarID: sportradarID,
				MissingIn:          "database",
			}
		}
	}

	return nil
}

// compareNBAPlayerStats compares individual player stats using reflection to iterate
// over all Decimal fields. This makes the comparison future-proof when new statistical
// fields are added to NBAStats.
func compareNBAPlayerStats(gameID string, gameSportradarID string, playerName string, playerSportradarID string, dbStats *models_nba.NBAStats, srStats *models_nba.NBAStats) error {
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
			if shouldExcludeStatDiscrepancy(gameID, playerSportradarID, field.Name) {
				continue
			}

			return &NBADiscrepancy{
				GameID:          gameID,
				GameSportradarID:    gameSportradarID,
				PlayerName:      playerName,
				PlayerSportradarID:  playerSportradarID,
				Field:           field.Name,
				DBValue:         dbDecimal,
				SportradarValue: srDecimal,
			}
		}
	}

	return nil
}
