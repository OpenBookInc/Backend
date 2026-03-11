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
	store_nfl "github.com/openbook/population-scripts/store/nfl"
	"github.com/openbook/shared/utils"
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
		fatal("Failed to load configuration: %v\nPlease ensure NFL_GAME_ID is set as a database UUID in .env file or as an environment variable", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NFL Box Score Data Generator")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Game ID (database): %s\n", cfg.NFLGameID)
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

	// Step 1: Get existing box scores before starting the transaction
	fmt.Println("\nGetting existing box scores from database...")
	gameUUID, err := utils.ParseUUID(cfg.NFLGameID)
	if err != nil {
		fatal("Failed to parse NFL game ID %s as UUID: %v", cfg.NFLGameID, err)
	}
	existingBoxScores, err := store_nfl.GetNFLBoxScoresByGameID(dbStore, ctx, gameUUID)
	if err != nil {
		fatal("Failed to get existing box scores: %v", err)
	}
	fmt.Printf("Found %d existing box scores\n", len(existingBoxScores))

	// Step 2: Begin transaction for box score persistence and deletion checking
	fmt.Println("\nPersisting box scores to database...")
	tx, err := dbStore.BeginTx(ctx)
	if err != nil {
		fatal("Failed to begin transaction: %v", err)
	}

	// Persist box scores
	upsertedBoxScores, err := persister_nfl.PersistNFLBoxScores(ctx, dbStore, tx, pbpData)
	if err != nil {
		tx.Rollback(ctx)
		fatal("Failed to persist box scores: %v", err)
	}

	// Check and update deletions
	if err := persister_nfl.CheckAndUpdateNFLBoxScoreDeletions(ctx, dbStore, tx, existingBoxScores, upsertedBoxScores); err != nil {
		tx.Rollback(ctx)
		fatal("Failed to check and update deletions: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		fatal("Failed to commit transaction: %v", err)
	}
	fmt.Printf("Successfully persisted %d box scores!\n", boxScoreCount)

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Box score generation completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
