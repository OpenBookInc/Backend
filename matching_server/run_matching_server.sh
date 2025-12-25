#!/bin/bash

# =============================================================================
# Run Matching Server Script
# =============================================================================
# Starts the Rust matching engine gRPC server.
# Uses the .env file in the matching_server/ directory.
#
# Usage: ./run_matching_server.sh
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Change to the matching_server directory where the .env file lives
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check for .env file
if [ ! -f ".env" ]; then
    echo -e "${RED}Error: .env file not found in $SCRIPT_DIR${NC}"
    echo "Please create a .env file with MATCHING_SERVER_HOST and MATCHING_SERVER_PORT"
    exit 1
fi

echo -e "${BLUE}Starting Matching Server...${NC}"
echo ""

# Run the matching server
cargo run
