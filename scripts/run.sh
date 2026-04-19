#!/bin/bash
# Convenience wrapper for Repo Necromancer
# Loads .env and runs necro with all environment variables set

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NECRO_DIR="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$NECRO_DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
    echo "Error: .env file not found at $ENV_FILE"
    exit 1
fi

# Load environment variables from .env
set -a
source "$ENV_FILE"
set +a

# Change to necro directory and run
cd "$NECRO_DIR"
./necro "$@"
