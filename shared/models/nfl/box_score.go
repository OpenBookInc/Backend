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
	ID                  int                // Database ID (auto-increment)
	Game                *models.Game       `json:"-"` // Game information (pointer to registry instance)
	Individual          *models.Individual `json:"-"` // Player information (pointer to registry instance)
	PassingCompletions  decimal.Decimal    // Pass completions
	ReceivingReceptions decimal.Decimal    // Receptions
	InterceptionsCaught decimal.Decimal    // Interceptions caught (defensive stat)
	FumblesCommitted    decimal.Decimal    // Fumbles committed (offensive stat)
	SacksMade           decimal.Decimal    // Sacks made (defensive stat)
	SackAssistsMade     decimal.Decimal    // Sack assists (defensive stat)
	FieldGoalAttempts   decimal.Decimal    // Field goal attempts
	FieldGoalMakes      decimal.Decimal    // Field goals made
	ExtraPointAttempts  decimal.Decimal    // Extra point attempts
	ExtraPointMakes     decimal.Decimal    // Extra points made
	PassingAttempts     decimal.Decimal    // Passing attempts
	RushingAttempts     decimal.Decimal    // Rushing attempts
	ReceivingTargets    decimal.Decimal    // Receiving targets
	PassingYards        decimal.Decimal    // Passing yards
	RushingYards        decimal.Decimal    // Rushing yards
	ReceivingYards      decimal.Decimal    // Receiving yards
	PassingTouchdowns   decimal.Decimal    // Passing touchdowns
	RushingTouchdowns   decimal.Decimal    // Rushing touchdowns
	ReceivingTouchdowns decimal.Decimal    // Receiving touchdowns
	InterceptionsThrown decimal.Decimal    // Interceptions thrown (QB stat)
	SacksTaken          decimal.Decimal    // Sacks taken (QB stat)
}

// IndividualBoxScore combines player information with their game statistics
type IndividualBoxScore struct {
	Individual *models.Individual
	Stats      *NFLStats
}

// NFLBoxScore represents a complete box score for an NFL game
type NFLBoxScore struct {
	Game    *models.Game
	Players []*IndividualBoxScore // All players with stats for this game
}

// String returns a formatted table representation of the box score with all players in one section
func (bs *NFLBoxScore) String() string {
	var sb strings.Builder

	// Header
	sb.WriteString(strings.Repeat("=", 500))
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

	sb.WriteString(strings.Repeat("=", 500))
	sb.WriteString("\n")

	// All players section
	sb.WriteString(bs.stringRosterBoxScore("All Players", bs.Players))

	// Footer
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 500))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Players: %d\n", len(bs.Players)))

	return sb.String()
}

// StringWithRosters returns a formatted table representation organized by team rosters.
// Players are split into away team, home team, and unknown sections based on roster membership.
func (bs *NFLBoxScore) StringWithRosters(awayRoster, homeRoster *models.Roster) string {
	var sb strings.Builder

	// Header
	sb.WriteString(strings.Repeat("=", 500))
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

	sb.WriteString(strings.Repeat("=", 500))
	sb.WriteString("\n")

	// Split players by roster
	var awayPlayers, homePlayers, unknownPlayers []*IndividualBoxScore

	for _, player := range bs.Players {
		if player.Individual == nil {
			unknownPlayers = append(unknownPlayers, player)
			continue
		}

		individualID := int64(player.Individual.ID)

		// Check if player is in away roster
		inAwayRoster := false
		if awayRoster != nil {
			for _, rosterID := range awayRoster.IndividualIDs {
				if rosterID == individualID {
					inAwayRoster = true
					break
				}
			}
		}

		// Check if player is in home roster
		inHomeRoster := false
		if homeRoster != nil {
			for _, rosterID := range homeRoster.IndividualIDs {
				if rosterID == individualID {
					inHomeRoster = true
					break
				}
			}
		}

		// Assign to appropriate list
		if inAwayRoster {
			awayPlayers = append(awayPlayers, player)
		} else if inHomeRoster {
			homePlayers = append(homePlayers, player)
		} else {
			unknownPlayers = append(unknownPlayers, player)
		}
	}

	// Away team section
	sb.WriteString(bs.stringRosterBoxScore(fmt.Sprintf("%s (Away Team)", awayTeamName), awayPlayers))

	// Home team section
	sb.WriteString(bs.stringRosterBoxScore(fmt.Sprintf("%s (Home Team)", homeTeamName), homePlayers))

	// Unknown section (only if there are unknown players)
	if len(unknownPlayers) > 0 {
		sb.WriteString(bs.stringRosterBoxScore("Unknown Team", unknownPlayers))
	}

	// Footer
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 500))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Players: %d (Away: %d, Home: %d, Unknown: %d)\n",
		len(bs.Players),
		len(awayPlayers),
		len(homePlayers),
		len(unknownPlayers)))

	return sb.String()
}

// stringRosterBoxScore returns a formatted section for a list of players with a given header
func (bs *NFLBoxScore) stringRosterBoxScore(header string, players []*IndividualBoxScore) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n%s - %d Players\n", header, len(players)))
	sb.WriteString(strings.Repeat("-", 500))
	sb.WriteString("\n")
	writeTableHeader(&sb)
	writePlayerRows(&sb, players)

	return sb.String()
}

// Csv returns a CSV-formatted string of all player stats for the game.
// Columns: player_name, and all stats in lowercase with underscores.
// Rows: all players in the game.
func (bs *NFLBoxScore) Csv() string {
	var sb strings.Builder

	// Header row
	sb.WriteString("player_name,passing_completions,receiving_receptions,interceptions_caught,fumbles_committed,sacks_made,sack_assists_made,passing_attempts,rushing_attempts,receiving_targets,passing_yards,rushing_yards,receiving_yards,passing_touchdowns,rushing_touchdowns,receiving_touchdowns,interceptions_thrown,sacks_taken,field_goal_attempts,field_goal_makes,extra_point_attempts,extra_point_makes\n")

	// Write all players
	writeCsvRows(&sb, bs.Players)

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
			sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
				playerName,
				formatStat(player.Stats.PassingCompletions),
				formatStat(player.Stats.ReceivingReceptions),
				formatStat(player.Stats.InterceptionsCaught),
				formatStat(player.Stats.FumblesCommitted),
				formatStat(player.Stats.SacksMade),
				formatStat(player.Stats.SackAssistsMade),
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
				formatStat(player.Stats.SacksTaken),
				formatStat(player.Stats.FieldGoalAttempts),
				formatStat(player.Stats.FieldGoalMakes),
				formatStat(player.Stats.ExtraPointAttempts),
				formatStat(player.Stats.ExtraPointMakes)))
		}
	}
}

// writeTableHeader writes the column headers for the stats table
func writeTableHeader(sb *strings.Builder) {
	sb.WriteString(fmt.Sprintf("%-25s | %-4s | %19s | %19s | %16s | %14s | %19s | %20s | %16s | %14s | %19s | %18s | %19s | %16s | %21s | %11s | %18s | %19s | %14s | %14s | %20s | %17s | %21s | %18s\n",
		"Player Name", "Pos",
		"Passing Completions", "Receiving Receptions", "Passing Attempts", "Passing Yards", "Passing Touchdowns", "Interceptions Thrown",
		"Rushing Attempts", "Rushing Yards", "Rushing Touchdowns",
		"Receiving Targets", "Receiving Receptions", "Receiving Yards", "Receiving Touchdowns",
		"Sacks Made", "Sack Assists Made", "Interceptions Caught", "Fumbles Committed", "Sacks Taken",
		"Field Goal Attempts", "Field Goal Makes", "Extra Point Attempts", "Extra Point Makes"))

	sb.WriteString(strings.Repeat("-", 500))
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
			sb.WriteString(fmt.Sprintf("%-25s | %-4s | %19s | %19s | %16s | %14s | %19s | %20s | %16s | %14s | %19s | %18s | %19s | %16s | %21s | %11s | %18s | %19s | %14s | %14s | %20s | %17s | %21s | %18s\n",
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
				formatStat(player.Stats.SackAssistsMade),
				formatStat(player.Stats.InterceptionsCaught),
				formatStat(player.Stats.FumblesCommitted),
				formatStat(player.Stats.SacksTaken),
				formatStat(player.Stats.FieldGoalAttempts),
				formatStat(player.Stats.FieldGoalMakes),
				formatStat(player.Stats.ExtraPointAttempts),
				formatStat(player.Stats.ExtraPointMakes)))
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
