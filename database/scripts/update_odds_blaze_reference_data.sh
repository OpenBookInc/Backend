#!/bin/bash

# =============================================================================
# Update OddsBlaze Reference Data Script
# =============================================================================
# Maps OddsBlaze entity IDs to existing database entities and stores
# the mappings in entity_vendor_ids.
# Uses the shared .env file in the database/ directory.
#
# Required env vars: ODDS_BLAZE_API_KEY, ODDS_BLAZE_SPORTSBOOKS, ODDS_BLAZE_LEAGUE
# Optional env vars: ODDS_BLAZE_TIMESTAMP
#
# Usage: ./update_odds_blaze_reference_data.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-odds-blaze-reference-data" "Update OddsBlaze Reference Data"
