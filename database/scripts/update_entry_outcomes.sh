#!/bin/bash

# =============================================================================
# Update Entry Outcomes Script
# =============================================================================
# Calculates and updates user entry outcomes every ~1 minute.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_entry_outcomes.sh
# =============================================================================

set -e

# Change to the database directory where the shared .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATABASE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$DATABASE_DIR"

# Run the Go script from the database directory
go run scripts/cmd/update-entry-outcomes/main.go && \
    echo 'Update Entry Outcomes Script stopped' || \
    echo 'Update Entry Outcomes Script failed'
