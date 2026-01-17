package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// IndividualStatusForUpsert contains the data needed to upsert an individual status
type IndividualStatusForUpsert struct {
	IndividualID int
	Status       string // DB enum value as string (e.g., "active", "questionable")
}

// UpsertIndividualStatus inserts or updates an individual's status in the database
// Uses individual_id as the unique identifier (ON CONFLICT) - one status per player
// Returns the database ID of the individual status
func (s *Store) UpsertIndividualStatus(ctx context.Context, status *IndividualStatusForUpsert) (int, error) {
	query := `
		INSERT INTO individual_statuses (individual_id, status)
		VALUES ($1, $2::individual_status_type)
		ON CONFLICT (individual_id)
		DO UPDATE SET
			status = EXCLUDED.status
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		status.IndividualID,
		status.Status,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert individual status (individual_id: %d): %w", status.IndividualID, err)
	}

	return id, nil
}

// GetIndividualStatusByID retrieves an individual status by database ID.
// Uses the registry for caching and resolves the nested Individual pointer.
func (s *Store) GetIndividualStatusByID(ctx context.Context, id int) (*models.IndividualStatus, error) {
	// Check registry first
	if status := models.Registry.GetIndividualStatus(id); status != nil {
		return status, nil
	}

	// Query database
	query := `
		SELECT id, individual_id, status
		FROM individual_statuses
		WHERE id = $1
	`

	var status models.IndividualStatus
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&status.ID,
		&status.IndividualID,
		&status.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual status with id %d: %w", id, err)
	}

	// Resolve nested Individual pointer
	individual, err := s.GetIndividualByID(ctx, int(status.IndividualID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve individual for status %d: %w", id, err)
	}
	status.Individual = individual

	// Register and return
	return models.Registry.RegisterIndividualStatus(&status), nil
}

// GetIndividualStatusByIndividualID retrieves an individual's status by individual_id.
// Uses the registry for caching and resolves the nested Individual pointer.
func (s *Store) GetIndividualStatusByIndividualID(ctx context.Context, individualID int64) (*models.IndividualStatus, error) {
	// Query database to get the ID first
	query := `
		SELECT id, individual_id, status
		FROM individual_statuses
		WHERE individual_id = $1
	`

	var status models.IndividualStatus
	err := s.pool.QueryRow(ctx, query, individualID).Scan(
		&status.ID,
		&status.IndividualID,
		&status.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual status with individual_id %d: %w", individualID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetIndividualStatus(status.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Individual pointer
	individual, err := s.GetIndividualByID(ctx, int(status.IndividualID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve individual for status (individual_id %d): %w", individualID, err)
	}
	status.Individual = individual

	// Register and return
	return models.Registry.RegisterIndividualStatus(&status), nil
}
