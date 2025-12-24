#!/bin/bash

# =============================================================================
# Update Reference Data Script
# =============================================================================
# Populates AWS database with schedules, players, rosters, etc.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_reference_data.sh
# =============================================================================

set -e

# Change to the database directory where the shared .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATABASE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$DATABASE_DIR"

# Run the Go script from the database directory
go run scripts/cmd/update-reference-data/main.go > scripts/update_reference_data_output.txt && \
    echo 'Update Reference Data Script completed successfully' || \
    echo 'Update Reference Data Script failed'
