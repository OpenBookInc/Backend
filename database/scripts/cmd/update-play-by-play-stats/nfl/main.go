package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/population-scripts/config"
	decorator_nfl "github.com/openbook/population-scripts/decorator/nfl"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	persister_nfl "github.com/openbook/population-scripts/persister/nfl"
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
	cfg, err := config.LoadPlayByPlayConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease ensure NFL_GAME_ID is set in .env file or as an environment variable", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NFL Play-by-Play Data Updater")
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

	// Lookup game by database ID to get vendor_id for API call
	fmt.Println("\nLooking up game in database...")
	game, err := dbStore.GetGameByID(ctx, cfg.NFLGameID)
	if err != nil {
		fatal("Failed to lookup game with id %d: %v\nEnsure the game exists in the database (run update_reference_data first)", cfg.NFLGameID, err)
	}
	fmt.Printf("Found game with vendor_id: %s\n", game.VendorID)

	// Create API client with configured rate limit and access level
	clientConfig := &sportradar.ClientConfig{
		AccessLevel:    cfg.SportradarAccessLevel,
		RateLimitDelay: time.Duration(cfg.RateLimitDelayMilliseconds) * time.Millisecond,
		Timeout:        30 * time.Second,
		ApiKeys:        cfg.SportradarAPIKeys,
	}
	apiClient := sportradar.NewClientWithConfig(clientConfig)

	// Fetch play-by-play data
	fmt.Println("\nFetching play-by-play data from Sportradar API...")
	playByPlay, err := fetcher_nfl.FetchNFLPlayByPlay(apiClient, game.VendorID)
	if err != nil {
		fatal("Failed to fetch play-by-play data: %v", err)
	}
	fmt.Println("Successfully fetched play-by-play data!")

	// Validate that API response matches our expected game
	if playByPlay.ID != game.VendorID {
		fatal("API response vendor_id mismatch: expected %s, got %s", game.VendorID, playByPlay.ID)
	}

	// Decorate the fetched data with derived statistics (currently a no-op for NFL)
	playByPlay = decorator_nfl.DecoratePlayByPlay(playByPlay)

	// Step 1: Persist missing individuals (outside transaction, may make API calls)
	fmt.Println("\nEnsuring all referenced players exist in database...")
	if err := persister_nfl.PersistMissingNFLIndividuals(ctx, dbStore, apiClient, playByPlay); err != nil {
		fatal("Failed to persist missing individuals: %v", err)
	}

	// Step 2: Begin transaction for play-by-play persistence and deletion checking
	fmt.Println("\nPersisting play-by-play data to database...")
	tx, err := dbStore.BeginTx(ctx)
	if err != nil {
		fatal("Failed to begin transaction: %v", err)
	}

	// Persist play-by-play data
	if err := persister_nfl.PersistNFLPlayByPlay(ctx, dbStore, tx, cfg.NFLGameID, playByPlay); err != nil {
		tx.Rollback(ctx)
		fatal("Failed to persist play-by-play data: %v", err)
	}

	// Check and update deletions
	if err := persister_nfl.CheckAndUpdateNFLPlayByPlayDeletions(ctx, dbStore, tx, cfg.NFLGameID, playByPlay); err != nil {
		tx.Rollback(ctx)
		fatal("Failed to check and update deletions: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		fatal("Failed to commit transaction: %v", err)
	}
	fmt.Println("Successfully persisted play-by-play data!")

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Play-by-play data update completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
