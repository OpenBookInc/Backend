package nba

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openbook/population-scripts/client/sportradar"
)

// PlayByPlayResponse represents the complete NBA play-by-play API response
type PlayByPlayResponse struct {
	ID              string      `json:"id"`
	Title           string      `json:"title"`
	Status          string      `json:"status"`
	Coverage        string      `json:"coverage"`
	Scheduled       string      `json:"scheduled"`
	Duration        string      `json:"duration"`
	Attendance      int         `json:"attendance"`
	LeadChanges     int         `json:"lead_changes"`
	TimesTied       int         `json:"times_tied"`
	Clock           string      `json:"clock"`
	Quarter         int         `json:"quarter"`
	TrackOnCourt    bool        `json:"track_on_court"`
	Reference       string      `json:"reference"`
	EntryMode       string      `json:"entry_mode"`
	SrID            string      `json:"sr_id"`
	ClockDecimal    string      `json:"clock_decimal"`
	Broadcasts      []Broadcast `json:"broadcasts"`
	TimeZones       *TimeZones  `json:"time_zones"`
	Season          *Season     `json:"season"`
	Home            *Team       `json:"home"`
	Away            *Team       `json:"away"`
	Periods         []Period    `json:"periods"`
}

// String returns a formatted string representation of the play-by-play data
func (p *PlayByPlayResponse) String() string {
	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString("NBA PLAY-BY-PLAY RESPONSE\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("Game ID:      %s\n", p.ID))
	sb.WriteString(fmt.Sprintf("Title:        %s\n", p.Title))
	sb.WriteString(fmt.Sprintf("Status:       %s\n", p.Status))
	sb.WriteString(fmt.Sprintf("Scheduled:    %s\n", p.Scheduled))
	sb.WriteString(fmt.Sprintf("Duration:     %s\n", p.Duration))
	sb.WriteString(fmt.Sprintf("Attendance:   %d\n", p.Attendance))
	sb.WriteString(fmt.Sprintf("Lead Changes: %d\n", p.LeadChanges))
	sb.WriteString(fmt.Sprintf("Times Tied:   %d\n", p.TimesTied))
	sb.WriteString(fmt.Sprintf("Quarter:      %d\n", p.Quarter))
	sb.WriteString(fmt.Sprintf("Clock:        %s\n", p.Clock))

	if p.Season != nil {
		sb.WriteString(fmt.Sprintf("\nSeason:       %d %s (%s)\n", p.Season.Year, p.Season.Type, p.Season.Name))
	}

	if p.Home != nil {
		sb.WriteString("\n--- HOME TEAM ---\n")
		sb.WriteString(p.Home.String())
	}

	if p.Away != nil {
		sb.WriteString("\n--- AWAY TEAM ---\n")
		sb.WriteString(p.Away.String())
	}

	sb.WriteString("\n================================================================================\n")
	sb.WriteString(fmt.Sprintf("PERIODS (%d)\n", len(p.Periods)))
	sb.WriteString("================================================================================\n")

	totalEvents := 0
	for _, period := range p.Periods {
		totalEvents += len(period.Events)
	}

	for i, period := range p.Periods {
		sb.WriteString(fmt.Sprintf("\n--- PERIOD %d (Type: %s, %d events) ---\n", i+1, period.Type, len(period.Events)))
		if period.Scoring != nil {
			sb.WriteString(fmt.Sprintf("  Home: %d  Away: %d  Lead Changes: %d  Times Tied: %d\n",
				period.Scoring.Home.Points,
				period.Scoring.Away.Points,
				period.Scoring.LeadChanges,
				period.Scoring.TimesTied))
		}

		// Show first 5 events from each period as examples
		eventsToShow := 5
		if len(period.Events) < eventsToShow {
			eventsToShow = len(period.Events)
		}

		if eventsToShow > 0 {
			sb.WriteString(fmt.Sprintf("\n  Sample Events (showing %d of %d):\n", eventsToShow, len(period.Events)))
			for j := 0; j < eventsToShow; j++ {
				event := period.Events[j]
				sb.WriteString(fmt.Sprintf("    [%s] %s: %s (Score: %d-%d)\n",
					event.ID,
					event.Clock,
					event.Description,
					event.HomePoints,
					event.AwayPoints))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\nTotal Events: %d\n", totalEvents))
	sb.WriteString("================================================================================\n")

	return sb.String()
}

// JSON returns the complete JSON representation of the play-by-play data
func (p *PlayByPlayResponse) JSON() (string, error) {
	bytes, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal play-by-play response to JSON: %w", err)
	}
	return string(bytes), nil
}

// Broadcast represents broadcast information for the game
type Broadcast struct {
	Type   string `json:"type"`
	Locale string `json:"locale"`
	Network string `json:"network"`
}

// TimeZones represents timezone information for the game
type TimeZones struct {
	Venue string `json:"venue"`
	Home  string `json:"home"`
	Away  string `json:"away"`
}

// Season represents season information
type Season struct {
	ID   string `json:"id"`
	Year int    `json:"year"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// Team represents a team in the game
type Team struct {
	Name              string  `json:"name"`
	Alias             string  `json:"alias"`
	Market            string  `json:"market"`
	ID                string  `json:"id"`
	Points            int     `json:"points"`
	Bonus             bool    `json:"bonus"`
	SrID              string  `json:"sr_id"`
	RemainingTimeouts int     `json:"remaining_timeouts"`
	Reference         string  `json:"reference"`
	Record            *Record `json:"record"`
}

func (t *Team) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  %s %s (%s)\n", t.Market, t.Name, t.Alias))
	sb.WriteString(fmt.Sprintf("  ID:        %s\n", t.ID))
	sb.WriteString(fmt.Sprintf("  Points:    %d\n", t.Points))
	sb.WriteString(fmt.Sprintf("  Timeouts:  %d remaining\n", t.RemainingTimeouts))
	sb.WriteString(fmt.Sprintf("  Bonus:     %t\n", t.Bonus))
	if t.Record != nil {
		sb.WriteString(fmt.Sprintf("  Record:    %d-%d\n", t.Record.Wins, t.Record.Losses))
	}
	return sb.String()
}

// Record represents a team's win-loss record
type Record struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
}

// Period represents a quarter or overtime period in the game
type Period struct {
	Type     string   `json:"type"`
	ID       string   `json:"id"`
	Number   int      `json:"number"`
	Sequence int      `json:"sequence"`
	Scoring  *Scoring `json:"scoring"`
	Events   []Event  `json:"events"`
}

// Scoring represents scoring summary for a period
type Scoring struct {
	TimesTied   int        `json:"times_tied"`
	LeadChanges int        `json:"lead_changes"`
	Home        *TeamScore `json:"home"`
	Away        *TeamScore `json:"away"`
}

// TeamScore represents a team's scoring in a period
type TeamScore struct {
	Name      string `json:"name"`
	Market    string `json:"market"`
	ID        string `json:"id"`
	Points    int    `json:"points"`
	Reference string `json:"reference"`
}

// Event represents a single play-by-play event
type Event struct {
	ID           string       `json:"id"`
	Clock        string       `json:"clock"`
	Updated      string       `json:"updated"`
	Description  string       `json:"description"`
	WallClock    string       `json:"wall_clock"`
	Sequence     int64        `json:"sequence"`
	HomePoints   int          `json:"home_points"`
	AwayPoints   int          `json:"away_points"`
	ClockDecimal string       `json:"clock_decimal"`
	Created      string       `json:"created"`
	Number       int          `json:"number"`
	EventType    string       `json:"event_type"`
	Attempt      string       `json:"attempt"`       // For free throws: "1 of 2", etc.
	TurnoverType string       `json:"turnover_type"` // For turnovers
	Attribution  *Attribution `json:"attribution"`
	Location     *Location    `json:"location"`
	Possession   *Possession  `json:"possession"`
	OnCourt      *OnCourt     `json:"on_court"`
	Statistics   []Statistic  `json:"statistics"`
	Qualifiers   []Qualifier  `json:"qualifiers"`
}

// Attribution represents which team is attributed with an event
type Attribution struct {
	Name       string `json:"name"`
	Market     string `json:"market"`
	ID         string `json:"id"`
	TeamBasket string `json:"team_basket"` // "left" or "right"
	SrID       string `json:"sr_id"`
	Reference  string `json:"reference"`
}

// Location represents court coordinates for an event
type Location struct {
	CoordX     int    `json:"coord_x"`
	CoordY     int    `json:"coord_y"`
	ActionArea string `json:"action_area"`
}

// Possession represents which team has possession
type Possession struct {
	Name      string `json:"name"`
	Market    string `json:"market"`
	ID        string `json:"id"`
	SrID      string `json:"sr_id"`
	Reference string `json:"reference"`
}

// OnCourt represents the players on the court for both teams
type OnCourt struct {
	Home *TeamOnCourt `json:"home"`
	Away *TeamOnCourt `json:"away"`
}

// TeamOnCourt represents one team's players on the court
type TeamOnCourt struct {
	Name      string   `json:"name"`
	Market    string   `json:"market"`
	ID        string   `json:"id"`
	SrID      string   `json:"sr_id"`
	Reference string   `json:"reference"`
	Players   []Player `json:"players"`
}

// Player represents a player on the court
type Player struct {
	FullName     string `json:"full_name"`
	JerseyNumber string `json:"jersey_number"`
	ID           string `json:"id"`
	SrID         string `json:"sr_id"`
	Reference    string `json:"reference"`
}

// Statistic represents a single statistic entry for an event
type Statistic struct {
	Type           string      `json:"type"`
	Made           bool        `json:"made"`           // For field goals and free throws
	ShotType       string      `json:"shot_type"`      // For field goals: "jump shot", "layup", etc.
	ShotTypeDesc   string      `json:"shot_type_desc"` // For field goals: "pullup", "driving", etc.
	ThreePointShot bool        `json:"three_point_shot"`
	ShotDistance   float64     `json:"shot_distance"`
	FreeThrowType  string      `json:"free_throw_type"` // For free throws: "regular", "technical", "flagrant"
	Points         int         `json:"points"`          // Points scored
	ReboundType    string      `json:"rebound_type"`    // "defensive" or "offensive"
	TurnoverType   string      `json:"turnover_type"`   // Type of turnover
	Team           *TeamRef    `json:"team"`
	Player         *PlayerRef  `json:"player"`
}

// TeamRef represents a team reference in statistics
type TeamRef struct {
	Name      string `json:"name"`
	Market    string `json:"market"`
	ID        string `json:"id"`
	SrID      string `json:"sr_id"`
	Reference string `json:"reference"`
}

// PlayerRef represents a player reference in statistics
type PlayerRef struct {
	FullName     string `json:"full_name"`
	JerseyNumber string `json:"jersey_number"`
	ID           string `json:"id"`
	SrID         string `json:"sr_id"`
	Reference    string `json:"reference"`
}

// Qualifier represents additional qualifiers for an event
type Qualifier struct {
	Qualifier string `json:"qualifier"`
}

// FetchNBAPlayByPlay fetches play-by-play data for a specific NBA game
func FetchNBAPlayByPlay(apiClient *sportradar.Client, gameID string) (*PlayByPlayResponse, error) {
	data, err := apiClient.GetNBAPlayByPlay(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NBA play-by-play data: %w", err)
	}

	var response PlayByPlayResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse NBA play-by-play response: %w", err)
	}

	return &response, nil
}
