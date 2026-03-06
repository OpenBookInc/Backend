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
func PersistNFLPlayerProfile(ctx context.Context, dbStore *store.Store, profile *fetcher_nfl.PlayerProfile, leagueID int) (string, error) {
	result, err := dbStore.UpsertIndividual(ctx, &store.IndividualForUpsert{
		SportradarID:        profile.ID,
		DisplayName:     profile.GetDisplayName(),
		AbbreviatedName: profile.GetAbbreviatedName(),
		DateOfBirth:     profile.GetDateOfBirth(),
		LeagueID:        leagueID,
		Position:        profile.Position,
		JerseyNumber:    profile.Jersey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to persist NFL player profile %s (sportradar_id: %s): %w",
			profile.GetDisplayName(), profile.ID, err)
	}

	return result.ID, nil
}

// PersistNBAPlayerProfile persists an NBA player profile as an individual in the database.
// Returns the database ID of the individual.
func PersistNBAPlayerProfile(ctx context.Context, dbStore *store.Store, profile *fetcher_nba.PlayerProfile, leagueID int) (string, error) {
	result, err := dbStore.UpsertIndividual(ctx, &store.IndividualForUpsert{
		SportradarID:        profile.ID,
		DisplayName:     profile.GetDisplayName(),
		AbbreviatedName: profile.GetAbbreviatedName(),
		DateOfBirth:     profile.GetDateOfBirth(),
		LeagueID:        leagueID,
		Position:        profile.Position,
		JerseyNumber:    profile.JerseyNumber,
	})
	if err != nil {
		return "", fmt.Errorf("failed to persist NBA player profile %s (sportradar_id: %s): %w",
			profile.GetDisplayName(), profile.ID, err)
	}

	return result.ID, nil
}

// RosterPlayerData contains roster-level fields used to detect stale individual data.
type RosterPlayerData struct {
	JerseyNumber string
	Position     string
}

// isIndividualStale checks whether the database individual's data differs from the roster data.
func isIndividualStale(existing *models.Individual, rosterData *RosterPlayerData) bool {
	return existing.JerseyNumber != rosterData.JerseyNumber || existing.Position != rosterData.Position
}

// UpsertIndividualIfMissing checks if an individual exists by Sportradar ID.
// If missing, fetches their profile from the API and persists them.
// Returns the individual and whether a profile was fetched from the API.
func UpsertIndividualIfMissing(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, sportradarID string, leagueName string) (*models.Individual, bool, error) {
	existing, err := dbStore.GetIndividualBySportradarID(ctx, sportradarID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("failed to check if individual exists: %w", err)
	}

	return fetchAndPersistIndividual(ctx, dbStore, apiClient, sportradarID, leagueName)
}

// UpsertIndividualIfMissingOrInvalid checks if an individual exists by Sportradar ID.
// If missing, or if the existing record is stale compared to roster data, fetches
// their profile from the API and persists them.
// Returns the individual and whether a profile was fetched from the API.
func UpsertIndividualIfMissingOrInvalid(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, sportradarID string, leagueName string, rosterData RosterPlayerData) (*models.Individual, bool, error) {
	existing, err := dbStore.GetIndividualBySportradarID(ctx, sportradarID)
	if err == nil && !isIndividualStale(existing, &rosterData) {
		return existing, false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("failed to check if individual exists: %w", err)
	}

	return fetchAndPersistIndividual(ctx, dbStore, apiClient, sportradarID, leagueName)
}

// fetchAndPersistIndividual fetches a player profile from the API and persists it.
func fetchAndPersistIndividual(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, sportradarID string, leagueName string) (*models.Individual, bool, error) {
	league, err := dbStore.GetLeagueByName(ctx, leagueName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get %s league: %w", leagueName, err)
	}

	switch leagueName {
	case "NFL":
		profile, err := fetcher_nfl.FetchPlayerProfile(apiClient, sportradarID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch NFL player profile: %w", err)
		}
		_, err = PersistNFLPlayerProfile(ctx, dbStore, profile, league.ID)
		if err != nil {
			return nil, false, err
		}
	case "NBA":
		profile, err := fetcher_nba.FetchPlayerProfile(apiClient, sportradarID)
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

	individual, err := dbStore.GetIndividualBySportradarID(ctx, sportradarID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to retrieve individual after upsert: %w", err)
	}

	return individual, true, nil
}
