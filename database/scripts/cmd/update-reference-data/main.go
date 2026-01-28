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
	"github.com/openbook/population-scripts/fetcher"
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
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
	cfg, err := config.LoadReferenceDataConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease set your Sportradar API key in .env file or as an environment variable", err)
	}

	// Create context for database operations
	ctx := context.Background()

	// Create database connection
	fmt.Println("Connecting to database...")
	dbStore, err := store.New(ctx, cfg.PGHost, cfg.PGPort, cfg.PGDatabase, cfg.PGUser, cfg.PGPassword, cfg.PGKeyPath)
	if err != nil {
		fatal("Failed to connect to database: %v", err)
	}
	defer dbStore.Close()
	fmt.Println("Successfully connected to database")

	// Create API client with configured rate limit and access level
	clientConfig := &sportradar.ClientConfig{
		AccessLevel:    cfg.SportradarAccessLevel,
		RateLimitDelay: time.Duration(cfg.RateLimitDelayMilliseconds) * time.Millisecond,
		Timeout:        30 * time.Second,
		ApiKeys:        cfg.SportradarAPIKeys,
	}
	apiClient := sportradar.NewClientWithConfig(clientConfig)

	// Create in-memory data store and add leagues
	dataStore := fetcher.NewReferenceData()
	dataStore.AddLeague(&fetcher.League{SportID: 1, Name: "NFL"})
	dataStore.AddLeague(&fetcher.League{SportID: 2, Name: "NBA"})

	fmt.Println("\nStarting data population...")
	fmt.Println(strings.Repeat("=", 72))

	// Fetch NFL data
	fmt.Println("\nFetching NFL data from Sportradar API...")
	if err := fetcher_nfl.FetchNFLHierarchyData(apiClient, dataStore); err != nil {
		fatal("Failed to fetch NFL data: %v", err)
	}
	fmt.Printf("Successfully fetched NFL data\n")

	// Fetch NBA data
	fmt.Println("\nFetching NBA data from Sportradar API...")
	if err := fetcher_nba.FetchNBAHierarchyData(apiClient, dataStore); err != nil {
		fatal("Failed to fetch NBA data: %v", err)
	}
	fmt.Printf("Successfully fetched NBA data\n")

	// Fetch NFL game schedules
	fmt.Println("\nFetching NFL game schedules...")
	if err := fetcher.FetchNFLGames(apiClient, dataStore, cfg.NFLSeasonStartYear, cfg.NFLSeasonType); err != nil {
		fatal("Failed to fetch NFL games: %v", err)
	}
	fmt.Printf("Successfully fetched NFL games\n")

	// Fetch NBA game schedules
	fmt.Println("\nFetching NBA game schedules...")
	if err := fetcher.FetchNBAGames(apiClient, dataStore, cfg.NBASeasonStartYear, cfg.NBASeasonType); err != nil {
		fatal("Failed to fetch NBA games: %v", err)
	}
	fmt.Printf("Successfully fetched NBA games\n")

	// Fetch NFL player statuses (injuries)
	fmt.Println("\nFetching NFL player statuses...")
	if err := fetcher.FetchNFLPlayerStatuses(apiClient, dataStore, cfg.NFLSeasonStartYear, cfg.NFLSeasonType, cfg.NFLWeek); err != nil {
		fatal("Failed to fetch NFL player statuses: %v", err)
	}
	fmt.Printf("Successfully fetched NFL player statuses\n")

	// Fetch NBA player statuses (injuries)
	fmt.Println("\nFetching NBA player statuses...")
	if err := fetcher.FetchNBAPlayerStatuses(apiClient, dataStore); err != nil {
		fatal("Failed to fetch NBA player statuses: %v", err)
	}
	fmt.Printf("Successfully fetched NBA player statuses\n")

	// Set default "Active" status for all players without an injury status
	fmt.Println("\nSetting default active statuses for remaining players...")
	fetcher.SetDefaultActiveStatuses(dataStore)
	fmt.Printf("Successfully set default statuses\n")

	// Persist data to database
	fmt.Println("\nPersisting data to database...")
	fmt.Println(strings.Repeat("=", 72))

	if err := persister.PersistReferenceData(ctx, dbStore, dataStore, apiClient); err != nil {
		fatal("Failed to persist data to database: %v", err)
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Data Population Summary:")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Total Leagues: %d\n", len(dataStore.Leagues))
	fmt.Printf("Total Conferences: %d\n", len(dataStore.Conferences))
	fmt.Printf("Total Divisions: %d\n", len(dataStore.Divisions))
	fmt.Printf("Total Teams: %d\n", len(dataStore.Teams))
	fmt.Printf("Total Players: %d\n", len(dataStore.Individuals))
	fmt.Printf("Total Rosters: %d\n", len(dataStore.Rosters))
	fmt.Printf("Total Games: %d\n", len(dataStore.Games))
	fmt.Printf("Total Player Statuses: %d\n", len(dataStore.IndividualStatuses))

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Data successfully persisted to database!")
	fmt.Println(strings.Repeat("=", 72))

	// Print all persisted data
	printPersistedData(dataStore)
}

// printPersistedData prints all data that was persisted to the database
func printPersistedData(dataStore *fetcher.ReferenceData) {
	// Print Leagues
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("LEAGUES")
	fmt.Println(strings.Repeat("=", 72))
	for _, league := range dataStore.Leagues {
		fmt.Print(league)
	}

	// Print Conferences
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("CONFERENCES")
	fmt.Println(strings.Repeat("=", 72))
	for _, conference := range dataStore.Conferences {
		fmt.Print(conference)
	}

	// Print Divisions
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("DIVISIONS")
	fmt.Println(strings.Repeat("=", 72))
	for _, division := range dataStore.Divisions {
		fmt.Print(division)
	}

	// Print Teams
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("TEAMS")
	fmt.Println(strings.Repeat("=", 72))
	for _, team := range dataStore.Teams {
		fmt.Print(team)
	}

	// Print Individuals (sample - too many to print all)
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Printf("INDIVIDUALS (showing first 10 of %d)\n", len(dataStore.Individuals))
	fmt.Println(strings.Repeat("=", 72))
	count := 0
	for _, individual := range dataStore.Individuals {
		if count >= 10 {
			fmt.Printf("\n... and %d more individuals\n", len(dataStore.Individuals)-10)
			break
		}
		fmt.Print(individual)
		count++
	}

	// Print Rosters
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ROSTERS")
	fmt.Println(strings.Repeat("=", 72))
	for _, roster := range dataStore.Rosters {
		fmt.Print(roster)
	}

	// Print Games (showing all)
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Printf("GAMES (showing all %d)\n", len(dataStore.Games))
	fmt.Println(strings.Repeat("=", 72))
	for _, game := range dataStore.Games {
		fmt.Print(game)
	}

	// Print Individual Statuses (showing all)
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Printf("INDIVIDUAL STATUSES (showing all %d)\n", len(dataStore.IndividualStatuses))
	fmt.Println(strings.Repeat("=", 72))
	for _, status := range dataStore.IndividualStatuses {
		fmt.Print(status)
	}
}
