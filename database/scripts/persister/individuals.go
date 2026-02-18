package persister

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/client/sportradar"
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	"github.com/openbook/population-scripts/store"
	models "github.com/openbook/shared/models"
)

// PersistNFLPlayerProfile persists an NFL player profile as an individual in the database.
// Returns the database ID of the individual.
func PersistNFLPlayerProfile(ctx context.Context, dbStore *store.Store, profile *fetcher_nfl.PlayerProfile, leagueID int) (int, error) {
	result, err := dbStore.UpsertIndividual(ctx, &store.IndividualForUpsert{
		VendorID:        profile.ID,
		DisplayName:     profile.GetDisplayName(),
		AbbreviatedName: profile.GetAbbreviatedName(),
		DateOfBirth:     profile.GetDateOfBirth(),
		LeagueID:        leagueID,
		Position:        profile.Position,
		JerseyNumber:    profile.Jersey,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to persist NFL player profile %s (vendor_id: %s): %w",
			profile.GetDisplayName(), profile.ID, err)
	}

	return result.ID, nil
}

// PersistNBAPlayerProfile persists an NBA player profile as an individual in the database.
// Returns the database ID of the individual.
func PersistNBAPlayerProfile(ctx context.Context, dbStore *store.Store, profile *fetcher_nba.PlayerProfile, leagueID int) (int, error) {
	result, err := dbStore.UpsertIndividual(ctx, &store.IndividualForUpsert{
		VendorID:        profile.ID,
		DisplayName:     profile.GetDisplayName(),
		AbbreviatedName: profile.GetAbbreviatedName(),
		DateOfBirth:     profile.GetDateOfBirth(),
		LeagueID:        leagueID,
		Position:        profile.Position,
		JerseyNumber:    profile.JerseyNumber,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to persist NBA player profile %s (vendor_id: %s): %w",
			profile.GetDisplayName(), profile.ID, err)
	}

	return result.ID, nil
}

// UpsertIndividualIfMissing checks if an individual exists by vendor ID.
// If missing, fetches their profile from the API and persists them.
// Returns the individual (existing or newly created) and whether it was created.
func UpsertIndividualIfMissing(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, vendorID string, leagueName string) (*models.Individual, bool, error) {
	// Check if individual already exists
	existing, err := dbStore.GetIndividualByVendorID(ctx, vendorID)
	if err == nil {
		// Individual exists, return it
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("failed to check if individual exists: %w", err)
	}

	// Individual not found - fetch and persist their profile

	// Get league ID for the new individual
	league, err := dbStore.GetLeagueByName(ctx, leagueName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get %s league: %w", leagueName, err)
	}

	switch leagueName {
	case "NFL":
		profile, err := fetcher_nfl.FetchPlayerProfile(apiClient, vendorID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch NFL player profile: %w", err)
		}
		_, err = PersistNFLPlayerProfile(ctx, dbStore, profile, league.ID)
		if err != nil {
			return nil, false, err
		}
	case "NBA":
		profile, err := fetcher_nba.FetchPlayerProfile(apiClient, vendorID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch NBA player profile: %w", err)
		}
		_, err = PersistNBAPlayerProfile(ctx, dbStore, profile, league.ID)
		if err != nil {
			return nil, false, err
		}
	default:
		return nil, false, fmt.Errorf("unexpected league %q", leagueName)
	}

	// Retrieve the newly created individual
	individual, err := dbStore.GetIndividualByVendorID(ctx, vendorID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to retrieve newly created individual: %w", err)
	}

	return individual, true, nil
}
