package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/population-scripts/cmd/common"
	"github.com/openbook/population-scripts/config"
	decorator_nba "github.com/openbook/population-scripts/decorator/nba"
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	persister_nba "github.com/openbook/population-scripts/persister/nba"
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
		fatal("Failed to load configuration: %v\nPlease ensure NBA_GAME_ID is set in .env file or as an environment variable", err)
	}

	// Validate that NBA_GAME_ID is set
	if cfg.NBAGameID == 0 {
		fatal("NBA_GAME_ID is required but not set\nPlease set NBA_GAME_ID in .env file or as an environment variable")
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NBA Play-by-Play Data Fetcher")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Game ID (database): %d\n", cfg.NBAGameID)
	fmt.Println(strings.Repeat("=", 72))

	ctx := context.Background()

	// Connect to database
	dbStore, err := common.ConnectToDatabase(ctx, &common.DatabaseConfig{
		Host:     cfg.PGHost,
		Port:     cfg.PGPort,
		Database: cfg.PGDatabase,
		User:     cfg.PGUser,
		Password: cfg.PGPassword,
		KeyPath:  cfg.PGKeyPath,
	})
	if err != nil {
		fatal("%v", err)
	}
	defer dbStore.Close()

	// Lookup game by database ID to get vendor_id for API call
	fmt.Println("\nLooking up game in database...")
	game, err := dbStore.GetGameByID(ctx, cfg.NBAGameID)
	if err != nil {
		fatal("Failed to lookup game with id %d: %v\nEnsure the game exists in the database (run update_reference_data first)", cfg.NBAGameID, err)
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
	playByPlay, err := fetcher_nba.FetchNBAPlayByPlay(apiClient, game.VendorID)
	if err != nil {
		fatal("Failed to fetch play-by-play data: %v", err)
	}
	fmt.Println("Successfully fetched play-by-play data!")

	// Validate that API response matches our expected game
	if playByPlay.ID != game.VendorID {
		fatal("API response vendor_id mismatch: expected %s, got %s", game.VendorID, playByPlay.ID)
	}

	// Decorate the fetched data with derived statistics (e.g., heave blocks)
	fmt.Println("\nDecorating play-by-play data...")
	playByPlay = decorator_nba.DecoratePlayByPlay(playByPlay)
	fmt.Println("Decoration complete!")

	// Print formatted output
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("FORMATTED OUTPUT")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Println(playByPlay.String())

	// Print raw JSON output
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("RAW JSON OUTPUT")
	fmt.Println(strings.Repeat("=", 72))
	jsonOutput, err := playByPlay.JSON()
	if err != nil {
		fatal("Failed to generate JSON output: %v", err)
	}
	fmt.Println(jsonOutput)

	// Step 1: Persist missing individuals (outside transaction, may make API calls)
	fmt.Println("\nEnsuring all referenced players exist in database...")
	if err := persister_nba.PersistMissingNBAIndividuals(ctx, dbStore, apiClient, playByPlay); err != nil {
		fatal("Failed to persist missing individuals: %v", err)
	}

	// Step 2: Persist play-by-play data in a transaction
	fmt.Println("\nPersisting play-by-play data to database...")
	err = common.WithTransaction(ctx, dbStore, func(tx pgx.Tx) error {
		if err := persister_nba.PersistNBAPlayByPlay(ctx, dbStore, tx, cfg.NBAGameID, playByPlay); err != nil {
			return fmt.Errorf("failed to persist play-by-play data: %w", err)
		}
		if err := persister_nba.CheckAndUpdateNBAPlayByPlayDeletions(ctx, dbStore, tx, cfg.NBAGameID, playByPlay); err != nil {
			return fmt.Errorf("failed to check and update deletions: %w", err)
		}
		return nil
	})
	if err != nil {
		fatal("%v", err)
	}
	fmt.Println("Successfully persisted play-by-play data!")

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Play-by-play data fetch and persistence completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
