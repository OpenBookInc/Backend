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
	"github.com/openbook/shared/models/gen"
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

	// Convert league config to MarketEntity enum
	leagueName := strings.ToUpper(cfg.OddsBlazeLeague)
	marketEntity, err := marketEntityForLeague(leagueName)
	if err != nil {
		fatal("%v", err)
	}

	fmt.Println("\nStarting grade market outcomes...")
	fmt.Printf("League: %s\n", leagueName)
	fmt.Println(strings.Repeat("=", 72))

	// Query ungraded market IDs
	fmt.Println("Querying ungraded closed market IDs...")
	ungradedIDs, err := dbStore.GetUngradedOddsBlazeMarketIDs(ctx, marketEntity)
	if err != nil {
		fatal("Failed to query ungraded market IDs: %v", err)
	}
	fmt.Printf("Found %d ungraded market IDs\n", len(ungradedIDs))

	if len(ungradedIDs) == 0 {
		fmt.Println("\nNo ungraded markets to process.")
		fmt.Println(strings.Repeat("=", 72))
		fmt.Println("Grade market outcomes complete!")
		fmt.Println(strings.Repeat("=", 72))
		return
	}

	apiClient := oddsblaze.NewClient(&oddsblaze.ClientConfig{
		APIKey:         cfg.OddsBlazeAPIKey,
		Timeout:        30 * time.Second,
		RateLimitDelay: time.Duration(cfg.OddsBlazeRateLimitDelayMilliseconds) * time.Millisecond,
	})

	// Grade each market
	gradedCount := 0
	for i, oddsBlazeID := range ungradedIDs {
		fmt.Printf("  [%d/%d] Grading %s...\n", i+1, len(ungradedIDs), oddsBlazeID)

		graderResp, err := fetcher_oddsblaze.FetchGraderResult(apiClient, oddsBlazeID)
		if err != nil {
			fatal("Failed to fetch grader result for %s: %v", oddsBlazeID, err)
		}

		err = persister_oddsblaze.PersistMarketOutcome(ctx, dbStore, oddsBlazeID, graderResp)
		if err != nil {
			fatal("Failed to persist market outcome for %s: %v", oddsBlazeID, err)
		}

		gradedCount++
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Grade Market Outcomes Summary:")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Total Markets Graded: %d\n", gradedCount)
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Grade market outcomes complete!")
	fmt.Println(strings.Repeat("=", 72))
}

// marketEntityForLeague converts a league name string to its MarketEntity enum value.
func marketEntityForLeague(league string) (gen.MarketEntity, error) {
	switch league {
	case "NBA":
		return gen.MarketEntityNbaMarket, nil
	case "NFL":
		return gen.MarketEntityNflMarket, nil
	default:
		return "", fmt.Errorf("unsupported league: %s", league)
	}
}
