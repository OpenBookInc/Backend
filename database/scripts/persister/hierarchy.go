package persister

import (
	"context"
	"fmt"

	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	"github.com/openbook/population-scripts/store"
)

// PersistNFLHierarchy persists conferences, divisions, and teams from the NFL hierarchy response.
// Returns the counts of conferences, divisions, and teams upserted.
func PersistNFLHierarchy(ctx context.Context, dbStore *store.Store, hierarchy *fetcher_nfl.NFLHierarchyResponse) (int, int, int, error) {
	league, err := dbStore.GetLeagueByName(ctx, "NFL")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get NFL league: %w", err)
	}

	conferenceCount := 0
	divisionCount := 0
	teamCount := 0

	for _, confData := range hierarchy.Conferences {
		conference, err := dbStore.UpsertConference(ctx, &store.ConferenceForUpsert{
			VendorID: confData.ID,
			Name:     confData.Name,
			LeagueID: league.ID,
			Alias:    confData.Alias,
		})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to upsert conference %s: %w", confData.Name, err)
		}
		conferenceCount++

		for _, divData := range confData.Divisions {
			division, err := dbStore.UpsertDivision(ctx, &store.DivisionForUpsert{
				VendorID:     divData.ID,
				Name:         divData.Name,
				ConferenceID: conference.ID,
				Alias:        divData.Alias,
			})
			if err != nil {
				return 0, 0, 0, fmt.Errorf("failed to upsert division %s: %w", divData.Name, err)
			}
			divisionCount++

			for _, teamData := range divData.Teams {
				err := dbStore.UpsertTeam(ctx, &store.TeamForUpsert{
					VendorID:   teamData.ID,
					Name:       teamData.Name,
					Market:     teamData.Market,
					Alias:      teamData.Alias,
					DivisionID: division.ID,
					VenueName:  teamData.Venue.Name,
					VenueCity:  teamData.Venue.City,
					VenueState: teamData.Venue.State,
				})
				if err != nil {
					return 0, 0, 0, fmt.Errorf("failed to upsert team %s %s: %w", teamData.Market, teamData.Name, err)
				}
				teamCount++
			}
		}
	}

	return conferenceCount, divisionCount, teamCount, nil
}

// PersistNBAHierarchy persists conferences, divisions, and teams from the NBA hierarchy response.
// Returns the counts of conferences, divisions, and teams upserted.
func PersistNBAHierarchy(ctx context.Context, dbStore *store.Store, hierarchy *fetcher_nba.NBAHierarchyResponse) (int, int, int, error) {
	league, err := dbStore.GetLeagueByName(ctx, "NBA")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get NBA league: %w", err)
	}

	conferenceCount := 0
	divisionCount := 0
	teamCount := 0

	for _, confData := range hierarchy.Conferences {
		conference, err := dbStore.UpsertConference(ctx, &store.ConferenceForUpsert{
			VendorID: confData.ID,
			Name:     confData.Name,
			LeagueID: league.ID,
			Alias:    confData.Alias,
		})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to upsert conference %s: %w", confData.Name, err)
		}
		conferenceCount++

		for _, divData := range confData.Divisions {
			division, err := dbStore.UpsertDivision(ctx, &store.DivisionForUpsert{
				VendorID:     divData.ID,
				Name:         divData.Name,
				ConferenceID: conference.ID,
				Alias:        divData.Alias,
			})
			if err != nil {
				return 0, 0, 0, fmt.Errorf("failed to upsert division %s: %w", divData.Name, err)
			}
			divisionCount++

			for _, teamData := range divData.Teams {
				err := dbStore.UpsertTeam(ctx, &store.TeamForUpsert{
					VendorID:   teamData.ID,
					Name:       teamData.Name,
					Market:     teamData.Market,
					Alias:      teamData.Alias,
					DivisionID: division.ID,
					VenueName:  teamData.Venue.Name,
					VenueCity:  teamData.Venue.City,
					VenueState: teamData.Venue.State,
				})
				if err != nil {
					return 0, 0, 0, fmt.Errorf("failed to upsert team %s %s: %w", teamData.Market, teamData.Name, err)
				}
				teamCount++
			}
		}
	}

	return conferenceCount, divisionCount, teamCount, nil
}
