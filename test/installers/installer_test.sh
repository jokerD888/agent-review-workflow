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

# Ensure local test binary exists for installer test
mkdir -p "$REPO_ROOT/bin"
[ -f "$REPO_ROOT/bin/arw" ] || touch "$REPO_ROOT/bin/arw"

# Test Global Installation in isolated HOME
TEST_HOME="$TMP_DIR/home"
mkdir -p "$TEST_HOME"

HOME="$TEST_HOME" sh "$INSTALLER" --scope global --host antigravity --force
if [ ! -f "$TEST_HOME/.gemini/config/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Antigravity official config skill not found" >&2
  exit 1
fi
if [ ! -f "$TEST_HOME/.gemini/antigravity/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Antigravity fallback skill not found" >&2
  exit 1
fi
echo "PASS: Global antigravity official and fallback skills installed"

# Test Repo Installation across all hosts
TEST_REPO="$TMP_DIR/repo"
mkdir -p "$TEST_REPO"
sh "$INSTALLER" --scope repo --host all --target-repo "$TEST_REPO" --force
if [ ! -f "$TEST_REPO/.agents/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Repo .agents skill not found" >&2
  exit 1
fi
if [ ! -f "$TEST_REPO/.claude/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Repo .claude skill not found" >&2
  exit 1
fi
if [ ! -f "$TEST_REPO/.codex/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Repo .codex skill not found" >&2
  exit 1
fi
if [ ! -f "$TEST_REPO/.opencode/skills/agent-review-workflow/SKILL.md" ]; then
  echo "Test failed: Repo .opencode skill not found" >&2
  exit 1
fi
echo "PASS: Repo-scoped skills installed across all host directories"

echo "ALL POSIX INSTALLER TESTS PASSED!"
