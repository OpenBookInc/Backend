#!/bin/bash

# =============================================================================
# Run Matching Engine Gateway Script
# =============================================================================
# Starts the Go gateway service that connects PostgreSQL to the matching engine.
# Uses the .env file in the matching-clients/ directory.
#
# Usage: ./run_gateway.sh
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Change to the matching-clients directory where the .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check for .env file
if [ ! -f ".env" ]; then
    echo -e "${RED}Error: .env file not found in $SCRIPT_DIR${NC}"
    echo "Please create a .env file with database and matching server configuration"
    exit 1
fi

echo -e "${BLUE}Starting Matching Engine Gateway...${NC}"
echo ""

# Run the gateway
go run cmd/matching-engine-gateway/main.go
