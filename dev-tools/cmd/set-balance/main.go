package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/shared/envloader"
)

func main() {
	if err := envloader.LoadEnvFile(""); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading .env: %v\n", err)
		os.Exit(1)
	}

	requiredVars := []string{"PG_HOST", "PG_PORT", "PG_DATABASE", "PG_USER", "PG_PASSWORD", "PG_KEY_PATH", "SET_BALANCE_USER_ID", "SET_BALANCE_NEW_BALANCE"}
	if err := envloader.ValidateRequired(requiredVars); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	userID := os.Getenv("SET_BALANCE_USER_ID")
	newBalance, err := strconv.ParseInt(os.Getenv("SET_BALANCE_NEW_BALANCE"), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: SET_BALANCE_NEW_BALANCE must be an integer, got %q\n", os.Getenv("SET_BALANCE_NEW_BALANCE"))
		os.Exit(1)
	}

	host := os.Getenv("PG_HOST")
	port := os.Getenv("PG_PORT")
	database := os.Getenv("PG_DATABASE")
	user := os.Getenv("PG_USER")
	password := os.Getenv("PG_PASSWORD")
	keyPath := os.Getenv("PG_KEY_PATH")

	ctx := context.Background()

	caCert, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading SSL certificate from %s: %v\n", keyPath, err)
		os.Exit(1)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		fmt.Fprintf(os.Stderr, "Error: failed to parse SSL certificate from %s\n", keyPath)
		os.Exit(1)
	}

	tlsConfig := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: host,
	}

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=verify-full",
		user, password, host, port, database)

	connConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing database config: %v\n", err)
		os.Exit(1)
	}
	connConfig.TLSConfig = tlsConfig

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	var resultUserID string
	var totalBalance int64
	err = conn.QueryRow(ctx, "SELECT user_id, total_balance FROM set_balance($1, $2)", userID, newBalance).Scan(&resultUserID, &totalBalance)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error calling set_balance: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Balance updated successfully.\n  user_id: %s\n  total_balance: %d\n", resultUserID, totalBalance)
}