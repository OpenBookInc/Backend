package persister

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/fetcher"
	"github.com/openbook/population-scripts/store"
)

// PersistNFLPlayerStatuses persists injury statuses from the NFL injuries response.
// Only processes players that appear in rosterPlayerVendorIDs (current roster players).
// Returns the set of vendor IDs that had statuses persisted.
func PersistNFLPlayerStatuses(ctx context.Context, dbStore *store.Store, injuries *fetcher.NFLInjuriesResponse, rosterPlayerVendorIDs map[string]bool) (map[string]bool, error) {
	processedVendorIDs := make(map[string]bool)

	for _, team := range injuries.Teams {
		for _, playerData := range team.Players {
			if !rosterPlayerVendorIDs[playerData.ID] {
				continue
			}

			statusStr := "Active"
			if len(playerData.Injuries) > 0 && playerData.Injuries[0].Status != "" {
				statusStr = playerData.Injuries[0].Status
			}

			mappedStatus, err := MapIndividualStatusToDB(statusStr)
			if err != nil {
				return nil, fmt.Errorf("invalid status for player %s: %w", playerData.ID, err)
			}

			individual, err := dbStore.GetIndividualByVendorID(ctx, playerData.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get individual %s: %w", playerData.ID, err)
			}

			err = dbStore.UpsertIndividualStatus(ctx, &store.IndividualStatusForUpsert{
				IndividualID: individual.ID,
				Status:       mappedStatus,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to upsert individual status for %s: %w",
					individual.DisplayName, err)
			}

			processedVendorIDs[playerData.ID] = true
		}
	}

	return processedVendorIDs, nil
}

// PersistNBAPlayerStatuses persists injury statuses from the NBA injuries response.
// Only processes players that appear in rosterPlayerVendorIDs (current roster players).
// Returns the set of vendor IDs that had statuses persisted.
func PersistNBAPlayerStatuses(ctx context.Context, dbStore *store.Store, injuries *fetcher.NBAInjuriesResponse, rosterPlayerVendorIDs map[string]bool) (map[string]bool, error) {
	processedVendorIDs := make(map[string]bool)

	for _, team := range injuries.Teams {
		for _, playerData := range team.Players {
			if !rosterPlayerVendorIDs[playerData.ID] {
				continue
			}

			statusStr := "Active"
			if len(playerData.Injuries) > 0 && playerData.Injuries[0].Status != "" {
				statusStr = playerData.Injuries[0].Status
			}

			mappedStatus, err := MapIndividualStatusToDB(statusStr)
			if err != nil {
				return nil, fmt.Errorf("invalid status for player %s: %w", playerData.ID, err)
			}

			individual, err := dbStore.GetIndividualByVendorID(ctx, playerData.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get individual %s: %w", playerData.ID, err)
			}

			err = dbStore.UpsertIndividualStatus(ctx, &store.IndividualStatusForUpsert{
				IndividualID: individual.ID,
				Status:       mappedStatus,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to upsert individual status for %s: %w",
					individual.DisplayName, err)
			}

			processedVendorIDs[playerData.ID] = true
		}
	}

	return processedVendorIDs, nil
}

// PersistDefaultActiveStatuses upserts "active" status for all roster players
// that were not already processed by the injury status persisters.
// Returns the number of active statuses upserted.
func PersistDefaultActiveStatuses(ctx context.Context, dbStore *store.Store, rosterPlayerVendorIDs map[string]bool, processedVendorIDs map[string]bool) (int, error) {
	mappedActive, err := MapIndividualStatusToDB("Active")
	if err != nil {
		return 0, fmt.Errorf("failed to map Active status: %w", err)
	}

	count := 0
	for vendorID := range rosterPlayerVendorIDs {
		if processedVendorIDs[vendorID] {
			continue
		}

		individual, err := dbStore.GetIndividualByVendorID(ctx, vendorID)
		if err != nil {
			return 0, fmt.Errorf("failed to get individual %s: %w", vendorID, err)
		}

		err = dbStore.UpsertIndividualStatus(ctx, &store.IndividualStatusForUpsert{
			IndividualID: individual.ID,
			Status:       mappedActive,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to upsert active status for %s: %w",
				individual.DisplayName, err)
		}

		count++
	}

	return count, nil
}
