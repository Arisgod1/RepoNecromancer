#!/bin/bash
# Demonstrate all Repo Necromancer commands with example repos

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/run.sh"

echo "=============================================="
echo "Repo Necromancer - Command Examples"
echo "=============================================="
echo

# Example repos
REPO_SCAN="https://github.com/torvalds/linux"
REPO_AUTOPSY="https://github.com/kubernetes/kubernetes"
REPO_REBORN="https://github.com/facebook/react"

echo "----------------------------------------------"
echo "1. SCAN - Analyze a repository for僵尸代码"
echo "----------------------------------------------"
echo "Scanning: $REPO_SCAN"
echo
necro scan "$REPO_SCAN"
echo

echo "----------------------------------------------"
echo "2. AUTOPSY - Deep analysis of code decay"
echo "----------------------------------------------"
echo "Autopsy: $REPO_AUTOPSY"
echo
necro autopsy "$REPO_AUTOPSY"
echo

echo "----------------------------------------------"
echo "3. REPORT - Generate analysis report"
echo "----------------------------------------------"
echo "Generating report for: $REPO_AUTOPSY"
echo
necro report "$REPO_AUTOPSY" --format markdown --output necromancer-report.md
echo "Report saved to: necromancer-report.md"
echo

echo "----------------------------------------------"
echo "4. REBORN - Revive dead/unmaintained code"
echo "----------------------------------------------"
echo "Attempting to reborn: $REPO_REBORN"
echo
necro reborn "$REPO_REBORN" --strategy=reactivate
echo

echo "=============================================="
echo "Examples complete!"
echo "=============================================="
