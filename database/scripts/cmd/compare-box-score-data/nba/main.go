package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
	compare_nba "github.com/openbook/population-scripts/cmd/compare-box-score-data/compare/nba"
	"github.com/openbook/population-scripts/cmd/compare-box-score-data/fetcher"
	"github.com/openbook/population-scripts/cmd/compare-box-score-data/translator"
	"github.com/openbook/population-scripts/config"
	reader_nba "github.com/openbook/population-scripts/reader/nba"
	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
	"github.com/openbook/shared/utils"
)

// fatal prints an error message to stderr and exits with code 1
func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	// Parse command-line flags
	envFile := flag.String("env", "", "path to environment file (default: .env)")
	flag.Parse()

	// Load configuration from environment file and variables
	cfg, err := config.LoadCompareBoxScoreConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("NBA Box Score Comparison Tool (Sportradar)")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Date Range: %s to %s\n", cfg.NBAGameDateStartInclusive.Format("2006-01-02"), cfg.NBAGameDateEndInclusive.Format("2006-01-02"))
	fmt.Printf("Sportradar Rate Limit: %dms\n", cfg.SportradarRateLimitDelayMilliseconds)
	fmt.Println(strings.Repeat("=", 72))

	ctx := context.Background()

	// Connect to database
	fmt.Println("\nConnecting to database...")
	dbStore, err := store.New(ctx, cfg.PGHost, cfg.PGPort, cfg.PGDatabase, cfg.PGUser, cfg.PGPassword, cfg.PGKeyPath)
	if err != nil {
		fatal("Failed to connect to database: %v", err)
	}
	defer dbStore.Close()
	fmt.Println("Connected to database successfully!")

	// Create Sportradar client
	sportradarClient := sportradar.NewClientWithConfig(&sportradar.ClientConfig{
		AccessLevel:    cfg.SportradarAccessLevel,
		RateLimitDelay: time.Duration(cfg.SportradarRateLimitDelayMilliseconds) * time.Millisecond,
		Timeout:        30 * time.Second,
		ApiKeys:        cfg.SportradarAPIKeys,
	})

	// Get all games with box scores from database within date range
	fmt.Println("\nQuerying games with box scores from database...")
	gameIDs, err := store_nba.GetAllNBAGamesWithBoxScores(dbStore, ctx, cfg.NBAGameDateStartInclusive, cfg.NBAGameDateEndInclusive, cfg.TimeZoneForDate)
	if err != nil {
		fatal("Failed to get games with box scores: %v", err)
	}

	if len(gameIDs) == 0 {
		fmt.Println("\nNo NBA games with box scores found in database.")
		fmt.Println("Run update_nba_box_score_data.sh first to generate box scores.")
		fmt.Println(strings.Repeat("=", 72))
		return
	}

	fmt.Printf("Found %d games with box scores to compare\n", len(gameIDs))

	// Compare each game
	fmt.Println("\n" + strings.Repeat("-", 72))
	fmt.Println("Starting comparison...")
	fmt.Println(strings.Repeat("-", 72))

	for i, gameID := range gameIDs {
		fmt.Printf("\n[%d/%d] Comparing game ID %s...\n", i+1, len(gameIDs), gameID)

		// Read database box score
		gameUUID, err := utils.ParseUUID(gameID)
		if err != nil {
			fatal("Failed to parse game ID %s as UUID: %v", gameID, err)
		}
		dbBoxScore, err := reader_nba.ReadNBABoxScore(ctx, dbStore, gameUUID)
		if err != nil {
			fatal("Failed to read database box score for game %s: %v", gameID, err)
		}

		if dbBoxScore.Game == nil || dbBoxScore.Game.TeamA == nil || dbBoxScore.Game.TeamB == nil {
			fatal("Game %s is missing team information", gameID)
		}

		game := dbBoxScore.Game
		homeTeam := game.TeamA.Alias
		awayTeam := game.TeamB.Alias
		gameSportradarID := game.SportradarID
		gameDate := game.ScheduledStartTime

		fmt.Printf("  Game: %s @ %s on %s\n", awayTeam, homeTeam, gameDate.Format("2006-01-02"))
		fmt.Printf("  Sportradar ID: %s\n", gameSportradarID)

		// Fetch Sportradar game summary
		summary, err := fetcher.FetchNBAGameSummary(sportradarClient, gameSportradarID)
		if err != nil {
			fatal("Failed to fetch Sportradar game summary for game %s: %v", gameSportradarID, err)
		}

		// Translate Sportradar response to NBABoxScore model
		sportradarBoxScore, err := translator.TranslateNBABoxScore(ctx, game, summary, dbStore)
		if err != nil {
			fatal("Failed to translate Sportradar box score for game %s: %v", gameSportradarID, err)
		}

		fmt.Printf("  Database players: %d, Sportradar players: %d\n", len(dbBoxScore.Players), len(sportradarBoxScore.Players))

		// Compare box scores
		if err := compare_nba.CompareNBABoxScores(gameID, gameSportradarID, dbBoxScore, sportradarBoxScore); err != nil {
			fatal("%v", err)
		}

		fmt.Println("  All stats match!")
	}

	// Success summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Printf("SUCCESS: All %d NBA games validated successfully!\n", len(gameIDs))
	fmt.Println(strings.Repeat("=", 72))
}
