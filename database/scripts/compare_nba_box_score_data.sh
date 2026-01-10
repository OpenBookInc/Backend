#!/bin/bash

# =============================================================================
# Compare NBA Box Score Data Script
# =============================================================================
# Reads and displays NBA box score data for a game from the database.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./compare_nba_box_score_data.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "compare-box-score-data/nba" "Compare NBA Box Score Data"
