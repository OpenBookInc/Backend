package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/config"
	"github.com/openbook/population-scripts/fetcher/nfl"
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
	fmt.Println("NFL Play-by-Play Data Fetcher")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Game ID: %s\n", cfg.NFLGameID)
	fmt.Println(strings.Repeat("=", 72))

	// Create API client with configured rate limit
	apiClient := client.NewClientWithDelay(cfg.SportradarAPIKey, cfg.RateLimitDelayMilliseconds)

	// Fetch play-by-play data
	fmt.Println("\nFetching play-by-play data from Sportradar API...")
	playByPlay, err := nfl.FetchNFLPlayByPlay(apiClient, cfg.NFLGameID)
	if err != nil {
		fatal("Failed to fetch play-by-play data: %v", err)
	}

	fmt.Println("Successfully fetched play-by-play data!")

	// Print the play-by-play data
	fmt.Println("\n" + playByPlay.String())

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Play-by-play data fetch completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}
