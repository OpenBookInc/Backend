package nfl

import (
	"time"

	genEnums "github.com/openbook/shared/models/gen"
	genNfl "github.com/openbook/shared/models/gen/nfl"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Play-by-Play Database Models
// =============================================================================
// These models represent the database entities for NFL play-by-play data.
// They are separate from the Sportradar API response structs in fetcher/nfl/.
//
// Enum types are auto-generated from the database schema in gen/nfl/enums.go
// and gen/enums.go. Do not manually define enum types here.
// =============================================================================

// Type aliases for generated enum types (for backwards compatibility)
type PeriodType = genNfl.Period
type StatType = genNfl.Stat
type GameStatusType = genEnums.GameStatus

// =============================================================================
// Database Entity Models
// =============================================================================

// Drive represents a drive in an NFL game (database entity)
type Drive struct {
	ID               int             // Database ID (auto-increment)
	GameID           int             // Foreign key to games table
	VendorID         string          // Sportradar drive UUID
	VendorSequence   decimal.Decimal // Drive sequence for ordering
	PossessionTeamID int             // Foreign key to teams table
	CreatedAt        time.Time       // Record creation time
	UpdatedAt        time.Time       // Record update time
}

// Play represents a play/event in an NFL game (database entity)
type Play struct {
	ID                     int             // Database ID (auto-increment)
	DriveID                int             // Foreign key to nfl_drives table
	VendorID               string          // Sportradar event UUID
	VendorSequence         decimal.Decimal // Play sequence for ordering
	PeriodType   PeriodType // quarter or overtime
	PeriodNumber int        // 1-4 for quarters, 5+ for overtime
	Description  string     // Play description
	Nullified    bool       // Whether play was nullified by penalty
	VendorCreatedAt        time.Time       // Sportradar creation timestamp
	VendorUpdatedAt        time.Time       // Sportradar update timestamp
	CreatedAt              time.Time       // Record creation time
	UpdatedAt              time.Time       // Record update time
}

// PlayStatistic represents a player statistic for a play (database entity)
type PlayStatistic struct {
	ID                  int             // Database ID (auto-increment)
	PlayID              int             // Foreign key to nfl_plays table
	IndividualID        int             // Foreign key to individuals table
	StatType            genNfl.Stat     // Type of statistic
	PassingAttempts     decimal.Decimal // Passing attempts
	RushingAttempts     decimal.Decimal // Rushing attempts
	ReceivingTargets    decimal.Decimal // Receiving targets
	PassingYards        decimal.Decimal // Passing yards
	RushingYards        decimal.Decimal // Rushing yards
	ReceivingYards      decimal.Decimal // Receiving yards
	PassingTouchdowns   decimal.Decimal // Passing touchdowns
	RushingTouchdowns   decimal.Decimal // Rushing touchdowns
	ReceivingTouchdowns decimal.Decimal // Receiving touchdowns
	PassingCompletions  decimal.Decimal // Pass completions
	ReceivingReceptions decimal.Decimal // Receptions
	InterceptionsThrown decimal.Decimal // Interceptions thrown (QB stat)
	InterceptionsCaught decimal.Decimal // Interceptions caught (defensive stat)
	FumblesForced       decimal.Decimal // Fumbles forced (defensive stat)
	FumblesCommitted    decimal.Decimal // Fumbles committed (offensive stat)
	SacksTaken          decimal.Decimal // Sacks taken (QB stat)
	SacksMade           decimal.Decimal // Sacks made (defensive stat, can be 0.5 for split credit)
	SackAssistsMade     decimal.Decimal // Sack assists (defensive stat)
	TacklesMade         decimal.Decimal // Tackles made (can be 0.5 for split credit)
	TackleAssistsMade   decimal.Decimal // Assisted tackles (defensive stat)
	FieldGoalAttempts   decimal.Decimal // Field goal attempts
	FieldGoalMakes      decimal.Decimal // Field goals made
	FieldGoalMakeYards  decimal.Decimal // Yards of successful field goals
	ExtraPointAttempts  decimal.Decimal // Extra point attempts
	ExtraPointMakes     decimal.Decimal // Extra points made
	Nullified           bool            // Whether stat was nullified
}

// GameStatus represents the status of a game (database entity)
type GameStatus struct {
	GameID    int                 // Primary key, foreign key to games table
	Status    genEnums.GameStatus // Current game status
	UpdatedAt time.Time           // Last update time
}
