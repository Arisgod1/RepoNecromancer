#!/bin/bash
# Build and install Repo Necromancer to /usr/local/bin

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NECRO_DIR="$(dirname "$SCRIPT_DIR")"

echo "Building necro..."
cd "$NECRO_DIR"
go build -o necro ./cmd/necro

echo "Installing to /usr/local/bin..."
if [[ -w /usr/local/bin ]]; then
    cp necro /usr/local/bin/necro
    echo "Successfully installed to /usr/local/bin/necro"
else
    echo "Note: /usr/local/bin is not writable. Running with sudo..."
    sudo cp necro /usr/local/bin/necro
    echo "Successfully installed to /usr/local/bin/necro"
fi

echo "Verifying installation..."
/usr/local/bin/necro --version || /usr/local/bin/necro --help
