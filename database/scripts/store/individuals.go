package store

import (
	"context"
	"fmt"
	"time"

	models "github.com/openbook/shared/models"
	"github.com/openbook/shared/models/gen"
)

// IndividualForUpsert contains the data needed to upsert an individual
type IndividualForUpsert struct {
	SportradarID    string
	DisplayName     string
	AbbreviatedName string
	DateOfBirth     *time.Time
	LeagueID        int
	Position        string
	JerseyNumber    string
}

// UpsertIndividual inserts or updates an individual in the database.
// Uses sportradar_id as the unique identifier (ON CONFLICT).
// Resolves the League pointer, registers in the singleton registry, and returns the individual.
func (s *Store) UpsertIndividual(ctx context.Context, individual *IndividualForUpsert) (*models.Individual, error) {
	query := `
		INSERT INTO individuals (display_name, abbreviated_name, date_of_birth, sportradar_id, league_id, position, jersey_number)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (sportradar_id)
		DO UPDATE SET
			display_name = EXCLUDED.display_name,
			abbreviated_name = EXCLUDED.abbreviated_name,
			date_of_birth = EXCLUDED.date_of_birth,
			league_id = EXCLUDED.league_id,
			position = EXCLUDED.position,
			jersey_number = EXCLUDED.jersey_number
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		individual.DisplayName,
		individual.AbbreviatedName,
		individual.DateOfBirth, // Can be nil for NULL
		individual.SportradarID,
		individual.LeagueID,
		individual.Position,
		individual.JerseyNumber,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert individual %s (sportradar_id: %s): %w",
			individual.DisplayName, individual.SportradarID, err)
	}

	league, err := s.GetLeagueByID(ctx, individual.LeagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for individual %s: %w", individual.SportradarID, err)
	}

	return models.Registry.RegisterIndividual(&models.Individual{
		ID:              id,
		DisplayName:     individual.DisplayName,
		AbbreviatedName: individual.AbbreviatedName,
		DateOfBirth:     individual.DateOfBirth,
		SportradarID:    individual.SportradarID,
		LeagueID:        int64(individual.LeagueID),
		Position:        individual.Position,
		JerseyNumber:    individual.JerseyNumber,
		League:          league,
	})
}

// GetIndividualByID retrieves an individual by database ID.
// Uses the registry for caching and resolves the nested League pointer.
func (s *Store) GetIndividualByID(ctx context.Context, id int) (*models.Individual, error) {
	// Check registry first
	if individual := models.Registry.GetIndividual(id); individual != nil {
		return individual, nil
	}

	// Query database
	query := `
		SELECT id, display_name, abbreviated_name, date_of_birth, sportradar_id, league_id, position, jersey_number
		FROM individuals
		WHERE id = $1
	`

	var individual models.Individual
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&individual.ID,
		&individual.DisplayName,
		&individual.AbbreviatedName,
		&individual.DateOfBirth,
		&individual.SportradarID,
		&individual.LeagueID,
		&individual.Position,
		&individual.JerseyNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual with id %d: %w", id, err)
	}

	// Resolve nested League pointer
	league, err := s.GetLeagueByID(ctx, int(individual.LeagueID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for individual %d: %w", id, err)
	}
	individual.League = league

	// Register and return
	return models.Registry.RegisterIndividual(&individual)
}

// GetIndividualsByLeague retrieves all individuals for a given league name in a single query.
// JOINs through leagues and fetches all columns inline, registering in the registry.
func (s *Store) GetIndividualsByLeague(ctx context.Context, leagueName string) ([]*models.Individual, error) {
	query := `
		SELECT
			i.id, i.display_name, i.abbreviated_name, i.date_of_birth, i.sportradar_id,
			i.league_id, i.position, i.jersey_number,
			l.id, l.sport_id, l.name
		FROM individuals i
		JOIN leagues l ON i.league_id = l.id
		WHERE l.name = $1
		ORDER BY i.id ASC
	`

	rows, err := s.pool.Query(ctx, query, leagueName)
	if err != nil {
		return nil, fmt.Errorf("failed to query individuals for league %s: %w", leagueName, err)
	}
	defer rows.Close()

	var individuals []*models.Individual
	for rows.Next() {
		var ind models.Individual
		var league models.League

		if err := rows.Scan(
			&ind.ID, &ind.DisplayName, &ind.AbbreviatedName, &ind.DateOfBirth, &ind.SportradarID,
			&ind.LeagueID, &ind.Position, &ind.JerseyNumber,
			&league.ID, &league.SportID, &league.Name,
		); err != nil {
			return nil, fmt.Errorf("failed to scan individual row: %w", err)
		}

		ind.League = models.Registry.RegisterLeague(&league)
		regInd, err := models.Registry.RegisterIndividual(&ind)
		if err != nil {
			return nil, fmt.Errorf("failed to register individual %s: %w", ind.DisplayName, err)
		}
		individuals = append(individuals, regInd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating individual rows: %w", err)
	}

	return individuals, nil
}

// GetIndividualBySportradarID retrieves an individual by sportradar_id.
// Uses the registry for caching and resolves the nested League pointer.
func (s *Store) GetIndividualBySportradarID(ctx context.Context, sportradarID string) (*models.Individual, error) {
	// Query database to get the ID first
	query := `
		SELECT id, display_name, abbreviated_name, date_of_birth, sportradar_id, league_id, position, jersey_number
		FROM individuals
		WHERE sportradar_id = $1
	`

	var individual models.Individual
	err := s.pool.QueryRow(ctx, query, sportradarID).Scan(
		&individual.ID,
		&individual.DisplayName,
		&individual.AbbreviatedName,
		&individual.DateOfBirth,
		&individual.SportradarID,
		&individual.LeagueID,
		&individual.Position,
		&individual.JerseyNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual with sportradar_id %s: %w", sportradarID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetIndividual(individual.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested League pointer
	league, err := s.GetLeagueByID(ctx, int(individual.LeagueID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for individual %s: %w", sportradarID, err)
	}
	individual.League = league

	// Register and return
	return models.Registry.RegisterIndividual(&individual)
}

// GetIndividualByVendorID retrieves an individual by vendor ID.
// First checks the registry, then falls back to querying entity_vendor_ids.
// Registers the vendor mapping in the registry if found via DB query.
func (s *Store) GetIndividualByVendorID(ctx context.Context, vendor gen.Vendor, vendorID string) (*models.Individual, error) {
	// Check registry first
	if entityID, ok := models.Registry.GetEntityIDByVendorID(gen.EntityIndividual, vendor, vendorID); ok {
		return s.GetIndividualByID(ctx, entityID)
	}

	// Query entity_vendor_ids table
	query := `
		SELECT entity_id
		FROM entity_vendor_ids
		WHERE entity_type = $1 AND vendor = $2 AND vendor_id = $3
	`

	var entityID int
	err := s.pool.QueryRow(ctx, query, gen.EntityIndividual, vendor, vendorID).Scan(&entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to find individual with vendor_id %s (vendor=%s): %w", vendorID, vendor, err)
	}

	// Register the vendor mapping in the registry
	models.Registry.RegisterVendorID(gen.EntityIndividual, entityID, vendor, vendorID)

	return s.GetIndividualByID(ctx, entityID)
}
