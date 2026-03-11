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
	decorator_nfl "github.com/openbook/population-scripts/decorator/nfl"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	persister_nfl "github.com/openbook/population-scripts/persister/nfl"
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
	cfg, err := config.LoadPlayByPlayConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease ensure NFL_GAME_ID is set in .env file or as an environment variable", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NFL Play-by-Play Data Updater")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Game ID (database): %s\n", cfg.NFLGameID)
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

	// Lookup game by database ID to get sportradar_id for API call
	fmt.Println("\nLooking up game in database...")
	gameUUID, err := utils.ParseUUID(cfg.NFLGameID)
	if err != nil {
		fatal("Failed to parse NFL game ID %s as UUID: %v", cfg.NFLGameID, err)
	}
	game, err := dbStore.GetGameByID(ctx, gameUUID)
	if err != nil {
		fatal("Failed to lookup game with id %s: %v\nEnsure the game exists in the database (run update_reference_data first)", cfg.NFLGameID, err)
	}
	fmt.Printf("Found game with sportradar_id: %s\n", game.SportradarID)

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
	playByPlay, err := fetcher_nfl.FetchNFLPlayByPlay(apiClient, game.SportradarID)
	if err != nil {
		fatal("Failed to fetch play-by-play data: %v", err)
	}
	fmt.Println("Successfully fetched play-by-play data!")

	// Validate that API response matches our expected game
	if playByPlay.ID != game.SportradarID {
		fatal("API response sportradar_id mismatch: expected %s, got %s", game.SportradarID, playByPlay.ID)
	}

	// Decorate the fetched data with derived statistics (currently a no-op for NFL)
	playByPlay = decorator_nfl.DecoratePlayByPlay(playByPlay)

	// Step 1: Persist missing individuals (outside transaction, may make API calls)
	fmt.Println("\nEnsuring all referenced players exist in database...")
	if err := persister_nfl.PersistMissingNFLIndividuals(ctx, dbStore, apiClient, playByPlay); err != nil {
		fatal("Failed to persist missing individuals: %v", err)
	}

	// Step 2: Persist play-by-play data in a transaction
	fmt.Println("\nPersisting play-by-play data to database...")
	err = common.WithTransaction(ctx, dbStore, func(tx pgx.Tx) error {
		if err := persister_nfl.PersistNFLPlayByPlay(ctx, dbStore, tx, cfg.NFLGameID, playByPlay); err != nil {
			return fmt.Errorf("failed to persist play-by-play data: %w", err)
		}
		if err := persister_nfl.CheckAndUpdateNFLPlayByPlayDeletions(ctx, dbStore, tx, cfg.NFLGameID, playByPlay); err != nil {
			return fmt.Errorf("failed to check and update deletions: %w", err)
		}
		return nil
	})
	if err != nil {
		fatal("%v", err)
	}
	fmt.Println("Successfully persisted play-by-play data!")

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Play-by-play data update completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
