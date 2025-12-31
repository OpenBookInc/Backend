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
	Completions         decimal.Decimal // Pass completions
	Incompletions       decimal.Decimal // Pass incompletions
	Receptions          decimal.Decimal // Receptions
	Interceptions       decimal.Decimal // Interceptions (defensive stat)
	Fumbles             decimal.Decimal // Fumbles
	FumblesLost         decimal.Decimal // Fumbles lost
	Sacks               decimal.Decimal // Sacks (defensive stat)
	Tackles             decimal.Decimal // Tackles
	Assists             decimal.Decimal // Assisted tackles
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
	sb.WriteString(strings.Repeat("=", 280))
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

	sb.WriteString(strings.Repeat("=", 280))
	sb.WriteString("\n")

	// Away team section
	sb.WriteString(fmt.Sprintf("\n%s (Away Team) - %d Players\n", awayTeamName, len(bs.AwayTeamPlayers)))
	sb.WriteString(strings.Repeat("-", 280))
	sb.WriteString("\n")
	writeTableHeader(&sb)
	writePlayerRows(&sb, bs.AwayTeamPlayers)

	// Home team section
	sb.WriteString(fmt.Sprintf("\n%s (Home Team) - %d Players\n", homeTeamName, len(bs.HomeTeamPlayers)))
	sb.WriteString(strings.Repeat("-", 280))
	sb.WriteString("\n")
	writeTableHeader(&sb)
	writePlayerRows(&sb, bs.HomeTeamPlayers)

	// Footer
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 280))
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
	sb.WriteString("player_name,completions,incompletions,receptions,interceptions,fumbles,fumbles_lost,sacks,tackles,assists,passing_attempts,rushing_attempts,receiving_targets,passing_yards,rushing_yards,receiving_yards,passing_touchdowns,rushing_touchdowns,receiving_touchdowns,interceptions_thrown,sacks_taken\n")

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
			sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
				playerName,
				formatStat(player.Stats.Completions),
				formatStat(player.Stats.Incompletions),
				formatStat(player.Stats.Receptions),
				formatStat(player.Stats.Interceptions),
				formatStat(player.Stats.Fumbles),
				formatStat(player.Stats.FumblesLost),
				formatStat(player.Stats.Sacks),
				formatStat(player.Stats.Tackles),
				formatStat(player.Stats.Assists),
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
	sb.WriteString(fmt.Sprintf("%-25s | %-4s | %12s | %12s | %16s | %14s | %19s | %20s | %16s | %14s | %19s | %18s | %12s | %16s | %21s | %7s | %9s | %9s | %15s | %9s | %14s | %14s\n",
		"Player Name", "Pos",
		"Completions", "Incompletions", "Passing Attempts", "Passing Yards", "Passing Touchdowns", "Interceptions Thrown",
		"Rushing Attempts", "Rushing Yards", "Rushing Touchdowns",
		"Receiving Targets", "Receptions", "Receiving Yards", "Receiving Touchdowns",
		"Sacks", "Tackles", "Assists", "Interceptions", "Fumbles", "Fumbles Lost", "Sacks Taken"))

	sb.WriteString(strings.Repeat("-", 280))
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
			sb.WriteString(fmt.Sprintf("%-25s | %-4s | %12s | %12s | %16s | %14s | %19s | %20s | %16s | %14s | %19s | %18s | %12s | %16s | %21s | %7s | %9s | %9s | %15s | %9s | %14s | %14s\n",
				playerName, position,
				formatStat(player.Stats.Completions),
				formatStat(player.Stats.Incompletions),
				formatStat(player.Stats.PassingAttempts),
				formatStat(player.Stats.PassingYards),
				formatStat(player.Stats.PassingTouchdowns),
				formatStat(player.Stats.InterceptionsThrown),
				formatStat(player.Stats.RushingAttempts),
				formatStat(player.Stats.RushingYards),
				formatStat(player.Stats.RushingTouchdowns),
				formatStat(player.Stats.ReceivingTargets),
				formatStat(player.Stats.Receptions),
				formatStat(player.Stats.ReceivingYards),
				formatStat(player.Stats.ReceivingTouchdowns),
				formatStat(player.Stats.Sacks),
				formatStat(player.Stats.Tackles),
				formatStat(player.Stats.Assists),
				formatStat(player.Stats.Interceptions),
				formatStat(player.Stats.Fumbles),
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
