package nfl

import (
	"fmt"
	"reflect"

	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Box Score Comparison
// =============================================================================
// Compares NFL box score data between our database and Sportradar.
// Returns detailed error on any discrepancy.
// =============================================================================

// NFLDiscrepancy represents a single stat discrepancy
type NFLDiscrepancy struct {
	GameID            string
	GameSportradarID      string
	PlayerName        string
	PlayerSportradarID    string
	Field             string
	DBValue           decimal.Decimal
	SportradarValue   decimal.Decimal
}

func (d *NFLDiscrepancy) Error() string {
	return fmt.Sprintf("Discrepancy found for game %s (vendor: %s)\n  Player: %s (vendor: %s)\n  Field: %s\n    Database: %s\n    Sportradar: %s",
		d.GameID, d.GameSportradarID, d.PlayerName, d.PlayerSportradarID, d.Field, d.DBValue.String(), d.SportradarValue.String())
}

// NFLMissingPlayerError represents a player missing from one source
type NFLMissingPlayerError struct {
	GameID         string
	GameSportradarID   string
	PlayerName     string
	PlayerSportradarID string
	MissingIn      string // "database" or "sportradar"
}

func (e *NFLMissingPlayerError) Error() string {
	return fmt.Sprintf("Player mismatch for game %s (vendor: %s)\n  Player: %s (vendor: %s)\n  Missing in: %s",
		e.GameID, e.GameSportradarID, e.PlayerName, e.PlayerSportradarID, e.MissingIn)
}

// CompareNFLBoxScores compares database box score with Sportradar box score
// Returns nil if all stats match, otherwise returns the first discrepancy found
func CompareNFLBoxScores(gameID string, gameSportradarID string, dbBoxScore *models_nfl.NFLBoxScore, sportradarBoxScore *models_nfl.NFLBoxScore) error {
	// Build map of Sportradar players by vendor ID
	sportradarPlayers := make(map[string]*models_nfl.IndividualBoxScore)
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
			return &NFLMissingPlayerError{
				GameID:             gameID,
				GameSportradarID:   gameSportradarID,
				PlayerName:         playerName,
				PlayerSportradarID: sportradarID,
				MissingIn:          "sportradar",
			}
		}

		matchedSportradarPlayers[sportradarID] = true

		if err := compareNFLPlayerStats(gameID, gameSportradarID, playerName, sportradarID, dbPlayer.Stats, sportradarPlayer.Stats); err != nil {
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
			return &NFLMissingPlayerError{
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

// compareNFLPlayerStats compares individual player stats using reflection to iterate
// over all Decimal fields. This makes the comparison future-proof when new statistical
// fields are added to NFLStats.
func compareNFLPlayerStats(gameID string, gameSportradarID string, playerName string, playerSportradarID string, dbStats *models_nfl.NFLStats, srStats *models_nfl.NFLStats) error {
	dbVal := reflect.ValueOf(*dbStats)
	srVal := reflect.ValueOf(*srStats)
	dbType := dbVal.Type()
	decimalType := reflect.TypeOf(decimal.Decimal{})

	for i := 0; i < dbVal.NumField(); i++ {
		field := dbType.Field(i)

		if field.Type != decimalType {
			continue
		}

		if shouldExcludeFieldFromComparison(field.Name) {
			continue
		}

		dbDecimal := dbVal.Field(i).Interface().(decimal.Decimal)
		srDecimal := srVal.Field(i).Interface().(decimal.Decimal)

		if !dbDecimal.Equal(srDecimal) {
			if shouldExcludeStatDiscrepancy(gameID, playerSportradarID, field.Name) {
				continue
			}

			return &NFLDiscrepancy{
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

	// Compare sacks using combined credits. Sportradar reports a single sack total where
	// each assist counts as 0.5 credits, while our database tracks them separately.
	// e.g., DB: 2 sacks + 2 assists = Sportradar: 3.0 (2 + 2*0.5)
	if err := compareSackCredits(gameID, gameSportradarID, playerName, playerSportradarID, dbStats, srStats); err != nil {
		return err
	}

	return nil
}

// shouldExcludeFieldFromComparison returns true for fields that require special comparison
// logic and should not be compared directly via reflection.
func shouldExcludeFieldFromComparison(fieldName string) bool {
	// SacksMade and SackAssistsMade are compared separately via compareSackCredits()
	// because Sportradar combines them into a single value where assists count as 0.5
	return fieldName == "SacksMade" || fieldName == "SackAssistsMade"
}

// compareSackCredits compares sack stats using combined "sack credits".
// Sportradar counts each sack assist as 0.5 of a sack, so we compare totals rather
// than individual components since Sportradar doesn't separate them.
func compareSackCredits(gameID string, gameSportradarID string, playerName string, playerSportradarID string, dbStats *models_nfl.NFLStats, srStats *models_nfl.NFLStats) error {
	half := decimal.NewFromFloat(0.5)

	// Calculate combined sack credits: sacks_made + 0.5 * sack_assists_made
	dbSackCredits := dbStats.SacksMade.Add(dbStats.SackAssistsMade.Mul(half))

	// Sportradar provides combined sacks directly (SackAssistsMade is zero in translator)
	srSackCredits := srStats.SacksMade

	if !dbSackCredits.Equal(srSackCredits) {
		return &NFLDiscrepancy{
			GameID:          gameID,
			GameSportradarID:    gameSportradarID,
			PlayerName:      playerName,
			PlayerSportradarID:  playerSportradarID,
			Field:           "sack_credits (sacks_made + 0.5*sack_assists_made)",
			DBValue:         dbSackCredits,
			SportradarValue: srSackCredits,
		}
	}

	return nil
}
