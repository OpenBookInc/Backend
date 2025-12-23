#!/bin/bash

# Populates AWS database with schedules, players, rosters, etc.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "$SCRIPT_DIR/Database/UpdateDatabaseScripts"

go run main.go > update_reference_data_output.txt && echo 'Update Reference Data Script completed successfully' || echo 'Update Reference Data Script failed'