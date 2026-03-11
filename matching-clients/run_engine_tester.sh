#!/bin/bash

# =============================================================================
# Matching Engine Tester Script
# =============================================================================
# Runs the matching engine tester web UI for testing the matching server.
# Uses the .env file in the matching-clients/ directory.
#
# Usage: ./run_engine_tester.sh
#
# The tester provides a web UI at http://localhost:8080 (configurable via WEB_PORT)
# for sending orders and viewing matching engine responses.
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
    echo "Please create a .env file based on .env.example"
    exit 1
fi

echo -e "${BLUE}Starting Matching Engine Tester...${NC}"
echo -e "${BLUE}Web UI will be available at http://localhost:${WEB_PORT:-8080}${NC}"
echo ""

# Run the tester
go run cmd/matching-engine-tester/main.go
