#!/bin/bash

# =============================================================================
# Database Migration Script
# =============================================================================
# Wrapper around goose for running database migrations.
# 
# Usage:
#   ./migrate.sh up        # Apply all pending migrations
#   ./migrate.sh down      # Roll back the last migration
#   ./migrate.sh status    # Show migration status
#   ./migrate.sh create <name>  # Create a new migration file
#   ./migrate.sh redo      # Roll back and reapply the last migration
#   ./migrate.sh reset     # Roll back all migrations
#   ./migrate.sh version   # Show current migration version
#
# For more goose commands, run: goose --help
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Change to the Database directory (parent of Migrations)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATABASE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$DATABASE_DIR"

# Load environment variables from Database/.env
if [ -f ".env" ]; then
    set -a
    source .env
    set +a
else
    echo -e "${RED}Error: .env file not found in $DATABASE_DIR${NC}"
    echo "Please create a .env file based on .env.example"
    exit 1
fi

# Check if goose is installed
if ! command -v goose &> /dev/null; then
    echo -e "${RED}Error: goose is not installed${NC}"
    echo "Install it with: brew install goose"
    exit 1
fi

# Show help if no arguments
if [ $# -eq 0 ]; then
    echo -e "${BLUE}Database Migration Script${NC}"
    echo ""
    echo "Usage: ./migrate.sh <command> [args]"
    echo ""
    echo "Commands:"
    echo "  up              Apply all pending migrations"
    echo "  down            Roll back the last migration"
    echo "  status          Show migration status"
    echo "  create <name>   Create a new migration file"
    echo "  redo            Roll back and reapply the last migration"
    echo "  reset           Roll back all migrations"
    echo "  version         Show current migration version"
    echo ""
    echo "Note:"
    echo "  Use 'goose --help' for more commands and options."
    echo ""
    exit 0
fi

COMMAND=$1
shift

case "$COMMAND" in
    up|down|status|redo|reset|version)
        echo -e "${BLUE}Running: goose $COMMAND${NC}"
        goose "$COMMAND" "$@"
        echo -e "${GREEN}✅ Migration $COMMAND completed${NC}"
        ;;
    create)
        if [ -z "$1" ]; then
            echo -e "${RED}Error: Migration name required${NC}"
            echo "Usage: ./migrate.sh create <migration_name>"
            exit 1
        fi
        echo -e "${BLUE}Creating migration: $1${NC}"
        goose create "$1" sql
        echo -e "${GREEN}✅ Migration file created${NC}"
        ;;
    *)
        echo -e "${YELLOW}Passing through to goose: $COMMAND $@${NC}"
        goose "$COMMAND" "$@"
        ;;
esac
