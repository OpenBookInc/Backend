# OpenBook Holdings Backend - AI Agent Instructions

## System Architecture

This is a **multi-language polyglot monorepo** for a proprietary sports wagering platform. Three main subsystems communicate via gRPC:

1. **Sports Data Population** (Go) - `database/scripts/` - Fetches NBA/NFL data from Sportradar API, persists to PostgreSQL with complex dual-ID upsert patterns
2. **Matching Engine** (Rust) - `matching_server/` - Order matching server handling proprietary lineup pooling algorithms
3. **Gateway/Clients** (Go) - `matching-clients/` - Go gRPC clients and gateway connecting database to matching engine

### Critical Cross-Service Patterns

- **Proto definitions** are duplicated across `/proto`, `matching-clients/proto`, and `matching_server/proto` - proto changes require manual sync across all three
- **Go workspace**: Uses `go.work` with modules in `database/scripts`, `matching-clients`, and `shared`
- **Shared models**: `shared/models/` contains common Go data structures used by database scripts and gateway

## Database & Sports Data Population

### Dual ID System (CRITICAL)

Every entity has TWO identifiers - misusing them breaks upserts:

- **VendorID** (string): Sportradar UUID, populated during API fetch, used for `ON CONFLICT (vendor_id) DO UPDATE`
- **ID** (int): PostgreSQL auto-increment, set ONLY after database upsert, used for foreign keys

**During API fetch**: VendorID is set, ID remains 0  
**After persistence**: Both are set  
**Never use ID for lookups before database persistence**

### Persistence Ordering (MUST FOLLOW)

Foreign key dependencies require this exact order:

1. Leagues (no deps) 2. Conferences (needs league ID) 3. Divisions (needs conference ID) 4. Teams (needs division ID) 5. Rosters + Individuals (needs team/league IDs) 6. Games (needs team IDs) 7. IndividualStatuses (needs individual ID)

Each step sets database IDs before next step consumes them.

### DataStore Keying Strategy

Maps in `models.DataStore` use DIFFERENT keys by entity:

- `Leagues`: keyed by DB ID (int) - set after persistence
- `Conferences, Divisions, Teams, Individuals, Games`: keyed by `vendor_id` (string)
- `Rosters`: keyed by team's `vendor_id` (NOT `team_id` which is 0 before persistence)
- `IndividualStatuses`: keyed by individual's `vendor_id` (one status per player)

### Fail-Fast Philosophy

This codebase **never logs warnings** - all unexpected behavior is fatal:

- Invalid data → `fatal()` (script exits immediately)
- Missing entities → fatal error
- Invalid enum values → fatal error
- API/DB errors → fatal with full context

**Only exception**: Intentional exclusions in `fetcher/exclusions.go` (e.g., TBD teams) are silently skipped.

### Database Commands

```bash
# Run migrations (from database/migrations/)
./migrate.sh up|down|status|create <name>|redo|reset|version

# Populate sports data (from database/scripts/)
./update_reference_data.sh  # Implemented - fetches schedules, rosters, players
./update_game_stats.sh      # Placeholder - will update live game stats every 1-5s
./update_play_by_play.sh    # Placeholder - will update play-by-play data every ~15s
./update_entry_outcomes.sh  # Placeholder - will calculate entry outcomes every ~1min

# Or run directly with custom env file
go run cmd/update-reference-data/main.go [--env=.env.custom]
```

All `.sh` scripts auto-discover the appropriate `.env` file by navigating to parent directories.

## Matching Engine (Rust)

### Architecture

- **Entry Pooling**: Proprietary algorithm for matching player lineup entries at different multipliers/portions
- **gRPC Streaming**: Bidirectional stream (`CreateTradeStream`) for order flow
- **Sequence Numbers**: Strict sequence validation - mismatch triggers `fatal_error` and process exit
- **Pool Manager**: Routes orders to appropriate `EntryPool` based on leg combinations

### Build & Run

```bash
cd matching_server

# Compile protos and build
cargo build

# Run server (loads .env from matching_server/)
cargo run

# Tests (if any exist)
cargo test
```

### Proto Compilation

`build.rs` compiles protos at build time using `tonic-build`. Common types use `extern_path` to avoid duplication.

## Gateway/Clients (Go)

### Gateway Service

Bidirectional gRPC gateway connecting PostgreSQL to Rust matching engine. Maintains:

- Order state in `confirmed_bets` table with `OrderStatus` enum
- Pending order tracking via `clientOrderId`
- Three gRPC streams: `orderNewStream`, `orderCancelStream`, `heartbeatStream`

### Code Generation

```bash
cd matching-clients

# Install protoc plugins locally
make setup

# Generate Go code from protos
make codegen

# Build binaries
make build
```

Codegen creates files in `src/gen/` with package structure `matching-clients/src/gen`.

### Running Services

```bash
# Matching server (from matching_server/)
cargo run  # or create run_matching_server.sh

# Gateway (from matching-clients/)
go run cmd/matching-engine-gateway/main.go  # or create run_gateway.sh

# Tester web UI (from matching-clients/)
./run_tester.sh  # Serves UI at http://localhost:8080
```

## Development Environment

### Initial Setup

```bash
# Install all dependencies with pinned versions (from repo root)
./setup-dev-env.sh
```

Installs: Go 1.25.4, Rust 1.91.1, protoc 33.2, goose v3.26.0

**Convenience Scripts**: The repo uses `.sh` files for all common workflows - they auto-discover `.env` files and handle directory navigation. VS Code Task Explorer automatically detects shell scripts in the workspace.

### Environment Files

Each subsystem has its own `.env` file:

- `database/.env` - PostgreSQL connection, SSL cert path
- `database/scripts/.env` - Above + Sportradar API key, rate limits, season params
- `matching-clients/.env` - Matching server host/port, database connection
- `matching_server/.env` - gRPC server host/port

**Required env vars** (see `CLAUDE.md` and `README.md` in each directory for full lists):

- `SPORTRADAR_API_KEY` - Trial API key
- `PG_*` - PostgreSQL connection details
- `PG_KEY_PATH` - Path to SSL certificate (.pem)

## Testing

### Go Tests

```bash
# Unit tests in matching-clients
cd matching-clients/src/utils
go test -v  # Tests proprietary math utilities
```

### Manual Testing

1. Start matching server: `cd matching_server && cargo run`
2. Start gateway: `cd matching-clients && go run cmd/matching-engine-gateway/main.go`
3. Use tester: `./run_tester.sh` and access web UI

## Key Conventions

### Error Handling

- Database scripts: Fatal on any error, never continue with suspect data
- Matching engine: Fatal on sequence mismatch or invalid state
- Gateway: Fatal on unrecoverable errors, log and continue for client errors

### Code Organization

- `fetcher/` - API-specific logic (NBA vs NFL)
- `store/` - Database persistence layer (pgx/v5)
- `models/` - In-memory data structures with pointer relationships
- `config/` - Environment-based configuration

### Type Safety

- PostgreSQL enums (e.g., `individual_status_type`) have Go type wrappers with validation
- Protobuf enums use generated types with `try_from()` validation
- Always validate enum conversions - invalid values are fatal

## Sportradar API

**Trial endpoints** (v7 NFL, v8 NBA) - URLs in `CLAUDE.md`. API calls include:

- Rate limiting via `RATE_LIMIT_DELAY_MS` (default 1000ms)
- Full error context (status, body, URL) on 404/errors
- Season parameters: `NFL_SEASON_YEAR`, `NFL_SEASON_TYPE` (REG/PST/PRE), `NBA_SEASON_YEAR`, `NBA_SEASON_TYPE` (REG/PST)

## Known Limitations

- Proto files must be manually synced across three locations
- No automated integration tests between services
- Database migrations require manual goose execution
- Matching engine has no persistent state (in-memory only)
