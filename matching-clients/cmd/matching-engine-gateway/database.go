package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connectDB establishes a connection to the PostgreSQL database
func (gw *Gateway) connectDB(ctx context.Context) error {
	// Load SSL certificate
	caCert, err := os.ReadFile(gw.config.PGKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSL certificate from %s: %w", gw.config.PGKeyPath, err)
	}

	// Create certificate pool
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to parse SSL certificate from %s", gw.config.PGKeyPath)
	}

	// Configure TLS with ServerName for SNI
	tlsConfig := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: gw.config.PGHost,
	}

	// Build connection string
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=require",
		gw.config.PGUser, gw.config.PGPassword, gw.config.PGHost, gw.config.PGPort, gw.config.PGDatabase)

	// Parse config
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}

	// Set TLS config
	poolConfig.ConnConfig.TLSConfig = tlsConfig

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	gw.db = pool
	log.Println("Connected to database successfully")
	return nil
}

// CheckBalance checks if a user has sufficient balance for an order
// TODO: STUB - Replace with actual balance check implementation
func (gw *Gateway) CheckBalance(ctx context.Context, userID string, amount int64) (bool, error) {
	var availableBalance int64
	err := gw.db.QueryRow(ctx,
		"SELECT available_balance FROM balances WHERE user_id = $1",
		userID,
	).Scan(&availableBalance)

	if err != nil {
		return false, fmt.Errorf("failed to query balance for user %s: %w", userID, err)
	}

	return availableBalance >= amount, nil
}

// DeductBalance deducts the specified amount from a user's balance
// TODO: STUB - Replace with stored procedure call for atomic operations
func (gw *Gateway) DeductBalance(ctx context.Context, userID string, amount int64) error {
	// TODO: Replace with stored procedure call for atomic operations
	// The stored procedure should handle the balance deduction atomically
	// along with any other required operations

	result, err := gw.db.Exec(ctx,
		"UPDATE balances SET available_balance = available_balance - $1, updated_at = NOW() WHERE user_id = $2 AND available_balance >= $1",
		amount, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to deduct balance for user %s: %w", userID, err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("insufficient balance or user not found")
	}

	log.Printf("Deducted %d from user %s balance\n", amount, userID)
	return nil
}

// InsertConfirmedBet inserts a new bet record into the database
// TODO: STUB - Table schema may need adjustment
func (gw *Gateway) InsertConfirmedBet(ctx context.Context, req *OrderRequest, clientOrderID uint64) (int64, error) {
	var id int64
	timePlaced := time.Now().UnixNano()

	err := gw.db.QueryRow(ctx,
		`INSERT INTO confirmed_bets
(user_id, button_id, line, wager_amount, total_payout, commission, time_placed, order_status, bet_status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id`,
		req.UserID,
		req.ButtonID,
		req.Line,
		req.WagerAmount,
		req.TotalPayout,
		req.Commission,
		timePlaced,
		OrderStatusSubmitted,
		BetStatusPending,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to insert confirmed bet: %w", err)
	}

	log.Printf("Inserted confirmed_bet with id=%d for user %s\n", id, req.UserID)
	return id, nil
}

// UpdateOrderStatus updates the order status in the database
func (gw *Gateway) UpdateOrderStatus(ctx context.Context, id int64, status OrderStatus) error {
	result, err := gw.db.Exec(ctx,
		"UPDATE confirmed_bets SET order_status = $1, updated_at = NOW() WHERE id = $2",
		status, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update order status for id %d: %w", id, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no confirmed_bet found with id %d", id)
	}

	log.Printf("Updated order %d status to %s\n", id, status)
	return nil
}

// ValidateLegSecurityIDs validates that all leg security IDs exist and are valid
// TODO: STUB - Implement leg security ID validation
func (gw *Gateway) ValidateLegSecurityIDs(ctx context.Context, legs []LegRequest) error {
	// TODO: Query database to validate each leg security ID exists
	// For now, accept all leg security IDs
	return nil
}

// GetSelfMatchID translates a user ID to a self match ID (16-byte UUID) for the matching engine
// TODO: STUB - Implement user ID to self match ID translation
func (gw *Gateway) GetSelfMatchID(ctx context.Context, userID string) ([]byte, error) {
	// TODO: Implement proper mapping from user_id string to selfMatchId bytes
	// This should parse the user's UUID and return the raw 16 bytes
	// For now, return nil (no self-match protection)
	return nil, nil
}
