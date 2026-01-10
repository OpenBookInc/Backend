#!/bin/bash

# =============================================================================
# Update NBA Play-by-Play Stats Script
# =============================================================================
# Fetches NBA play-by-play data from Sportradar API and persists it to the
# database. Uses the shared .env file in the database/ directory.
#
# Usage: ./update_nba_play_by_play_stats.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-play-by-play-stats/nba" "Update NBA Play-by-Play Stats"
