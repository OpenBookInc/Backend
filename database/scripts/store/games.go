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

// UpsertGame inserts or updates a game in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the game
func (s *Store) UpsertGame(ctx context.Context, game *GameForUpsert) (int, error) {
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
		return 0, fmt.Errorf("failed to upsert game (vendor_id: %s): %w", game.VendorID, err)
	}

	return id, nil
}

// GetGameByVendorID retrieves a game by vendor_id
func (s *Store) GetGameByVendorID(ctx context.Context, vendorID string) (*models.Game, error) {
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

	return &game, nil
}

// GetGameByID retrieves a game by database ID
func (s *Store) GetGameByID(ctx context.Context, gameID int) (*models.Game, error) {
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

	return &game, nil
}

// GetGameWithTeamsByID retrieves a game by database ID and populates TeamA and TeamB.
// Uses JOINs to fetch team details for both contenders.
func (s *Store) GetGameWithTeamsByID(ctx context.Context, gameID int) (*models.Game, error) {
	query := `
		SELECT
			g.id, g.contender_id_a, g.contender_id_b, g.vendor_id, g.scheduled_start_time,
			ta.id, ta.name, ta.market, ta.alias, ta.vendor_id, ta.division_id, ta.venue_name, ta.venue_city, ta.venue_state,
			tb.id, tb.name, tb.market, tb.alias, tb.vendor_id, tb.division_id, tb.venue_name, tb.venue_city, tb.venue_state
		FROM games g
		JOIN teams ta ON g.contender_id_a = ta.id
		JOIN teams tb ON g.contender_id_b = tb.id
		WHERE g.id = $1
	`

	var game models.Game
	teamA := &models.Team{}
	teamB := &models.Team{}

	err := s.pool.QueryRow(ctx, query, gameID).Scan(
		&game.ID,
		&game.ContenderIDA,
		&game.ContenderIDB,
		&game.VendorID,
		&game.ScheduledStartTime,
		&teamA.ID,
		&teamA.Name,
		&teamA.Market,
		&teamA.Alias,
		&teamA.VendorID,
		&teamA.DivisionID,
		&teamA.VenueName,
		&teamA.VenueCity,
		&teamA.VenueState,
		&teamB.ID,
		&teamB.Name,
		&teamB.Market,
		&teamB.Alias,
		&teamB.VendorID,
		&teamB.DivisionID,
		&teamB.VenueName,
		&teamB.VenueCity,
		&teamB.VenueState,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get game with teams for id %d: %w", gameID, err)
	}

	game.TeamA = teamA
	game.TeamB = teamB

	return &game, nil
}

// GetGamesByLeagueAndDateRange retrieves games for a league within a date range (inclusive).
// Returns games ordered by scheduled_start_time ascending.
func (s *Store) GetGamesByLeagueAndDateRange(ctx context.Context, leagueName string, startDate, endDate time.Time) ([]*models.Game, error) {
	query := `
		SELECT g.id, g.contender_id_a, g.contender_id_b, g.vendor_id, g.scheduled_start_time
		FROM games g
		JOIN teams t ON g.contender_id_a = t.id
		JOIN divisions d ON t.division_id = d.id
		JOIN conferences c ON d.conference_id = c.id
		JOIN leagues l ON c.league_id = l.id
		WHERE l.name = $1
		  AND g.scheduled_start_time >= $2
		  AND g.scheduled_start_time < $3
		ORDER BY g.scheduled_start_time ASC
	`

	// Add one day to end date to make it inclusive (query uses < for end)
	endDateExclusive := endDate.AddDate(0, 0, 1)

	rows, err := s.pool.Query(ctx, query, leagueName, startDate, endDateExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to query games for league %s: %w", leagueName, err)
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		var game models.Game
		err := rows.Scan(
			&game.ID,
			&game.ContenderIDA,
			&game.ContenderIDB,
			&game.VendorID,
			&game.ScheduledStartTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan game row: %w", err)
		}
		games = append(games, &game)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating game rows: %w", err)
	}

	return games, nil
}
