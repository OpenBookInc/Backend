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
	matcher_oddsblaze "github.com/openbook/population-scripts/matcher/oddsblaze"
	reducer_oddsblaze "github.com/openbook/population-scripts/reducer/oddsblaze"
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

	fmt.Println("\nStarting OddsBlaze reference data mapping...")
	fmt.Printf("League: %s\n", leagueName)
	fmt.Printf("Sportsbooks: %s\n", strings.Join(cfg.OddsBlazeSportsbooks, ", "))
	fmt.Println(strings.Repeat("=", 72))

	totalTeams := 0
	totalIndividuals := 0
	totalGames := 0

	for _, sportsbook := range cfg.OddsBlazeSportsbooks {
		fmt.Printf("\n--- Sportsbook: %s ---\n", sportsbook)

		// Step 1: Fetch odds
		fmt.Printf("  Fetching odds from OddsBlaze API...\n")
		oddsResp, err := fetcher_oddsblaze.FetchOdds(apiClient, sportsbook, cfg.OddsBlazeLeague, timestamp)
		if err != nil {
			fatal("Failed to fetch odds for sportsbook %s: %v", sportsbook, err)
		}
		fmt.Printf("  Fetched %d events\n", len(oddsResp.Events))

		// Step 2: Reduce to unique entities
		fmt.Printf("  Reducing to unique entities...\n")
		reduced, err := reducer_oddsblaze.ReduceOddsResponse(oddsResp)
		if err != nil {
			fatal("Failed to reduce odds response for sportsbook %s: %v", sportsbook, err)
		}
		fmt.Printf("  Found %d teams, %d individuals, %d games\n",
			len(reduced.Teams), len(reduced.Individuals), len(reduced.Games))

		// Step 3: Match against database
		fmt.Printf("  Matching against database...\n")
		matched, err := matcher_oddsblaze.MatchEntities(ctx, dbStore, leagueName, reduced)
		if err != nil {
			fatal("Failed to match entities for sportsbook %s: %v", sportsbook, err)
		}
		fmt.Printf("  Matched %d teams, %d individuals, %d games\n",
			len(matched.Teams), len(matched.Individuals), len(matched.Games))

		// Step 4: Persist to entity_vendor_ids
		fmt.Printf("  Persisting entity vendor IDs...\n")
		teams, individuals, games, err := persister_oddsblaze.PersistMatchedEntities(dbStore, ctx, matched)
		if err != nil {
			fatal("Failed to persist entity vendor IDs for sportsbook %s: %v", sportsbook, err)
		}
		fmt.Printf("  Persisted %d teams, %d individuals, %d games\n", teams, individuals, games)

		totalTeams += teams
		totalIndividuals += individuals
		totalGames += games
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("OddsBlaze Reference Data Mapping Summary:")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Total Teams Mapped: %d\n", totalTeams)
	fmt.Printf("Total Individuals Mapped: %d\n", totalIndividuals)
	fmt.Printf("Total Games Mapped: %d\n", totalGames)
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("OddsBlaze reference data mapping complete!")
	fmt.Println(strings.Repeat("=", 72))
}
