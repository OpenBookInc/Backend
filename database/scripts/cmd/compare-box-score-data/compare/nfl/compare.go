package nfl

import (
	"fmt"
	"reflect"
	"strings"

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
	GameID            int
	GameVendorID      string
	PlayerName        string
	PlayerVendorID    string
	Field             string
	DBValue           decimal.Decimal
	SportradarValue   decimal.Decimal
}

func (d *NFLDiscrepancy) Error() string {
	return fmt.Sprintf("Discrepancy found for game %d (vendor: %s)\n  Player: %s (vendor: %s)\n  Field: %s\n    Database: %s\n    Sportradar: %s",
		d.GameID, d.GameVendorID, d.PlayerName, d.PlayerVendorID, d.Field, d.DBValue.String(), d.SportradarValue.String())
}

// NFLMissingPlayerError represents a player missing from one source
type NFLMissingPlayerError struct {
	GameID         int
	GameVendorID   string
	PlayerName     string
	PlayerVendorID string
	MissingIn      string // "database" or "sportradar"
}

func (e *NFLMissingPlayerError) Error() string {
	return fmt.Sprintf("Player mismatch for game %d (vendor: %s)\n  Player: %s (vendor: %s)\n  Missing in: %s",
		e.GameID, e.GameVendorID, e.PlayerName, e.PlayerVendorID, e.MissingIn)
}

// CompareNFLBoxScores compares database box score with Sportradar box score
// Returns nil if all stats match, otherwise returns the first discrepancy found
func CompareNFLBoxScores(gameID int, gameVendorID string, dbBoxScore *models_nfl.NFLBoxScore, sportradarBoxScore *models_nfl.NFLBoxScore) error {
	// Build map of Sportradar players by vendor ID
	sportradarPlayers := make(map[string]*models_nfl.IndividualBoxScore)
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
			return &NFLMissingPlayerError{
				GameID:         gameID,
				GameVendorID:   gameVendorID,
				PlayerName:     playerName,
				PlayerVendorID: vendorID,
				MissingIn:      "sportradar",
			}
		}

		matchedSportradarPlayers[vendorID] = true

		if err := compareNFLPlayerStats(gameID, gameVendorID, playerName, vendorID, dbPlayer.Stats, sportradarPlayer.Stats); err != nil {
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
			return &NFLMissingPlayerError{
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

// compareNFLPlayerStats compares individual player stats using reflection to iterate
// over all Decimal fields. This makes the comparison future-proof when new statistical
// fields are added to NFLStats.
func compareNFLPlayerStats(gameID int, gameVendorID string, playerName string, playerVendorID string, dbStats *models_nfl.NFLStats, srStats *models_nfl.NFLStats) error {
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
			return &NFLDiscrepancy{
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

	// Compare sacks using combined credits. Sportradar reports a single sack total where
	// each assist counts as 0.5 credits, while our database tracks them separately.
	// e.g., DB: 2 sacks + 2 assists = Sportradar: 3.0 (2 + 2*0.5)
	if err := compareSackCredits(gameID, gameVendorID, playerName, playerVendorID, dbStats, srStats); err != nil {
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

// compareSackCredits compares sack stats using combined "sack credits".
// Sportradar counts each sack assist as 0.5 of a sack, so we compare totals rather
// than individual components since Sportradar doesn't separate them.
func compareSackCredits(gameID int, gameVendorID string, playerName string, playerVendorID string, dbStats *models_nfl.NFLStats, srStats *models_nfl.NFLStats) error {
	half := decimal.NewFromFloat(0.5)

	// Calculate combined sack credits: sacks_made + 0.5 * sack_assists_made
	dbSackCredits := dbStats.SacksMade.Add(dbStats.SackAssistsMade.Mul(half))

	// Sportradar provides combined sacks directly (SackAssistsMade is zero in translator)
	srSackCredits := srStats.SacksMade

	if !dbSackCredits.Equal(srSackCredits) {
		return &NFLDiscrepancy{
			GameID:          gameID,
			GameVendorID:    gameVendorID,
			PlayerName:      playerName,
			PlayerVendorID:  playerVendorID,
			Field:           "sack_credits (sacks_made + 0.5*sack_assists_made)",
			DBValue:         dbSackCredits,
			SportradarValue: srSackCredits,
		}
	}

	return nil
}
