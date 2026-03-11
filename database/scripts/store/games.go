package store

import (
	"context"
	"fmt"
	"time"

	models "github.com/openbook/shared/models"
	"github.com/openbook/shared/models/gen"
	"github.com/openbook/shared/utils"
)

// GameForUpsert contains the data needed to upsert a game
type GameForUpsert struct {
	SportradarID       string
	HomeTeamID         utils.UUID
	AwayTeamID         utils.UUID
	ScheduledStartTime time.Time
}

// UpsertGame inserts or updates a game in the database.
// Uses sportradar_id as the unique identifier (ON CONFLICT).
// Resolves the Team pointers and registers in the singleton registry.
func (s *Store) UpsertGame(ctx context.Context, game *GameForUpsert) error {
	query := `
		INSERT INTO games (contender_id_a, contender_id_b, sportradar_id, scheduled_start_time)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sportradar_id)
		DO UPDATE SET
			contender_id_a = EXCLUDED.contender_id_a,
			contender_id_b = EXCLUDED.contender_id_b,
			scheduled_start_time = EXCLUDED.scheduled_start_time
		RETURNING id
	`

	var id utils.UUID
	err := s.pool.QueryRow(ctx, query,
		game.HomeTeamID,
		game.AwayTeamID,
		game.SportradarID,
		game.ScheduledStartTime,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to upsert game (sportradar_id: %s): %w", game.SportradarID, err)
	}

	teamA, err := s.GetTeamByID(ctx, game.HomeTeamID)
	if err != nil {
		return fmt.Errorf("failed to resolve home team for game %s: %w", game.SportradarID, err)
	}

	teamB, err := s.GetTeamByID(ctx, game.AwayTeamID)
	if err != nil {
		return fmt.Errorf("failed to resolve away team for game %s: %w", game.SportradarID, err)
	}

	if _, err := models.Registry.RegisterGame(&models.Game{
		ID:                 id,
		ContenderIDA:       game.HomeTeamID,
		ContenderIDB:       game.AwayTeamID,
		SportradarID:       game.SportradarID,
		ScheduledStartTime: game.ScheduledStartTime,
		TeamA:              teamA,
		TeamB:              teamB,
	}); err != nil {
		return fmt.Errorf("failed to register game %s: %w", game.SportradarID, err)
	}
	return nil
}

// GetGameByID retrieves a game by database ID.
// Uses the registry for caching and resolves the nested Team pointers.
func (s *Store) GetGameByID(ctx context.Context, gameID utils.UUID) (*models.Game, error) {
	// Check registry first
	if game := models.Registry.GetGame(gameID); game != nil {
		return game, nil
	}

	// Query database
	query := `
		SELECT id, contender_id_a, contender_id_b, sportradar_id, scheduled_start_time
		FROM games
		WHERE id = $1
	`

	var game models.Game
	err := s.pool.QueryRow(ctx, query, gameID).Scan(
		&game.ID,
		&game.ContenderIDA,
		&game.ContenderIDB,
		&game.SportradarID,
		&game.ScheduledStartTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get game with id %s: %w", gameID, err)
	}

	// Resolve nested Team pointers
	teamA, err := s.GetTeamByID(ctx, game.ContenderIDA)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team A for game %s: %w", gameID, err)
	}
	game.TeamA = teamA

	teamB, err := s.GetTeamByID(ctx, game.ContenderIDB)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team B for game %s: %w", gameID, err)
	}
	game.TeamB = teamB

	// Register and return
	return models.Registry.RegisterGame(&game)
}

// GetGameBySportradarID retrieves a game by sportradar_id.
// Uses the registry for caching and resolves the nested Team pointers.
func (s *Store) GetGameBySportradarID(ctx context.Context, sportradarID string) (*models.Game, error) {
	// Query database to get the ID first
	query := `
		SELECT id, contender_id_a, contender_id_b, sportradar_id, scheduled_start_time
		FROM games
		WHERE sportradar_id = $1
	`

	var game models.Game
	err := s.pool.QueryRow(ctx, query, sportradarID).Scan(
		&game.ID,
		&game.ContenderIDA,
		&game.ContenderIDB,
		&game.SportradarID,
		&game.ScheduledStartTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get game with sportradar_id %s: %w", sportradarID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetGame(game.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Team pointers
	teamA, err := s.GetTeamByID(ctx, game.ContenderIDA)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team A for game %s: %w", sportradarID, err)
	}
	game.TeamA = teamA

	teamB, err := s.GetTeamByID(ctx, game.ContenderIDB)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team B for game %s: %w", sportradarID, err)
	}
	game.TeamB = teamB

	// Register and return
	return models.Registry.RegisterGame(&game)
}

// GetGameWithTeamsByID retrieves a game by database ID with TeamA and TeamB populated.
// Delegates to GetGameByID which now automatically resolves team pointers via the registry.
func (s *Store) GetGameWithTeamsByID(ctx context.Context, gameID utils.UUID) (*models.Game, error) {
	return s.GetGameByID(ctx, gameID)
}

// GetGamesByLeague retrieves all games for a given league name in a single query.
// JOINs through teams -> divisions -> conferences -> leagues and fetches all game columns inline.
// Expects teams to already be registered in the registry (call GetTeamsByLeague first).
func (s *Store) GetGamesByLeague(ctx context.Context, leagueName string) ([]*models.Game, error) {
	query := `
		SELECT g.id, g.contender_id_a, g.contender_id_b, g.sportradar_id, g.scheduled_start_time
		FROM games g
		JOIN teams t ON g.contender_id_a = t.id
		JOIN divisions d ON t.division_id = d.id
		JOIN conferences c ON d.conference_id = c.id
		JOIN leagues l ON c.league_id = l.id
		WHERE l.name = $1
		ORDER BY g.scheduled_start_time ASC
	`

	rows, err := s.pool.Query(ctx, query, leagueName)
	if err != nil {
		return nil, fmt.Errorf("failed to query games for league %s: %w", leagueName, err)
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		var game models.Game
		if err := rows.Scan(
			&game.ID, &game.ContenderIDA, &game.ContenderIDB,
			&game.SportradarID, &game.ScheduledStartTime,
		); err != nil {
			return nil, fmt.Errorf("failed to scan game row: %w", err)
		}

		// Resolve team pointers from registry (teams should already be loaded)
		game.TeamA = models.Registry.GetTeam(game.ContenderIDA)
		game.TeamB = models.Registry.GetTeam(game.ContenderIDB)

		regGame, err := models.Registry.RegisterGame(&game)
		if err != nil {
			return nil, fmt.Errorf("failed to register game %s: %w", game.SportradarID, err)
		}
		games = append(games, regGame)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating game rows: %w", err)
	}

	return games, nil
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

	var gameIDs []utils.UUID
	for rows.Next() {
		var gameID utils.UUID
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
			return nil, fmt.Errorf("failed to get game %s: %w", gameID, err)
		}
		games = append(games, game)
	}

	return games, nil
}

// GetGameByVendorID retrieves a game by vendor ID.
// First checks the registry, then falls back to querying entity_vendor_ids.
// Registers the vendor mapping in the registry if found via DB query.
func (s *Store) GetGameByVendorID(ctx context.Context, vendor gen.Vendor, vendorID string) (*models.Game, error) {
	// Check registry first
	if entityID, ok := models.Registry.GetEntityIDByVendorID(gen.EntityGame, vendor, vendorID); ok {
		return s.GetGameByID(ctx, entityID)
	}

	// Query entity_vendor_ids table
	query := `
		SELECT entity_id
		FROM entity_vendor_ids
		WHERE entity_type = $1 AND vendor = $2 AND vendor_id = $3
	`

	var entityID utils.UUID
	err := s.pool.QueryRow(ctx, query, gen.EntityGame, vendor, vendorID).Scan(&entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to find game with vendor_id %s (vendor=%s): %w", vendorID, vendor, err)
	}

	// Register the vendor mapping in the registry
	models.Registry.RegisterVendorID(gen.EntityGame, entityID, vendor, vendorID)

	return s.GetGameByID(ctx, entityID)
}
