#!/bin/bash

# =============================================================================
# Update Entry Outcomes Script
# =============================================================================
# Calculates and updates user entry outcomes every ~1 minute.
# Uses the shared .env file in the database/ directory.
#
# Usage: ./update_entry_outcomes.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-entry-outcomes" "Update Entry Outcomes"
