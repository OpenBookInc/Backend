#!/bin/bash

# =============================================================================
# Update NFL Play-by-Play Stats Script
# =============================================================================
# Fetches and updates NFL play-by-play game statistics.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_nfl_play_by_play_stats.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-play-by-play-stats/nfl" "Update NFL Play-by-Play Stats"
