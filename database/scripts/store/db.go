package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides database operations
type Store struct {
	pool *pgxpool.Pool
}

// New creates a new Store with a connection pool
func New(ctx context.Context, host, port, database, user, password, sslKeyPath string) (*Store, error) {
	// Load SSL certificate
	caCert, err := os.ReadFile(sslKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSL certificate from %s: %w", sslKeyPath, err)
	}

	// Create certificate pool
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse SSL certificate from %s", sslKeyPath)
	}

	// Configure TLS with ServerName for SNI (Server Name Indication)
	tlsConfig := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: host,
	}

	// Build connection string
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=require",
		user, password, host, port, database)

	// Parse config
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Set TLS config
	config.ConnConfig.TLSConfig = tlsConfig

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close closes the database connection pool
func (s *Store) Close() {
	s.pool.Close()
}

// Pool returns the underlying connection pool for use by extension packages
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// BeginTx starts a new database transaction
func (s *Store) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}
