package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/population-scripts/config"
	decorator_nba "github.com/openbook/population-scripts/decorator/nba"
	decorator_nfl "github.com/openbook/population-scripts/decorator/nfl"
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	persister_nba "github.com/openbook/population-scripts/persister/nba"
	persister_nfl "github.com/openbook/population-scripts/persister/nfl"
	reader_nba "github.com/openbook/population-scripts/reader/nba"
	reader_nfl "github.com/openbook/population-scripts/reader/nfl"
	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
	models "github.com/openbook/shared/models"
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
	cfg, err := config.LoadBatchUpdateConfigFromFile(*envFile)
	if err != nil {
		fatal("Failed to load configuration: %v", err)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Batch Update - Play-by-Play and Box Scores")
	fmt.Println(strings.Repeat("=", 72))
	if cfg.HasNFLDateRange() {
		fmt.Printf("NFL Date Range: %s to %s\n",
			cfg.NFLGameDateStartInclusive.Format("2006-01-02"),
			cfg.NFLGameDateEndInclusive.Format("2006-01-02"))
	}
	if cfg.HasNBADateRange() {
		fmt.Printf("NBA Date Range: %s to %s\n",
			cfg.NBAGameDateStartInclusive.Format("2006-01-02"),
			cfg.NBAGameDateEndInclusive.Format("2006-01-02"))
	}
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

	// Create API client with configured rate limit and access level
	clientConfig := &sportradar.ClientConfig{
		AccessLevel:    cfg.SportradarAccessLevel,
		RateLimitDelay: time.Duration(cfg.RateLimitDelayMilliseconds) * time.Millisecond,
		Timeout:        30 * time.Second,
		ApiKeys:        cfg.SportradarAPIKeys,
	}
	apiClient := sportradar.NewClientWithConfig(clientConfig)

	// Process NFL games if date range is configured
	if cfg.HasNFLDateRange() {
		if err := processNFLGames(ctx, dbStore, apiClient, cfg); err != nil {
			fatal("NFL processing failed: %v", err)
		}
	}

	// Process NBA games if date range is configured
	if cfg.HasNBADateRange() {
		if err := processNBAGames(ctx, dbStore, apiClient, cfg); err != nil {
			fatal("NBA processing failed: %v", err)
		}
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("Batch update completed successfully!")
	fmt.Println(strings.Repeat("=", 72))
}

// processNFLGames fetches and processes all NFL games in the configured date range
func processNFLGames(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, cfg *config.BatchUpdateConfig) error {
	fmt.Println("\n" + strings.Repeat("-", 72))
	fmt.Println("Processing NFL Games")
	fmt.Println(strings.Repeat("-", 72))

	games, err := dbStore.GetGamesByLeagueAndDateRange(ctx, "NFL", cfg.NFLGameDateStartInclusive, cfg.NFLGameDateEndInclusive)
	if err != nil {
		return fmt.Errorf("failed to query NFL games: %w", err)
	}

	fmt.Printf("Found %d NFL games in date range\n", len(games))

	for i, game := range games {
		fmt.Printf("\n[%d/%d] Processing NFL game %d (vendor: %s, scheduled: %s)\n",
			i+1, len(games), game.ID, game.VendorID, game.ScheduledStartTime.Format("2006-01-02 15:04"))

		if err := processNFLGame(ctx, dbStore, apiClient, game); err != nil {
			return fmt.Errorf("failed to process NFL game %d: %w", game.ID, err)
		}
	}

	fmt.Printf("\nSuccessfully processed %d NFL games\n", len(games))
	return nil
}

// processNFLGame fetches play-by-play and generates box scores for a single NFL game
func processNFLGame(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, game *models.Game) error {
	// Fetch play-by-play data from API
	fmt.Printf("  Fetching play-by-play data from Sportradar API...\n")
	playByPlay, err := fetcher_nfl.FetchNFLPlayByPlay(apiClient, game.VendorID)
	if err != nil {
		return fmt.Errorf("failed to fetch play-by-play: %w", err)
	}

	// Validate API response matches expected game
	if playByPlay.ID != game.VendorID {
		return fmt.Errorf("API response vendor_id mismatch: expected %s, got %s", game.VendorID, playByPlay.ID)
	}

	// Decorate the fetched data with derived statistics (currently a no-op for NFL)
	playByPlay = decorator_nfl.DecoratePlayByPlay(playByPlay)

	// Persist missing individuals (outside transaction, may make API calls)
	fmt.Printf("  Ensuring all referenced players exist in database...\n")
	if err := persister_nfl.PersistMissingNFLIndividuals(ctx, dbStore, apiClient, playByPlay); err != nil {
		return fmt.Errorf("failed to persist missing individuals: %w", err)
	}

	// Persist play-by-play data in a transaction
	fmt.Printf("  Persisting play-by-play data...\n")
	pbpTx, err := dbStore.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin play-by-play transaction: %w", err)
	}

	if err := persister_nfl.PersistNFLPlayByPlay(ctx, dbStore, pbpTx, game.ID, playByPlay); err != nil {
		pbpTx.Rollback(ctx)
		return fmt.Errorf("failed to persist play-by-play: %w", err)
	}

	if err := persister_nfl.CheckAndUpdateNFLPlayByPlayDeletions(ctx, dbStore, pbpTx, game.ID, playByPlay); err != nil {
		pbpTx.Rollback(ctx)
		return fmt.Errorf("failed to check and update play-by-play deletions: %w", err)
	}

	if err := pbpTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit play-by-play transaction: %w", err)
	}

	// Read play-by-play data back for box score generation
	fmt.Printf("  Reading play-by-play data for box score generation...\n")
	pbpData, err := reader_nfl.ReadNFLPlayByPlay(ctx, dbStore, game.ID)
	if err != nil {
		return fmt.Errorf("failed to read play-by-play: %w", err)
	}

	// Generate and persist box scores
	if len(pbpData.Statistics) == 0 {
		fmt.Printf("  No play statistics found, skipping box score generation\n")
		return nil
	}

	boxScoreCount := persister_nfl.GetBoxScoreCount(pbpData)
	fmt.Printf("  Generating box scores for %d players...\n", boxScoreCount)

	// Get existing box scores before starting the transaction
	existingBoxScores, err := store_nfl.GetNFLBoxScoresByGameID(dbStore, ctx, game.ID)
	if err != nil {
		return fmt.Errorf("failed to get existing box scores: %w", err)
	}

	// Persist box scores in a transaction
	bsTx, err := dbStore.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin box score transaction: %w", err)
	}

	upsertedBoxScores, err := persister_nfl.PersistNFLBoxScores(ctx, dbStore, bsTx, pbpData)
	if err != nil {
		bsTx.Rollback(ctx)
		return fmt.Errorf("failed to persist box scores: %w", err)
	}

	if err := persister_nfl.CheckAndUpdateNFLBoxScoreDeletions(ctx, dbStore, bsTx, existingBoxScores, upsertedBoxScores); err != nil {
		bsTx.Rollback(ctx)
		return fmt.Errorf("failed to check and update box score deletions: %w", err)
	}

	if err := bsTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit box score transaction: %w", err)
	}

	fmt.Printf("  Successfully processed game %d\n", game.ID)
	return nil
}

// processNBAGames fetches and processes all NBA games in the configured date range
func processNBAGames(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, cfg *config.BatchUpdateConfig) error {
	fmt.Println("\n" + strings.Repeat("-", 72))
	fmt.Println("Processing NBA Games")
	fmt.Println(strings.Repeat("-", 72))

	games, err := dbStore.GetGamesByLeagueAndDateRange(ctx, "NBA", cfg.NBAGameDateStartInclusive, cfg.NBAGameDateEndInclusive)
	if err != nil {
		return fmt.Errorf("failed to query NBA games: %w", err)
	}

	fmt.Printf("Found %d NBA games in date range\n", len(games))

	for i, game := range games {
		fmt.Printf("\n[%d/%d] Processing NBA game %d (vendor: %s, scheduled: %s)\n",
			i+1, len(games), game.ID, game.VendorID, game.ScheduledStartTime.Format("2006-01-02 15:04"))

		if err := processNBAGame(ctx, dbStore, apiClient, game); err != nil {
			return fmt.Errorf("failed to process NBA game %d: %w", game.ID, err)
		}
	}

	fmt.Printf("\nSuccessfully processed %d NBA games\n", len(games))
	return nil
}

// processNBAGame fetches play-by-play and generates box scores for a single NBA game
func processNBAGame(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, game *models.Game) error {
	// Fetch play-by-play data from API
	fmt.Printf("  Fetching play-by-play data from Sportradar API...\n")
	playByPlay, err := fetcher_nba.FetchNBAPlayByPlay(apiClient, game.VendorID)
	if err != nil {
		return fmt.Errorf("failed to fetch play-by-play: %w", err)
	}

	// Validate API response matches expected game
	if playByPlay.ID != game.VendorID {
		return fmt.Errorf("API response vendor_id mismatch: expected %s, got %s", game.VendorID, playByPlay.ID)
	}

	// Decorate the fetched data with derived statistics (e.g., heave blocks)
	playByPlay = decorator_nba.DecoratePlayByPlay(playByPlay)

	// Persist missing individuals (outside transaction, may make API calls)
	fmt.Printf("  Ensuring all referenced players exist in database...\n")
	if err := persister_nba.PersistMissingNBAIndividuals(ctx, dbStore, apiClient, playByPlay); err != nil {
		return fmt.Errorf("failed to persist missing individuals: %w", err)
	}

	// Persist play-by-play data in a transaction
	fmt.Printf("  Persisting play-by-play data...\n")
	pbpTx, err := dbStore.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin play-by-play transaction: %w", err)
	}

	if err := persister_nba.PersistNBAPlayByPlay(ctx, dbStore, pbpTx, game.ID, playByPlay); err != nil {
		pbpTx.Rollback(ctx)
		return fmt.Errorf("failed to persist play-by-play: %w", err)
	}

	if err := persister_nba.CheckAndUpdateNBAPlayByPlayDeletions(ctx, dbStore, pbpTx, game.ID, playByPlay); err != nil {
		pbpTx.Rollback(ctx)
		return fmt.Errorf("failed to check and update play-by-play deletions: %w", err)
	}

	if err := pbpTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit play-by-play transaction: %w", err)
	}

	// Read play-by-play data back for box score generation
	fmt.Printf("  Reading play-by-play data for box score generation...\n")
	pbpData, err := reader_nba.ReadNBAPlayByPlay(ctx, dbStore, game.ID)
	if err != nil {
		return fmt.Errorf("failed to read play-by-play: %w", err)
	}

	// Generate and persist box scores
	if len(pbpData.Statistics) == 0 {
		fmt.Printf("  No play statistics found, skipping box score generation\n")
		return nil
	}

	boxScoreCount := persister_nba.GetBoxScoreCount(pbpData)
	fmt.Printf("  Generating box scores for %d players...\n", boxScoreCount)

	// Get existing box scores before starting the transaction
	existingBoxScores, err := store_nba.GetNBABoxScoresByGameID(dbStore, ctx, game.ID)
	if err != nil {
		return fmt.Errorf("failed to get existing box scores: %w", err)
	}

	// Persist box scores in a transaction
	bsTx, err := dbStore.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin box score transaction: %w", err)
	}

	upsertedBoxScores, err := persister_nba.PersistNBABoxScores(ctx, dbStore, bsTx, pbpData)
	if err != nil {
		bsTx.Rollback(ctx)
		return fmt.Errorf("failed to persist box scores: %w", err)
	}

	if err := persister_nba.CheckAndUpdateNBABoxScoreDeletions(ctx, dbStore, bsTx, existingBoxScores, upsertedBoxScores); err != nil {
		bsTx.Rollback(ctx)
		return fmt.Errorf("failed to check and update box score deletions: %w", err)
	}

	if err := bsTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit box score transaction: %w", err)
	}

	fmt.Printf("  Successfully processed game %d\n", game.ID)
	return nil
}
