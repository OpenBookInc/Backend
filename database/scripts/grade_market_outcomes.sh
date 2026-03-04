#!/bin/bash

# =============================================================================
# Grade Market Outcomes Script
# =============================================================================
# Grades closed OddsBlaze markets and persists outcomes (Win/Loss) to the
# database. Uses the shared .env file in the database/ directory.
#
# Usage: ./grade_market_outcomes.sh
# =============================================================================

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "grade-market-outcomes" "Grade Market Outcomes"
