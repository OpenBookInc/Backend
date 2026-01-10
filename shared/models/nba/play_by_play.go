package nba

import (
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Play-by-Play Database Models
// =============================================================================
// These models represent the database entities for NBA play-by-play data.
// They are separate from the Sportradar API response structs in fetcher/nba/.
//
// Unlike NFL, NBA plays are directly associated with games (no drives).
// =============================================================================

// PlayStatistic represents a player statistic for a play (database entity)
type PlayStatistic struct {
	ID                     int             // Database ID (auto-increment)
	PlayID                 int             // Foreign key to nba_plays table
	IndividualID           int             // Foreign key to individuals table
	StatType               string          // Type of statistic (nba_stat_type enum)
	TwoPointAttempts       decimal.Decimal // Two-point field goal attempts
	TwoPointMakes          decimal.Decimal // Two-point field goals made
	ThreePointAttempts     decimal.Decimal // Three-point field goal attempts
	ThreePointMakes        decimal.Decimal // Three-point field goals made
	FreeThrowAttempts      decimal.Decimal // Free throw attempts
	FreeThrowMakes         decimal.Decimal // Free throws made
	Assists                decimal.Decimal // Assists
	DefensiveRebounds      decimal.Decimal // Defensive rebounds
	OffensiveRebounds      decimal.Decimal // Offensive rebounds
	Steals                 decimal.Decimal // Steals
	Blocks                 decimal.Decimal // Blocks
	TurnoversCommitted     decimal.Decimal // Turnovers committed
	PersonalFoulsCommitted decimal.Decimal // Personal fouls committed
}
