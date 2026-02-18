package store

import (
	"context"
	"fmt"
	"time"

	models "github.com/openbook/shared/models"
)

// GameForUpsert contains the data needed to upsert a game
type GameForUpsert struct {
	VendorID           string
	HomeTeamID         int
	AwayTeamID         int
	ScheduledStartTime time.Time
}

// UpsertGame inserts or updates a game in the database.
// Uses vendor_id as the unique identifier (ON CONFLICT).
// Resolves the Team pointers and registers in the singleton registry.
func (s *Store) UpsertGame(ctx context.Context, game *GameForUpsert) error {
	query := `
		INSERT INTO games (contender_id_a, contender_id_b, vendor_id, scheduled_start_time)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (vendor_id)
		DO UPDATE SET
			contender_id_a = EXCLUDED.contender_id_a,
			contender_id_b = EXCLUDED.contender_id_b,
			scheduled_start_time = EXCLUDED.scheduled_start_time
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		game.HomeTeamID,
		game.AwayTeamID,
		game.VendorID,
		game.ScheduledStartTime,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to upsert game (vendor_id: %s): %w", game.VendorID, err)
	}

	teamA, err := s.GetTeamByID(ctx, game.HomeTeamID)
	if err != nil {
		return fmt.Errorf("failed to resolve home team for game %s: %w", game.VendorID, err)
	}

	teamB, err := s.GetTeamByID(ctx, game.AwayTeamID)
	if err != nil {
		return fmt.Errorf("failed to resolve away team for game %s: %w", game.VendorID, err)
	}

	models.Registry.RegisterGame(&models.Game{
		ID:                 id,
		ContenderIDA:       int64(game.HomeTeamID),
		ContenderIDB:       int64(game.AwayTeamID),
		VendorID:           game.VendorID,
		ScheduledStartTime: game.ScheduledStartTime,
		TeamA:              teamA,
		TeamB:              teamB,
	})
	return nil
}

// GetGameByID retrieves a game by database ID.
// Uses the registry for caching and resolves the nested Team pointers.
func (s *Store) GetGameByID(ctx context.Context, gameID int) (*models.Game, error) {
	// Check registry first
	if game := models.Registry.GetGame(gameID); game != nil {
		return game, nil
	}

	// Query database
	query := `
		SELECT id, contender_id_a, contender_id_b, vendor_id, scheduled_start_time
		FROM games
		WHERE id = $1
	`

	var game models.Game
	err := s.pool.QueryRow(ctx, query, gameID).Scan(
		&game.ID,
		&game.ContenderIDA,
		&game.ContenderIDB,
		&game.VendorID,
		&game.ScheduledStartTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get game with id %d: %w", gameID, err)
	}

	// Resolve nested Team pointers
	teamA, err := s.GetTeamByID(ctx, int(game.ContenderIDA))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team A for game %d: %w", gameID, err)
	}
	game.TeamA = teamA

	teamB, err := s.GetTeamByID(ctx, int(game.ContenderIDB))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team B for game %d: %w", gameID, err)
	}
	game.TeamB = teamB

	// Register and return
	return models.Registry.RegisterGame(&game), nil
}

// GetGameByVendorID retrieves a game by vendor_id.
// Uses the registry for caching and resolves the nested Team pointers.
func (s *Store) GetGameByVendorID(ctx context.Context, vendorID string) (*models.Game, error) {
	// Query database to get the ID first
	query := `
		SELECT id, contender_id_a, contender_id_b, vendor_id, scheduled_start_time
		FROM games
		WHERE vendor_id = $1
	`

	var game models.Game
	err := s.pool.QueryRow(ctx, query, vendorID).Scan(
		&game.ID,
		&game.ContenderIDA,
		&game.ContenderIDB,
		&game.VendorID,
		&game.ScheduledStartTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get game with vendor_id %s: %w", vendorID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetGame(game.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Team pointers
	teamA, err := s.GetTeamByID(ctx, int(game.ContenderIDA))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team A for game %s: %w", vendorID, err)
	}
	game.TeamA = teamA

	teamB, err := s.GetTeamByID(ctx, int(game.ContenderIDB))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team B for game %s: %w", vendorID, err)
	}
	game.TeamB = teamB

	// Register and return
	return models.Registry.RegisterGame(&game), nil
}

// GetGameWithTeamsByID retrieves a game by database ID with TeamA and TeamB populated.
// Delegates to GetGameByID which now automatically resolves team pointers via the registry.
func (s *Store) GetGameWithTeamsByID(ctx context.Context, gameID int) (*models.Game, error) {
	return s.GetGameByID(ctx, gameID)
}

// GetGamesByLeagueAndDateRange retrieves games for a league within a date range (inclusive).
// Uses the registry for caching and resolves nested Team pointers for each game.
// The timeZone parameter is an IANA timezone name (e.g., "America/Los_Angeles") used to
// interpret which calendar date a game falls on. This is necessary because a game stored
// as e.g. 2026-01-27 02:00 UTC is actually on 2026-01-26 in Pacific time.
// Returns games ordered by scheduled_start_time ascending.
func (s *Store) GetGamesByLeagueAndDateRange(ctx context.Context, leagueName string, startDate, endDate time.Time, timeZone string) ([]*models.Game, error) {
	query := `
		SELECT g.id
		FROM games g
		JOIN teams t ON g.contender_id_a = t.id
		JOIN divisions d ON t.division_id = d.id
		JOIN conferences c ON d.conference_id = c.id
		JOIN leagues l ON c.league_id = l.id
		WHERE l.name = $1
		  AND (g.scheduled_start_time AT TIME ZONE $4)::date >= $2
		  AND (g.scheduled_start_time AT TIME ZONE $4)::date <= $3
		ORDER BY g.scheduled_start_time ASC
	`

	rows, err := s.pool.Query(ctx, query, leagueName, startDate, endDate, timeZone)
	if err != nil {
		return nil, fmt.Errorf("failed to query games for league %s: %w", leagueName, err)
	}
	defer rows.Close()

	var gameIDs []int
	for rows.Next() {
		var gameID int
		if err := rows.Scan(&gameID); err != nil {
			return nil, fmt.Errorf("failed to scan game id: %w", err)
		}
		gameIDs = append(gameIDs, gameID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating game rows: %w", err)
	}

	// Fetch each game using GetGameByID (which uses registry and resolves teams)
	var games []*models.Game
	for _, gameID := range gameIDs {
		game, err := s.GetGameByID(ctx, gameID)
		if err != nil {
			return nil, fmt.Errorf("failed to get game %d: %w", gameID, err)
		}
		games = append(games, game)
	}

	return games, nil
}
