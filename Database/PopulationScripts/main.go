package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/config"
	"github.com/openbook/population-scripts/fetcher"
	"github.com/openbook/population-scripts/models"
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
	cfg, err := config.LoadFromFile(*envFile)
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

	// Create API client with configured rate limit
	apiClient := client.NewClientWithDelay(cfg.SportradarAPIKey, cfg.RateLimitDelayMilliseconds)

	// Create in-memory data store
	dataStore := models.NewDataStore()

	fmt.Println("\nStarting data population...")
	fmt.Println(strings.Repeat("=", 72))

	// Fetch NFL data
	fmt.Println("\nFetching NFL data from Sportradar API...")
	if err := fetcher.FetchNFLData(apiClient, dataStore); err != nil {
		fatal("Failed to fetch NFL data: %v", err)
	}
	fmt.Printf("Successfully fetched NFL data\n")

	// Wait between NFL and NBA data fetching to respect rate limits
	fmt.Println("\nWaiting before fetching NBA data...")
	apiClient.RateLimitWait()

	// Fetch NBA data
	fmt.Println("\nFetching NBA data from Sportradar API...")
	if err := fetcher.FetchNBAData(apiClient, dataStore); err != nil {
		fatal("Failed to fetch NBA data: %v", err)
	}
	fmt.Printf("Successfully fetched NBA data\n")

	// Persist data to database
	fmt.Println("\nPersisting data to database...")
	fmt.Println(strings.Repeat("=", 72))

	if err := persistToDatabase(ctx, dbStore, dataStore); err != nil {
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

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Data successfully persisted to database!")
	fmt.Println(strings.Repeat("=", 72))

	// Print all persisted data
	printPersistedData(dataStore)
}

// printPersistedData prints all data that was persisted to the database
func printPersistedData(dataStore *models.DataStore) {
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
}

// persistToDatabase persists all in-memory data to the database
func persistToDatabase(ctx context.Context, dbStore *store.Store, dataStore *models.DataStore) error {
	// Step 1: Create/update leagues
	fmt.Println("Upserting leagues...")
	nflLeague := &models.League{SportID: 1, Name: "NFL"}
	nflLeagueID, err := dbStore.UpsertLeague(ctx, nflLeague)
	if err != nil {
		return fmt.Errorf("failed to upsert NFL league: %w", err)
	}
	nflLeague.ID = nflLeagueID
	dataStore.AddLeague(nflLeague)

	nbaLeague := &models.League{SportID: 2, Name: "NBA"}
	nbaLeagueID, err := dbStore.UpsertLeague(ctx, nbaLeague)
	if err != nil {
		return fmt.Errorf("failed to upsert NBA league: %w", err)
	}
	nbaLeague.ID = nbaLeagueID
	dataStore.AddLeague(nbaLeague)
	fmt.Printf("  Upserted %d leagues\n", 2)

	// Step 2: Upsert conferences and set LeagueID
	fmt.Println("Upserting conferences...")
	conferenceCount := 0
	for _, conference := range dataStore.Conferences {
		// Determine league based on conference structure
		// NFL has AFC/NFC, NBA has Eastern/Western
		if strings.Contains(conference.Name, "AFC") || strings.Contains(conference.Name, "NFC") {
			conference.LeagueID = int64(nflLeagueID)
			conference.League = nflLeague
		} else {
			conference.LeagueID = int64(nbaLeagueID)
			conference.League = nbaLeague
		}

		conferenceID, err := dbStore.UpsertConference(ctx, conference)
		if err != nil {
			return fmt.Errorf("failed to upsert conference %s: %w", conference.Name, err)
		}
		conference.ID = conferenceID
		conferenceCount++
	}
	fmt.Printf("  Upserted %d conferences\n", conferenceCount)

	// Step 3: Upsert divisions and set ConferenceID
	fmt.Println("Upserting divisions...")
	divisionCount := 0
	for _, division := range dataStore.Divisions {
		// ConferenceID is set from the pointer relationship
		if division.Conference != nil {
			division.ConferenceID = int64(division.Conference.ID)
		}

		divisionID, err := dbStore.UpsertDivision(ctx, division)
		if err != nil {
			return fmt.Errorf("failed to upsert division %s: %w", division.Name, err)
		}
		division.ID = divisionID
		divisionCount++
	}
	fmt.Printf("  Upserted %d divisions\n", divisionCount)

	// Step 4: Upsert teams and set DivisionID
	fmt.Println("Upserting teams...")
	teamCount := 0
	for _, team := range dataStore.Teams {
		// DivisionID is set from the pointer relationship
		if team.Division != nil {
			team.DivisionID = int64(team.Division.ID)
		}

		teamID, err := dbStore.UpsertTeam(ctx, team)
		if err != nil {
			return fmt.Errorf("failed to upsert team %s %s: %w", team.Market, team.Name, err)
		}
		team.ID = teamID
		teamCount++
	}
	fmt.Printf("  Upserted %d teams\n", teamCount)

	// Step 5: Count individuals (will be upserted in Step 6 with rosters)
	// Note: We'll set LeagueID when processing rosters since individuals are linked to teams via rosters
	individualCount := len(dataStore.Individuals)

	// Step 6: Upsert rosters and individuals
	fmt.Println("Upserting rosters and linking individuals...")
	rosterCount := 0
	for _, roster := range dataStore.Rosters {
		// Get team DB ID
		if roster.Team != nil {
			roster.TeamID = int64(roster.Team.ID)

			// Determine league for all players in this roster
			var leagueID int64
			if roster.Team.Division != nil && roster.Team.Division.Conference != nil && roster.Team.Division.Conference.League != nil {
				leagueID = int64(roster.Team.Division.Conference.League.ID)
			}

			// Upsert all individual players and collect their DB IDs
			roster.IndividualIDs = []int64{}
			for _, player := range roster.Players {
				player.LeagueID = leagueID
				player.League = roster.Team.Division.Conference.League

				playerID, err := dbStore.UpsertIndividual(ctx, player)
				if err != nil {
					return fmt.Errorf("failed to upsert individual %s: %w", player.DisplayName, err)
				}
				player.ID = playerID
				roster.IndividualIDs = append(roster.IndividualIDs, int64(playerID))
			}

			// Upsert roster with team ID and individual IDs
			rosterID, err := dbStore.UpsertRoster(ctx, roster)
			if err != nil {
				return fmt.Errorf("failed to upsert roster for team %s %s: %w", roster.Team.Market, roster.Team.Name, err)
			}
			roster.ID = rosterID
			rosterCount++
		}
	}
	fmt.Printf("  Upserted %d rosters\n", rosterCount)
	fmt.Printf("  Upserted %d individuals\n", individualCount)

	return nil
}
