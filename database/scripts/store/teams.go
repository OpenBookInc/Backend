package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// TeamForUpsert contains the data needed to upsert a team
type TeamForUpsert struct {
	VendorID        string
	VendorUnifiedID string // Sportradar unified ID (e.g., "sr:competitor:4418")
	Name            string
	Market          string
	Alias           string
	DivisionID      int
	VenueName       string
	VenueCity       string
	VenueState      string
}

// UpsertTeam inserts or updates a team in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the team
func (s *Store) UpsertTeam(ctx context.Context, team *TeamForUpsert) (int, error) {
	query := `
		INSERT INTO teams (name, market, alias, vendor_id, vendor_unified_id, division_id, venue_name, venue_city, venue_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (vendor_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			market = EXCLUDED.market,
			alias = EXCLUDED.alias,
			vendor_unified_id = EXCLUDED.vendor_unified_id,
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
		team.VendorUnifiedID,
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

// GetTeamByID retrieves a team by database ID.
// Uses the registry for caching and resolves the nested Division pointer.
func (s *Store) GetTeamByID(ctx context.Context, id int) (*models.Team, error) {
	// Check registry first
	if team := models.Registry.GetTeam(id); team != nil {
		return team, nil
	}

	// Query database
	query := `
		SELECT id, name, market, alias, vendor_id, vendor_unified_id, division_id, venue_name, venue_city, venue_state
		FROM teams
		WHERE id = $1
	`

	var team models.Team
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.Market,
		&team.Alias,
		&team.VendorID,
		&team.VendorUnifiedID,
		&team.DivisionID,
		&team.VenueName,
		&team.VenueCity,
		&team.VenueState,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get team with id %d: %w", id, err)
	}

	// Resolve nested Division pointer
	division, err := s.GetDivisionByID(ctx, int(team.DivisionID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve division for team %d: %w", id, err)
	}
	team.Division = division

	// Register and return
	return models.Registry.RegisterTeam(&team), nil
}

// GetTeamByVendorID retrieves a team by vendor_id.
// Uses the registry for caching and resolves the nested Division pointer.
func (s *Store) GetTeamByVendorID(ctx context.Context, vendorID string) (*models.Team, error) {
	// Query database to get the ID first
	query := `
		SELECT id, name, market, alias, vendor_id, vendor_unified_id, division_id, venue_name, venue_city, venue_state
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
		&team.VendorUnifiedID,
		&team.DivisionID,
		&team.VenueName,
		&team.VenueCity,
		&team.VenueState,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get team with vendor_id %s: %w", vendorID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetTeam(team.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Division pointer
	division, err := s.GetDivisionByID(ctx, int(team.DivisionID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve division for team %s: %w", vendorID, err)
	}
	team.Division = division

	// Register and return
	return models.Registry.RegisterTeam(&team), nil
}
