#!/bin/bash

# =============================================================================
# Update NFL Box Score Data Script
# =============================================================================
# Fetches and updates NFL live box score data every 1-5 seconds.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_nfl_box_score_data.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-box-score-data/nfl" "Update NFL Box Score Data"
