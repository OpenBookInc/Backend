#!/bin/bash

# =============================================================================
# Update Reference Data Script
# =============================================================================
# Populates AWS database with schedules, players, rosters, etc.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_reference_data.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-reference-data" "Update Reference Data"
