package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
	gen "github.com/openbook/shared/models/gen"
)

// IndividualStatusForUpsert contains the data needed to upsert an individual status
type IndividualStatusForUpsert struct {
	IndividualID string
	Status       string // DB enum value as string (e.g., "active", "questionable")
}

// UpsertIndividualStatus inserts or updates an individual's status in the database.
// Uses individual_id as the unique identifier (ON CONFLICT) - one status per player.
// Resolves the Individual pointer and registers in the singleton registry.
func (s *Store) UpsertIndividualStatus(ctx context.Context, status *IndividualStatusForUpsert) error {
	query := `
		INSERT INTO individual_statuses (individual_id, status)
		VALUES ($1, $2::individual_status_type)
		ON CONFLICT (individual_id)
		DO UPDATE SET
			status = EXCLUDED.status
	`

	_, err := s.pool.Exec(ctx, query,
		status.IndividualID,
		status.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert individual status (individual_id: %s): %w", status.IndividualID, err)
	}

	individual, err := s.GetIndividualByID(ctx, status.IndividualID)
	if err != nil {
		return fmt.Errorf("failed to resolve individual for status (individual_id %s): %w", status.IndividualID, err)
	}

	models.Registry.RegisterIndividualStatus(&models.IndividualStatus{
		IndividualID: status.IndividualID,
		Status:       gen.IndividualStatus(status.Status),
		Individual:   individual,
	})
	return nil
}

// GetIndividualStatusByIndividualID retrieves an individual's status by individual_id.
// Uses the registry for caching and resolves the nested Individual pointer.
func (s *Store) GetIndividualStatusByIndividualID(ctx context.Context, individualID string) (*models.IndividualStatus, error) {
	// Check registry first
	if status := models.Registry.GetIndividualStatus(individualID); status != nil {
		return status, nil
	}

	// Query database
	query := `
		SELECT individual_id, status
		FROM individual_statuses
		WHERE individual_id = $1
	`

	var status models.IndividualStatus
	err := s.pool.QueryRow(ctx, query, individualID).Scan(
		&status.IndividualID,
		&status.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual status with individual_id %s: %w", individualID, err)
	}

	// Resolve nested Individual pointer
	individual, err := s.GetIndividualByID(ctx, status.IndividualID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve individual for status (individual_id %s): %w", individualID, err)
	}
	status.Individual = individual

	// Register and return
	return models.Registry.RegisterIndividualStatus(&status), nil
}
