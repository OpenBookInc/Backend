package persister

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/store"
)

// PersistLeagues upserts the NFL and NBA leagues to the database.
// Returns the number of leagues upserted.
func PersistLeagues(ctx context.Context, dbStore *store.Store) (int, error) {
	leagues := []struct {
		SportID int
		Name    string
	}{
		{SportID: 1, Name: "NFL"},
		{SportID: 2, Name: "NBA"},
	}

	for _, league := range leagues {
		err := dbStore.UpsertLeague(ctx, &store.LeagueForUpsert{
			SportID: league.SportID,
			Name:    league.Name,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to upsert league %s: %w", league.Name, err)
		}
	}

	return len(leagues), nil
}
