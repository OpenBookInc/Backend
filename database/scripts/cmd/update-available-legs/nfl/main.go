package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/population-scripts/cmd/common"
	available_legs_common "github.com/openbook/population-scripts/cmd/update-available-legs/common"
	"github.com/openbook/population-scripts/config"
	"github.com/openbook/population-scripts/fetcher"
	"github.com/openbook/population-scripts/store"
	models "github.com/openbook/shared/models"
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
	cfg, err := config.LoadAvailableLegsConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NFL Available Legs - Build Player Props Mappings")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Game Date: %s\n", cfg.GameDate.Format("2006-01-02"))
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

	// Query games for the given date
	fmt.Println("\nQuerying NFL games for date...")
	games, err := dbStore.GetGamesByLeagueAndDateRange(ctx, "NFL", cfg.GameDate, cfg.GameDate, cfg.TimeZoneForDate)
	if err != nil {
		fatal("Failed to query NFL games: %v", err)
	}
	fmt.Printf("Found %d NFL game(s)\n", len(games))
	if len(games) == 0 {
		fmt.Println("\nNo NFL games found for this date. Nothing to do.")
		return
	}

	// Query rosters for all teams in those games
	fmt.Println("\nQuerying rosters for game teams...")
	rosters, err := readRostersForGames(ctx, dbStore, games)
	if err != nil {
		fatal("Failed to read rosters: %v", err)
	}
	fmt.Printf("Loaded %d roster(s)\n", len(rosters))

	// Create API client
	clientConfig := &sportradar.ClientConfig{
		AccessLevel:    cfg.SportradarAccessLevel,
		RateLimitDelay: time.Duration(cfg.RateLimitDelayMilliseconds) * time.Millisecond,
		Timeout:        30 * time.Second,
		ApiKeys:        cfg.SportradarAPIKeys,
	}
	apiClient := sportradar.NewClientWithConfig(clientConfig)

	// Fetch the three mapping endpoints
	fmt.Println("\nFetching player props mappings from Sportradar API...")

	sportEventMappings, err := fetcher.FetchPlayerPropsSportEventMappings(apiClient)
	if err != nil {
		fatal("Failed to fetch sport event mappings: %v", err)
	}
	apiClient.RateLimitWait()

	competitorMappings, err := fetcher.FetchPlayerPropsCompetitorMappings(apiClient)
	if err != nil {
		fatal("Failed to fetch competitor mappings: %v", err)
	}
	apiClient.RateLimitWait()

	playerMappings, err := fetcher.FetchPlayerPropsPlayerMappings(apiClient)
	if err != nil {
		fatal("Failed to fetch player mappings: %v", err)
	}

	fmt.Printf("Fetched mappings: %d sport events, %d competitors, %d players\n",
		len(sportEventMappings.Mappings), len(competitorMappings.Mappings), len(playerMappings.Mappings))

	// Build mappings from DB entities to player props IDs
	fmt.Println("\nBuilding entity mappings...")
	mappings, unmappedIndividuals, err := available_legs_common.BuildMappings(
		games, rosters, sportEventMappings, competitorMappings, playerMappings,
	)
	if err != nil {
		fatal("Failed to build mappings: %v", err)
	}

	// Print unmapped individuals as info
	if len(unmappedIndividuals) > 0 {
		fmt.Printf("\n%d player(s) could not be mapped to player props IDs:\n", len(unmappedIndividuals))
		for _, individual := range unmappedIndividuals {
			fmt.Printf("  - %s (vendor_id: %s)\n", individual.DisplayName, individual.VendorID)
		}
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("MAPPING SUMMARY")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Print(mappings.String())

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("NFL available legs mapping completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}

// readRostersForGames reads rosters for all unique teams across the given games.
func readRostersForGames(ctx context.Context, dbStore *store.Store, games []*models.Game) ([]*models.Roster, error) {
	seen := make(map[int64]bool)
	var rosters []*models.Roster

	for _, game := range games {
		for _, teamID := range []int64{game.ContenderIDA, game.ContenderIDB} {
			if seen[teamID] {
				continue
			}
			seen[teamID] = true

			roster, err := dbStore.GetRosterByTeamID(ctx, teamID)
			if err != nil {
				return nil, fmt.Errorf("failed to read roster for team_id %d: %w", teamID, err)
			}
			rosters = append(rosters, roster)
		}
	}

	return rosters, nil
}
