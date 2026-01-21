package common

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/store"
)

// DatabaseConfig contains the parameters needed to connect to the database.
type DatabaseConfig struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
	KeyPath  string
}

// ConnectToDatabase establishes a connection to the database and prints status messages.
// The caller is responsible for calling Close() on the returned store when done.
func ConnectToDatabase(ctx context.Context, cfg *DatabaseConfig) (*store.Store, error) {
	fmt.Println("\nConnecting to database...")
	dbStore, err := store.New(ctx, cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.Password, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	fmt.Println("Connected to database successfully!")
	return dbStore, nil
}
