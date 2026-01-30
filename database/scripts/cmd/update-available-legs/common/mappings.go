package common

import (
	"fmt"
	"strings"

	"github.com/openbook/population-scripts/fetcher"
	models "github.com/openbook/shared/models"
)

// DatabaseToPropMappings holds the mappings from database entities to Sportradar player props IDs.
type DatabaseToPropMappings struct {
	// GameMappings maps Game pointers to Sportradar sport event IDs (e.g., "sr:sport_event:12345")
	GameMappings map[*models.Game]string
	// TeamMappings maps Team pointers to Sportradar competitor IDs (e.g., "sr:competitor:123")
	TeamMappings map[*models.Team]string
	// IndividualMappings maps Individual pointers to Sportradar player IDs (e.g., "sr:player:12345")
	IndividualMappings map[*models.Individual]string
}

// String returns a formatted summary of the mappings.
func (m *DatabaseToPropMappings) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Games mapped: %d\n", len(m.GameMappings)))
	sb.WriteString(fmt.Sprintf("Teams mapped: %d\n", len(m.TeamMappings)))
	sb.WriteString(fmt.Sprintf("Individuals mapped: %d\n", len(m.IndividualMappings)))
	return sb.String()
}

// BuildMappings maps database entities to Sportradar player props IDs.
// Game mappings come from the sport event mappings API response.
// Team and individual mappings come from the database vendor_unified_id fields.
// Games and teams that cannot be mapped cause a fatal error (returned as error).
// Individuals that cannot be mapped are returned in the unmappedIndividuals slice.
func BuildMappings(
	games []*models.Game,
	rosters []*models.Roster,
	sportEventMappings *fetcher.PlayerPropsMappingResponse,
) (*DatabaseToPropMappings, []*models.Individual, error) {
	// Build lookup map from external_id → sr:id for sport events
	sportEventLookup := buildLookup(sportEventMappings.Mappings)

	result := &DatabaseToPropMappings{
		GameMappings:       make(map[*models.Game]string, len(games)),
		TeamMappings:       make(map[*models.Team]string),
		IndividualMappings: make(map[*models.Individual]string),
	}

	// Map games using API sport event mappings
	for _, game := range games {
		propID, ok := sportEventLookup[game.VendorID]
		if !ok {
			return nil, nil, fmt.Errorf("game %d (vendor_id: %s) has no player props sport event mapping", game.ID, game.VendorID)
		}
		result.GameMappings[game] = propID
	}

	// Map teams using database vendor_unified_id (from games, deduplicated via pointer identity)
	for _, game := range games {
		if _, exists := result.TeamMappings[game.TeamA]; !exists {
			if game.TeamA.VendorUnifiedID == "" {
				return nil, nil, fmt.Errorf("team %d %s %s (vendor_id: %s) has no vendor_unified_id in database",
					game.TeamA.ID, game.TeamA.Market, game.TeamA.Name, game.TeamA.VendorID)
			}
			result.TeamMappings[game.TeamA] = game.TeamA.VendorUnifiedID
		}
		if _, exists := result.TeamMappings[game.TeamB]; !exists {
			if game.TeamB.VendorUnifiedID == "" {
				return nil, nil, fmt.Errorf("team %d %s %s (vendor_id: %s) has no vendor_unified_id in database",
					game.TeamB.ID, game.TeamB.Market, game.TeamB.Name, game.TeamB.VendorID)
			}
			result.TeamMappings[game.TeamB] = game.TeamB.VendorUnifiedID
		}
	}

	// Map individuals from rosters using database vendor_unified_id
	var unmappedIndividuals []*models.Individual
	for _, roster := range rosters {
		for _, player := range roster.Players {
			if _, exists := result.IndividualMappings[player]; exists {
				continue
			}
			if !player.HasVendorUnifiedID() {
				unmappedIndividuals = append(unmappedIndividuals, player)
				continue
			}
			vendorUnifiedID, _ := player.VendorUnifiedID()
			result.IndividualMappings[player] = vendorUnifiedID
		}
	}

	return result, unmappedIndividuals, nil
}

// buildLookup converts a slice of mappings into a map from external_id to sr:id.
func buildLookup(mappings []fetcher.PlayerPropsMapping) map[string]string {
	lookup := make(map[string]string, len(mappings))
	for _, m := range mappings {
		lookup[m.ExternalID] = m.ID
	}
	return lookup
}
