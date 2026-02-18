package persister

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	"github.com/openbook/population-scripts/store"
)

// PersistNFLRostersAndIndividuals persists all NFL individuals and rosters from roster responses.
// For each player, checks if they exist in the database; if missing, fetches their profile
// from the API and persists them. Returns the count of new individuals created and rosters upserted.
func PersistNFLRostersAndIndividuals(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, rosters map[string]*fetcher_nfl.NFLTeamRosterResponse) (int, int, error) {
	newIndividualCount := 0
	rosterCount := 0

	for teamVendorID, roster := range rosters {
		team, err := dbStore.GetTeamByVendorID(ctx, teamVendorID)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get team %s: %w", teamVendorID, err)
		}

		leagueName := team.Division.Conference.League.Name

		fmt.Printf("  Processing players for %s %s (%d players)...\n",
			team.Market, team.Name, len(roster.Players))

		var individualIDs []int64
		teamNewCount := 0
		for i, player := range roster.Players {
			individual, created, err := UpsertIndividualIfMissing(ctx, dbStore, apiClient, player.ID, leagueName)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to upsert individual %s: %w", player.ID, err)
			}

			individualIDs = append(individualIDs, int64(individual.ID))
			if created {
				newIndividualCount++
				teamNewCount++
			}

			if (i+1)%10 == 0 || i+1 == len(roster.Players) {
				fmt.Printf("    Processed %d/%d players (%d new)\n", i+1, len(roster.Players), teamNewCount)
			}
		}

		err = dbStore.UpsertRoster(ctx, &store.RosterForUpsert{
			TeamID:        team.ID,
			IndividualIDs: individualIDs,
		})
		if err != nil {
			return 0, 0, fmt.Errorf("failed to upsert roster for team %s %s: %w",
				team.Market, team.Name, err)
		}

		rosterCount++
	}

	return newIndividualCount, rosterCount, nil
}

// PersistNBARostersAndIndividuals persists all NBA individuals and rosters from roster responses.
// For each player, checks if they exist in the database; if missing, fetches their profile
// from the API and persists them. Returns the count of new individuals created and rosters upserted.
func PersistNBARostersAndIndividuals(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, rosters map[string]*fetcher_nba.NBATeamProfileResponse) (int, int, error) {
	newIndividualCount := 0
	rosterCount := 0

	for teamVendorID, roster := range rosters {
		team, err := dbStore.GetTeamByVendorID(ctx, teamVendorID)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get team %s: %w", teamVendorID, err)
		}

		leagueName := team.Division.Conference.League.Name

		fmt.Printf("  Processing players for %s %s (%d players)...\n",
			team.Market, team.Name, len(roster.Players))

		var individualIDs []int64
		teamNewCount := 0
		for i, player := range roster.Players {
			individual, created, err := UpsertIndividualIfMissing(ctx, dbStore, apiClient, player.ID, leagueName)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to upsert individual %s: %w", player.ID, err)
			}

			individualIDs = append(individualIDs, int64(individual.ID))
			if created {
				newIndividualCount++
				teamNewCount++
			}

			if (i+1)%10 == 0 || i+1 == len(roster.Players) {
				fmt.Printf("    Processed %d/%d players (%d new)\n", i+1, len(roster.Players), teamNewCount)
			}
		}

		err = dbStore.UpsertRoster(ctx, &store.RosterForUpsert{
			TeamID:        team.ID,
			IndividualIDs: individualIDs,
		})
		if err != nil {
			return 0, 0, fmt.Errorf("failed to upsert roster for team %s %s: %w",
				team.Market, team.Name, err)
		}

		rosterCount++
	}

	return newIndividualCount, rosterCount, nil
}
