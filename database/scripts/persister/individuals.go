package persister

import (
	"context"
	"fmt"

	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	"github.com/openbook/population-scripts/store"
)

// PersistNFLPlayerProfile persists an NFL player profile as an individual in the database.
// Returns the database ID of the individual.
func PersistNFLPlayerProfile(ctx context.Context, dbStore *store.Store, profile *fetcher_nfl.PlayerProfile, leagueID int) (int, error) {
	individual := &store.IndividualForUpsert{
		VendorID:        profile.ID,
		DisplayName:     profile.GetDisplayName(),
		AbbreviatedName: profile.GetAbbreviatedName(),
		DateOfBirth:     profile.GetDateOfBirth(),
		LeagueID:        leagueID,
		Position:        profile.Position,
		JerseyNumber:    profile.Jersey,
	}

	id, err := dbStore.UpsertIndividual(ctx, individual)
	if err != nil {
		return 0, fmt.Errorf("failed to persist NFL player profile %s (vendor_id: %s): %w",
			profile.GetDisplayName(), profile.ID, err)
	}

	return id, nil
}

// PersistNBAPlayerProfile persists an NBA player profile as an individual in the database.
// Returns the database ID of the individual.
func PersistNBAPlayerProfile(ctx context.Context, dbStore *store.Store, profile *fetcher_nba.PlayerProfile, leagueID int) (int, error) {
	individual := &store.IndividualForUpsert{
		VendorID:        profile.ID,
		DisplayName:     profile.GetDisplayName(),
		AbbreviatedName: profile.GetAbbreviatedName(),
		DateOfBirth:     profile.GetDateOfBirth(),
		LeagueID:        leagueID,
		Position:        profile.Position,
		JerseyNumber:    profile.JerseyNumber,
	}

	id, err := dbStore.UpsertIndividual(ctx, individual)
	if err != nil {
		return 0, fmt.Errorf("failed to persist NBA player profile %s (vendor_id: %s): %w",
			profile.GetDisplayName(), profile.ID, err)
	}

	return id, nil
}
