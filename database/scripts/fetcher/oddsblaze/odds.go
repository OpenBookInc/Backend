package oddsblaze

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/oddsblaze"
)

// OddsResponse is the top-level response from the OddsBlaze odds endpoint
type OddsResponse struct {
	Updated    string     `json:"updated"`
	League     League     `json:"league"`
	Sportsbook Sportsbook `json:"sportsbook"`
	Events     []Event    `json:"events"`
}

// League identifies the league in the response
type League struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Sport string `json:"sport"`
}

// Sportsbook identifies the sportsbook in the response
type Sportsbook struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Event represents a single game/event with its odds
type Event struct {
	ID    string     `json:"id"`
	Teams EventTeams `json:"teams"`
	Date  string     `json:"date"`
	Live  bool       `json:"live"`
	Odds  []Odd      `json:"odds"`
}

// EventTeams contains the away and home teams for an event
type EventTeams struct {
	Away TeamInfo `json:"away"`
	Home TeamInfo `json:"home"`
}

// TeamInfo contains team identification data
type TeamInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

// Odd represents a single odds entry
type Odd struct {
	ID     string      `json:"id"`
	Market string      `json:"market"`
	Name   string      `json:"name"`
	Price  string      `json:"price"`
	Main   bool        `json:"main"`
	Player *PlayerInfo `json:"player,omitempty"`
}

// PlayerInfo contains player identification data within an odds entry
type PlayerInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Position string   `json:"position"`
	Number   string   `json:"number"`
	Team     TeamInfo `json:"team"`
}

// FetchOdds fetches and parses odds data from OddsBlaze
func FetchOdds(client *oddsblaze.Client, sportsbook, league string, timestamp *string) (*OddsResponse, error) {
	rawData, err := client.GetOdds(sportsbook, league, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OddsBlaze odds: %w", err)
	}

	var resp OddsResponse
	if err := json.Unmarshal(rawData, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse OddsBlaze odds response: %w", err)
	}

	return &resp, nil
}
