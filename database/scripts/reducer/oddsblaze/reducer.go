package oddsblaze

import (
	"fmt"
	"time"

	fetcher_oddsblaze "github.com/openbook/population-scripts/fetcher/oddsblaze"
)

// ReducedTeam is a unique team extracted from OddsBlaze data
type ReducedTeam struct {
	OddsBlazeID  string
	Name         string
	Abbreviation string
}

// ReducedIndividual is a unique player extracted from OddsBlaze data
type ReducedIndividual struct {
	OddsBlazeID     string
	Name            string
	Position        string
	JerseyNumber    string
	TeamOddsBlazeID string
}

// ReducedGame is a unique game extracted from OddsBlaze data
type ReducedGame struct {
	OddsBlazeID        string
	HomeTeamOddsBlazeID string
	AwayTeamOddsBlazeID string
	ScheduledStartTime  time.Time
}

// ReducedEntities holds all unique entities extracted from an OddsBlaze response
type ReducedEntities struct {
	Teams       []ReducedTeam
	Individuals []ReducedIndividual
	Games       []ReducedGame
}

// ReduceOddsResponse extracts unique teams, individuals, and games from an OddsBlaze response
func ReduceOddsResponse(resp *fetcher_oddsblaze.OddsResponse) (*ReducedEntities, error) {
	teamsSeen := make(map[string]bool)
	individualsSeen := make(map[string]bool)
	gamesSeen := make(map[string]bool)

	var teams []ReducedTeam
	var individuals []ReducedIndividual
	var games []ReducedGame

	for _, event := range resp.Events {
		// Collect home team
		if !teamsSeen[event.Teams.Home.ID] {
			teamsSeen[event.Teams.Home.ID] = true
			teams = append(teams, ReducedTeam{
				OddsBlazeID:  event.Teams.Home.ID,
				Name:         event.Teams.Home.Name,
				Abbreviation: event.Teams.Home.Abbreviation,
			})
		}

		// Collect away team
		if !teamsSeen[event.Teams.Away.ID] {
			teamsSeen[event.Teams.Away.ID] = true
			teams = append(teams, ReducedTeam{
				OddsBlazeID:  event.Teams.Away.ID,
				Name:         event.Teams.Away.Name,
				Abbreviation: event.Teams.Away.Abbreviation,
			})
		}

		// Collect game
		if !gamesSeen[event.ID] {
			gamesSeen[event.ID] = true
			scheduledStartTime, err := time.Parse(time.RFC3339, event.Date)
			if err != nil {
				return nil, fmt.Errorf("failed to parse event date %q for event %s: %w", event.Date, event.ID, err)
			}
			games = append(games, ReducedGame{
				OddsBlazeID:        event.ID,
				HomeTeamOddsBlazeID: event.Teams.Home.ID,
				AwayTeamOddsBlazeID: event.Teams.Away.ID,
				ScheduledStartTime:  scheduledStartTime,
			})
		}

		// Collect individuals from odds
		for _, odd := range event.Odds {
			if odd.Player == nil {
				continue
			}
			if !individualsSeen[odd.Player.ID] {
				individualsSeen[odd.Player.ID] = true
				individuals = append(individuals, ReducedIndividual{
					OddsBlazeID:     odd.Player.ID,
					Name:            odd.Player.Name,
					Position:        odd.Player.Position,
					JerseyNumber:    odd.Player.Number,
					TeamOddsBlazeID: odd.Player.Team.ID,
				})
			}
		}
	}

	return &ReducedEntities{
		Teams:       teams,
		Individuals: individuals,
		Games:       games,
	}, nil
}
