#!/bin/bash

# =============================================================================
# Update Available Markets Script
# =============================================================================
# Updates AWS database with available markets data.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_available_markets.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-available-markets" "Update Available Markets"
