#!/bin/bash

# =============================================================================
# Update Available Legs Script
# =============================================================================
# Updates AWS database with available legs data.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_available_legs.sh
# =============================================================================

set -e

# Change to the database directory where the shared .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATABASE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$DATABASE_DIR"

# Run the Go script from the database directory
go run scripts/cmd/update-available-legs/main.go > scripts/update_available_legs_output.txt && \
    echo 'Update Available Legs Script completed successfully' || \
    echo 'Update Available Legs Script failed'
