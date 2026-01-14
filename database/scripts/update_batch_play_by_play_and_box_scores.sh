#!/bin/bash

# =============================================================================
# Batch Update Play-by-Play and Box Scores Script
# =============================================================================
# Fetches play-by-play data and generates box scores for NFL/NBA games
# within a configured date range.
# Uses the shared .env file in the database/ directory.
#
# Required environment variables (in .env or exported):
#   - At least one complete date range:
#     NFL_GAME_DATE_START_INCLUSIVE, NFL_GAME_DATE_END_INCLUSIVE
#     or
#     NBA_GAME_DATE_START_INCLUSIVE, NBA_GAME_DATE_END_INCLUSIVE
#   - Date format: YYYY-MM-DD
#
# Usage: ./update_batch_play_by_play_and_box_scores.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-batch-play-by-play-and-box-scores" "Batch Update Play-by-Play and Box Scores"
