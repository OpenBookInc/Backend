#!/bin/bash

# =============================================================================
# Run Set Balance Script
# =============================================================================
# Sets a user's balance in the database.
# Uses the .env file in the dev-tools/ directory.
#
# Usage: ./run_set_balance.sh
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Change to the dev-tools directory where the .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check for .env file
if [ ! -f ".env" ]; then
    echo -e "${RED}Error: .env file not found in $SCRIPT_DIR${NC}"
    echo "Please create a .env file based on .env.example"
    exit 1
fi

echo -e "${BLUE}Running Set Balance...${NC}"
echo ""

# Run the set-balance tool
go run ./cmd/set-balance
