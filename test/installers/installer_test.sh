#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
INSTALLER="$REPO_ROOT/installers/install-skill.sh"

echo "Running POSIX installer tests..."

# Check shell syntax
sh -n "$INSTALLER"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

# Test Global Installation in isolated HOME
TEST_HOME="$TMP_DIR/home"
mkdir -p "$TEST_HOME"

HOME="$TEST_HOME" sh "$INSTALLER" --scope global --host antigravity --force
if [ ! -f "$TEST_HOME/.gemini/antigravity/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Antigravity skill not found" >&2
  exit 1
fi
echo "PASS: Global antigravity skill installed"

# Test Repo Installation
TEST_REPO="$TMP_DIR/repo"
mkdir -p "$TEST_REPO"
sh "$INSTALLER" --scope repo --target-repo "$TEST_REPO" --force
if [ ! -f "$TEST_REPO/.agents/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Repo-scoped skill not found" >&2
  exit 1
fi
echo "PASS: Repo-scoped skill installed"

echo "ALL POSIX INSTALLER TESTS PASSED!"
