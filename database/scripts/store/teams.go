package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// TeamForUpsert contains the data needed to upsert a team
type TeamForUpsert struct {
	VendorID   string
	Name       string
	Market     string
	Alias      string
	DivisionID int
	VenueName  string
	VenueCity  string
	VenueState string
}

// UpsertTeam inserts or updates a team in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the team
func (s *Store) UpsertTeam(ctx context.Context, team *TeamForUpsert) (int, error) {
	query := `
		INSERT INTO teams (name, market, alias, vendor_id, division_id, venue_name, venue_city, venue_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (vendor_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			market = EXCLUDED.market,
			alias = EXCLUDED.alias,
			division_id = EXCLUDED.division_id,
			venue_name = EXCLUDED.venue_name,
			venue_city = EXCLUDED.venue_city,
			venue_state = EXCLUDED.venue_state
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		team.Name,
		team.Market,
		team.Alias,
		team.VendorID,
		team.DivisionID,
		team.VenueName,
		team.VenueCity,
		team.VenueState,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert team %s %s (vendor_id: %s): %w",
			team.Market, team.Name, team.VendorID, err)
	}

	return id, nil
}

// GetTeamByVendorID retrieves a team by vendor_id
func (s *Store) GetTeamByVendorID(ctx context.Context, vendorID string) (*models.Team, error) {
	query := `
		SELECT id, name, market, alias, vendor_id, division_id, venue_name, venue_city, venue_state
		FROM teams
		WHERE vendor_id = $1
	`

	var team models.Team
	err := s.pool.QueryRow(ctx, query, vendorID).Scan(
		&team.ID,
		&team.Name,
		&team.Market,
		&team.Alias,
		&team.VendorID,
		&team.DivisionID,
		&team.VenueName,
		&team.VenueCity,
		&team.VenueState,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get team with vendor_id %s: %w", vendorID, err)
	}

	return &team, nil
}
