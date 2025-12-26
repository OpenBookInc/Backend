#!/bin/bash

# =============================================================================
# Update Game Stats Script
# =============================================================================
# Fetches and updates live game statistics every 1-5 seconds.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_game_stats.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-game-stats" "Update Game Stats"
