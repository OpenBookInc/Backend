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
	fmt.Printf("Game ID: %s\n", cfg.NFLGameID)
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

	// Create API client with configured rate limit
	apiClient := client.NewClientWithDelay(cfg.SportradarAPIKey, cfg.RateLimitDelayMilliseconds)

	// Fetch play-by-play data
	fmt.Println("\nFetching play-by-play data from Sportradar API...")
	playByPlay, err := nfl.FetchNFLPlayByPlay(apiClient, cfg.NFLGameID)
	if err != nil {
		fatal("Failed to fetch play-by-play data: %v", err)
	}
	fmt.Println("Successfully fetched play-by-play data!")

	// Persist play-by-play data to database
	fmt.Println("\nPersisting play-by-play data to database...")
	if err := persister.PersistNFLPlayByPlay(ctx, dbStore, playByPlay); err != nil {
		fatal("Failed to persist play-by-play data: %v", err)
	}
	fmt.Println("Successfully persisted play-by-play data!")

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Play-by-play data update completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
