package nfl

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openbook/population-scripts/client"
)

// PlayByPlayResponse represents the complete NFL play-by-play API response
type PlayByPlayResponse struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Scheduled  string     `json:"scheduled"`
	Attendance int        `json:"attendance"`
	EntryMode  string     `json:"entry_mode"`
	Weather    *Weather   `json:"weather"`
	Coin_Toss  *CoinToss  `json:"coin_toss"`
	Summary    *Summary   `json:"summary"`
	Situation  *Situation `json:"situation"`
	LastEvent  *Event     `json:"last_event"`
	Home       *Team      `json:"home"`
	Away       *Team      `json:"away"`
	Periods    []Period   `json:"periods"`
}

func (p *PlayByPlayResponse) String() string {
	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString("PLAY-BY-PLAY RESPONSE\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("ID:         %s\n", p.ID))
	sb.WriteString(fmt.Sprintf("Status:     %s\n", p.Status))
	sb.WriteString(fmt.Sprintf("Scheduled:  %s\n", p.Scheduled))
	sb.WriteString(fmt.Sprintf("Attendance: %d\n", p.Attendance))
	sb.WriteString(fmt.Sprintf("Entry Mode: %s\n", p.EntryMode))

	if p.Weather != nil {
		sb.WriteString("\n")
		sb.WriteString(p.Weather.String())
	}

	if p.Coin_Toss != nil {
		sb.WriteString("\n")
		sb.WriteString(p.Coin_Toss.String())
	}

	if p.Summary != nil {
		sb.WriteString("\n")
		sb.WriteString(p.Summary.String())
	}

	if p.Situation != nil {
		sb.WriteString("\n")
		sb.WriteString(p.Situation.String())
	}

	if p.LastEvent != nil {
		sb.WriteString("\n--- LAST EVENT ---\n")
		sb.WriteString(p.LastEvent.String())
	}

	if p.Home != nil {
		sb.WriteString("\n")
		sb.WriteString("--- HOME TEAM ---\n")
		sb.WriteString(p.Home.String())
	}

	if p.Away != nil {
		sb.WriteString("\n")
		sb.WriteString("--- AWAY TEAM ---\n")
		sb.WriteString(p.Away.String())
	}

	sb.WriteString("\n================================================================================\n")
	sb.WriteString(fmt.Sprintf("PERIODS (%d)\n", len(p.Periods)))
	sb.WriteString("================================================================================\n")
	for i, period := range p.Periods {
		sb.WriteString(fmt.Sprintf("\n--- PERIOD %d ---\n", i+1))
		sb.WriteString(period.String())
	}

	return sb.String()
}

// Weather represents weather conditions for the game
type Weather struct {
	Condition string `json:"condition"`
	Humidity  int    `json:"humidity"`
	Temp      int    `json:"temp"`
	Wind      *Wind  `json:"wind"`
}

func (w *Weather) String() string {
	var sb strings.Builder
	sb.WriteString("WEATHER:\n")
	sb.WriteString(fmt.Sprintf("  Condition: %s\n", w.Condition))
	sb.WriteString(fmt.Sprintf("  Humidity:  %d%%\n", w.Humidity))
	sb.WriteString(fmt.Sprintf("  Temp:      %d°F\n", w.Temp))
	if w.Wind != nil {
		sb.WriteString(fmt.Sprintf("  Wind:      %s\n", w.Wind.String()))
	}
	return sb.String()
}

// Wind represents wind conditions
type Wind struct {
	Speed     int    `json:"speed"`
	Direction string `json:"direction"`
}

func (w *Wind) String() string {
	return fmt.Sprintf("%d mph %s", w.Speed, w.Direction)
}

// CoinToss represents the coin toss result
type CoinToss struct {
	HomeCall  string `json:"home_call"`
	Winner    string `json:"winner"`
	Direction string `json:"direction"`
	Decision  string `json:"decision"`
}

func (c *CoinToss) String() string {
	var sb strings.Builder
	sb.WriteString("COIN TOSS:\n")
	sb.WriteString(fmt.Sprintf("  Home Call: %s\n", c.HomeCall))
	sb.WriteString(fmt.Sprintf("  Winner:    %s\n", c.Winner))
	sb.WriteString(fmt.Sprintf("  Direction: %s\n", c.Direction))
	sb.WriteString(fmt.Sprintf("  Decision:  %s\n", c.Decision))
	return sb.String()
}

// Summary represents the game scoring summary
type Summary struct {
	Season *SeasonInfo  `json:"season"`
	Week   *WeekInfo    `json:"week"`
	Venue  *Venue       `json:"venue"`
	Home   *TeamSummary `json:"home"`
	Away   *TeamSummary `json:"away"`
}

func (s *Summary) String() string {
	var sb strings.Builder
	sb.WriteString("SUMMARY:\n")
	if s.Season != nil {
		sb.WriteString(fmt.Sprintf("  Season: %d %s\n", s.Season.Year, s.Season.Type))
	}
	if s.Week != nil {
		sb.WriteString(fmt.Sprintf("  Week:   %d (%s)\n", s.Week.Sequence, s.Week.Title))
	}
	if s.Venue != nil {
		sb.WriteString("\n")
		sb.WriteString(s.Venue.String())
	}
	if s.Home != nil {
		sb.WriteString("\n  HOME SUMMARY:\n")
		sb.WriteString(s.Home.String())
	}
	if s.Away != nil {
		sb.WriteString("\n  AWAY SUMMARY:\n")
		sb.WriteString(s.Away.String())
	}
	return sb.String()
}

// SeasonInfo represents season information
type SeasonInfo struct {
	ID   string `json:"id"`
	Year int    `json:"year"`
	Type string `json:"type"`
	Name string `json:"name"`
}

func (s *SeasonInfo) String() string {
	return fmt.Sprintf("%d %s (%s)", s.Year, s.Type, s.Name)
}

// WeekInfo represents week information
type WeekInfo struct {
	ID       string `json:"id"`
	Sequence int    `json:"sequence"`
	Title    string `json:"title"`
}

func (w *WeekInfo) String() string {
	return fmt.Sprintf("Week %d: %s", w.Sequence, w.Title)
}

// Venue represents the stadium/venue information
type Venue struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	City     string    `json:"city"`
	State    string    `json:"state"`
	Country  string    `json:"country"`
	Zip      string    `json:"zip"`
	Address  string    `json:"address"`
	Capacity int       `json:"capacity"`
	Surface  string    `json:"surface"`
	RoofType string    `json:"roof_type"`
	Location *Location `json:"location"`
}

func (v *Venue) String() string {
	var sb strings.Builder
	sb.WriteString("  VENUE:\n")
	sb.WriteString(fmt.Sprintf("    Name:     %s\n", v.Name))
	sb.WriteString(fmt.Sprintf("    Address:  %s\n", v.Address))
	sb.WriteString(fmt.Sprintf("    City:     %s, %s %s\n", v.City, v.State, v.Zip))
	sb.WriteString(fmt.Sprintf("    Country:  %s\n", v.Country))
	sb.WriteString(fmt.Sprintf("    Capacity: %d\n", v.Capacity))
	sb.WriteString(fmt.Sprintf("    Surface:  %s\n", v.Surface))
	sb.WriteString(fmt.Sprintf("    Roof:     %s\n", v.RoofType))
	if v.Location != nil {
		sb.WriteString(fmt.Sprintf("    Location: %s\n", v.Location.String()))
	}
	return sb.String()
}

// Location represents GPS coordinates
type Location struct {
	Lat string `json:"lat"`
	Lng string `json:"lng"`
}

func (l *Location) String() string {
	return fmt.Sprintf("(%s, %s)", l.Lat, l.Lng)
}

// TeamSummary represents a team's scoring summary
type TeamSummary struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Market  string    `json:"market"`
	Alias   string    `json:"alias"`
	Points  int       `json:"points"`
	Scoring []Scoring `json:"scoring"`
}

func (t *TeamSummary) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    %s %s (%s)\n", t.Market, t.Name, t.Alias))
	sb.WriteString(fmt.Sprintf("    Total Points: %d\n", t.Points))
	if len(t.Scoring) > 0 {
		sb.WriteString("    Scoring by Quarter:\n")
		for _, s := range t.Scoring {
			sb.WriteString(fmt.Sprintf("      Q%d: %d points\n", s.Sequence, s.Points))
		}
	}
	return sb.String()
}

// Scoring represents scoring by period/quarter
type Scoring struct {
	Sequence int `json:"sequence"`
	Points   int `json:"points"`
}

func (s *Scoring) String() string {
	return fmt.Sprintf("Q%d: %d", s.Sequence, s.Points)
}

// Situation represents the current game situation
type Situation struct {
	Clock      string         `json:"clock"`
	Down       int            `json:"down"`
	YFD        int            `json:"yfd"`
	Possession *Possession    `json:"possession"`
	Location   *FieldLocation `json:"location"`
}

func (s *Situation) String() string {
	var sb strings.Builder
	sb.WriteString("SITUATION:\n")
	sb.WriteString(fmt.Sprintf("  Clock: %s\n", s.Clock))
	sb.WriteString(fmt.Sprintf("  Down:  %d\n", s.Down))
	sb.WriteString(fmt.Sprintf("  YFD:   %d\n", s.YFD))
	if s.Possession != nil {
		sb.WriteString(fmt.Sprintf("  Possession: %s\n", s.Possession.String()))
	}
	if s.Location != nil {
		sb.WriteString(fmt.Sprintf("  Location: %s\n", s.Location.String()))
	}
	return sb.String()
}

// Possession represents which team has possession
type Possession struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Market string `json:"market"`
	Alias  string `json:"alias"`
}

func (p *Possession) String() string {
	return fmt.Sprintf("%s %s (%s)", p.Market, p.Name, p.Alias)
}

// FieldLocation represents the field position
type FieldLocation struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Market   string `json:"market"`
	Alias    string `json:"alias"`
	Yardline int    `json:"yardline"`
}

func (f *FieldLocation) String() string {
	return fmt.Sprintf("%s %d", f.Alias, f.Yardline)
}

// Team represents a team in the game
type Team struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Market               string `json:"market"`
	Alias                string `json:"alias"`
	SRId                 string `json:"sr_id"`
	Used_Timeouts        int    `json:"used_timeouts"`
	Remaining_Timeouts   int    `json:"remaining_timeouts"`
	Points               int    `json:"points"`
	Used_Challenges      int    `json:"used_challenges"`
	Remaining_Challenges int    `json:"remaining_challenges"`
}

func (t *Team) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  ID:     %s\n", t.ID))
	sb.WriteString(fmt.Sprintf("  Name:   %s %s (%s)\n", t.Market, t.Name, t.Alias))
	sb.WriteString(fmt.Sprintf("  Points: %d\n", t.Points))
	sb.WriteString(fmt.Sprintf("  Timeouts: %d used, %d remaining\n", t.Used_Timeouts, t.Remaining_Timeouts))
	sb.WriteString(fmt.Sprintf("  Challenges: %d used, %d remaining\n", t.Used_Challenges, t.Remaining_Challenges))
	return sb.String()
}

// Period represents a quarter/period in the game
type Period struct {
	ID         string  `json:"id"`
	Number     int     `json:"number"`
	Sequence   int     `json:"sequence"`
	PeriodType string  `json:"period_type"`
	PBP        []Drive `json:"pbp"`
}

func (p *Period) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Period ID:   %s\n", p.ID))
	sb.WriteString(fmt.Sprintf("Number:      %d\n", p.Number))
	sb.WriteString(fmt.Sprintf("Sequence:    %d\n", p.Sequence))
	sb.WriteString(fmt.Sprintf("Period Type: %s\n", p.PeriodType))
	sb.WriteString(fmt.Sprintf("Drives:      %d\n", len(p.PBP)))

	for i, drive := range p.PBP {
		sb.WriteString(fmt.Sprintf("\n  --- Drive %d ---\n", i+1))
		sb.WriteString(drive.String())
	}

	return sb.String()
}

// Event represents a game event (play, timeout, etc.)
type Event struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	PlayType       string         `json:"play_type"`
	Sequence       float64        `json:"sequence"`
	ReferenceID    string         `json:"reference_id"`
	Clock          string         `json:"clock"`
	WallClock      string         `json:"wall_clock"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
	AltDescription string         `json:"alt_description"`
	Description    string         `json:"description"`
	HomePoints     int            `json:"home_points"`
	AwayPoints     int            `json:"away_points"`
	PlayClock      int            `json:"play_clock"`
	Source         string         `json:"source"`
	Official       bool           `json:"official"`
	Score          *Score         `json:"score"`
	StartSituation *PlaySituation `json:"start_situation"`
	EndSituation   *PlaySituation `json:"end_situation"`
	Statistics     []Statistic    `json:"statistics"`
	Details        []Detail       `json:"details"`
}

func (e *Event) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("      ID:          %s\n", e.ID))
	sb.WriteString(fmt.Sprintf("      Type:        %s\n", e.Type))
	if e.PlayType != "" {
		sb.WriteString(fmt.Sprintf("      Play Type:   %s\n", e.PlayType))
	}
	sb.WriteString(fmt.Sprintf("      Sequence:    %.1f\n", e.Sequence))
	sb.WriteString(fmt.Sprintf("      Clock:       %s\n", e.Clock))
	sb.WriteString(fmt.Sprintf("      Wall Clock:  %s\n", e.WallClock))
	sb.WriteString(fmt.Sprintf("      Created At:  %s\n", e.CreatedAt))
	sb.WriteString(fmt.Sprintf("      Updated At:  %s\n", e.UpdatedAt))
	sb.WriteString(fmt.Sprintf("      Description: %s\n", e.Description))
	if e.AltDescription != "" {
		sb.WriteString(fmt.Sprintf("      Alt Desc:    %s\n", e.AltDescription))
	}
	sb.WriteString(fmt.Sprintf("      Score:       Home %d - Away %d\n", e.HomePoints, e.AwayPoints))
	sb.WriteString(fmt.Sprintf("      Official:    %t\n", e.Official))

	if e.Score != nil {
		sb.WriteString(fmt.Sprintf("      Scoring:     %s\n", e.Score.String()))
	}

	if e.StartSituation != nil {
		sb.WriteString("\n      START SITUATION:\n")
		sb.WriteString(e.StartSituation.String())
	}

	if e.EndSituation != nil {
		sb.WriteString("\n      END SITUATION:\n")
		sb.WriteString(e.EndSituation.String())
	}

	if len(e.Statistics) > 0 {
		sb.WriteString(fmt.Sprintf("\n      STATISTICS (%d):\n", len(e.Statistics)))
		for _, stat := range e.Statistics {
			sb.WriteString(stat.String())
		}
	}

	if len(e.Details) > 0 {
		sb.WriteString(fmt.Sprintf("\n      DETAILS (%d):\n", len(e.Details)))
		for _, detail := range e.Details {
			sb.WriteString(detail.String())
		}
	}

	return sb.String()
}

// Score represents scoring information for an event
type Score struct {
	Sequence   int    `json:"sequence"`
	Clock      string `json:"clock"`
	Points     int    `json:"points"`
	Type       string `json:"type"`
	AwayPoints int    `json:"away_points"`
	HomePoints int    `json:"home_points"`
}

func (s *Score) String() string {
	return fmt.Sprintf("%s (%d pts) - Home: %d, Away: %d", s.Type, s.Points, s.HomePoints, s.AwayPoints)
}

// Statistic represents a single statistic entry for a play
type Statistic struct {
	StatType      string     `json:"stat_type"`
	Category      string     `json:"category"`
	Attempt       float64    `json:"attempt"`
	Yards         float64    `json:"yards"`
	NetYards      float64    `json:"net_yards"`
	Touchback     float64    `json:"touchback"`
	TouchbackPct  float64    `json:"touchback_pct"`
	OnsideAttempt float64    `json:"onside_attempt"`
	OnsideSuccess float64    `json:"onside_success"`
	SquibKick     float64    `json:"squib_kick"`
	Complete      float64    `json:"complete"`
	Incomplete    float64    `json:"incomplete"`
	Interception  float64    `json:"interception"`
	Sack          float64    `json:"sack"`
	SackYards     float64    `json:"sack_yards"`
	Touchdown     float64    `json:"touchdown"`
	FirstDown     float64    `json:"firstdown"`
	GoalToGo      float64    `json:"goaltogo"`
	Inside20      float64    `json:"inside_20"`
	Broken        float64    `json:"broken"`
	Kneel         float64    `json:"kneel"`
	Scramble      float64    `json:"scramble"`
	OnTarget      float64    `json:"on_target"`
	Reception     float64    `json:"reception"`
	Target        float64    `json:"target"`
	Tackle        float64    `json:"tackle"`
	AstTackle     float64    `json:"ast_tackle"`
	AstSack       float64    `json:"ast_sack"`
	Made          float64    `json:"made"`
	AttYards      float64    `json:"att_yards"`
	Nullified     bool       `json:"nullified"`
	Penalty       float64    `json:"penalty"`
	Fumble        float64    `json:"fumble"`
	Lost          float64    `json:"lost"`
	Forced        float64    `json:"forced"`
	ForcedFumble  float64    `json:"forced_fumble"`
	Player        *PlayerRef `json:"player"`
	Team          *TeamRef   `json:"team"`
}

func (s *Statistic) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("        [%s]", s.StatType))
	if s.Category != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", s.Category))
	}
	if s.Player != nil {
		sb.WriteString(fmt.Sprintf(" %s", s.Player.String()))
	}
	if s.Team != nil {
		sb.WriteString(fmt.Sprintf(" [%s]", s.Team.Alias))
	}

	// Show relevant stats based on type
	parts := []string{}
	if s.Yards != 0 {
		parts = append(parts, fmt.Sprintf("%.0f yds", s.Yards))
	}
	if s.Attempt > 0 {
		parts = append(parts, fmt.Sprintf("%.0f att", s.Attempt))
	}
	if s.Complete > 0 {
		parts = append(parts, fmt.Sprintf("%.0f cmp", s.Complete))
	}
	if s.Reception > 0 {
		parts = append(parts, fmt.Sprintf("%.0f rec", s.Reception))
	}
	if s.Touchdown > 0 {
		parts = append(parts, fmt.Sprintf("%.0f TD", s.Touchdown))
	}
	if s.Touchback > 0 {
		parts = append(parts, fmt.Sprintf("%.0f TB", s.Touchback))
	}
	if s.FirstDown > 0 {
		parts = append(parts, fmt.Sprintf("%.0f 1D", s.FirstDown))
	}
	if s.Tackle > 0 {
		parts = append(parts, fmt.Sprintf("%.0f tkl", s.Tackle))
	}
	if s.Sack > 0 {
		parts = append(parts, fmt.Sprintf("%.1f sack", s.Sack))
	}
	if s.Interception > 0 {
		parts = append(parts, fmt.Sprintf("%.0f INT", s.Interception))
	}

	if len(parts) > 0 {
		sb.WriteString(": ")
		sb.WriteString(strings.Join(parts, ", "))
	}
	sb.WriteString("\n")
	return sb.String()
}

// PlaySituation represents the game situation at start/end of a play
type PlaySituation struct {
	Clock      string          `json:"clock"`
	Down       int             `json:"down"`
	YFD        int             `json:"yfd"`
	Possession *PossessionInfo `json:"possession"`
	Location   *LocationInfo   `json:"location"`
}

func (p *PlaySituation) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("      Clock: %s, Down: %d, YFD: %d\n", p.Clock, p.Down, p.YFD))
	if p.Possession != nil {
		sb.WriteString(fmt.Sprintf("      Possession: %s\n", p.Possession.String()))
	}
	if p.Location != nil {
		sb.WriteString(fmt.Sprintf("      Location: %s\n", p.Location.String()))
	}
	return sb.String()
}

// PossessionInfo represents possession team info
type PossessionInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Market string `json:"market"`
	Alias  string `json:"alias"`
	SRId   string `json:"sr_id"`
}

func (p *PossessionInfo) String() string {
	return fmt.Sprintf("%s %s (%s)", p.Market, p.Name, p.Alias)
}

// LocationInfo represents field position info
type LocationInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Market   string `json:"market"`
	Alias    string `json:"alias"`
	SRId     string `json:"sr_id"`
	Yardline int    `json:"yardline"`
}

func (l *LocationInfo) String() string {
	return fmt.Sprintf("%s %d yard line", l.Alias, l.Yardline)
}

// Drive represents a drive in the game.
// Note: The Sportradar API returns different entry types in the pbp array:
// - type="drive": Regular offensive drives with nested events
// - type="event": Timeouts, end-of-period markers (no play data)
// - type="play": Standalone plays like extra points after special teams TDs
//
// For type="play" entries (e.g., PATs after punt/kick return TDs), the structure
// differs from regular drives: they have no offensive_team, but contain play_type,
// start_situation, and statistics at the drive level instead of nested in events.
type Drive struct {
	Type          string        `json:"type"`
	ID            string        `json:"id"`
	Sequence      float64       `json:"sequence"`
	StartReason   string        `json:"start_reason"`
	EndReason     string        `json:"end_reason"`
	PlayCount     int           `json:"play_count"`
	Duration      string        `json:"duration"`
	FirstDowns    int           `json:"first_downs"`
	Gain          int           `json:"gain"`
	Penalty_Yards int           `json:"penalty_yards"`
	Inside20      bool          `json:"inside_20"`
	Scoring_Drive bool          `json:"scoring_drive"`
	OffensiveTeam *DriveTeam    `json:"offensive_team"`
	DefensiveTeam *DriveTeam    `json:"defensive_team"`
	StartLocation *LocationInfo `json:"start_location"`
	EndLocation   *LocationInfo `json:"end_location"`
	Events        []Event       `json:"events"`

	// Fields for standalone play entries (type="play"), such as extra points
	// after special teams touchdowns. These are populated when the entry
	// represents a single play rather than a multi-play drive.
	// The API returns these entries with Event-like fields at the drive level.
	PlayType       string         `json:"play_type"`
	Clock          string         `json:"clock"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
	WallClock      string         `json:"wall_clock"`
	Description    string         `json:"description"`
	HomePoints     int            `json:"home_points"`
	AwayPoints     int            `json:"away_points"`
	Official       bool           `json:"official"`
	StartSituation *PlaySituation `json:"start_situation"`
	EndSituation   *PlaySituation `json:"end_situation"`
	Statistics     []Statistic    `json:"statistics"`
	Details        []Detail       `json:"details"`
}

// DriveTeam represents minimal team info in a drive (only ID and points)
type DriveTeam struct {
	ID     string `json:"id"`
	Points int    `json:"points"`
}

func (dt *DriveTeam) String() string {
	return fmt.Sprintf("ID: %s, Points: %d", dt.ID, dt.Points)
}

func (d *Drive) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    Type:          %s\n", d.Type))
	sb.WriteString(fmt.Sprintf("    ID:            %s\n", d.ID))
	sb.WriteString(fmt.Sprintf("    Sequence:      %.0f\n", d.Sequence))
	sb.WriteString(fmt.Sprintf("    Start Reason:  %s\n", d.StartReason))
	sb.WriteString(fmt.Sprintf("    End Reason:    %s\n", d.EndReason))
	sb.WriteString(fmt.Sprintf("    Play Count:    %d\n", d.PlayCount))
	sb.WriteString(fmt.Sprintf("    Duration:      %s\n", d.Duration))
	sb.WriteString(fmt.Sprintf("    First Downs:   %d\n", d.FirstDowns))
	sb.WriteString(fmt.Sprintf("    Gain:          %d yards\n", d.Gain))
	sb.WriteString(fmt.Sprintf("    Penalty Yards: %d\n", d.Penalty_Yards))
	sb.WriteString(fmt.Sprintf("    Inside 20:     %t\n", d.Inside20))
	sb.WriteString(fmt.Sprintf("    Scoring Drive: %t\n", d.Scoring_Drive))

	// Print team information (show null explicitly for debugging)
	if d.OffensiveTeam != nil {
		sb.WriteString(fmt.Sprintf("    OffensiveTeam: %s\n", d.OffensiveTeam.String()))
	} else {
		sb.WriteString("    OffensiveTeam: <nil>\n")
	}
	if d.DefensiveTeam != nil {
		sb.WriteString(fmt.Sprintf("    DefensiveTeam: %s\n", d.DefensiveTeam.String()))
	} else {
		sb.WriteString("    DefensiveTeam: <nil>\n")
	}
	if d.StartLocation != nil {
		sb.WriteString(fmt.Sprintf("    Start:         %s\n", d.StartLocation.String()))
	}
	if d.EndLocation != nil {
		sb.WriteString(fmt.Sprintf("    End:           %s\n", d.EndLocation.String()))
	}
	sb.WriteString(fmt.Sprintf("    Events:        %d\n", len(d.Events)))

	for i, event := range d.Events {
		sb.WriteString(fmt.Sprintf("\n      --- Play %d ---\n", i+1))
		sb.WriteString(event.String())
	}

	return sb.String()
}

// AsEvent converts a standalone play Drive entry (type="play") to an Event struct.
// This is used for standalone plays like extra points after special teams TDs,
// where the API returns Event-like fields at the drive level instead of nested
// in the events array. The Drive's ID, Sequence, and other fields are mapped
// to their Event equivalents.
func (d *Drive) AsEvent() *Event {
	return &Event{
		ID:             d.ID,
		Type:           d.Type,
		PlayType:       d.PlayType,
		Sequence:       d.Sequence,
		Clock:          d.Clock,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
		WallClock:      d.WallClock,
		Description:    d.Description,
		HomePoints:     d.HomePoints,
		AwayPoints:     d.AwayPoints,
		Official:       d.Official,
		StartSituation: d.StartSituation,
		EndSituation:   d.EndSituation,
		Statistics:     d.Statistics,
		Details:        d.Details,
		// Fields not available in standalone play entries - use zero values
		AltDescription: "",
	}
}

// TeamRef represents a team reference
type TeamRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Market string `json:"market"`
	Alias  string `json:"alias"`
	SRId   string `json:"sr_id"`
}

func (t *TeamRef) String() string {
	return fmt.Sprintf("%s %s (%s)", t.Market, t.Name, t.Alias)
}

// PlayerRef represents a player reference in statistics
type PlayerRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Jersey   string `json:"jersey"`
	Position string `json:"position"`
	SRId     string `json:"sr_id"`
}

func (p *PlayerRef) String() string {
	return fmt.Sprintf("#%s %s (%s)", p.Jersey, p.Name, p.Position)
}

// Detail represents a play detail
type Detail struct {
	Category      string         `json:"category"`
	Description   string         `json:"description"`
	Sequence      int            `json:"sequence"`
	Direction     string         `json:"direction"`
	Result        string         `json:"result"`
	Yards         int            `json:"yards"`
	StartLocation *LocationInfo  `json:"start_location"`
	EndLocation   *LocationInfo  `json:"end_location"`
	Players       []DetailPlayer `json:"players"`
}

func (d *Detail) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("      [%d] %s: %s\n", d.Sequence, d.Category, d.Description))
	if d.Result != "" {
		sb.WriteString(fmt.Sprintf("          Result: %s", d.Result))
		if d.Yards != 0 {
			sb.WriteString(fmt.Sprintf(", %d yds", d.Yards))
		}
		if d.Direction != "" {
			sb.WriteString(fmt.Sprintf(", %s", d.Direction))
		}
		sb.WriteString("\n")
	}
	if d.StartLocation != nil && d.EndLocation != nil {
		sb.WriteString(fmt.Sprintf("          From: %s To: %s\n", d.StartLocation.String(), d.EndLocation.String()))
	}
	if len(d.Players) > 0 {
		sb.WriteString("          Players:\n")
		for _, p := range d.Players {
			sb.WriteString(fmt.Sprintf("            %s\n", p.String()))
		}
	}
	return sb.String()
}

// DetailPlayer represents a player in a play detail
type DetailPlayer struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Jersey   string   `json:"jersey"`
	Position string   `json:"position"`
	Role     string   `json:"role"`
	Team     *TeamRef `json:"team"`
}

func (d *DetailPlayer) String() string {
	role := d.Role
	if role == "" {
		role = "player"
	}
	return fmt.Sprintf("#%s %s (%s) - %s", d.Jersey, d.Name, d.Position, role)
}

// FetchNFLPlayByPlay fetches play-by-play data for a specific NFL game
func FetchNFLPlayByPlay(apiClient *client.Client, gameID string) (*PlayByPlayResponse, error) {
	data, err := apiClient.GetNFLPlayByPlay(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL play-by-play data: %w", err)
	}

	var response PlayByPlayResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse NFL play-by-play response: %w", err)
	}

	return &response, nil
}
