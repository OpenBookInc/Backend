package nfl

import (
	"fmt"
	"strings"

	"github.com/openbook/shared/models"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Box Score Database Models
// =============================================================================
// These models represent box score data for NFL games.
// A box score aggregates all play-by-play statistics for each player in a game.
// =============================================================================

// NFLStats holds aggregated statistics for a player in a single game
type NFLStats struct {
	ID                  int             // Database ID (auto-increment)
	GameID              int             // Foreign key to games table
	IndividualID        int             // Foreign key to individuals table
	PassingCompletions  decimal.Decimal // Pass completions
	ReceivingReceptions decimal.Decimal // Receptions
	InterceptionsCaught decimal.Decimal // Interceptions caught (defensive stat)
	FumblesForced       decimal.Decimal // Fumbles forced (defensive stat)
	FumblesLost         decimal.Decimal // Fumbles lost (offensive stat)
	SacksMade           decimal.Decimal // Sacks made (defensive stat)
	TacklesMade         decimal.Decimal // Tackles made (defensive stat)
	AssistsMade         decimal.Decimal // Assisted tackles (defensive stat)
	PassingAttempts     decimal.Decimal // Passing attempts
	RushingAttempts     decimal.Decimal // Rushing attempts
	ReceivingTargets    decimal.Decimal // Receiving targets
	PassingYards        decimal.Decimal // Passing yards
	RushingYards        decimal.Decimal // Rushing yards
	ReceivingYards      decimal.Decimal // Receiving yards
	PassingTouchdowns   decimal.Decimal // Passing touchdowns
	RushingTouchdowns   decimal.Decimal // Rushing touchdowns
	ReceivingTouchdowns decimal.Decimal // Receiving touchdowns
	InterceptionsThrown decimal.Decimal // Interceptions thrown (QB stat)
	SacksTaken          decimal.Decimal // Sacks taken (QB stat)
}

// IndividualBoxScore combines player information with their game statistics
type IndividualBoxScore struct {
	Individual *models.Individual
	Stats      *NFLStats
	TeamID     int // The team this player belongs to (from roster)
}

// NFLBoxScore represents a complete box score for an NFL game
type NFLBoxScore struct {
	Game                 *models.Game
	HomeTeamPlayers      []*IndividualBoxScore // Players on home team (ContenderA)
	AwayTeamPlayers      []*IndividualBoxScore // Players on away team (ContenderB)
}

// String returns a formatted table representation of the box score organized by team
func (bs *NFLBoxScore) String() string {
	var sb strings.Builder

	// Header
	sb.WriteString(strings.Repeat("=", 560))
	sb.WriteString("\n")

	// Game info
	homeTeamName := "Unknown"
	awayTeamName := "Unknown"
	if bs.Game != nil {
		if bs.Game.TeamA != nil {
			homeTeamName = fmt.Sprintf("%s %s", bs.Game.TeamA.Market, bs.Game.TeamA.Name)
		}
		if bs.Game.TeamB != nil {
			awayTeamName = fmt.Sprintf("%s %s", bs.Game.TeamB.Market, bs.Game.TeamB.Name)
		}
		sb.WriteString(fmt.Sprintf("NFL Box Score: %s vs %s\n", awayTeamName, homeTeamName))
		sb.WriteString(fmt.Sprintf("Game ID: %d | %s\n", bs.Game.ID, bs.Game.ScheduledStartTime.Format("2006-01-02 15:04:05 MST")))
	} else {
		sb.WriteString("NFL Box Score: Unknown Game\n")
	}

	sb.WriteString(strings.Repeat("=", 560))
	sb.WriteString("\n")

	// Away team section
	sb.WriteString(fmt.Sprintf("\n%s (Away Team) - %d Players\n", awayTeamName, len(bs.AwayTeamPlayers)))
	sb.WriteString(strings.Repeat("-", 560))
	sb.WriteString("\n")
	writeTableHeader(&sb)
	writePlayerRows(&sb, bs.AwayTeamPlayers)

	// Home team section
	sb.WriteString(fmt.Sprintf("\n%s (Home Team) - %d Players\n", homeTeamName, len(bs.HomeTeamPlayers)))
	sb.WriteString(strings.Repeat("-", 560))
	sb.WriteString("\n")
	writeTableHeader(&sb)
	writePlayerRows(&sb, bs.HomeTeamPlayers)

	// Footer
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 560))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Players: %d (Away: %d, Home: %d)\n",
		len(bs.AwayTeamPlayers)+len(bs.HomeTeamPlayers),
		len(bs.AwayTeamPlayers),
		len(bs.HomeTeamPlayers)))

	return sb.String()
}

// Csv returns a CSV-formatted string of all player stats for the game.
// Columns: player_name, and all stats in lowercase with underscores.
// Rows: all players from both teams.
func (bs *NFLBoxScore) Csv() string {
	var sb strings.Builder

	// Header row
	sb.WriteString("player_name,passing_completions,receiving_receptions,interceptions_caught,fumbles_forced,fumbles_lost,sacks_made,tackles_made,assists_made,passing_attempts,rushing_attempts,receiving_targets,passing_yards,rushing_yards,receiving_yards,passing_touchdowns,rushing_touchdowns,receiving_touchdowns,interceptions_thrown,sacks_taken\n")

	// Write all players (away team first, then home team)
	writeCsvRows(&sb, bs.AwayTeamPlayers)
	writeCsvRows(&sb, bs.HomeTeamPlayers)

	return sb.String()
}

// writeCsvRows writes CSV rows for a list of players
func writeCsvRows(sb *strings.Builder, players []*IndividualBoxScore) {
	for _, player := range players {
		playerName := "Unknown"
		if player.Individual != nil {
			playerName = player.Individual.DisplayName
		}

		// Escape player name if it contains commas or quotes
		if strings.ContainsAny(playerName, ",\"") {
			playerName = "\"" + strings.ReplaceAll(playerName, "\"", "\"\"") + "\""
		}

		if player.Stats != nil {
			sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
				playerName,
				formatStat(player.Stats.PassingCompletions),
				formatStat(player.Stats.ReceivingReceptions),
				formatStat(player.Stats.InterceptionsCaught),
				formatStat(player.Stats.FumblesForced),
				formatStat(player.Stats.FumblesLost),
				formatStat(player.Stats.SacksMade),
				formatStat(player.Stats.TacklesMade),
				formatStat(player.Stats.AssistsMade),
				formatStat(player.Stats.PassingAttempts),
				formatStat(player.Stats.RushingAttempts),
				formatStat(player.Stats.ReceivingTargets),
				formatStat(player.Stats.PassingYards),
				formatStat(player.Stats.RushingYards),
				formatStat(player.Stats.ReceivingYards),
				formatStat(player.Stats.PassingTouchdowns),
				formatStat(player.Stats.RushingTouchdowns),
				formatStat(player.Stats.ReceivingTouchdowns),
				formatStat(player.Stats.InterceptionsThrown),
				formatStat(player.Stats.SacksTaken)))
		}
	}
}

// writeTableHeader writes the column headers for the stats table
func writeTableHeader(sb *strings.Builder) {
	sb.WriteString(fmt.Sprintf("%-25s | %-4s | %19s | %19s | %16s | %14s | %19s | %20s | %16s | %14s | %19s | %18s | %19s | %16s | %21s | %11s | %13s | %13s | %19s | %14s | %14s | %14s\n",
		"Player Name", "Pos",
		"Passing Completions", "Receiving Receptions", "Passing Attempts", "Passing Yards", "Passing Touchdowns", "Interceptions Thrown",
		"Rushing Attempts", "Rushing Yards", "Rushing Touchdowns",
		"Receiving Targets", "Receiving Receptions", "Receiving Yards", "Receiving Touchdowns",
		"Sacks Made", "Tackles Made", "Assists Made", "Interceptions Caught", "Fumbles Forced", "Fumbles Lost", "Sacks Taken"))

	sb.WriteString(strings.Repeat("-", 560))
	sb.WriteString("\n")
}

// writePlayerRows writes the stat rows for a list of players
func writePlayerRows(sb *strings.Builder, players []*IndividualBoxScore) {
	for _, player := range players {
		playerName := "Unknown"
		position := "?"
		if player.Individual != nil {
			playerName = player.Individual.DisplayName
			position = player.Individual.Position
		}

		// Truncate player name if too long
		if len(playerName) > 25 {
			playerName = playerName[:22] + "..."
		}

		if player.Stats != nil {
			sb.WriteString(fmt.Sprintf("%-25s | %-4s | %19s | %19s | %16s | %14s | %19s | %20s | %16s | %14s | %19s | %18s | %19s | %16s | %21s | %11s | %13s | %13s | %19s | %14s | %14s | %14s\n",
				playerName, position,
				formatStat(player.Stats.PassingCompletions),
				formatStat(player.Stats.ReceivingReceptions),
				formatStat(player.Stats.PassingAttempts),
				formatStat(player.Stats.PassingYards),
				formatStat(player.Stats.PassingTouchdowns),
				formatStat(player.Stats.InterceptionsThrown),
				formatStat(player.Stats.RushingAttempts),
				formatStat(player.Stats.RushingYards),
				formatStat(player.Stats.RushingTouchdowns),
				formatStat(player.Stats.ReceivingTargets),
				formatStat(player.Stats.ReceivingReceptions),
				formatStat(player.Stats.ReceivingYards),
				formatStat(player.Stats.ReceivingTouchdowns),
				formatStat(player.Stats.SacksMade),
				formatStat(player.Stats.TacklesMade),
				formatStat(player.Stats.AssistsMade),
				formatStat(player.Stats.InterceptionsCaught),
				formatStat(player.Stats.FumblesForced),
				formatStat(player.Stats.FumblesLost),
				formatStat(player.Stats.SacksTaken)))
		}
	}
}

// formatStat formats a decimal stat value for display
func formatStat(d decimal.Decimal) string {
	// If the value is a whole number, display without decimal
	if d.Equal(d.Truncate(0)) {
		return d.StringFixed(0)
	}
	// Otherwise show one decimal place
	return d.StringFixed(1)
}
