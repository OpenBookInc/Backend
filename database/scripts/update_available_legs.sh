#!/bin/bash

# =============================================================================
# Update Available Legs Script
# =============================================================================
# Updates AWS database with available legs data.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_available_legs.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-available-legs" "Update Available Legs"
