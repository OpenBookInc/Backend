package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	models "github.com/openbook/shared/models"
)

// TeamForUpsert contains the data needed to upsert a team
type TeamForUpsert struct {
	SportradarID string
	Name         string
	Market       string
	Alias        string
	DivisionID   int
	VenueName    string
	VenueCity    string
	VenueState   string
}

// UpsertTeam inserts or updates a team in the database.
// Uses sportradar_id as the unique identifier (ON CONFLICT).
// Resolves the Division pointer and registers in the singleton registry.
func (s *Store) UpsertTeam(ctx context.Context, team *TeamForUpsert) error {
	query := `
		INSERT INTO teams (name, market, alias, sportradar_id, division_id, venue_name, venue_city, venue_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (sportradar_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			market = EXCLUDED.market,
			alias = EXCLUDED.alias,
			division_id = EXCLUDED.division_id,
			venue_name = EXCLUDED.venue_name,
			venue_city = EXCLUDED.venue_city,
			venue_state = EXCLUDED.venue_state
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		team.Name,
		team.Market,
		team.Alias,
		team.SportradarID,
		team.DivisionID,
		team.VenueName,
		team.VenueCity,
		team.VenueState,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to upsert team %s %s (sportradar_id: %s): %w",
			team.Market, team.Name, team.SportradarID, err)
	}

	division, err := s.GetDivisionByID(ctx, team.DivisionID)
	if err != nil {
		return fmt.Errorf("failed to resolve division for team %s: %w", team.SportradarID, err)
	}

	if _, err := models.Registry.RegisterTeam(&models.Team{
		ID:           id,
		Name:         team.Name,
		Market:       team.Market,
		Alias:        team.Alias,
		SportradarID: team.SportradarID,
		DivisionID:   int64(team.DivisionID),
		VenueName:    team.VenueName,
		VenueCity:    team.VenueCity,
		VenueState:   team.VenueState,
		Division:     division,
	}); err != nil {
		return fmt.Errorf("failed to register team %s %s: %w", team.Market, team.Name, err)
	}
	return nil
}

// GetTeamByID retrieves a team by database ID.
// Uses the registry for caching and resolves the nested Division pointer.
func (s *Store) GetTeamByID(ctx context.Context, id int) (*models.Team, error) {
	// Check registry first
	if team := models.Registry.GetTeam(id); team != nil {
		return team, nil
	}

	// Query database
	query := `
		SELECT id, name, market, alias, sportradar_id, division_id, venue_name, venue_city, venue_state
		FROM teams
		WHERE id = $1
	`

	var team models.Team
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.Market,
		&team.Alias,
		&team.SportradarID,
		&team.DivisionID,
		&team.VenueName,
		&team.VenueCity,
		&team.VenueState,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get team with id %d: %w", id, err)
	}

	// Resolve nested Division pointer
	division, err := s.GetDivisionByID(ctx, int(team.DivisionID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve division for team %d: %w", id, err)
	}
	team.Division = division

	// Register and return
	return models.Registry.RegisterTeam(&team)
}

// GetTeamsByLeague retrieves all teams for a given league name in a single query.
// JOINs through divisions → conferences → leagues and fetches all columns inline,
// registering the full hierarchy (league, conference, division, team) in the registry.
func (s *Store) GetTeamsByLeague(ctx context.Context, leagueName string) ([]*models.Team, error) {
	query := `
		SELECT
			t.id, t.name, t.market, t.alias, t.sportradar_id, t.division_id,
			t.venue_name, t.venue_city, t.venue_state,
			d.id, d.name, d.conference_id, d.sportradar_id, d.alias,
			c.id, c.name, c.league_id, c.sportradar_id, c.alias,
			l.id, l.sport_id, l.name
		FROM teams t
		JOIN divisions d ON t.division_id = d.id
		JOIN conferences c ON d.conference_id = c.id
		JOIN leagues l ON c.league_id = l.id
		WHERE l.name = $1
		ORDER BY t.id ASC
	`

	rows, err := s.pool.Query(ctx, query, leagueName)
	if err != nil {
		return nil, fmt.Errorf("failed to query teams for league %s: %w", leagueName, err)
	}
	defer rows.Close()

	var teams []*models.Team
	for rows.Next() {
		var team models.Team
		var div models.Division
		var conf models.Conference
		var league models.League

		if err := rows.Scan(
			&team.ID, &team.Name, &team.Market, &team.Alias, &team.SportradarID, &team.DivisionID,
			&team.VenueName, &team.VenueCity, &team.VenueState,
			&div.ID, &div.Name, &div.ConferenceID, &div.SportradarID, &div.Alias,
			&conf.ID, &conf.Name, &conf.LeagueID, &conf.SportradarID, &conf.Alias,
			&league.ID, &league.SportID, &league.Name,
		); err != nil {
			return nil, fmt.Errorf("failed to scan team row: %w", err)
		}

		// Register hierarchy bottom-up so pointers resolve
		regLeague := models.Registry.RegisterLeague(&league)
		conf.League = regLeague
		regConf := models.Registry.RegisterConference(&conf)
		div.Conference = regConf
		regDiv := models.Registry.RegisterDivision(&div)
		team.Division = regDiv

		regTeam, err := models.Registry.RegisterTeam(&team)
		if err != nil {
			return nil, fmt.Errorf("failed to register team %s %s: %w", team.Market, team.Name, err)
		}
		teams = append(teams, regTeam)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating team rows: %w", err)
	}

	return teams, nil
}

// GetTeamBySportradarID retrieves a team by sportradar_id.
// Uses the registry for caching and resolves the nested Division pointer.
func (s *Store) GetTeamBySportradarID(ctx context.Context, sportradarID string) (*models.Team, error) {
	// Query database to get the ID first
	query := `
		SELECT id, name, market, alias, sportradar_id, division_id, venue_name, venue_city, venue_state
		FROM teams
		WHERE sportradar_id = $1
	`

	var team models.Team
	err := s.pool.QueryRow(ctx, query, sportradarID).Scan(
		&team.ID,
		&team.Name,
		&team.Market,
		&team.Alias,
		&team.SportradarID,
		&team.DivisionID,
		&team.VenueName,
		&team.VenueCity,
		&team.VenueState,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("failed to get team with sportradar_id %s: %w", sportradarID, ErrTeamNotFound)
		}
		return nil, fmt.Errorf("failed to get team with sportradar_id %s: %w", sportradarID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetTeam(team.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Division pointer
	division, err := s.GetDivisionByID(ctx, int(team.DivisionID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve division for team %s: %w", sportradarID, err)
	}
	team.Division = division

	// Register and return
	return models.Registry.RegisterTeam(&team)
}
