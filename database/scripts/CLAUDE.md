# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sports data population tool that fetches NBA/NFL data from Sportradar API and persists it to PostgreSQL. Supports both reference data (teams, players, games) and play-by-play statistics.

## Commands

```bash
# Validate/compile all packages
go build ./...

# Run reference data update (teams, players, games, rosters, injuries)
./update_reference_data.sh

# Run play-by-play update for a single game
./update_nfl_play_by_play_stats.sh
./update_nba_play_by_play_stats.sh

# Run box score generation for a single game (from play-by-play data)
./update_nfl_box_score_data.sh
./update_nba_box_score_data.sh

# Run batch play-by-play and box score update for games in a date range
./update_batch_play_by_play_and_box_scores.sh

# Compare box score data against Sportradar (validates all games with box scores)
./compare_nfl_box_score_data.sh
./compare_nba_box_score_data.sh

# Map OddsBlaze entity IDs to existing database entities (writes to entity_vendor_ids)
./update_odds_blaze_reference_data.sh

# Fetch player prop markets from OddsBlaze and persist to database
./update_available_markets.sh

# Grade closed OddsBlaze markets and persist outcomes (Win/Loss)
./grade_market_outcomes.sh

# Download dependencies
go mod download
```

## Architecture

### Package Structure & Dependencies

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              main.go                                     │
│                    (simple wrapper, orchestration only)                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
              ┌──────────┐   ┌───────────┐   ┌──────────┐
              │ fetcher/ │   │ persister/│   │  store/  │
              └──────────┘   └───────────┘   └──────────┘
                    │               │               │
                    ▼               ▼               ▼
              ┌──────────┐   ┌───────────┐   ┌──────────┐
              │ client/  │   │  store/   │   │shared/   │
              │          │   │ fetcher/  │   │models    │
              └──────────┘   └───────────┘   │(reads +  │
                    │                        │ registry)│
                    ▼                        └──────────┘
              ┌───────────┐
              │ decorator/│  (enriches fetcher output before persister)
              └───────────┘
```

**Package Roles:**

- **client/**: Communicates with external APIs (Sportradar, OddsBlaze). Handles HTTP requests, rate limiting, error handling. Each vendor has its own subdirectory (e.g., `client/sportradar/`, `client/oddsblaze/`).

- **fetcher/**: Returns raw API response structs. NO dependency on models/ or persister/. Defines its own structs that mirror the API response format exactly. **CRITICAL**: Field names in fetcher structs MUST match the JSON field names from the API exactly (e.g., `Sequence`, not `VendorSequence`), even if the database uses different column names. The fetcher is a pure representation of the API response.
  - Sport-specific code organized in subdirectories (e.g., `fetcher/nfl/`)
  - Vendor-specific code organized in subdirectories (e.g., `fetcher/oddsblaze/`)

- **decorator/**: Enriches fetcher output with derived data that is missing from the raw API response. Accepts fetcher structs and returns the same type with additional data filled in. This sits between fetcher and persister in the data flow: `fetch → decorate → persist`. The decorator keeps the fetcher pure while allowing the persister to process data normally without special cases.
  - Sport-specific code organized in subdirectories (e.g., `decorator/nba/`)
  - Example: NBA heave events lack a statistics array when blocked, but the blocker is mentioned in the description. The decorator parses this and adds the block statistic.

- **persister/**: Maps API structs → database entries. Handles enum transformation (API strings → DB enum strings). Takes fetcher structs (possibly decorated) and calls store methods. This is where the mapping from API field names to database column names happens. Does NOT interact with the singleton registry directly — registry registration is handled by store upsert methods.
  - Sport-specific code organized in subdirectories (e.g., `persister/nfl/`)

- **reader/**: Reads data from the database for display and comparison. Returns shared/models types.
  - Sport-specific code organized in subdirectories (e.g., `reader/nfl/`)

- **store/**: Communicates with database using pgx/v5. Defines internal structs for WRITE operations (e.g., `PlayStatisticForUpsert`, `GameStatusForUpsert`). May import shared/models for READ operations only.
  - Sport-specific code organized in subdirectories (e.g., `store/nfl/`)
  - Methods in sport subdirectories use `*store.Store` receiver to extend the Store type

- **reducer/**: Extracts unique entities from vendor API responses into simple POD structs. Deduplicates by vendor ID. Vendor-specific code in subdirectories (e.g., `reducer/oddsblaze/`).

- **matcher/**: Matches vendor entities against existing database entities. Fatal on first unmatched entity (fault-intolerant). Vendor-specific code in subdirectories (e.g., `matcher/oddsblaze/`).

- **config/**: Environment-based configuration (API keys, database credentials, rate limits, season parameters).

- **main.go**: Simple wrapper that orchestrates the data flow. Should contain minimal logic.

### Dependency Rules

**Critical**: These dependency rules ensure clean separation of concerns:

1. **fetcher/** → depends only on `client/` (no models/, no persister/, no decorator/)
2. **decorator/** → depends only on `fetcher/` (enriches fetcher output, no models/, no persister/)
3. **persister/** → depends on `fetcher/` and `store/` (may import `shared/models` for return types, but does NOT interact with the registry)
4. **store/** → depends on shared/models for READs and for registry registration during WRITEs; uses internal `ForUpsert` structs as write inputs
5. **main.go** → depends on all packages but contains minimal logic

### Import Naming Conventions

**Standard practice**: All imports use explicit aliases for clarity and consistency.

**Base packages** (shared/models):
```go
import models "github.com/openbook/shared/models"
```

**Sport-specific packages** (use underscore separator):
```go
import (
    models_nfl "github.com/openbook/shared/models/nfl"
    decorator_nfl "github.com/openbook/population-scripts/decorator/nfl"
    fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
    persister_nfl "github.com/openbook/population-scripts/persister/nfl"
    reader_nfl "github.com/openbook/population-scripts/reader/nfl"
    store_nfl "github.com/openbook/population-scripts/store/nfl"
)
```

**Vendor-specific packages** (use underscore separator):
```go
import (
    fetcher_oddsblaze "github.com/openbook/population-scripts/fetcher/oddsblaze"
    reducer_oddsblaze "github.com/openbook/population-scripts/reducer/oddsblaze"
    matcher_oddsblaze "github.com/openbook/population-scripts/matcher/oddsblaze"
    store_oddsblaze "github.com/openbook/population-scripts/store/oddsblaze"
)
```

**Why explicit aliases?**:
- Prevents naming conflicts between parent and child packages
- Makes code more readable (clear which package types/functions come from)
- Consistent pattern across the codebase

**Package naming in subdirectories**: Sport-specific subdirectories use the sport name as the package name (e.g., files in `store/nfl/` use `package nfl`).

### Data Flow

**Reference Data:**
```
Sportradar API → client/ → fetcher/ (raw API structs) → persister/ (transformation) → store/ (DB write + registry registration) → PostgreSQL
```

**Play-by-Play:**
```
Sportradar API → client/ → fetcher/ (raw API structs) → decorator/ (enrichment) → persister/ (transformation) → store/ → PostgreSQL
```

**Box Scores (aggregated from play-by-play):**
```
PostgreSQL → reader/nfl/ or reader/nba/ (read play stats) → persister/nfl/ or persister/nba/ (aggregation) → store/nfl/ or store/nba/ → PostgreSQL
```

**OddsBlaze Reference Data Mapping:**
```
OddsBlaze API → client/oddsblaze/ → fetcher/oddsblaze/ (raw API structs) → reducer/oddsblaze/ (unique entities) → matcher/oddsblaze/ (match to DB) → store/oddsblaze/ → entity_vendor_ids table
```

**Available Markets (Player Props):**
```
OddsBlaze API → client/oddsblaze/ → fetcher/oddsblaze/ (raw API structs) → persister/oddsblaze/ (market transformation using entity_vendor_ids) → store/ → player_prop_markets table
```

**Market Outcome Grading:**
```
PostgreSQL (ungraded closed markets) → store/ → odds_blaze_ids[] → OddsBlaze Grader API → client/oddsblaze/ → fetcher/oddsblaze/ (grader response) → persister/oddsblaze/ (result mapping) → store/ → odds_blaze_market_outcomes table
```

**Box Score Comparison (validates database against Sportradar):**
```
Database Box Score: PostgreSQL → reader/ → models_nfl.NFLBoxScore / models_nba.NBABoxScore
Sportradar Box Score: Sportradar API → client/sportradar/ → cmd/compare-box-score-data/fetcher/ → cmd/compare-box-score-data/translator/ → models
Comparison: cmd/compare-box-score-data/compare/ → success or discrepancy report
```

## Critical Design Patterns

### Fault-Intolerant Architecture

**Critical**: This codebase is designed to fail immediately on any unexpected behavior. No warnings, no silent failures.

**Philosophy**:
- Invalid data → fatal error (script exits with non-zero status)
- Missing required entities → fatal error (e.g., team not found for game)
- Unexpected enum values → fatal error (not silently ignored)
- API errors → fatal error (with full context for debugging)
- Database errors → fatal error with rollback

**Why**: Data integrity is paramount. Better to fail and fix the issue than silently accept bad data that corrupts the database or causes downstream problems.

**Implementation**:
- Use `fatal()` function in main.go or return errors that bubble up to main
- Never log warnings and continue execution when data is suspect
- If you want to skip certain data, explicitly code the skip condition (not a silent catch-all)

**Exceptions**: Only known, intentional exclusions (via persister exclusion files like `persister/exclusions.go`, `persister/nfl/exclusions.go`, `persister/nba/exclusions.go`) are silently skipped. These are documented business logic, not error conditions.

### Enum Transformation Pattern

Enums are handled as strings throughout the codebase, with the database performing validation:

1. **API Response**: Sportradar returns strings (e.g., `"Questionable"`, `"quarter"`)
2. **Fetcher**: Stores raw API strings as-is (no validation)
3. **Persister**: Transforms API strings to DB enum format (e.g., `"Questionable"` → `"questionable"`)
4. **Store**: Passes string to database with enum cast (e.g., `$1::individual_status_type`)
5. **Database**: Validates enum value; returns error if invalid

**Mapping functions** live in sport-specific persister packages (e.g., `persister/nfl/enum_mapping.go` contains `MapIndividualStatusToDB()`, `MapStatTypeToDB()`). These functions return an error for unexpected values (fault-intolerant).

**Why no Go enum types for writes?**: The database is the source of truth for valid enum values. This avoids maintaining duplicate enum definitions and ensures consistency.

### Transaction Patterns

**Single Transaction** (e.g., play-by-play):
- All operations succeed together or fail together
- Use `store.BeginTx()` to start transaction
- Defer rollback, commit on success
- Appropriate when data is tightly coupled (e.g., one game's drives, plays, and statistics)

**Multiple Transactions** (e.g., reference data):
- Each entity upserted independently
- Partial success is acceptable
- Appropriate when entities are independent (e.g., different teams can succeed/fail independently)

**Discretionary**: The choice between single vs. multiple transactions is left to the coder based on the use case.

### Foreign Key Lookup Patterns

Two approaches for resolving foreign keys, both acceptable:

**Database Subquery** (inline lookup):
```sql
INSERT INTO nfl_play_statistics (individual_id, ...)
VALUES ((SELECT id FROM individuals WHERE vendor_id = $1), ...)
```
- Used when you only have vendor_id
- Database resolves the FK inline
- Good for play-by-play where you don't pre-load all entities

**Registry Lookup** (pre-resolved via singleton registry):
```go
team := models.Registry.GetTeam(teamID)
roster.TeamID = team.ID
```
- Used when entities have already been registered in the singleton registry
- Good for reference data where you process hierarchically and register as you go
- Store getter methods (e.g., `GetTeamByVendorID`) check the registry before querying the database

**Discretionary**: The choice between subquery vs. in-memory is left to the coder based on what's already available in context.

### Dual ID System

Every entity has two identifiers:
- **VendorID** (string): Sportradar UUID, set immediately during API fetch
- **ID** (int): PostgreSQL auto-increment primary key, set ONLY after database upsert

**Why**: VendorID enables idempotent upserts (`ON CONFLICT (vendor_id) DO UPDATE`). ID is for foreign key relationships in database.

### Alternate Vendor ID Mapping (entity_vendor_ids)

The `entity_vendor_ids` table maps IDs from alternate vendors (e.g., OddsBlaze) to existing entities in the database. This allows looking up entities by vendor-specific identifiers beyond the primary Sportradar vendor ID stored on each entity.

**Schema:**
- `entity_type` (entity_type enum): The type of entity (e.g., team, individual, game)
- `entity_id` (int): The database primary key ID of the entity
- `vendor` (vendor_type enum): The vendor providing the alternate ID (e.g., OddsBlaze)
- `vendor_id` (varchar): The vendor's identifier for the entity

**Constraints:**
- Primary key: `(entity_type, entity_id, vendor)` — one vendor ID per entity per vendor
- Unique: `(entity_type, vendor, vendor_id)` — vendor IDs are unique within a vendor+entity_type combination
- Index: `(entity_type, entity_id)` — fast lookups by entity

### Singleton Registry for Reference Data

See `shared/models/CLAUDE.md` for full registry documentation including thread safety and usage patterns.

### Struct Patterns for Store Operations

**For WRITE operations** (Insert/Update), store packages use internal `ForUpsert` structs as input.
Reference data upserts return `*models.Entity` (resolved with parent pointers and registered in the singleton registry).
Sport-specific play-by-play upserts may use different return patterns:
```go
// store/teams.go (reference data - returns registered model)
func (s *Store) UpsertTeam(ctx context.Context, team *TeamForUpsert) (*models.Team, error)

// store/nfl/play_statistics.go (play-by-play - uses *store.Store receiver)
func (s *store.Store) ReplaceNFLPlayStatistics(ctx context.Context, ...) error
```

**For READ operations**, store uses shared/models with registry caching:
```go
// store/teams.go
func (s *Store) GetTeamByVendorID(ctx context.Context, vendorID string) (*models.Team, error)
```

### Persistence Ordering (Reference Data)

**Must persist in this exact order** due to foreign key dependencies:
1. Leagues (no dependencies)
2. Conferences (requires league ID)
3. Divisions (requires conference ID)
4. Teams (requires division ID)
5. Individuals (requires league ID)
6. Rosters (requires team ID and individual IDs)
7. Games (requires team IDs for both contenders)
8. IndividualStatuses (requires individual ID)

### Upsert Requirements

All upserts require UNIQUE constraints:
- `leagues.name` - league upsert by name ("NFL", "NBA")
- `conferences.vendor_id` - Sportradar UUID
- `divisions.vendor_id` - Sportradar UUID
- `teams.vendor_id` - Sportradar UUID
- `individuals.vendor_id` - Sportradar UUID
- `rosters.team_id` - one roster per team
- `games.vendor_id` - Sportradar game UUID
- `individual_statuses.individual_id` - one status per player
- `game_statuses.game_id` - one status per game
- `nfl_drives(game_id, vendor_id)` - unique drive per game
- `nfl_plays(drive_id, vendor_id)` - unique play per drive
- `nba_plays(game_id, vendor_id)` - unique play per game
- `nfl_box_scores(game_id, individual_id)` - one box score per player per game
- `nba_box_scores(game_id, individual_id)` - one box score per player per game

Missing constraints cause: `ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification`

### Vendor Deletion (Soft Deletes)

The codebase implements soft deletes via a `vendor_deleted` boolean column on play-by-play and box score tables. This allows data to be "deleted" without losing it, enabling audit trails and potential recovery.

**Tables with `vendor_deleted`:**
- `nfl_drives`
- `nfl_plays`
- `nba_plays`
- `nfl_box_scores`
- `nba_box_scores`

**How it works:**

1. **Sportradar API `deleted` field**: The API response includes a `deleted` boolean field on drives (NFL) and events/plays (NFL/NBA). When Sportradar marks data as deleted, this field is set to `true`.

2. **Upserts reset `vendor_deleted` to FALSE**: When a drive, play, or box score is upserted, it always sets `vendor_deleted = FALSE`. This handles the case where previously-deleted data is "undeleted" by Sportradar.

3. **Deletion checking after persistence**: After persisting play-by-play data, `CheckAndUpdate*Deletions()` functions scan for entities marked as deleted in the API and mark them as deleted in the database.

4. **Read filtering**: All getter functions (e.g., `GetNFLDriveByVendorID`) filter by `vendor_deleted = FALSE`. Deletion checking uses these same functions - if an entity is already deleted (not found), we skip marking it again.

**Play-by-Play Deletion Logic:**

```
CheckAndUpdateNFLPlayByPlayDeletions:
  For each drive in API response:
    If drive.Deleted == true:
      Look up drive in DB (non-deleted only)
      If found: Mark drive deleted (cascades to plays)
      If not found: Skip (doesn't exist or already deleted)
    Else:
      For each event in drive:
        If event.Deleted == true:
          Look up drive in DB → Look up play in DB (non-deleted only)
          If found: Mark play deleted
          If not found: Skip (doesn't exist or already deleted)

CheckAndUpdateNBAPlayByPlayDeletions:
  For each event in API response:
    If event.Deleted == true:
      Look up play in DB (non-deleted only)
      If found: Mark play deleted
      If not found: Skip (doesn't exist or already deleted)
```

**Box Score Deletion Logic:**

Box scores are derived from play-by-play data, so deletion is detected differently:
```
CheckAndUpdateNFLBoxScoreDeletions / CheckAndUpdateNBABoxScoreDeletions:
  1. Query existing box scores BEFORE the transaction
  2. Persist new box scores (returns list of upserted records)
  3. Compare: any existing box score whose individual_id is NOT in the upserted list → mark deleted
```

**NFL Drive → Play Cascade:**

When an NFL drive is marked deleted, all its plays are automatically marked deleted in the same operation:
```go
// MarkNFLDriveDeleted cascades to plays
UPDATE nfl_plays SET vendor_deleted = TRUE WHERE drive_id = $1
UPDATE nfl_drives SET vendor_deleted = TRUE WHERE id = $1
```

**Edge Cases Handled:**

| Edge Case | Behavior |
|-----------|----------|
| Entity marked deleted in API but doesn't exist in DB | Silently skipped (nothing to delete) |
| Entity marked deleted in API but already deleted in DB | Silently skipped (lookup returns not found) |
| Previously deleted entity reappears in API (not deleted) | Upsert resets `vendor_deleted = FALSE` |
| NFL play's parent drive is deleted | Play excluded from reads (filtered by both play AND drive `vendor_deleted`) |
| Box score for player who no longer has stats | Marked as deleted via comparison logic |
| Statistics associated with deleted plays | Not directly marked; excluded via play's deleted status |

**Transaction Scope:**

All deletion checks happen within the same transaction as the persist operations:
1. `PersistMissing*Individuals()` - Outside transaction (may make API calls)
2. `BeginTx()`
3. `Persist*PlayByPlay()` or `Persist*BoxScores()`
4. `CheckAndUpdate*Deletions()`
5. `Commit()`

### Data Exclusion Rules

Exclusion logic lives in the persister layer, filtering data before writing to the database:

**persister/exclusions.go** - Filters games during persistence:
- `shouldExcludeGame()`: Filters TBD teams (NBA Cup games with undetermined competitors) and games with empty team IDs (undetermined playoff matchups)

**persister/nfl/exclusions.go** - Filters NFL data during persistence (before writing to database):
- `shouldPersistDrive()`: Filters "event" type entries (timeouts, end-of-period markers)
- `shouldPersistPlay()`: Filters non-play events and unofficial plays
- `shouldPersistPlayStatistic()`: Filters team-level stats and ignoreable stat types

**persister/nba/exclusions.go** - Filters NBA data during persistence (before writing to database):
- `shouldPersistPlay()`: Filters game management events (jumpball, teamtimeout, lineupchange, etc.) and events with zero persistable statistics
- `shouldPersistPlayStatistic()`: Filters team-level stats and excluded stat types (fouldrawn, technicalfoul, attemptblocked)

Excluded entities are explicitly skipped. These are documented business logic, not error conditions.

### Code Commenting Philosophy

**Principle**: When commenting code, focus on **what** is being called, not **how** it works internally.

**Never comment on implementation details** of functions or methods that the code calls. Those implementation details belong in the called function's own documentation or code, not in the caller's comments.

**If a comment is worthwhile**, discuss only the high-level API contract:
- What the function does (its purpose)
- What it returns
- Why we're calling it in this context

**Examples:**

❌ **Bad** (commenting implementation details):
```go
// Loops through all statistics and filters out team-level stats and excluded types
if shouldPersistPlayStatistic(&stat) {
    // ...
}
```

✅ **Good** (commenting the API contract or context):
```go
// Filter out non-persistable statistics (team-level and excluded types)
if shouldPersistPlayStatistic(&stat) {
    // ...
}
```

✅ **Better** (no comment when function name is self-documenting):
```go
if shouldPersistPlayStatistic(&stat) {
    // ...
}
```

**Why**:
- Implementation details create maintenance burden (comments go stale when implementation changes)
- Well-named functions should be self-documenting
- Comments should explain *why* code exists, not *what* it does (which should be obvious from reading the code)

## Database

### Connection
- Uses pgx/v5 with connection pooling
- SSL/TLS required with ServerName for SNI (AWS RDS)
- Certificate loaded from PG_KEY_PATH environment variable

### Schema Location
Database schemas and migrations are in the `migrations/` directory (sibling to `scripts/`).

Migration scripts use goose.

### Enum Types
PostgreSQL enum types are defined in migrations and auto-generated to Go in `shared/models/gen/`:
- `individual_status_type`: active, day_to_day, doubtful, out, out_for_season, questionable
- `game_status`: scheduled, in_progress, halftime, complete, closed, etc.
- `period_type`: quarter, overtime (shared by NFL and NBA)
- `nfl_stat_type`: passing, rushing, receiving, defense, fumble, interception, field_goal, extra_point
- `nba_stat_type`: field_goal, free_throw, assist, rebound, steal, block, turnover, personal_foul

## Configuration

Environment variables loaded from `.env` (auto-loaded) or via `--env` flag:

**Required (all scripts):**
- `PG_HOST`, `PG_PORT`, `PG_DATABASE`, `PG_USER`, `PG_PASSWORD`: PostgreSQL connection
- `PG_KEY_PATH`: Path to SSL certificate (.pem file)

**Required (Sportradar API scripts):**
- `SPORTRADAR_API_KEY`: Sportradar API key
- `SPORTRADAR_ACCESS_LEVEL`: API access level (trial, production)
- `SPORTRADAR_RATE_LIMIT_DELAY_MS`: Milliseconds between Sportradar API requests

**Required (update_reference_data):**
- `NFL_SEASON_START_YEAR`: NFL season start year
- `NFL_SEASON_TYPE`: Season type - REG, PST, PRE
- `NFL_WEEK`: Week number for injury data
- `NBA_SEASON_START_YEAR`: NBA season start year
- `NBA_SEASON_TYPE`: Season type - REG, PST

**Required (single game scripts: update_*_play_by_play_stats, update_*_box_score_data):**
- `NFL_GAME_ID`: Database integer ID for NFL game (not vendor UUID)
- `NBA_GAME_ID`: Database integer ID for NBA game (not vendor UUID)

**Required (update_batch_play_by_play_and_box_scores):**
- At least one complete date range must be set:
  - `NFL_GAME_DATE_START_INCLUSIVE`, `NFL_GAME_DATE_END_INCLUSIVE`: NFL date range (YYYY-MM-DD)
  - `NBA_GAME_DATE_START_INCLUSIVE`, `NBA_GAME_DATE_END_INCLUSIVE`: NBA date range (YYYY-MM-DD)

**Required (update_odds_blaze_reference_data):**
- `ODDS_BLAZE_API_KEY`: OddsBlaze API key
- `ODDS_BLAZE_SPORTSBOOKS`: Comma-separated sportsbooks (allowed: `draftkings`, `fanatics`)
- `ODDS_BLAZE_LEAGUE`: League to fetch (one of: `nba`, `nfl`, `nhl`, `mlb`)
- `ODDS_BLAZE_RATE_LIMIT_DELAY_MS`: Milliseconds between OddsBlaze API requests (must be positive)
- `ODDS_BLAZE_TIMESTAMP`: *(optional)* Timestamp for historical data via rewind endpoint

**Required (update_available_markets):**
- `ODDS_BLAZE_API_KEY`: OddsBlaze API key
- `ODDS_BLAZE_SPORTSBOOKS`: Comma-separated sportsbooks (allowed: `draftkings`, `fanatics`)
- `ODDS_BLAZE_LEAGUE`: League to fetch (one of: `nba`, `nfl`, `nhl`, `mlb`)
- `ODDS_BLAZE_RATE_LIMIT_DELAY_MS`: Milliseconds between OddsBlaze API requests (must be positive)
- `ODDS_BLAZE_TIMESTAMP`: *(optional)* Timestamp for historical data via rewind endpoint

**Required (grade_market_outcomes):**
- `ODDS_BLAZE_API_KEY`: OddsBlaze API key
- `ODDS_BLAZE_LEAGUE`: League to grade (one of: `nba`, `nfl`)
- `ODDS_BLAZE_RATE_LIMIT_DELAY_MS`: Milliseconds between OddsBlaze API requests (must be positive)
- `ODDS_BLAZE_SPORTSBOOKS`: Required by config loader but not used by this script

**Required (compare_*_box_score_data):**
- `SPORTRADAR_API_KEYS`: Sportradar API keys (same as other Sportradar scripts)
- `SPORTRADAR_ACCESS_LEVEL`: API access level (trial, production)
- `SPORTRADAR_RATE_LIMIT_DELAY_MS`: Milliseconds between Sportradar API requests
- `NFL_GAME_DATE_START_INCLUSIVE`, `NFL_GAME_DATE_END_INCLUSIVE`: NFL date range (YYYY-MM-DD)
- `NBA_GAME_DATE_START_INCLUSIVE`, `NBA_GAME_DATE_END_INCLUSIVE`: NBA date range (YYYY-MM-DD)

## API Endpoints

Uses Sportradar **trial** endpoints (v7 for NFL, v8 for NBA):

**NBA:**
- Hierarchy: `/nba/trial/v8/en/league/hierarchy.json`
- Team Profile: `/nba/trial/v8/en/teams/{teamID}/profile.json` (provides roster player IDs)
- Player Profile: `/nba/trial/v8/en/players/{playerID}/profile.json` (source of individual data)
- Season Schedule: `/nba/trial/v8/en/games/{year}/{seasonType}/schedule.json`
- Injuries: `/nba/trial/v8/en/league/injuries.json` (current, no date parameter)
- Play-by-Play: `/nba/trial/v8/en/games/{gameID}/pbp.json`
- Game Summary: `/nba/trial/v8/en/games/{gameID}/summary.json` (box score comparison)

**NFL:**
- Hierarchy: `/nfl/official/trial/v7/en/league/hierarchy.json`
- Team Roster: `/nfl/official/trial/v7/en/teams/{teamID}/full_roster.json` (provides roster player IDs)
- Player Profile: `/nfl/official/trial/v7/en/players/{playerID}/profile.json` (source of individual data)
- Season Schedule: `/nfl/official/trial/v7/en/games/{year}/{seasonType}/schedule.json`
- Weekly Injuries: `/nfl/official/trial/v7/en/seasons/{year}/{seasonType}/{week}/injuries.json`
- Play-by-Play: `/nfl/official/trial/v7/en/games/{gameID}/pbp.json`
- Game Statistics: `/nfl/official/trial/v7/en/games/{gameID}/statistics.json` (box score comparison)

**OddsBlaze:**
- Odds (live): `https://odds.oddsblaze.com?key={key}&sportsbook={sportsbook}&league={league}&price=decimal`
- Odds (historical): `https://rewind.odds.oddsblaze.com?key={key}&sportsbook={sportsbook}&league={league}&price=decimal&timestamp={timestamp}`
- Grader: `https://grader.oddsblaze.com?key={key}&id={oddsBlazeID}` (grades a single market outcome)

**Error handling**: 404 errors include full URL in error message for debugging. All API errors include response body and status code.

## Known Constraints

- Fetchers process NFL first, then NBA (order doesn't affect database, just API rate limiting)
- Individual players limited to first 10 printed (too many for full display)
- Games limited to first 10 printed (too many for full display)
- Rosters table has only latest roster per team (no historical tracking)
- Individual statuses table has one status per player (current injury status only)
- Players not in roster data are silently skipped when fetching injury statuses
- NBA injuries endpoint returns current injuries only (not date-specific like NFL)
