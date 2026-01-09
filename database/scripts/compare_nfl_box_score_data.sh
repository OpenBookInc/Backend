#!/bin/bash

# =============================================================================
# Compare NFL Box Score Data Script
# =============================================================================
# Reads and displays NFL box score data for a game from the database.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./compare_nfl_box_score_data.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "compare-box-score-data/nfl" "Compare NFL Box Score Data"
