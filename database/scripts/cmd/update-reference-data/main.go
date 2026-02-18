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
	models "github.com/openbook/shared/models"
)

// fatal prints an error message to stderr and exits with code 1
func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	envFile := flag.String("env", "", "path to environment file (default: .env)")
	flag.Parse()

	cfg, err := config.LoadReferenceDataConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease set your Sportradar API key in .env file or as an environment variable", err)
	}

	ctx := context.Background()

	fmt.Println("Connecting to database...")
	dbStore, err := store.New(ctx, cfg.PGHost, cfg.PGPort, cfg.PGDatabase, cfg.PGUser, cfg.PGPassword, cfg.PGKeyPath)
	if err != nil {
		fatal("Failed to connect to database: %v", err)
	}
	defer dbStore.Close()
	fmt.Println("Successfully connected to database")

	clientConfig := &sportradar.ClientConfig{
		AccessLevel:    cfg.SportradarAccessLevel,
		RateLimitDelay: time.Duration(cfg.RateLimitDelayMilliseconds) * time.Millisecond,
		Timeout:        30 * time.Second,
		ApiKeys:        cfg.SportradarAPIKeys,
	}
	apiClient := sportradar.NewClientWithConfig(clientConfig)

	fmt.Println("\nStarting data population...")
	fmt.Println(strings.Repeat("=", 72))

	// Step 1: Persist leagues
	fmt.Println("\nUpserting leagues...")
	leagueCount, err := persister.PersistLeagues(ctx, dbStore)
	if err != nil {
		fatal("Failed to persist leagues: %v", err)
	}
	fmt.Printf("  Upserted %d leagues\n", leagueCount)

	// Step 2: Fetch hierarchies
	fmt.Println("\nFetching NFL hierarchy from Sportradar API...")
	nflHierarchy, err := fetcher_nfl.FetchNFLHierarchy(apiClient)
	if err != nil {
		fatal("Failed to fetch NFL hierarchy: %v", err)
	}
	fmt.Println("Successfully fetched NFL hierarchy")

	fmt.Println("\nFetching NBA hierarchy from Sportradar API...")
	nbaHierarchy, err := fetcher_nba.FetchNBAHierarchy(apiClient)
	if err != nil {
		fatal("Failed to fetch NBA hierarchy: %v", err)
	}
	fmt.Println("Successfully fetched NBA hierarchy")

	// Step 3: Persist hierarchies (conferences, divisions, teams)
	fmt.Println("\nUpserting NFL hierarchy...")
	nflConfCount, nflDivCount, nflTeamCount, err := persister.PersistNFLHierarchy(ctx, dbStore, nflHierarchy)
	if err != nil {
		fatal("Failed to persist NFL hierarchy: %v", err)
	}
	fmt.Printf("  Upserted %d conferences, %d divisions, %d teams\n", nflConfCount, nflDivCount, nflTeamCount)

	fmt.Println("\nUpserting NBA hierarchy...")
	nbaConfCount, nbaDivCount, nbaTeamCount, err := persister.PersistNBAHierarchy(ctx, dbStore, nbaHierarchy)
	if err != nil {
		fatal("Failed to persist NBA hierarchy: %v", err)
	}
	fmt.Printf("  Upserted %d conferences, %d divisions, %d teams\n", nbaConfCount, nbaDivCount, nbaTeamCount)

	// Step 4: Fetch rosters
	fmt.Println("\nFetching NFL rosters from Sportradar API...")
	nflRosters, err := fetchNFLRosters(apiClient, nflHierarchy)
	if err != nil {
		fatal("Failed to fetch NFL rosters: %v", err)
	}
	fmt.Printf("Successfully fetched %d NFL rosters\n", len(nflRosters))

	fmt.Println("\nFetching NBA rosters from Sportradar API...")
	nbaRosters, err := fetchNBARosters(apiClient, nbaHierarchy)
	if err != nil {
		fatal("Failed to fetch NBA rosters: %v", err)
	}
	fmt.Printf("Successfully fetched %d NBA rosters\n", len(nbaRosters))

	// Step 5: Persist rosters and individuals
	fmt.Println("\nUpserting NFL rosters and individuals...")
	nflNewIndividuals, nflRosterCount, err := persister.PersistNFLRostersAndIndividuals(ctx, dbStore, apiClient, nflRosters)
	if err != nil {
		fatal("Failed to persist NFL rosters and individuals: %v", err)
	}
	fmt.Printf("  Created %d new individuals, upserted %d rosters\n", nflNewIndividuals, nflRosterCount)

	fmt.Println("\nUpserting NBA rosters and individuals...")
	nbaNewIndividuals, nbaRosterCount, err := persister.PersistNBARostersAndIndividuals(ctx, dbStore, apiClient, nbaRosters)
	if err != nil {
		fatal("Failed to persist NBA rosters and individuals: %v", err)
	}
	fmt.Printf("  Created %d new individuals, upserted %d rosters\n", nbaNewIndividuals, nbaRosterCount)

	// Step 6: Fetch and persist games
	fmt.Println("\nFetching NFL game schedules...")
	nflSchedule, err := fetcher.FetchNFLGames(apiClient, cfg.NFLSeasonStartYear, cfg.NFLSeasonType)
	if err != nil {
		fatal("Failed to fetch NFL games: %v", err)
	}
	fmt.Println("Successfully fetched NFL games")

	fmt.Println("\nUpserting NFL games...")
	nflGameCount, err := persister.PersistNFLGames(ctx, dbStore, nflSchedule)
	if err != nil {
		fatal("Failed to persist NFL games: %v", err)
	}
	fmt.Printf("  Upserted %d games\n", nflGameCount)

	fmt.Println("\nFetching NBA game schedules...")
	nbaSchedule, err := fetcher.FetchNBAGames(apiClient, cfg.NBASeasonStartYear, cfg.NBASeasonType)
	if err != nil {
		fatal("Failed to fetch NBA games: %v", err)
	}
	fmt.Println("Successfully fetched NBA games")

	fmt.Println("\nUpserting NBA games...")
	nbaGameCount, err := persister.PersistNBAGames(ctx, dbStore, nbaSchedule)
	if err != nil {
		fatal("Failed to persist NBA games: %v", err)
	}
	fmt.Printf("  Upserted %d games\n", nbaGameCount)

	// Step 7: Fetch and persist player statuses
	rosterPlayerSportradarIDs := collectRosterPlayerSportradarIDs(nflRosters, nbaRosters)

	fmt.Println("\nFetching NFL player statuses...")
	nflInjuries, err := fetcher.FetchNFLPlayerStatuses(apiClient, cfg.NFLSeasonStartYear, cfg.NFLSeasonType, cfg.NFLWeek)
	if err != nil {
		fatal("Failed to fetch NFL player statuses: %v", err)
	}
	fmt.Println("Successfully fetched NFL player statuses")

	fmt.Println("\nUpserting NFL player statuses...")
	nflStatusSportradarIDs, err := persister.PersistNFLPlayerStatuses(ctx, dbStore, nflInjuries, rosterPlayerSportradarIDs)
	if err != nil {
		fatal("Failed to persist NFL player statuses: %v", err)
	}
	fmt.Printf("  Upserted %d injury statuses\n", len(nflStatusSportradarIDs))

	fmt.Println("\nFetching NBA player statuses...")
	nbaInjuries, err := fetcher.FetchNBAPlayerStatuses(apiClient)
	if err != nil {
		fatal("Failed to fetch NBA player statuses: %v", err)
	}
	fmt.Println("Successfully fetched NBA player statuses")

	fmt.Println("\nUpserting NBA player statuses...")
	nbaStatusSportradarIDs, err := persister.PersistNBAPlayerStatuses(ctx, dbStore, nbaInjuries, rosterPlayerSportradarIDs)
	if err != nil {
		fatal("Failed to persist NBA player statuses: %v", err)
	}
	fmt.Printf("  Upserted %d injury statuses\n", len(nbaStatusSportradarIDs))

	// Step 8: Set default active statuses for remaining players
	processedSportradarIDs := mergeSportradarIDSets(nflStatusSportradarIDs, nbaStatusSportradarIDs)

	fmt.Println("\nSetting default active statuses for remaining players...")
	activeCount, err := persister.PersistDefaultActiveStatuses(ctx, dbStore, rosterPlayerSportradarIDs, processedSportradarIDs)
	if err != nil {
		fatal("Failed to persist default active statuses: %v", err)
	}
	fmt.Printf("  Upserted %d active statuses\n", activeCount)

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Data Population Summary:")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Total Leagues: %d\n", leagueCount)
	fmt.Printf("Total Conferences: %d\n", nflConfCount+nbaConfCount)
	fmt.Printf("Total Divisions: %d\n", nflDivCount+nbaDivCount)
	fmt.Printf("Total Teams: %d\n", nflTeamCount+nbaTeamCount)
	fmt.Printf("Total New Players: %d\n", nflNewIndividuals+nbaNewIndividuals)
	fmt.Printf("Total Rosters: %d\n", nflRosterCount+nbaRosterCount)
	fmt.Printf("Total Games: %d\n", nflGameCount+nbaGameCount)
	fmt.Printf("Total Player Statuses: %d\n", len(processedSportradarIDs)+activeCount)

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Data successfully persisted to database!")
	fmt.Println(strings.Repeat("=", 72))

	// Print all persisted data from the singleton registry
	fmt.Print(models.Registry.String())
}

// fetchNFLRosters fetches roster data for every team in the NFL hierarchy response.
func fetchNFLRosters(apiClient *sportradar.Client, hierarchy *fetcher_nfl.NFLHierarchyResponse) (map[string]*fetcher_nfl.NFLTeamRosterResponse, error) {
	rosters := make(map[string]*fetcher_nfl.NFLTeamRosterResponse)
	for _, conf := range hierarchy.Conferences {
		for _, div := range conf.Divisions {
			for _, team := range div.Teams {
				roster, err := fetcher_nfl.FetchNFLTeamRoster(apiClient, team.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to fetch roster for team %s: %w", team.ID, err)
				}
				rosters[team.ID] = roster
			}
		}
	}
	return rosters, nil
}

// fetchNBARosters fetches roster data for every team in the NBA hierarchy response.
func fetchNBARosters(apiClient *sportradar.Client, hierarchy *fetcher_nba.NBAHierarchyResponse) (map[string]*fetcher_nba.NBATeamProfileResponse, error) {
	rosters := make(map[string]*fetcher_nba.NBATeamProfileResponse)
	for _, conf := range hierarchy.Conferences {
		for _, div := range conf.Divisions {
			for _, team := range div.Teams {
				roster, err := fetcher_nba.FetchNBATeamRoster(apiClient, team.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to fetch roster for team %s: %w", team.ID, err)
				}
				rosters[team.ID] = roster
			}
		}
	}
	return rosters, nil
}

// collectRosterPlayerSportradarIDs builds a set of all player sportradar IDs from roster responses.
func collectRosterPlayerSportradarIDs(nflRosters map[string]*fetcher_nfl.NFLTeamRosterResponse, nbaRosters map[string]*fetcher_nba.NBATeamProfileResponse) map[string]bool {
	sportradarIDs := make(map[string]bool)
	for _, roster := range nflRosters {
		for _, player := range roster.Players {
			sportradarIDs[player.ID] = true
		}
	}
	for _, roster := range nbaRosters {
		for _, player := range roster.Players {
			sportradarIDs[player.ID] = true
		}
	}
	return sportradarIDs
}

// mergeSportradarIDSets combines two sportradar ID sets into one.
func mergeSportradarIDSets(a, b map[string]bool) map[string]bool {
	merged := make(map[string]bool, len(a)+len(b))
	for k := range a {
		merged[k] = true
	}
	for k := range b {
		merged[k] = true
	}
	return merged
}
