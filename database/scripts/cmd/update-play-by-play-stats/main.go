package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/config"
	"github.com/openbook/population-scripts/fetcher/nfl"
	"github.com/openbook/population-scripts/persister"
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

	// Create API client with configured rate limit
	apiClient := client.NewClientWithDelay(cfg.SportradarAPIKey, cfg.RateLimitDelayMilliseconds)

	// Fetch play-by-play data
	fmt.Println("\nFetching play-by-play data from Sportradar API...")
	playByPlay, err := nfl.FetchNFLPlayByPlay(apiClient, game.VendorID)
	if err != nil {
		fatal("Failed to fetch play-by-play data: %v", err)
	}
	fmt.Println("Successfully fetched play-by-play data!")

	// Validate that API response matches our expected game
	if playByPlay.ID != game.VendorID {
		fatal("API response vendor_id mismatch: expected %s, got %s", game.VendorID, playByPlay.ID)
	}

	// Persist play-by-play data to database
	fmt.Println("\nPersisting play-by-play data to database...")
	if err := persister.PersistNFLPlayByPlay(ctx, dbStore, cfg.NFLGameID, playByPlay); err != nil {
		fatal("Failed to persist play-by-play data: %v", err)
	}
	fmt.Println("Successfully persisted play-by-play data!")

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Play-by-play data update completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
