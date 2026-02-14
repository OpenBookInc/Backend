#!/bin/bash
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
run_go_script "update-available-legs/nba" "Update NBA Available Legs"
