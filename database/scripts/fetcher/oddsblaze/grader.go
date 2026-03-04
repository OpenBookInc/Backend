package oddsblaze

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/oddsblaze"
)

// GraderResponse is the response from the OddsBlaze grader endpoint
type GraderResponse struct {
	ID        string          `json:"id"` // Unreliable — the API returns an empty id in error cases (e.g. "Player not found")
	Event     GraderEvent     `json:"event"`
	Market    string          `json:"market"`
	Name      string          `json:"name"`
	Selection GraderSelection `json:"selection"`
	Player    GraderPlayer    `json:"player"`
	Result    string          `json:"result"`
	Message   string          `json:"message"`
}

// GraderEvent contains event information in the grader response
type GraderEvent struct {
	ID     string           `json:"id"`
	Teams  GraderEventTeams `json:"teams"`
	Status string           `json:"status"`
}

// GraderEventTeams contains the away and home teams in the grader response
type GraderEventTeams struct {
	Away GraderTeamScore `json:"away"`
	Home GraderTeamScore `json:"home"`
}

// GraderTeamScore contains a team's name and score in the grader response
type GraderTeamScore struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// GraderSelection contains the selection details in the grader response
type GraderSelection struct {
	Name string  `json:"name"`
	Side string  `json:"side"`
	Line float64 `json:"line"`
}

// GraderPlayer contains player information in the grader response
type GraderPlayer struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

// FetchGraderResult fetches and parses a grading result from OddsBlaze
func FetchGraderResult(client *oddsblaze.Client, oddsBlazeID string) (*GraderResponse, error) {
	rawData, err := client.GetGraderResult(oddsBlazeID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OddsBlaze grader result: %w", err)
	}

	var resp GraderResponse
	if err := json.Unmarshal(rawData, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse OddsBlaze grader response: %w", err)
	}

	return &resp, nil
}
