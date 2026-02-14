package persister

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/population-scripts/fetcher"
	"github.com/openbook/population-scripts/store"
)

// =============================================================================
// Reference Data Persister
// =============================================================================
// This package handles the persistence of reference data (leagues, conferences,
// divisions, teams, individuals, rosters, games, individual statuses) to the
// database.
//
// Design principles:
// - Takes fetcher structs as input (no dependency on shared/models)
// - Handles enum transformation (API strings → DB enum strings)
// - Uses store ForUpsert structs for write operations
// - Multiple independent transactions (not a single atomic transaction)
// - Updates fetcher structs with database IDs after persistence
// =============================================================================

// PersistReferenceData persists all reference data from the fetcher ReferenceData to the database.
// This function handles the full persistence flow:
// 1. Leagues
// 2. Conferences (requires league IDs)
// 3. Divisions (requires conference IDs)
// 4. Teams (requires division IDs)
// 5. Individuals (requires league IDs) - fetches player profiles from API during persistence
// 6. Rosters (requires team IDs and individual IDs)
// 7. Games (requires team IDs)
// 8. Individual statuses (requires individual IDs)
//
// Each entity type is upserted independently. Partial success is acceptable.
// Returns an error if any upsert fails.
func PersistReferenceData(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData, apiClient *sportradar.Client) error {
	// Step 1: Upsert leagues
	fmt.Println("Upserting leagues...")
	if err := persistLeagues(ctx, dbStore, dataStore); err != nil {
		return err
	}
	fmt.Printf("  Upserted %d leagues\n", len(dataStore.Leagues))

	// Step 2: Upsert conferences
	fmt.Println("Upserting conferences...")
	if err := persistConferences(ctx, dbStore, dataStore); err != nil {
		return err
	}
	fmt.Printf("  Upserted %d conferences\n", len(dataStore.Conferences))

	// Step 3: Upsert divisions
	fmt.Println("Upserting divisions...")
	if err := persistDivisions(ctx, dbStore, dataStore); err != nil {
		return err
	}
	fmt.Printf("  Upserted %d divisions\n", len(dataStore.Divisions))

	// Step 4: Upsert teams
	fmt.Println("Upserting teams...")
	if err := persistTeams(ctx, dbStore, dataStore); err != nil {
		return err
	}
	fmt.Printf("  Upserted %d teams\n", len(dataStore.Teams))

	// Step 5 & 6: Upsert individuals and rosters (fetches player profiles from API)
	fmt.Println("Upserting rosters and individuals...")
	individualCount, rosterCount, err := persistRostersAndIndividuals(ctx, dbStore, dataStore, apiClient)
	if err != nil {
		return err
	}
	fmt.Printf("  Created %d new individuals\n", individualCount)
	fmt.Printf("  Upserted %d rosters\n", rosterCount)

	// Step 7: Upsert games
	fmt.Println("Upserting games...")
	if err := persistGames(ctx, dbStore, dataStore); err != nil {
		return err
	}
	fmt.Printf("  Upserted %d games\n", len(dataStore.Games))

	// Step 8: Upsert individual statuses
	fmt.Println("Upserting individual statuses...")
	if err := persistIndividualStatuses(ctx, dbStore, dataStore); err != nil {
		return err
	}
	fmt.Printf("  Upserted %d individual statuses\n", len(dataStore.IndividualStatuses))

	return nil
}

// persistLeagues upserts all leagues and updates their IDs in the dataStore
func persistLeagues(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData) error {
	for _, league := range dataStore.Leagues {
		leagueID, err := dbStore.UpsertLeague(ctx, &store.LeagueForUpsert{
			SportID: league.SportID,
			Name:    league.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to upsert league %s: %w", league.Name, err)
		}
		league.ID = leagueID
	}
	return nil
}

// persistConferences upserts all conferences and updates their IDs in the dataStore
func persistConferences(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData) error {
	for _, conference := range dataStore.Conferences {
		// Determine league ID from the pointer relationship
		leagueID := 0
		if conference.League != nil {
			leagueID = conference.League.ID
		}

		conferenceID, err := dbStore.UpsertConference(ctx, &store.ConferenceForUpsert{
			VendorID: conference.VendorID,
			Name:     conference.Name,
			LeagueID: leagueID,
			Alias:    conference.Alias,
		})
		if err != nil {
			return fmt.Errorf("failed to upsert conference %s: %w", conference.Name, err)
		}
		conference.ID = conferenceID
	}
	return nil
}

// persistDivisions upserts all divisions and updates their IDs in the dataStore
func persistDivisions(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData) error {
	for _, division := range dataStore.Divisions {
		// Determine conference ID from the pointer relationship
		conferenceID := 0
		if division.Conference != nil {
			conferenceID = division.Conference.ID
		}

		divisionID, err := dbStore.UpsertDivision(ctx, &store.DivisionForUpsert{
			VendorID:     division.VendorID,
			Name:         division.Name,
			ConferenceID: conferenceID,
			Alias:        division.Alias,
		})
		if err != nil {
			return fmt.Errorf("failed to upsert division %s: %w", division.Name, err)
		}
		division.ID = divisionID
	}
	return nil
}

// persistTeams upserts all teams and updates their IDs in the dataStore
func persistTeams(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData) error {
	for _, team := range dataStore.Teams {
		// Determine division ID from the pointer relationship
		divisionID := 0
		if team.Division != nil {
			divisionID = team.Division.ID
		}

		teamID, err := dbStore.UpsertTeam(ctx, &store.TeamForUpsert{
			VendorID:   team.VendorID,
			Name:       team.Name,
			Market:     team.Market,
			Alias:      team.Alias,
			DivisionID: divisionID,
			VenueName:  team.VenueName,
			VenueCity:  team.VenueCity,
			VenueState: team.VenueState,
		})
		if err != nil {
			return fmt.Errorf("failed to upsert team %s %s: %w", team.Market, team.Name, err)
		}
		team.ID = teamID
	}
	return nil
}

// persistRostersAndIndividuals upserts all rosters and their individuals.
// Only fetches player profiles from the API for individuals not already in the database.
// Returns the count of new individuals created and rosters upserted.
func persistRostersAndIndividuals(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData, apiClient *sportradar.Client) (int, int, error) {
	newIndividualCount := 0
	rosterCount := 0

	for _, roster := range dataStore.Rosters {
		if roster.Team == nil {
			continue
		}

		// Determine league from the team's division->conference->league chain
		leagueName := ""
		if roster.Team.Division != nil &&
			roster.Team.Division.Conference != nil &&
			roster.Team.Division.Conference.League != nil {
			leagueName = roster.Team.Division.Conference.League.Name
		}

		fmt.Printf("  Processing players for %s %s (%d players)...\n",
			roster.Team.Market, roster.Team.Name, len(roster.Players))

		// Upsert all individuals in this roster and collect their IDs.
		// Only fetches player profiles from the API for individuals not already in the database.
		var individualIDs []int64
		teamNewCount := 0
		for i, player := range roster.Players {
			individual, created, err := UpsertIndividualIfMissing(ctx, dbStore, apiClient, player.VendorID, leagueName)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to upsert individual %s: %w", player.VendorID, err)
			}

			player.ID = individual.ID
			player.DisplayName = individual.DisplayName
			player.AbbreviatedName = individual.AbbreviatedName
			player.DateOfBirth = individual.DateOfBirth
			player.Position = individual.Position
			player.JerseyNumber = individual.JerseyNumber

			individualIDs = append(individualIDs, int64(individual.ID))
			if created {
				newIndividualCount++
				teamNewCount++
			}

			// Progress output every 10 players
			if (i+1)%10 == 0 || i+1 == len(roster.Players) {
				fmt.Printf("    Processed %d/%d players (%d new)\n", i+1, len(roster.Players), teamNewCount)
			}
		}

		// Upsert the roster
		rosterID, err := dbStore.UpsertRoster(ctx, &store.RosterForUpsert{
			TeamID:        roster.Team.ID,
			IndividualIDs: individualIDs,
		})
		if err != nil {
			return 0, 0, fmt.Errorf("failed to upsert roster for team %s %s: %w",
				roster.Team.Market, roster.Team.Name, err)
		}
		roster.ID = rosterID
		rosterCount++
	}

	return newIndividualCount, rosterCount, nil
}

// persistGames upserts all games and updates their IDs in the dataStore
func persistGames(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData) error {
	for _, game := range dataStore.Games {
		// Determine team IDs from the pointer relationships
		homeTeamID := 0
		awayTeamID := 0
		if game.HomeTeam != nil {
			homeTeamID = game.HomeTeam.ID
		}
		if game.AwayTeam != nil {
			awayTeamID = game.AwayTeam.ID
		}

		gameID, err := dbStore.UpsertGame(ctx, &store.GameForUpsert{
			VendorID:           game.VendorID,
			HomeTeamID:         homeTeamID,
			AwayTeamID:         awayTeamID,
			ScheduledStartTime: game.ScheduledStartTime,
		})
		if err != nil {
			return fmt.Errorf("failed to upsert game %s: %w", game.VendorID, err)
		}
		game.ID = gameID
	}
	return nil
}

// persistIndividualStatuses upserts all individual statuses
func persistIndividualStatuses(ctx context.Context, dbStore *store.Store, dataStore *fetcher.ReferenceData) error {
	for _, status := range dataStore.IndividualStatuses {
		if status.Individual == nil {
			continue
		}

		// Map API status string to DB enum value
		mappedStatus, err := MapIndividualStatusToDB(status.Status)
		if err != nil {
			return fmt.Errorf("invalid status for player %s (%s): %w",
				status.Individual.DisplayName, status.Individual.VendorID, err)
		}

		statusID, err := dbStore.UpsertIndividualStatus(ctx, &store.IndividualStatusForUpsert{
			IndividualID: status.Individual.ID,
			Status:       mappedStatus,
		})
		if err != nil {
			return fmt.Errorf("failed to upsert individual status for %s: %w",
				status.Individual.DisplayName, err)
		}
		status.ID = statusID
	}
	return nil
}

