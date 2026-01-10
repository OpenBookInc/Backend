#!/bin/bash

# =============================================================================
# Update NBA Box Score Data Script
# =============================================================================
# Generates NBA box scores from play-by-play data in the database.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_nba_box_score_data.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-box-score-data/nba" "Update NBA Box Score Data"
