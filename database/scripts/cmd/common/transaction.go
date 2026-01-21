package common

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/store"
)

// WithTransaction executes the provided function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// If the function succeeds, the transaction is committed.
func WithTransaction(ctx context.Context, dbStore *store.Store, fn func(tx pgx.Tx) error) error {
	tx, err := dbStore.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
