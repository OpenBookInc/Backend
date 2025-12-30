#!/bin/bash

# =============================================================================
# Update Box Score Data Script
# =============================================================================
# Fetches and updates live box score data every 1-5 seconds.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_box_score_data.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-box-score-data" "Update Box Score Data"
