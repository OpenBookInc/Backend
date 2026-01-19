package nba

import (
	"fmt"
	"strings"

	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
)

// =============================================================================
// NBA Play-by-Play Decorator
// =============================================================================
// This package decorates (enriches) raw Sportradar API responses with data that
// is missing from the API but can be derived from other fields.
//
// The decorator sits between the fetcher and persister in the data flow:
//   fetch (raw API) → decorate (enriched) → persist (to database)
//
// Current decorations:
// - Heave block statistics: Sportradar returns "heave" events (desperation shots)
//   without a statistics array. When a heave is blocked, the block information
//   is only in the description text. This decorator extracts and adds the block
//   statistic so the persister can process it normally.
// =============================================================================

// DecoratePlayByPlay decorates a play-by-play response with derived data.
// It returns a modified copy of the response with additional statistics
// that were missing from the raw API response.
func DecoratePlayByPlay(pbp *fetcher_nba.PlayByPlayResponse) *fetcher_nba.PlayByPlayResponse {
	// Create a shallow copy - we'll only modify the Events slices
	decorated := *pbp

	// Process each period
	decoratedPeriods := make([]fetcher_nba.Period, len(pbp.Periods))
	for i, period := range pbp.Periods {
		decoratedPeriods[i] = decoratePeriod(&period)
	}
	decorated.Periods = decoratedPeriods

	return &decorated
}

// decoratePeriod decorates a single period's events.
func decoratePeriod(period *fetcher_nba.Period) fetcher_nba.Period {
	decorated := *period

	decoratedEvents := make([]fetcher_nba.Event, len(period.Events))
	for i, event := range period.Events {
		decoratedEvents[i] = decorateEvent(&event)
	}
	decorated.Events = decoratedEvents

	return decorated
}

// decorateEvent decorates a single event with derived statistics.
func decorateEvent(event *fetcher_nba.Event) fetcher_nba.Event {
	decorated := *event

	// Add heave block statistic if this is a blocked heave missing block data
	if isBlockedHeaveWithoutBlockStatistic(event) {
		blockStat, err := extractHeaveBlockStatistic(event)
		if err != nil {
			// Log warning but don't fail - fault tolerance for decoration
			// The persister will skip events with no statistics
			fmt.Printf("Warning: failed to extract heave block statistic (event_id: %s): %v\n", event.ID, err)
			return decorated
		}

		// Copy existing statistics and append the block
		decoratedStats := make([]fetcher_nba.Statistic, len(event.Statistics), len(event.Statistics)+1)
		copy(decoratedStats, event.Statistics)
		decoratedStats = append(decoratedStats, *blockStat)
		decorated.Statistics = decoratedStats
	}

	return decorated
}

// =============================================================================
// Heave Block Extraction
// =============================================================================
// Sportradar NBA API returns "heave" events (desperation end-of-quarter shots)
// with a different structure than regular plays. When a heave is blocked, the
// block information is only present in the description text, not in a statistics
// array.
//
// Example heave event:
//   event_type: "heave"
//   description: "Chet Holmgren blocks Mavericks heave shot"
//   attribution: { id: "dallas-team-id", ... }  // Team that took the shot
//   on_court: { away: { players: [...] }, home: { players: [...] } }
// =============================================================================

// heaveBlockSuffix is the suffix that indicates a blocked heave in the description.
const heaveBlockSuffix = " heave shot"

// heaveBlockMarker is the marker that separates the blocker name from the team name.
const heaveBlockMarker = " blocks "

// isBlockedHeaveWithoutBlockStatistic returns true if the event is a blocked heave
// that needs a block statistic to be added (i.e., the API didn't include one).
// Blocked heave descriptions follow the pattern: "{PlayerName} blocks {TeamName} heave shot"
func isBlockedHeaveWithoutBlockStatistic(event *fetcher_nba.Event) bool {
	if event.EventType != "heave" {
		return false
	}
	if !strings.Contains(event.Description, heaveBlockMarker) ||
		!strings.HasSuffix(event.Description, heaveBlockSuffix) {
		return false
	}
	// Don't decorate if a block statistic with a player already exists
	for _, stat := range event.Statistics {
		if stat.Type == "block" && stat.Player != nil {
			return false
		}
	}
	return true
}

// extractHeaveBlockStatistic extracts a block statistic from a blocked heave event.
// It parses the blocker's name from the description and looks up their info
// from the on_court players.
func extractHeaveBlockStatistic(event *fetcher_nba.Event) (*fetcher_nba.Statistic, error) {
	// Parse blocker name from description
	blockerName, err := parseBlockerNameFromDescription(event.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to parse blocker name: %w", err)
	}

	// Find blocker in on_court data
	blocker, blockerTeam, err := findBlockerInOnCourt(event, blockerName)
	if err != nil {
		return nil, err
	}

	// Create block statistic matching the fetcher struct format
	stat := &fetcher_nba.Statistic{
		Type: "block",
		Player: &fetcher_nba.PlayerRef{
			FullName:     blocker.FullName,
			JerseyNumber: blocker.JerseyNumber,
			ID:           blocker.ID,
			SrID:         blocker.SrID,
			Reference:    blocker.Reference,
		},
		Team: &fetcher_nba.TeamRef{
			Name:      blockerTeam.Name,
			Market:    blockerTeam.Market,
			ID:        blockerTeam.ID,
			SrID:      blockerTeam.SrID,
			Reference: blockerTeam.Reference,
		},
	}

	return stat, nil
}

// findBlockerInOnCourt finds the blocker player in the on_court data.
// The blocker is on the team that is NOT the attribution team (attribution = shooting team).
func findBlockerInOnCourt(event *fetcher_nba.Event, blockerName string) (*fetcher_nba.Player, *fetcher_nba.TeamOnCourt, error) {
	if event.OnCourt == nil {
		return nil, nil, fmt.Errorf("heave event missing on_court data")
	}

	if event.Attribution == nil {
		return nil, nil, fmt.Errorf("heave event missing attribution data")
	}

	// The blocker is on the team that is NOT the attribution team
	var blockingTeam *fetcher_nba.TeamOnCourt
	if event.OnCourt.Home != nil && event.OnCourt.Home.ID != event.Attribution.ID {
		blockingTeam = event.OnCourt.Home
	} else if event.OnCourt.Away != nil && event.OnCourt.Away.ID != event.Attribution.ID {
		blockingTeam = event.OnCourt.Away
	} else {
		return nil, nil, fmt.Errorf("could not determine blocking team from on_court data")
	}

	// Find the player by name
	for i := range blockingTeam.Players {
		if blockingTeam.Players[i].FullName == blockerName {
			return &blockingTeam.Players[i], blockingTeam, nil
		}
	}

	return nil, nil, fmt.Errorf("blocker %q not found in on_court players", blockerName)
}

// parseBlockerNameFromDescription extracts the blocker's name from a heave block description.
// Expected pattern: "{PlayerName} blocks {TeamName} heave shot"
func parseBlockerNameFromDescription(description string) (string, error) {
	idx := strings.Index(description, heaveBlockMarker)
	if idx == -1 {
		return "", fmt.Errorf("description does not contain '%s'", heaveBlockMarker)
	}

	blockerName := description[:idx]
	if blockerName == "" {
		return "", fmt.Errorf("blocker name is empty")
	}

	return blockerName, nil
}
