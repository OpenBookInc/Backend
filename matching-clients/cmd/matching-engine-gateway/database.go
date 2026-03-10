package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"matching-clients/src/utils"
	"os"
	"sort"
	"strings"
	"sync"

	gen "github.com/openbook/shared/models/gen"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SlateAndLineups represents the cached result from ensure_exchange_slate_and_lineups
type SlateAndLineups struct {
	Slate   SlateInfo
	Lineups []LineupInfo
}

type SlateInfo struct {
	ID         utils.UUID   `json:"id"`
	MarketIDs  []utils.UUID `json:"market_ids"`
	TotalUnits int64        `json:"total_units"`
}

type LineupInfo struct {
	ID          utils.UUID  `json:"id"`
	SlateID     utils.UUID  `json:"slate_id"`
	LineupIndex int         `json:"lineup_index"`
	Legs        []LineupLeg `json:"legs"`
}

type LineupLeg struct {
	MarketID utils.UUID     `json:"market_id"`
	Side     gen.MarketSide `json:"side"`
}

// slateCache caches slate+lineup results keyed by "sortedMarketID1,sortedMarketID2,..."
var (
	slateCache   = make(map[string]*SlateAndLineups)
	slateCacheMu sync.RWMutex
)

func buildSlateCacheKey(marketIDs []utils.UUID) string {
	strs := make([]string, len(marketIDs))
	for i, id := range marketIDs {
		strs[i] = id.String()
	}
	sort.Strings(strs)
	return strings.Join(strs, ",")
}

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

// EnsureSlateAndLineups ensures a slate and its lineups exist for the given markets.
// Results are cached locally so repeated calls for the same set of markets avoid DB round-trips.
func (gw *Gateway) EnsureSlateAndLineups(ctx context.Context, marketIDs []utils.UUID, totalUnits int64) (*SlateAndLineups, error) {
	cacheKey := buildSlateCacheKey(marketIDs)

	slateCacheMu.RLock()
	if cached, ok := slateCache[cacheKey]; ok {
		slateCacheMu.RUnlock()
		return cached, nil
	}
	slateCacheMu.RUnlock()

	marketIDStrs := make([]string, len(marketIDs))
	for i, id := range marketIDs {
		marketIDStrs[i] = id.String()
	}

	var resultJSON []byte
	err := gw.db.QueryRow(ctx,
		"SELECT ensure_exchange_slate_and_lineups($1::uuid[], $2)",
		marketIDStrs, totalUnits,
	).Scan(&resultJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure slate and lineups: %w", err)
	}

	var response struct {
		Slate   SlateInfo    `json:"slate"`
		Lineups []LineupInfo `json:"lineups"`
	}
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to parse slate response: %w", err)
	}

	result := &SlateAndLineups{
		Slate:   response.Slate,
		Lineups: response.Lineups,
	}

	slateCacheMu.Lock()
	slateCache[cacheKey] = result
	slateCacheMu.Unlock()

	log.Printf("Ensured slate id=%s with %d lineups for markets %v\n", result.Slate.ID.String(), len(result.Lineups), marketIDStrs)
	return result, nil
}

// FindLineup finds the lineup that matches the given legs (market_id + side).
// Returns the lineup UUID and the lineup index within the slate.
func FindLineup(slateAndLineups *SlateAndLineups, legs []LegRequest) (utils.UUID, int, error) {
	for _, lineup := range slateAndLineups.Lineups {
		if len(lineup.Legs) != len(legs) {
			continue
		}
		// Build a map of market_id -> side from the lineup
		lineupSides := make(map[utils.UUID]gen.MarketSide, len(lineup.Legs))
		for _, leg := range lineup.Legs {
			lineupSides[leg.MarketID] = leg.Side
		}
		match := true
		for i := range legs {
			expectedSide := gen.MarketSideUnder
			if legs[i].IsOver {
				expectedSide = gen.MarketSideOver
			}
			if lineupSides[legs[i].LegSecurityIDAsUUID()] != expectedSide {
				match = false
				break
			}
		}
		if match {
			return lineup.ID, lineup.LineupIndex, nil
		}
	}
	return utils.UUID{}, 0, fmt.Errorf("no matching lineup found for legs")
}

// CreateExchangeOrder creates a new exchange order and its initial state via the DB function.
// Returns the order UUID.
func (gw *Gateway) CreateExchangeOrder(ctx context.Context, lineupID utils.UUID, userID utils.UUID, orderType gen.ExchangeOrder, portion uint64, quantity uint64, clientOrderID uint64, status gen.ExchangeOrderStatus) (utils.UUID, error) {
	var resultJSON []byte
	err := gw.db.QueryRow(ctx,
		"SELECT create_exchange_order($1::uuid, $2::uuid, $3::exchange_order_type, $4, $5, $6, $7::exchange_order_status)",
		lineupID.String(), userID.String(), string(orderType), int64(portion), int64(quantity), int64(clientOrderID), string(status),
	).Scan(&resultJSON)
	if err != nil {
		return utils.UUID{}, fmt.Errorf("failed to create exchange order: %w", err)
	}

	var response struct {
		Order struct {
			ID utils.UUID `json:"id"`
		} `json:"order"`
	}
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		return utils.UUID{}, fmt.Errorf("failed to parse exchange order response: %w", err)
	}

	log.Printf("Created exchange order id=%s for user %s\n", response.Order.ID.String(), userID.String())
	return response.Order.ID, nil
}

// UpdateOrderStatus updates the status of an exchange order via the DB function.
func (gw *Gateway) UpdateOrderStatus(ctx context.Context, orderID utils.UUID, status gen.ExchangeOrderStatus) error {
	_, err := gw.db.Exec(ctx,
		"SELECT update_exchange_order_status($1::uuid, $2::exchange_order_status)",
		orderID.String(), string(status),
	)
	if err != nil {
		return fmt.Errorf("failed to update order status for order %s: %w", orderID.String(), err)
	}
	log.Printf("Updated order %s status to %s\n", orderID.String(), status)
	return nil
}

// CreateExchangeCancelRequest records a user-initiated cancel request for an exchange order.
func (gw *Gateway) CreateExchangeCancelRequest(ctx context.Context, orderID utils.UUID) error {
	_, err := gw.db.Exec(ctx,
		"SELECT create_exchange_cancel_request($1::uuid)",
		orderID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to create cancel request for order %s: %w", orderID.String(), err)
	}
	log.Printf("Created cancel request for order %s\n", orderID.String())
	return nil
}

// CancelExchangeOrderDueToExchange atomically cancels an order due to exchange-initiated
// cancellation and refunds the user's remaining (unfilled) balance.
func (gw *Gateway) CancelExchangeOrderDueToExchange(ctx context.Context, orderID utils.UUID) error {
	_, err := gw.db.Exec(ctx,
		"SELECT cancel_exchange_order_due_to_exchange($1::uuid)",
		orderID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to cancel order %s due to exchange: %w", orderID.String(), err)
	}
	log.Printf("Cancelled order %s due to exchange\n", orderID.String())
	return nil
}

// CancelExchangeOrderDueToUser atomically cancels an order due to user-initiated
// cancellation and refunds the user's remaining (unfilled) balance.
func (gw *Gateway) CancelExchangeOrderDueToUser(ctx context.Context, orderID utils.UUID) error {
	_, err := gw.db.Exec(ctx,
		"SELECT cancel_exchange_order_due_to_user($1::uuid)",
		orderID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to cancel order %s due to user: %w", orderID.String(), err)
	}
	log.Printf("Cancelled order %s due to user\n", orderID.String())
	return nil
}

// MatchFillParam represents one fill to pass to the create_exchange_match DB function.
type MatchFillParam struct {
	OrderID        string `json:"order_id"`
	MatchedPortion int64  `json:"matched_portion"`
}

// ExchangeMatchResult holds the parsed result of create_exchange_match.
type ExchangeMatchResult struct {
	MatchID utils.UUID
	Fills   []ExchangeFillResult
}

// ExchangeFillResult holds a single fill row returned by create_exchange_match.
type ExchangeFillResult struct {
	FillID         utils.UUID
	OrderID        utils.UUID
	MatchedPortion int64
}

// CreateExchangeMatch calls the create_exchange_match DB function to record a match and its fills.
func (gw *Gateway) CreateExchangeMatch(ctx context.Context, aggressorOrderID utils.UUID, matchedQuantity int64, fills []MatchFillParam) (*ExchangeMatchResult, error) {
	fillsJSON, err := json.Marshal(fills)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fills: %w", err)
	}

	var resultJSON []byte
	err = gw.db.QueryRow(ctx,
		"SELECT create_exchange_match($1::uuid, $2, $3::jsonb)",
		aggressorOrderID.String(), matchedQuantity, string(fillsJSON),
	).Scan(&resultJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create exchange match: %w", err)
	}

	var response struct {
		Match struct {
			ID utils.UUID `json:"id"`
		} `json:"match"`
		Fills []struct {
			ID             utils.UUID `json:"id"`
			OrderID        utils.UUID `json:"order_id"`
			MatchedPortion int64      `json:"matched_portion"`
		} `json:"fills"`
	}
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to parse exchange match response: %w", err)
	}

	result := &ExchangeMatchResult{
		MatchID: response.Match.ID,
		Fills:   make([]ExchangeFillResult, len(response.Fills)),
	}
	for i, f := range response.Fills {
		result.Fills[i] = ExchangeFillResult{
			FillID:         f.ID,
			OrderID:        f.OrderID,
			MatchedPortion: f.MatchedPortion,
		}
	}

	log.Printf("Created exchange match id=%s with %d fills\n", result.MatchID.String(), len(result.Fills))
	return result, nil
}

// GetSelfMatchID returns the user's UUID for self-match protection on the matching engine.
func (gw *Gateway) GetSelfMatchID(_ context.Context, userID utils.UUID) *utils.UUID {
	return &userID
}