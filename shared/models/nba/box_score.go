package nba

import (
	"fmt"
	"strings"

	"github.com/openbook/shared/models"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Box Score Database Models
// =============================================================================
// These models represent box score data for NBA games.
// A box score aggregates all play-by-play statistics for each player in a game.
// =============================================================================

// NBAStats holds aggregated statistics for a player in a single game
type NBAStats struct {
	Game                   *models.Game          `json:"-"` // Game information (pointer to registry instance)
	Individual             *models.Individual    `json:"-"` // Player information (pointer to registry instance)
	TwoPointAttempts       decimal.Decimal       // Two-point field goal attempts
	TwoPointMakes          decimal.Decimal       // Two-point field goals made
	ThreePointAttempts     decimal.Decimal       // Three-point field goal attempts
	ThreePointMakes        decimal.Decimal       // Three-point field goals made
	FreeThrowAttempts      decimal.Decimal       // Free throw attempts
	FreeThrowMakes         decimal.Decimal       // Free throws made
	Assists                decimal.Decimal       // Assists
	DefensiveRebounds      decimal.Decimal       // Defensive rebounds
	OffensiveRebounds      decimal.Decimal       // Offensive rebounds
	Steals                 decimal.Decimal       // Steals
	Blocks                 decimal.Decimal       // Blocks
	TurnoversCommitted     decimal.Decimal       // Turnovers committed
	PersonalFoulsCommitted decimal.Decimal       // Personal fouls committed
}

// IndividualBoxScore combines player information with their game statistics
type IndividualBoxScore struct {
	Individual *models.Individual
	Stats      *NBAStats
}

// NBABoxScore represents a complete box score for an NBA game
type NBABoxScore struct {
	Game    *models.Game
	Players []*IndividualBoxScore // All players with stats for this game
}

// tableWidth is the width of the formatted table for consistent separators
const tableWidth = 295

// String returns a formatted table representation of the box score with all players in one section
func (bs *NBABoxScore) String() string {
	var sb strings.Builder

	// Header
	sb.WriteString(strings.Repeat("=", tableWidth))
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
		sb.WriteString(fmt.Sprintf("NBA Box Score: %s vs %s\n", awayTeamName, homeTeamName))
		sb.WriteString(fmt.Sprintf("Game ID: %s | %s\n", bs.Game.ID, bs.Game.ScheduledStartTime.Format("2006-01-02 15:04:05 MST")))
	} else {
		sb.WriteString("NBA Box Score: Unknown Game\n")
	}

	sb.WriteString(strings.Repeat("=", tableWidth))
	sb.WriteString("\n")

	// All players section
	sb.WriteString(bs.stringRosterBoxScore("All Players", bs.Players))

	// Footer
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", tableWidth))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Players: %d\n", len(bs.Players)))

	return sb.String()
}

// StringWithRosters returns a formatted table representation organized by team rosters.
// Players are split into away team, home team, and unknown sections based on roster membership.
func (bs *NBABoxScore) StringWithRosters(awayRoster, homeRoster *models.Roster) string {
	var sb strings.Builder

	// Header
	sb.WriteString(strings.Repeat("=", tableWidth))
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
		sb.WriteString(fmt.Sprintf("NBA Box Score: %s vs %s\n", awayTeamName, homeTeamName))
		sb.WriteString(fmt.Sprintf("Game ID: %s | %s\n", bs.Game.ID, bs.Game.ScheduledStartTime.Format("2006-01-02 15:04:05 MST")))
	} else {
		sb.WriteString("NBA Box Score: Unknown Game\n")
	}

	sb.WriteString(strings.Repeat("=", tableWidth))
	sb.WriteString("\n")

	// Split players by roster
	var awayPlayers, homePlayers, unknownPlayers []*IndividualBoxScore

	for _, player := range bs.Players {
		if player.Individual == nil {
			unknownPlayers = append(unknownPlayers, player)
			continue
		}

		// Check if player is in away roster
		inAwayRoster := false
		if awayRoster != nil {
			for _, rosterID := range awayRoster.IndividualIDs {
				if rosterID == player.Individual.ID {
					inAwayRoster = true
					break
				}
			}
		}

		// Check if player is in home roster
		inHomeRoster := false
		if homeRoster != nil {
			for _, rosterID := range homeRoster.IndividualIDs {
				if rosterID == player.Individual.ID {
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
	sb.WriteString(strings.Repeat("=", tableWidth))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Players: %d (Away: %d, Home: %d, Unknown: %d)\n",
		len(bs.Players),
		len(awayPlayers),
		len(homePlayers),
		len(unknownPlayers)))

	return sb.String()
}

// stringRosterBoxScore returns a formatted section for a list of players with a given header
func (bs *NBABoxScore) stringRosterBoxScore(header string, players []*IndividualBoxScore) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n%s - %d Players\n", header, len(players)))
	sb.WriteString(strings.Repeat("-", tableWidth))
	sb.WriteString("\n")
	writeTableHeader(&sb)
	writePlayerRows(&sb, players)

	return sb.String()
}

// writeTableHeader writes the column headers for the stats table
func writeTableHeader(sb *strings.Builder) {
	sb.WriteString(fmt.Sprintf("%-25s | %-4s | %18s | %15s | %20s | %17s | %19s | %16s | %7s | %18s | %18s | %6s | %6s | %19s | %23s\n",
		"Player Name", "Pos",
		"Two Point Attempts", "Two Point Makes",
		"Three Point Attempts", "Three Point Makes",
		"Free Throw Attempts", "Free Throw Makes",
		"Assists", "Defensive Rebounds", "Offensive Rebounds",
		"Steals", "Blocks", "Turnovers Committed", "Personal Fouls Committed"))

	sb.WriteString(strings.Repeat("-", tableWidth))
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
			sb.WriteString(fmt.Sprintf("%-25s | %-4s | %18s | %15s | %20s | %17s | %19s | %16s | %7s | %18s | %18s | %6s | %6s | %19s | %23s\n",
				playerName, position,
				formatStat(player.Stats.TwoPointAttempts),
				formatStat(player.Stats.TwoPointMakes),
				formatStat(player.Stats.ThreePointAttempts),
				formatStat(player.Stats.ThreePointMakes),
				formatStat(player.Stats.FreeThrowAttempts),
				formatStat(player.Stats.FreeThrowMakes),
				formatStat(player.Stats.Assists),
				formatStat(player.Stats.DefensiveRebounds),
				formatStat(player.Stats.OffensiveRebounds),
				formatStat(player.Stats.Steals),
				formatStat(player.Stats.Blocks),
				formatStat(player.Stats.TurnoversCommitted),
				formatStat(player.Stats.PersonalFoulsCommitted)))
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
