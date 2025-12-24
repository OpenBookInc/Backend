#!/bin/bash

# =============================================================================
# Update Live Stats Script
# =============================================================================
# Fetches and updates live game statistics every 1-5 seconds.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_live_stats.sh
# =============================================================================

set -e

# Change to the database directory where the shared .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATABASE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$DATABASE_DIR"

# Run the Go script from the database directory
go run scripts/cmd/update-live-stats/main.go && \
    echo 'Update Live Stats Script stopped' || \
    echo 'Update Live Stats Script failed'
