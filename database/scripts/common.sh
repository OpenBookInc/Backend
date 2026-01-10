#!/bin/bash

# =============================================================================
# Common Shell Script Utilities
# =============================================================================
# Shared functionality for all database scripts.
# Source this file from other scripts: source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
# =============================================================================

set -e

# Get the directory where the scripts live
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Get the database directory (parent of scripts)
DATABASE_DIR="$(dirname "$SCRIPT_DIR")"

# Change to the database directory where the shared .env file lives
cd "$DATABASE_DIR"

# Create output directory if it doesn't exist
OUT_DIR="$SCRIPT_DIR/out"
mkdir -p "$OUT_DIR"

# run_go_script runs a Go command and outputs to a timestamped file
# Usage: run_go_script <cmd-name> <display-name>
# Example: run_go_script "update-reference-data" "Update Reference Data"
# Example: run_go_script "compare-box-score-data/nba" "Compare NBA Box Score Data"
run_go_script() {
    local cmd_name="$1"
    local display_name="$2"
    local timestamp=$(date +"%Y-%m-%d_%H-%M-%S")

    # Check if cmd_name contains a slash (sport-specific script)
    if [[ "$cmd_name" == */* ]]; then
        # Split into base directory and sport subdirectory
        local base_dir="${cmd_name%/*}"  # Everything before last slash
        local sport_dir="${cmd_name##*/}"  # Everything after last slash
        local output_file="$OUT_DIR/${base_dir}/${sport_dir}/${timestamp}_$$.txt"
    else
        # No subdirectory needed
        local output_file="$OUT_DIR/${cmd_name}/${timestamp}_$$.txt"
    fi

    echo "Running ${display_name} Script..."
    echo "Output file: ${output_file}"

    # Create parent directory for output file if it doesn't exist
    local output_dir=$(dirname "$output_file")
    mkdir -p "$output_dir"

    if go run "scripts/cmd/${cmd_name}/main.go" > "$output_file" 2>&1; then
        echo "${display_name} Script completed successfully"
    else
        echo "${display_name} Script failed"
        echo "Check output file for details: ${output_file}"
        exit 1
    fi
}
