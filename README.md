# OpenBook Backend

Backend infrastructure for OpenBook — a daily fantasy (DFS) pick'em exchange platform. This monorepo contains the matching engine, database population scripts, backend gateway server, and developer tools.

## Components

- **matching_server/** — Order matching engine written in Rust. Accepts orders via gRPC and matches them using a custom pooling algorithm.
- **matching-clients/** — Go gRPC clients and gateway server. The gateway sits between the app server and the matching engine, managing order lifecycle and database persistence. Also includes test harnesses for the matching engine and gateway.
- **database/** — PostgreSQL schema migrations (via goose) and data population scripts that fetch NFL/NBA data from Sportradar and OddsBlaze APIs.
- **dev-tools/** — Developer utilities (e.g., setting user balances directly in the database).
- **shared/** — Common Go library used across modules: environment loading, data models, and utilities.
- **proto/** — Protocol Buffer definitions for the matching engine and gateway gRPC services.
- **app-server/** — Backend REST server (work in progress).

## Prerequisites

- macOS or Linux
- [Homebrew](https://brew.sh) (macOS)

## Dev Environment Setup

Run the setup script to install all required tooling:

```bash
./setup-dev-env.sh
```

This installs and verifies the following at the pinned versions:

| Tool | Purpose |
|------|---------|
| Go | Primary backend language |
| Rust | Matching engine |
| protoc | Protocol Buffer compiler |
| protoc-gen-go / protoc-gen-go-grpc | Go code generation from .proto files |
| goose | Database migration runner |
| psql | PostgreSQL client |

After setup completes, follow the printed next steps to download Go and Rust dependencies.

To verify your environment without installing anything:

```bash
./setup-dev-env.sh --verify
```

## Building

```bash
# Go modules (from repo root — builds all modules in go.work)
go build ./database/scripts/... ./dev-tools/... ./matching-clients/... ./shared/...

# Matching engine (Rust)
cd matching_server && cargo build
```

Generated gRPC Go code is committed to Git, so `make codegen` only needs to be re-run when `.proto` files change.

## Database Migrations

Migrations live in `database/migrations/` and are managed with goose. See `database/migrations/CLAUDE.md` for details.
