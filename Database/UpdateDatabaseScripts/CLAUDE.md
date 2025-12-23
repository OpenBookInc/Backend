# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sports data population tool that fetches NBA/NFL data from Sportradar API and persists it to PostgreSQL. Maintains in-memory data structures with pointer relationships, then syncs to database using upsert logic.

## Commands

```bash
# Build
go build

# Run with default .env file
go run main.go

# Run with custom environment file
go run main.go --env=.env.production

# Download dependencies
go mod download

# Database migrations (run from parent ../MigrationScripts directory)
cd ../MigrationScripts
goose postgres "your-connection-string" up
goose postgres "your-connection-string" status
goose postgres "your-connection-string" down
```

## Architecture

### Data Flow
1. **Fetch**: Sportradar API → fetcher/ packages → models.DataStore (in-memory)
2. **Persist**: models.DataStore → store/ package → PostgreSQL (upsert via vendor_id)
3. **Print**: Display all persisted data with database IDs

### Package Structure
- **config/**: Environment-based configuration (Sportradar API key, database credentials, rate limits, season parameters)
- **client/**: Sportradar API client with rate limiting
- **fetcher/**: NBA and NFL data retrieval logic, injury status fetching, exclusion rules
- **models/**: In-memory data structures with pointer relationships, type-safe enums
- **store/**: PostgreSQL persistence layer using pgx/v5

## Critical Design Patterns

### Dual ID System
Every entity has two identifiers:
- **VendorID** (string): Sportradar UUID, set immediately during API fetch
- **ID** (int): PostgreSQL auto-increment primary key, set ONLY after database upsert

**Why**: VendorID enables idempotent upserts (`ON CONFLICT (vendor_id) DO UPDATE`). ID is for foreign key relationships in database.

**Implications**:
- During API fetch: VendorID is populated, ID remains 0
- After persistence: Both VendorID and ID are populated
- Never use ID for lookups before database persistence

### Pointer Relationships

In-memory models maintain full object hierarchy via pointers:
```
Team.Division → Division.Conference → Conference.League
Roster.Team → Team
Individual.League → League
```

During persistence, pointers are translated to foreign key IDs:
```go
// Before upsert
team.Division.ID = 10  // Set from previous upsert
team.DivisionID = int64(team.Division.ID)  // Translate pointer to FK
dbStore.UpsertTeam(ctx, team)
```

### DataStore Keying Strategy

**Critical**: Map keys differ by entity:
- `Leagues`: keyed by DB ID (int) - set after persistence
- `Conferences, Divisions, Teams, Individuals, Games`: keyed by vendor_id (string)
- `Rosters`: keyed by team's vendor_id (NOT team_id which is 0 until persisted)
- `IndividualStatuses`: keyed by individual's vendor_id (one status per player)

**Why Rosters use team vendor_id**: During API fetch, roster.TeamID is 0. Using roster.Team.VendorID allows proper map storage before database persistence.

**Why IndividualStatuses use individual vendor_id**: Ensures one status per player with upsert behavior in memory before database persistence.

### Persistence Ordering

**Must persist in this exact order** due to foreign key dependencies:
1. Leagues (no dependencies)
2. Conferences (requires league ID)
3. Divisions (requires conference ID)
4. Teams (requires division ID)
5. Rosters + Individuals (requires team ID and league ID)
6. Games (requires team IDs for both contenders)
7. IndividualStatuses (requires individual ID)

Each step sets the database ID on entities before the next step consumes them.

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

Missing constraints cause: `ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification`

### Type-Safe Enums

**IndividualStatusType**: PostgreSQL enum type (`individual_status_type`) with Go type wrapper
- Values: Active, Day To Day, Doubtful, Out, Out For Season, Questionable
- Validation: `models.ValidateIndividualStatusType()` returns error if invalid status string
- Database cast: SQL uses `::individual_status_type` when inserting
- Fail-fast: Invalid status values cause fatal errors during API parsing

### Data Exclusion Rules

**fetcher/exclusions.go** centralizes filtering logic:
- `shouldExcludeGame()`: Filters TBD teams (NBA Cup games with undetermined competitors)
- `shouldExcludeTeam()`: Currently no exclusions (placeholder for future use)
- `shouldExcludePlayer()`: Currently no exclusions (placeholder for future use)

Excluded entities are silently skipped during API fetch, not persisted to database.

### Fail-Fast Architecture

**Critical**: This codebase is designed to fail immediately on any unexpected behavior. No warnings, only fatal errors.

**Philosophy**:
- Invalid data → fatal error (script exits with non-zero status)
- Missing required entities → fatal error (e.g., team not found for game)
- Invalid enum values → fatal error (e.g., invalid player status)
- API errors → fatal error (with full context for debugging)
- Database errors → fatal error

**Why**: Data integrity is paramount. Better to fail loudly and fix the issue than silently accept bad data that corrupts the database or causes downstream problems.

**Implementation**: Use `fatal()` function in main.go or return errors that bubble up to main, which calls fatal(). Never log warnings and continue execution when data is suspect.

**Exceptions**: Only known, intentional exclusions (via `fetcher/exclusions.go`) are silently skipped. These are documented business logic, not error conditions.

## Database

### Connection
- Uses pgx/v5 with connection pooling
- SSL/TLS required with ServerName for SNI (AWS RDS)
- Certificate loaded from PG_KEY_PATH environment variable

### Schema Location
Database schemas and migrations are in parent directory: `../MigrationScripts/`

Migration scripts use goose and must be run from the MigrationScripts directory with a full PostgreSQL connection string.

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
- `NBA_SEASON_YEAR`: NBA season year (default: current year)
- `NBA_SEASON_TYPE`: Season type - REG, PST (default: REG)

Season type constants defined in `config/config.go`: SeasonTypeRegular, SeasonTypePostSeason, SeasonTypePreSeason

## API Endpoints

Uses Sportradar **trial** endpoints (v7 for NFL, v8 for NBA):

**NBA:**
- Hierarchy: `/nba/trial/v8/en/league/hierarchy.json`
- Team Profile: `/nba/trial/v8/en/teams/{teamID}/profile.json`
- Season Schedule: `/nba/trial/v8/en/games/{year}/{seasonType}/schedule.json`
- Injuries: `/nba/trial/v8/en/league/injuries.json` (current, no date parameter)

**NFL:**
- Hierarchy: `/nfl/official/trial/v7/en/league/hierarchy.json`
- Team Roster: `/nfl/official/trial/v7/en/teams/{teamID}/full_roster.json`
- Season Schedule: `/nfl/official/trial/v7/en/games/{year}/{seasonType}/schedule.json`
- Weekly Injuries: `/nfl/official/trial/v7/en/seasons/{year}/{seasonType}/{week}/injuries.json`

**Error handling**: 404 errors include full URL in error message for debugging. All API errors include response body and status code.

## Known Constraints

- Fetchers process NFL first, then NBA (order doesn't affect database, just API rate limiting)
- Individual players limited to first 10 printed (too many for full display)
- Games limited to first 10 printed (too many for full display)
- Rosters table has only latest roster per team (no historical tracking)
- Individual statuses table has one status per player (current injury status only)
- AddRoster() will panic if roster.Team is nil (intentional, fail-fast design)
- Players not in roster data are silently skipped when fetching injury statuses
- NBA injuries endpoint returns current injuries only (not date-specific like NFL)
- Script fails fatally on any invalid data (no warnings, fail-fast approach)
