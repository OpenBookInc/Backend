#!/bin/bash

# Populates AWS database with schedules, players, rosters, etc.

set -e

go run main.go > update_reference_data_output.txt && echo 'Update Reference Data Script completed successfully' || echo 'Update Reference Data Script failed'
