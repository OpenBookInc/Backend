package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openbook/population-scripts/config"
	"github.com/openbook/population-scripts/reader"
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
	// Reuse BoxScoreConfig since it has the same requirements (NFL_GAME_ID)
	cfg, err := config.LoadBoxScoreConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease ensure NFL_GAME_ID is set as a database integer ID in .env file or as an environment variable", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NFL Box Score Viewer")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Game ID (database): %d\n", cfg.NFLGameID)
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
	boxScore, err := reader.ReadNFLBoxScore(ctx, dbStore, cfg.NFLGameID)
	if err != nil {
		fatal("Failed to read box score data: %v", err)
	}

	// Check if there are any players
	totalPlayers := len(boxScore.HomeTeamPlayers) + len(boxScore.AwayTeamPlayers)
	if totalPlayers == 0 {
		fmt.Println("\nNo box score data found for this game.")
		fmt.Println("Ensure you have run update_box_score_data.sh for this game first.")
		fmt.Println(strings.Repeat("=", 72))
		return
	}

	fmt.Printf("Read box scores for %d players (Home: %d, Away: %d)\n",
		totalPlayers, len(boxScore.HomeTeamPlayers), len(boxScore.AwayTeamPlayers))

	// Print the box score
	fmt.Println("\n" + boxScore.String())
}
