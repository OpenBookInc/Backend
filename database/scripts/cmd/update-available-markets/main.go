package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openbook/population-scripts/client/oddsblaze"
	"github.com/openbook/population-scripts/cmd/common"
	"github.com/openbook/population-scripts/config"
	fetcher_oddsblaze "github.com/openbook/population-scripts/fetcher/oddsblaze"
	persister_oddsblaze "github.com/openbook/population-scripts/persister/oddsblaze"
)

// fatal prints an error message to stderr and exits with code 1
func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	envFile := flag.String("env", "", "path to environment file (default: .env)")
	flag.Parse()

	cfg, err := config.LoadOddsBlazeConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v", err)
	}

	ctx := context.Background()

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

	// Load entity vendor IDs into registry for fast lookups
	vendorIDCount, err := dbStore.LoadEntityVendorIDs(ctx)
	if err != nil {
		fatal("Failed to load entity vendor IDs: %v", err)
	}

	apiClient := oddsblaze.NewClient(&oddsblaze.ClientConfig{
		APIKey:         cfg.OddsBlazeAPIKey,
		Timeout:        30 * time.Second,
		RateLimitDelay: time.Duration(cfg.OddsBlazeRateLimitDelayMilliseconds) * time.Millisecond,
	})

	// Convert league to uppercase for DB lookup (e.g., "nba" -> "NBA")
	leagueName := strings.ToUpper(cfg.OddsBlazeLeague)

	// Optional timestamp pointer
	var timestamp *string
	if cfg.OddsBlazeTimestamp != "" {
		timestamp = &cfg.OddsBlazeTimestamp
	}

	fmt.Println("\nStarting update available markets...")
	fmt.Printf("League: %s\n", leagueName)
	fmt.Printf("Sportsbooks: %s\n", strings.Join(cfg.OddsBlazeSportsbooks, ", "))
	fmt.Printf("Vendor IDs loaded: %d\n", vendorIDCount)
	fmt.Println(strings.Repeat("=", 72))

	totalMarkets := 0

	for _, sportsbook := range cfg.OddsBlazeSportsbooks {
		fmt.Printf("\n--- Sportsbook: %s ---\n", sportsbook)

		// Fetch odds
		fmt.Printf("  Fetching odds from OddsBlaze API...\n")
		oddsResp, err := fetcher_oddsblaze.FetchOdds(apiClient, sportsbook, cfg.OddsBlazeLeague, timestamp)
		if err != nil {
			fatal("Failed to fetch odds for sportsbook %s: %v", sportsbook, err)
		}
		fmt.Printf("  Fetched %d events\n", len(oddsResp.Events))

		// Persist markets
		fmt.Printf("  Persisting player prop markets...\n")
		markets, err := persister_oddsblaze.PersistMarkets(ctx, dbStore, leagueName, oddsResp)
		if err != nil {
			fatal("Failed to persist markets for sportsbook %s: %v", sportsbook, err)
		}
		fmt.Printf("  Persisted %d markets\n", markets)

		totalMarkets += markets
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Update Available Markets Summary:")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Total Markets Persisted: %d\n", totalMarkets)
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Update available markets complete!")
	fmt.Println(strings.Repeat("=", 72))
}
