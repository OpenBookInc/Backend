package oddsblaze

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	reducer_oddsblaze "github.com/openbook/population-scripts/reducer/oddsblaze"
	"github.com/openbook/population-scripts/store"
	models "github.com/openbook/shared/models"
)

// MatchedTeam pairs an OddsBlaze team ID with the corresponding database team
type MatchedTeam struct {
	OddsBlazeID string
	DBTeam      *models.Team
}

// MatchedIndividual pairs an OddsBlaze player ID with the corresponding database individual
type MatchedIndividual struct {
	OddsBlazeID  string
	DBIndividual *models.Individual
}

// MatchedGame pairs an OddsBlaze event ID with the corresponding database game
type MatchedGame struct {
	OddsBlazeID string
	DBGame      *models.Game
}

// MatchedEntities holds all matched entity pairs
type MatchedEntities struct {
	Teams       []MatchedTeam
	Individuals []MatchedIndividual
	Games       []MatchedGame
}

// gameKey is used to look up games by the team pair and scheduled start time
type gameKey struct {
	TeamIDA            int
	TeamIDB            int
	ScheduledStartTime time.Time
}

// makeGameKey creates a normalized game key with sorted team IDs so order doesn't matter
func makeGameKey(teamIDA, teamIDB int, scheduledStartTime time.Time) gameKey {
	if teamIDA > teamIDB {
		teamIDA, teamIDB = teamIDB, teamIDA
	}
	return gameKey{
		TeamIDA:            teamIDA,
		TeamIDB:            teamIDB,
		ScheduledStartTime: scheduledStartTime.UTC(),
	}
}

// MatchEntities matches reduced OddsBlaze entities against database entities.
// Returns an error on the first unmatched entity.
func MatchEntities(ctx context.Context, dbStore *store.Store, leagueName string, reduced *reducer_oddsblaze.ReducedEntities) (*MatchedEntities, error) {
	// Match teams
	matchedTeams, teamsByOddsBlazeID, err := matchTeams(ctx, dbStore, leagueName, reduced.Teams)
	if err != nil {
		return nil, err
	}

	// Match individuals (by team + jersey number using roster data)
	matchedIndividuals, err := matchIndividuals(ctx, dbStore, leagueName, reduced.Individuals, teamsByOddsBlazeID)
	if err != nil {
		return nil, err
	}

	// Match games
	matchedGames, err := matchGames(ctx, dbStore, leagueName, reduced.Games, teamsByOddsBlazeID)
	if err != nil {
		return nil, err
	}

	return &MatchedEntities{
		Teams:       matchedTeams,
		Individuals: matchedIndividuals,
		Games:       matchedGames,
	}, nil
}

// matchTeams matches reduced teams against database teams by alias (abbreviation).
// Returns matched teams and a lookup map from OddsBlaze ID to DB team.
func matchTeams(ctx context.Context, dbStore *store.Store, leagueName string, reducedTeams []reducer_oddsblaze.ReducedTeam) ([]MatchedTeam, map[string]*models.Team, error) {
	dbTeams, err := dbStore.GetTeamsByLeague(ctx, leagueName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load teams for league %s: %w", leagueName, err)
	}

	// Build map from alias (uppercase) to DB team
	teamByAlias := make(map[string]*models.Team, len(dbTeams))
	for _, t := range dbTeams {
		teamByAlias[strings.ToUpper(t.Alias)] = t
	}

	var matched []MatchedTeam
	teamsByOddsBlazeID := make(map[string]*models.Team, len(reducedTeams))

	for _, rt := range reducedTeams {
		dbTeam, ok := teamByAlias[strings.ToUpper(rt.Abbreviation)]
		if !ok {
			return nil, nil, fmt.Errorf("unmatched OddsBlaze team: %s (abbreviation: %s) - no DB team with alias %q in league %s",
				rt.Name, rt.Abbreviation, rt.Abbreviation, leagueName)
		}
		matched = append(matched, MatchedTeam{
			OddsBlazeID: rt.OddsBlazeID,
			DBTeam:      dbTeam,
		})
		teamsByOddsBlazeID[rt.OddsBlazeID] = dbTeam
	}

	return matched, teamsByOddsBlazeID, nil
}

// teamDisplayNameKey identifies an individual by their team and display name.
type teamDisplayNameKey struct {
	TeamID      int
	DisplayName string
}

// teamJerseyNumberKey identifies an individual by their team and jersey number.
type teamJerseyNumberKey struct {
	TeamID       int
	JerseyNumber string
}

// normalizeName normalizes a display name for comparison by uppercasing and removing periods.
func normalizeName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, ".", ""))
}

// matchIndividuals matches reduced individuals against database individuals.
// First tries matching by team + display name, then falls back to team + jersey number.
// Loads all individuals for the league and uses roster data for team assignments.
func matchIndividuals(ctx context.Context, dbStore *store.Store, leagueName string, reducedIndividuals []reducer_oddsblaze.ReducedIndividual, teamsByOddsBlazeID map[string]*models.Team) ([]MatchedIndividual, error) {
	// Load all individuals for the league
	dbIndividuals, err := dbStore.GetIndividualsByLeague(ctx, leagueName)
	if err != nil {
		return nil, fmt.Errorf("failed to load individuals for league %s: %w", leagueName, err)
	}

	// Build individual ID → individual map
	individualByID := make(map[int]*models.Individual, len(dbIndividuals))
	for _, ind := range dbIndividuals {
		individualByID[ind.ID] = ind
	}

	// Load rosters for the league
	rosters, err := dbStore.GetRostersByLeague(ctx, leagueName)
	if err != nil {
		return nil, fmt.Errorf("failed to load rosters for league %s: %w", leagueName, err)
	}

	// Build lookup maps from roster data:
	//   (team_id, display_name) → individual
	//   (team_id, jersey_number) → individual
	individualByDisplayName := make(map[teamDisplayNameKey]*models.Individual)
	individualByJerseyNumber := make(map[teamJerseyNumberKey]*models.Individual)
	for _, roster := range rosters {
		for _, individualID := range roster.IndividualIDs {
			ind, ok := individualByID[int(individualID)]
			if !ok {
				continue
			}
			teamID := int(roster.TeamID)
			individualByDisplayName[teamDisplayNameKey{TeamID: teamID, DisplayName: normalizeName(ind.DisplayName)}] = ind
			individualByJerseyNumber[teamJerseyNumberKey{TeamID: teamID, JerseyNumber: ind.JerseyNumber}] = ind
		}
	}

	var matched []MatchedIndividual
	for _, ri := range reducedIndividuals {
		dbTeam, ok := teamsByOddsBlazeID[ri.TeamOddsBlazeID]
		if !ok {
			return nil, fmt.Errorf("individual %s (OddsBlaze ID: %s) references unknown team OddsBlaze ID %s",
				ri.Name, ri.OddsBlazeID, ri.TeamOddsBlazeID)
		}

		// Try matching by team + display name first
		dbInd, ok := individualByDisplayName[teamDisplayNameKey{TeamID: dbTeam.ID, DisplayName: normalizeName(ri.Name)}]
		if !ok {
			// Fall back to team + jersey number
			dbInd, ok = individualByJerseyNumber[teamJerseyNumberKey{TeamID: dbTeam.ID, JerseyNumber: ri.JerseyNumber}]
			if !ok {
				return nil, fmt.Errorf("unmatched OddsBlaze individual: %s #%s on %s (OddsBlaze ID: %s) - no DB individual with display name %q or jersey number %q on team %s %s (ID: %d) in league %s",
					ri.Name, ri.JerseyNumber, dbTeam.Alias, ri.OddsBlazeID, ri.Name, ri.JerseyNumber, dbTeam.Market, dbTeam.Name, dbTeam.ID, leagueName)
			}
		}
		matched = append(matched, MatchedIndividual{
			OddsBlazeID:  ri.OddsBlazeID,
			DBIndividual: dbInd,
		})
	}

	return matched, nil
}

// matchGames matches reduced games against database games by team pair and exact scheduled start time.
func matchGames(ctx context.Context, dbStore *store.Store, leagueName string, reducedGames []reducer_oddsblaze.ReducedGame, teamsByOddsBlazeID map[string]*models.Team) ([]MatchedGame, error) {
	dbGames, err := dbStore.GetGamesByLeague(ctx, leagueName)
	if err != nil {
		return nil, fmt.Errorf("failed to load games for league %s: %w", leagueName, err)
	}

	// Build map from (sorted team IDs, scheduled start time) to DB game
	gameByKey := make(map[gameKey]*models.Game, len(dbGames))
	for _, g := range dbGames {
		key := makeGameKey(int(g.ContenderIDA), int(g.ContenderIDB), g.ScheduledStartTime)
		gameByKey[key] = g
	}

	var matched []MatchedGame
	for _, rg := range reducedGames {
		homeTeam, ok := teamsByOddsBlazeID[rg.HomeTeamOddsBlazeID]
		if !ok {
			return nil, fmt.Errorf("game %s references unknown home team OddsBlaze ID %s", rg.OddsBlazeID, rg.HomeTeamOddsBlazeID)
		}
		awayTeam, ok := teamsByOddsBlazeID[rg.AwayTeamOddsBlazeID]
		if !ok {
			return nil, fmt.Errorf("game %s references unknown away team OddsBlaze ID %s", rg.OddsBlazeID, rg.AwayTeamOddsBlazeID)
		}

		key := makeGameKey(homeTeam.ID, awayTeam.ID, rg.ScheduledStartTime)
		dbGame, ok := gameByKey[key]
		if !ok {
			// Build a helpful error message showing available games for the same team pair
			teamIDA, teamIDB := homeTeam.ID, awayTeam.ID
			if teamIDA > teamIDB {
				teamIDA, teamIDB = teamIDB, teamIDA
			}
			var availableTimes []string
			for k, g := range gameByKey {
				if k.TeamIDA == teamIDA && k.TeamIDB == teamIDB {
					availableTimes = append(availableTimes, g.ScheduledStartTime.Format(time.RFC3339))
				}
			}
			sort.Strings(availableTimes)
			return nil, fmt.Errorf("unmatched OddsBlaze game: %s (%s vs %s at %s) - no DB game found for team pair (%d, %d) at that time. Available times: %v",
				rg.OddsBlazeID, homeTeam.Alias, awayTeam.Alias, rg.ScheduledStartTime.Format(time.RFC3339),
				homeTeam.ID, awayTeam.ID, availableTimes)
		}

		matched = append(matched, MatchedGame{
			OddsBlazeID: rg.OddsBlazeID,
			DBGame:      dbGame,
		})
	}

	return matched, nil
}
