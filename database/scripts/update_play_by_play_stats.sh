#!/bin/bash

# =============================================================================
# Update Play-by-Play Stats Script
# =============================================================================
# Fetches and updates play-by-play game statistics.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_play_by_play_stats.sh
# =============================================================================

set -e

# Change to the database directory where the shared .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATABASE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$DATABASE_DIR"

# Run the Go script from the database directory
go run scripts/cmd/update-play-by-play-stats/main.go && \
    echo 'Update Play-by-Play Stats Script stopped' || \
    echo 'Update Play-by-Play Stats Script failed'
