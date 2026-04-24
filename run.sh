#!/bin/bash
# Quick run script for cheryl-code

set -e

# Build if binary doesn't exist
if [ ! -f "bin/cheryl-code" ]; then
    echo "Building cheryl-code..."
    go build -o bin/cheryl-code ./cmd
fi

# Run with provided arguments
./bin/cheryl-code "$@"
