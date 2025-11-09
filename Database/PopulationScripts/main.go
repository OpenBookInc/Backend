package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/config"
	"github.com/openbook/population-scripts/fetcher"
	"github.com/openbook/population-scripts/models"
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

	// Create API client with configured rate limit
	apiClient := client.NewClientWithDelay(cfg.SportradarAPIKey, cfg.RateLimitDelayMilliseconds)

	// Create in-memory data store
	dataStore := models.NewDataStore()

	fmt.Println("Starting data population...")
	fmt.Println(strings.Repeat("=", 72))

	// Fetch NBA data
	fmt.Println("\nFetching NBA data...")
	if err := fetcher.FetchNBAData(apiClient, dataStore); err != nil {
		fatal("Failed to fetch NBA data: %v", err)
	}
	nbaTeams := dataStore.GetTeamsBySport(models.SportNBA)
	nbaPlayers := dataStore.GetIndividualsBySport(models.SportNBA)
	fmt.Printf("Successfully fetched %d NBA teams and %d players\n", len(nbaTeams), len(nbaPlayers))

	// Wait between NBA and NFL data fetching to respect rate limits
	fmt.Println("\nWaiting before fetching NFL data...")
	apiClient.RateLimitWait()

	// Fetch NFL data
	fmt.Println("\nFetching NFL data...")
	if err := fetcher.FetchNFLData(apiClient, dataStore); err != nil {
		fatal("Failed to fetch NFL data: %v", err)
	}
	nflTeams := dataStore.GetTeamsBySport(models.SportNFL)
	nflPlayers := dataStore.GetIndividualsBySport(models.SportNFL)
	fmt.Printf("Successfully fetched %d NFL teams and %d players\n", len(nflTeams), len(nflPlayers))

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Data Population Summary:")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Total Conferences: %d\n", len(dataStore.Conferences))
	fmt.Printf("Total Divisions: %d\n", len(dataStore.Divisions))
	fmt.Printf("Total Teams: %d\n", len(dataStore.Teams))
	fmt.Printf("Total Players: %d\n", len(dataStore.Individuals))
	fmt.Printf("Total Rosters: %d\n", len(dataStore.Rosters))

	// Print all NBA conferences
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NBA CONFERENCES")
	fmt.Println(strings.Repeat("=", 72))
	nbaConferences := dataStore.GetConferencesBySport(models.SportNBA)
	for _, conference := range nbaConferences {
		fmt.Print(conference)
	}

	// Print all NBA divisions
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NBA DIVISIONS")
	fmt.Println(strings.Repeat("=", 72))
	nbaDivisions := dataStore.GetDivisionsBySport(models.SportNBA)
	for _, division := range nbaDivisions {
		fmt.Print(division)
	}

	// Print all NBA teams
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NBA TEAMS")
	fmt.Println(strings.Repeat("=", 72))
	for _, team := range nbaTeams {
		fmt.Print(team)
	}

	// Print all NFL conferences
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NFL CONFERENCES")
	fmt.Println(strings.Repeat("=", 72))
	nflConferences := dataStore.GetConferencesBySport(models.SportNFL)
	for _, conference := range nflConferences {
		fmt.Print(conference)
	}

	// Print all NFL divisions
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NFL DIVISIONS")
	fmt.Println(strings.Repeat("=", 72))
	nflDivisions := dataStore.GetDivisionsBySport(models.SportNFL)
	for _, division := range nflDivisions {
		fmt.Print(division)
	}

	// Print all NFL teams
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NFL TEAMS")
	fmt.Println(strings.Repeat("=", 72))
	for _, team := range nflTeams {
		fmt.Print(team)
	}

	// Print all NBA players
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NBA PLAYERS")
	fmt.Println(strings.Repeat("=", 72))
	for _, player := range nbaPlayers {
		fmt.Print(player)
	}

	// Print all NFL players
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL NFL PLAYERS")
	fmt.Println(strings.Repeat("=", 72))
	for _, player := range nflPlayers {
		fmt.Print(player)
	}

	// Print all rosters
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("ALL ROSTERS")
	fmt.Println(strings.Repeat("=", 72))
	for _, roster := range dataStore.Rosters {
		team := dataStore.Teams[roster.TeamID]
		teamName := "Unknown"
		if team != nil {
			teamName = team.Market + " " + team.Name
		}
		fmt.Print(roster.StringWithTeamName(teamName))
	}

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("Data successfully loaded into in-memory data store!")
	fmt.Println(strings.Repeat("=", 72))
}
