package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openbook/population-scripts/config"
	"github.com/openbook/population-scripts/reader"
	reader_nba "github.com/openbook/population-scripts/reader/nba"
	"github.com/openbook/population-scripts/store"
)

// fatal prints an error message to stderr and exits with code 1
func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	// Parse command-line flags
	envFile := flag.String("env", "", "path to environment file (default: .env)")
	flag.Parse()

	// Load configuration from environment file and variables
	// Reuse BoxScoreConfig since it has the same requirements (NBA_GAME_ID)
	cfg, err := config.LoadBoxScoreConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease ensure NBA_GAME_ID is set as a database integer ID in .env file or as an environment variable", err)
	}

	// Ensure NBA_GAME_ID is specifically set (not just NFL_GAME_ID)
	if cfg.NBAGameID == 0 {
		fatal("NBA_GAME_ID must be set as a positive integer database ID in .env file or as an environment variable")
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NBA Box Score Viewer")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Game ID (database): %d\n", cfg.NBAGameID)
	fmt.Println(strings.Repeat("=", 72))

	ctx := context.Background()

	// Connect to database
	fmt.Println("\nConnecting to database...")
	dbStore, err := store.New(ctx, cfg.PGHost, cfg.PGPort, cfg.PGDatabase, cfg.PGUser, cfg.PGPassword, cfg.PGKeyPath)
	if err != nil {
		fatal("Failed to connect to database: %v", err)
	}
	defer dbStore.Close()
	fmt.Println("Connected to database successfully!")

	// Read box score data from database
	fmt.Println("\nReading box score data from database...")
	boxScore, err := reader_nba.ReadNBABoxScore(ctx, dbStore, cfg.NBAGameID)
	if err != nil {
		fatal("Failed to read box score data: %v", err)
	}

	// Check if there are any players
	totalPlayers := len(boxScore.Players)
	if totalPlayers == 0 {
		fmt.Println("\nNo box score data found for this game.")
		fmt.Println("Ensure you have run update_nba_box_score_data.sh for this game first.")
		fmt.Println(strings.Repeat("=", 72))
		return
	}

	fmt.Printf("Read box scores for %d players\n", totalPlayers)

	// Read rosters for home and away teams
	fmt.Println("\nReading rosters for home and away teams...")
	homeRoster, err := reader.ReadRosterByTeamID(ctx, dbStore, boxScore.Game.ContenderIDA)
	if err != nil {
		fatal("Failed to read home team roster (team_id %d): %v", boxScore.Game.ContenderIDA, err)
	}

	awayRoster, err := reader.ReadRosterByTeamID(ctx, dbStore, boxScore.Game.ContenderIDB)
	if err != nil {
		fatal("Failed to read away team roster (team_id %d): %v", boxScore.Game.ContenderIDB, err)
	}

	fmt.Printf("Successfully read rosters (Home: %d players, Away: %d players)\n",
		len(homeRoster.IndividualIDs), len(awayRoster.IndividualIDs))

	// Print the box score with roster organization
	fmt.Println("\n" + boxScore.StringWithRosters(awayRoster, homeRoster))
}
