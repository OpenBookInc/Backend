package store

import (
	"context"
	"fmt"

	"github.com/openbook/shared/models"
)

// UpsertConference inserts or updates a conference in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the conference
func (s *Store) UpsertConference(ctx context.Context, conference *models.Conference) (int, error) {
	query := `
		INSERT INTO conferences (name, league_id, vendor_id, alias)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (vendor_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			league_id = EXCLUDED.league_id,
			alias = EXCLUDED.alias
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		conference.Name,
		conference.LeagueID,
		conference.VendorID,
		conference.Alias,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert conference %s (vendor_id: %s): %w",
			conference.Name, conference.VendorID, err)
	}

	return id, nil
}

// GetConferenceByVendorID retrieves a conference by vendor_id
func (s *Store) GetConferenceByVendorID(ctx context.Context, vendorID string) (*models.Conference, error) {
	query := `SELECT id, name, league_id, vendor_id, alias FROM conferences WHERE vendor_id = $1`

	var conference models.Conference
	err := s.pool.QueryRow(ctx, query, vendorID).Scan(
		&conference.ID,
		&conference.Name,
		&conference.LeagueID,
		&conference.VendorID,
		&conference.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get conference with vendor_id %s: %w", vendorID, err)
	}

	return &conference, nil
}
