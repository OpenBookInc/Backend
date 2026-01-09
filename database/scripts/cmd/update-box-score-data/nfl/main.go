package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openbook/population-scripts/config"
	persister_nfl "github.com/openbook/population-scripts/persister/nfl"
	reader_nfl "github.com/openbook/population-scripts/reader/nfl"
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
	cfg, err := config.LoadBoxScoreConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease ensure NFL_GAME_ID is set as a database integer ID in .env file or as an environment variable", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NFL Box Score Data Generator")
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

	// Read play-by-play data from database
	fmt.Println("\nReading play-by-play data from database...")
	pbpData, err := reader_nfl.ReadNFLPlayByPlay(ctx, dbStore, cfg.NFLGameID)
	if err != nil {
		fatal("Failed to read play-by-play data: %v", err)
	}
	fmt.Printf("Read %d play statistics from database\n", len(pbpData.Statistics))

	// Check if there are any statistics to process
	if len(pbpData.Statistics) == 0 {
		fmt.Println("\nNo play statistics found for this game. Nothing to persist.")
		fmt.Println(strings.Repeat("=", 72))
		fmt.Println("Box score generation completed (no data)")
		fmt.Println(strings.Repeat("=", 72))
		return
	}

	// Calculate how many box scores will be generated
	boxScoreCount := persister_nfl.GetBoxScoreCount(pbpData)
	fmt.Printf("Will generate box scores for %d players\n", boxScoreCount)

	// Persist box scores to database
	fmt.Println("\nPersisting box scores to database...")
	if err := persister_nfl.PersistNFLBoxScores(ctx, dbStore, pbpData); err != nil {
		fatal("Failed to persist box scores: %v", err)
	}
	fmt.Printf("Successfully persisted %d box scores!\n", boxScoreCount)

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Box score generation completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
