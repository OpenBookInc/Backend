# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sports data population tool that fetches NBA/NFL data from Sportradar API and persists it to PostgreSQL. Supports both reference data (teams, players, games) and play-by-play statistics.

## Commands

```bash
# Validate/compile all packages
go build ./...

# Run reference data update
./update_reference_data.sh

# Run NFL play-by-play update
./update_nfl_play_by_play_stats.sh

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
              └──────────┘   └───────────┘   │(reads)   │
                                            └──────────┘
```

**Package Roles:**

- **client/**: Communicates with Sportradar API. Handles HTTP requests, rate limiting, error handling.

- **fetcher/**: Returns raw API response structs. NO dependency on models/ or persister/. Defines its own structs that mirror the Sportradar API response format exactly. **CRITICAL**: Field names in fetcher structs MUST match the JSON field names from the API exactly (e.g., `Sequence`, not `VendorSequence`), even if the database uses different column names. The fetcher is a pure representation of the API response.
  - Sport-specific code organized in subdirectories (e.g., `fetcher/nfl/`)

- **persister/**: Maps API structs → database entries. Handles enum transformation (API strings → DB enum strings). NO dependency on shared/models. Takes fetcher structs and calls store methods. This is where the mapping from API field names to database column names happens.
  - Sport-specific code organized in subdirectories (e.g., `persister/nfl/`)

- **reader/**: Reads data from the database for display and comparison. Returns shared/models types.
  - Sport-specific code organized in subdirectories (e.g., `reader/nfl/`)

- **store/**: Communicates with database using pgx/v5. Defines internal structs for WRITE operations (e.g., `PlayStatisticForUpsert`, `GameStatusForUpsert`). May import shared/models for READ operations only.
  - Sport-specific code organized in subdirectories (e.g., `store/nfl/`)
  - Methods in sport subdirectories use `*store.Store` receiver to extend the Store type

- **config/**: Environment-based configuration (API keys, database credentials, rate limits, season parameters).

- **main.go**: Simple wrapper that orchestrates fetcher → persister flow. Should contain minimal logic.

### Dependency Rules

**Critical**: These dependency rules ensure clean separation of concerns:

1. **fetcher/** → depends only on `client/` (no models/, no persister/)
2. **persister/** → depends on `fetcher/` and `store/` (no shared/models)
3. **store/** → depends on shared/models for READs only; uses internal structs for WRITEs
4. **main.go** → depends on all packages but contains minimal logic

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
    fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
    persister_nfl "github.com/openbook/population-scripts/persister/nfl"
    reader_nfl "github.com/openbook/population-scripts/reader/nfl"
    store_nfl "github.com/openbook/population-scripts/store/nfl"
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
Sportradar API → client/ → fetcher/ (raw API structs) → persister/ (transformation) → store/ → PostgreSQL
```

**Play-by-Play:**
```
Sportradar API → client/ → fetcher/nfl/ (raw API structs) → persister/nfl/ (transformation) → store/nfl/ → PostgreSQL
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

**Exceptions**: Only known, intentional exclusions (via `fetcher/exclusions.go` and sport-specific files like `persister/nfl/exclusions.go`) are silently skipped. These are documented business logic, not error conditions.

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

**In-Memory Lookup** (pre-resolved):
```go
team := dataStore.Teams[vendorID]
roster.TeamID = team.ID
```
- Used when you've already loaded entities into memory
- Good for reference data where you process hierarchically

**Discretionary**: The choice between subquery vs. in-memory is left to the coder based on what's already available in context.

### Dual ID System

Every entity has two identifiers:
- **VendorID** (string): Sportradar UUID, set immediately during API fetch
- **ID** (int): PostgreSQL auto-increment primary key, set ONLY after database upsert

**Why**: VendorID enables idempotent upserts (`ON CONFLICT (vendor_id) DO UPDATE`). ID is for foreign key relationships in database.

### Struct Patterns for Store Operations

**For WRITE operations** (Insert/Update), sport-specific store packages define internal structs:
```go
// store/nfl/play_statistics.go
package nfl

type PlayStatisticForUpsert struct {
    VendorPlayerID string  // Looked up via subquery
    StatType       string  // DB enum as string
    PassingYards   decimal.Decimal
    // ... other fields
}

// Methods use *store.Store receiver to extend the Store type
func (s *store.Store) ReplaceNFLPlayStatistics(ctx context.Context, ...) error
```

**For READ operations**, store may use shared/models:
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

Missing constraints cause: `ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification`

### Data Exclusion Rules

Exclusion logic is split between two files based on when filtering occurs:

**fetcher/exclusions.go** - Filters data during fetching (before adding to in-memory data store):
- `shouldExcludeGame()`: Filters TBD teams (NBA Cup games with undetermined competitors)

**persister/nfl/exclusions.go** - Filters NFL data during persistence (before writing to database):
- `shouldPersistDrive()`: Filters "event" type entries (timeouts, end-of-period markers)
- `shouldPersistPlay()`: Filters non-play events and unofficial plays
- `shouldPersistPlayStatistic()`: Filters team-level stats and ignoreable stat types

**persister/nba/exclusions.go** - Filters NBA data during persistence (before writing to database):
- `shouldPersistPlay()`: Filters game management events (jumpball, teamtimeout, lineupchange, etc.) and events with zero persistable statistics
- `shouldPersistPlayStatistic()`: Filters team-level stats and excluded stat types (fouldrawn, technicalfoul, attemptblocked)

Excluded entities are explicitly skipped. These are documented business logic, not error conditions.

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

**Required:**
- `SPORTRADAR_API_KEY`: Sportradar trial API key
- `PG_HOST`, `PG_PORT`, `PG_DATABASE`, `PG_USER`, `PG_PASSWORD`: PostgreSQL connection
- `PG_KEY_PATH`: Path to SSL certificate (.pem file)

**Optional (with defaults):**
- `RATE_LIMIT_DELAY_MS`: Milliseconds between API requests (default: 1000)
- `NFL_SEASON_YEAR`: NFL season year (default: current year)
- `NFL_SEASON_TYPE`: Season type - REG, PST, PRE (default: REG)
- `NFL_WEEK`: Week number for injury data (default: 1)
- `NFL_GAME_ID`: Specific game ID for NFL play-by-play fetch
- `NBA_SEASON_YEAR`: NBA season year (default: current year)
- `NBA_SEASON_TYPE`: Season type - REG, PST (default: REG)
- `NBA_GAME_ID`: Specific game ID for NBA play-by-play fetch

## API Endpoints

Uses Sportradar **trial** endpoints (v7 for NFL, v8 for NBA):

**NBA:**
- Hierarchy: `/nba/trial/v8/en/league/hierarchy.json`
- Team Profile: `/nba/trial/v8/en/teams/{teamID}/profile.json`
- Season Schedule: `/nba/trial/v8/en/games/{year}/{seasonType}/schedule.json`
- Injuries: `/nba/trial/v8/en/league/injuries.json` (current, no date parameter)
- Play-by-Play: `/nba/trial/v8/en/games/{gameID}/pbp.json`

**NFL:**
- Hierarchy: `/nfl/official/trial/v7/en/league/hierarchy.json`
- Team Roster: `/nfl/official/trial/v7/en/teams/{teamID}/full_roster.json`
- Season Schedule: `/nfl/official/trial/v7/en/games/{year}/{seasonType}/schedule.json`
- Weekly Injuries: `/nfl/official/trial/v7/en/seasons/{year}/{seasonType}/{week}/injuries.json`
- Play-by-Play: `/nfl/official/trial/v7/en/games/{gameID}/pbp.json`

**Error handling**: 404 errors include full URL in error message for debugging. All API errors include response body and status code.

## Known Constraints

- Fetchers process NFL first, then NBA (order doesn't affect database, just API rate limiting)
- Individual players limited to first 10 printed (too many for full display)
- Games limited to first 10 printed (too many for full display)
- Rosters table has only latest roster per team (no historical tracking)
- Individual statuses table has one status per player (current injury status only)
- Players not in roster data are silently skipped when fetching injury statuses
- NBA injuries endpoint returns current injuries only (not date-specific like NFL)
