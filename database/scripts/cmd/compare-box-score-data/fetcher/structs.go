package fetcher

// =============================================================================
// Sportradar API Response Structures
// =============================================================================
// These structs mirror the Sportradar API response format exactly.
// Field names match JSON field names from the API.
// =============================================================================

// =============================================================================
// NFL Game Statistics Response Structures
// =============================================================================

// NFLGameStatistics represents the top-level response from the NFL game statistics endpoint
type NFLGameStatistics struct {
	ID         string                   `json:"id"`
	Status     string                   `json:"status"`
	Statistics NFLTeamStatisticsWrapper `json:"statistics"`
}

// NFLTeamStatisticsWrapper contains home and away team statistics
type NFLTeamStatisticsWrapper struct {
	Home NFLTeamStatistics `json:"home"`
	Away NFLTeamStatistics `json:"away"`
}

// NFLTeamStatistics contains all statistics for a single team
type NFLTeamStatistics struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Market      string                   `json:"market"`
	Alias       string                   `json:"alias"`
	Rushing     NFLRushingStats          `json:"rushing"`
	Receiving   NFLReceivingStats        `json:"receiving"`
	Passing     NFLPassingStats          `json:"passing"`
	Defense     NFLDefenseStats          `json:"defense"`
	Fumbles     NFLFumblesStats          `json:"fumbles"`
	FieldGoals  NFLFieldGoalStats        `json:"field_goals"`
	ExtraPoints NFLExtraPointsStats      `json:"extra_points"`
}

// NFLRushingStats contains rushing statistics
type NFLRushingStats struct {
	Players []NFLRushingPlayer `json:"players"`
}

// NFLRushingPlayer contains rushing stats for a single player
type NFLRushingPlayer struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Jersey     string  `json:"jersey"`
	Position   string  `json:"position"`
	Attempts   int     `json:"attempts"`
	Yards      int     `json:"yards"`
	Touchdowns int     `json:"touchdowns"`
	AvgYards   float64 `json:"avg_yards"`
}

// NFLReceivingStats contains receiving statistics
type NFLReceivingStats struct {
	Players []NFLReceivingPlayer `json:"players"`
}

// NFLReceivingPlayer contains receiving stats for a single player
type NFLReceivingPlayer struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Jersey     string  `json:"jersey"`
	Position   string  `json:"position"`
	Receptions int     `json:"receptions"`
	Targets    int     `json:"targets"`
	Yards      int     `json:"yards"`
	Touchdowns int     `json:"touchdowns"`
	AvgYards   float64 `json:"avg_yards"`
}

// NFLPassingStats contains passing statistics
type NFLPassingStats struct {
	Players []NFLPassingPlayer `json:"players"`
}

// NFLPassingPlayer contains passing stats for a single player
type NFLPassingPlayer struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Jersey        string  `json:"jersey"`
	Position      string  `json:"position"`
	Attempts      int     `json:"attempts"`
	Completions   int     `json:"completions"`
	Yards         int     `json:"yards"`
	Touchdowns    int     `json:"touchdowns"`
	Interceptions int     `json:"interceptions"`
	Sacks         int     `json:"sacks"`
	SackYards     int     `json:"sack_yards"`
	Rating        float64 `json:"rating"`
	AvgYards      float64 `json:"avg_yards"`
}

// NFLDefenseStats contains defensive statistics
type NFLDefenseStats struct {
	Players []NFLDefensePlayer `json:"players"`
}

// NFLDefensePlayer contains defensive stats for a single player
type NFLDefensePlayer struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Jersey          string  `json:"jersey"`
	Position        string  `json:"position"`
	Tackles         int     `json:"tackles"`
	Assists         int     `json:"assists"`
	Combined        int     `json:"combined"`
	Sacks           float64 `json:"sacks"`
	SackYards       float64 `json:"sack_yards"`
	Interceptions   int     `json:"interceptions"`
	PassesDefended  int     `json:"passes_defended"`
	ForcedFumbles   int     `json:"forced_fumbles"`
	FumbleRecoveries int    `json:"fumble_recoveries"`
	QBHits          int     `json:"qb_hits"`
}

// NFLFumblesStats contains fumble statistics
type NFLFumblesStats struct {
	Players []NFLFumblesPlayer `json:"players"`
}

// NFLFumblesPlayer contains fumble stats for a single player
type NFLFumblesPlayer struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Jersey       string `json:"jersey"`
	Position     string `json:"position"`
	Fumbles      int    `json:"fumbles"`
	LostFumbles  int    `json:"lost_fumbles"`
	OwnRec       int    `json:"own_rec"`
	OwnRecYards  int    `json:"own_rec_yards"`
	OppRec       int    `json:"opp_rec"`
	OppRecYards  int    `json:"opp_rec_yards"`
	OutOfBounds  int    `json:"out_of_bounds"`
	ForcedFumbles int   `json:"forced_fumbles"`
}

// NFLFieldGoalStats contains field goal statistics
type NFLFieldGoalStats struct {
	Players []NFLFieldGoalPlayer `json:"players"`
}

// NFLFieldGoalPlayer contains field goal stats for a single player
type NFLFieldGoalPlayer struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Jersey   string  `json:"jersey"`
	Position string  `json:"position"`
	Attempts int     `json:"attempts"`
	Made     int     `json:"made"`
	Blocked  int     `json:"blocked"`
	Yards    int     `json:"yards"`
	AvgYards float64 `json:"avg_yards"`
	Longest  int     `json:"longest"`
	Missed   int     `json:"missed"`
	Pct      float64 `json:"pct"`
}

// NFLExtraPointsStats contains extra point statistics
type NFLExtraPointsStats struct {
	Kicks NFLExtraPointKicksStats `json:"kicks"`
}

// NFLExtraPointKicksStats contains extra point kick statistics
type NFLExtraPointKicksStats struct {
	Players []NFLExtraPointKickPlayer `json:"players"`
}

// NFLExtraPointKickPlayer contains extra point kick stats for a single player
type NFLExtraPointKickPlayer struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Jersey   string  `json:"jersey"`
	Position string  `json:"position"`
	Attempts int     `json:"attempts"`
	Made     int     `json:"made"`
	Blocked  int     `json:"blocked"`
	Missed   int     `json:"missed"`
	Pct      float64 `json:"pct"`
}

// =============================================================================
// NBA Game Summary Response Structures
// =============================================================================

// NBAGameSummary represents the top-level response from the NBA game summary endpoint
type NBAGameSummary struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Home   NBATeamData `json:"home"`
	Away   NBATeamData `json:"away"`
}

// NBATeamData contains all data for a single team including players
type NBATeamData struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Market  string      `json:"market"`
	Alias   string      `json:"alias"`
	Players []NBAPlayer `json:"players"`
}

// NBAPlayer contains player information and statistics
type NBAPlayer struct {
	ID              string            `json:"id"`
	FullName        string            `json:"full_name"`
	FirstName       string            `json:"first_name"`
	LastName        string            `json:"last_name"`
	JerseyNumber    string            `json:"jersey_number"`
	Position        string            `json:"position"`
	PrimaryPosition string            `json:"primary_position"`
	Active          bool              `json:"active"`
	Statistics      NBAPlayerStatistics `json:"statistics"`
}

// NBAPlayerStatistics contains all statistics for a single player
type NBAPlayerStatistics struct {
	Minutes            string  `json:"minutes"`
	FieldGoalsMade     int     `json:"field_goals_made"`
	FieldGoalsAtt      int     `json:"field_goals_att"`
	FieldGoalsPct      float64 `json:"field_goals_pct"`
	ThreePointsMade    int     `json:"three_points_made"`
	ThreePointsAtt     int     `json:"three_points_att"`
	ThreePointsPct     float64 `json:"three_points_pct"`
	TwoPointsMade      int     `json:"two_points_made"`
	TwoPointsAtt       int     `json:"two_points_att"`
	TwoPointsPct       float64 `json:"two_points_pct"`
	FreeThrowsMade     int     `json:"free_throws_made"`
	FreeThrowsAtt      int     `json:"free_throws_att"`
	FreeThrowsPct      float64 `json:"free_throws_pct"`
	OffensiveRebounds  int     `json:"offensive_rebounds"`
	DefensiveRebounds  int     `json:"defensive_rebounds"`
	Rebounds           int     `json:"rebounds"`
	Assists            int     `json:"assists"`
	Steals             int     `json:"steals"`
	Blocks             int     `json:"blocks"`
	Turnovers          int     `json:"turnovers"`
	PersonalFouls      int     `json:"personal_fouls"`
	Points             int     `json:"points"`
}
