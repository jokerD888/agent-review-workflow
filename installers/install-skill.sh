#!/usr/bin/env sh
set -eu

SCOPE="global"
HOST_TARGET="all"
TARGET_REPO="."
FORCE=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --scope)
      SCOPE="${2:?}"
      shift 2
      ;;
    --host)
      HOST_TARGET="${2:?}"
      shift 2
      ;;
    --target-repo)
      TARGET_REPO="${2:?}"
      shift 2
      ;;
    --force)
      FORCE=1
      shift
      ;;
    *)
      echo "Usage: install-skill.sh [--scope global|repo] [--host all|antigravity|claude|codex|opencode|generic] [--target-repo PATH] [--force]" >&2
      exit 2
      ;;
  esac
done

SCRIPT_DIR="$(cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

# Locate source skill directory
if [ -f "$REPO_ROOT/dist/agent-review-workflow/SKILL.md" ]; then
  SOURCE_SKILL="$REPO_ROOT/dist/agent-review-workflow"
elif [ -f "$REPO_ROOT/skills/agent-review-workflow/SKILL.md" ]; then
  SOURCE_SKILL="$REPO_ROOT/skills/agent-review-workflow"
else
  echo "Error: Could not locate agent-review-workflow skill source files." >&2
  exit 1
fi

get_target_paths() {
  scope=$1
  host=$2
  repo_dir=$3

  if [ "$scope" = "global" ]; then
    codex_home="${CODEX_HOME:-"$HOME/.codex"}/skills/agent-review-workflow"
    claude_home="$HOME/.claude/skills/agent-review-workflow"
    opencode_home="${XDG_CONFIG_HOME:-"$HOME/.config"}/opencode/skills/agent-review-workflow"
    antigravity_home="$HOME/.gemini/antigravity/skills/agent-review-workflow"
    generic_home="$HOME/.agents/skills/agent-review-workflow"

    case "$host" in
      codex) echo "$codex_home" ;;
      claude) echo "$claude_home" ;;
      opencode) echo "$opencode_home" ;;
      antigravity) echo "$antigravity_home" ;;
      generic) echo "$generic_home" ;;
      all)
        echo "$antigravity_home"
        echo "$codex_home"
        echo "$claude_home"
        echo "$opencode_home"
        echo "$generic_home"
        ;;
      *)
        echo "Unknown host: $host" >&2
        exit 1
        ;;
    esac
  else
    resolved_repo="$(cd -- "$repo_dir" && pwd)"
    case "$host" in
      codex) echo "$resolved_repo/.codex/skills/agent-review-workflow" ;;
      claude) echo "$resolved_repo/.claude/skills/agent-review-workflow" ;;
      opencode) echo "$resolved_repo/.opencode/skills/agent-review-workflow" ;;
      antigravity) echo "$resolved_repo/.agents/skills/agent-review-workflow" ;;
      generic|all) echo "$resolved_repo/.agents/skills/agent-review-workflow" ;;
      *)
        echo "Unknown host: $host" >&2
        exit 1
        ;;
    esac
  fi
}

echo "Installing ARW Skill (Scope: $SCOPE, Host: $HOST_TARGET)..."

TARGETS="$(get_target_paths "$SCOPE" "$HOST_TARGET" "$TARGET_REPO")"

echo "$TARGETS" | while IFS= read -r target; do
  [ -n "$target" ] || continue

  if [ -d "$target" ]; then
    if [ "$FORCE" -eq 0 ]; then
      echo "Skill already exists at $target. Use --force to overwrite."
      continue
    fi
    rm -rf "$target"
  fi

  mkdir -p "$target"

  # Copy SKILL.md
  cp "$SOURCE_SKILL/SKILL.md" "$target/SKILL.md"

  # Copy scripts
  if [ -d "$SOURCE_SKILL/scripts" ]; then
    mkdir -p "$target/scripts"
    cp -R "$SOURCE_SKILL/scripts/"* "$target/scripts/"
    chmod +x "$target/scripts/"*.sh 2>/dev/null || true
  fi

  # Copy references
  if [ -d "$SOURCE_SKILL/references" ]; then
    mkdir -p "$target/references"
    cp -R "$SOURCE_SKILL/references/"* "$target/references/"
  fi

  # Copy bin if available
  if [ -d "$SOURCE_SKILL/bin" ]; then
    mkdir -p "$target/bin"
    cp -R "$SOURCE_SKILL/bin/"* "$target/bin/"
    # Ensure binary execution permissions
    find "$target/bin" -type f -exec chmod +x {} + 2>/dev/null || true
  elif [ -f "$REPO_ROOT/bin/arw" ]; then
    UNAME_S="$(uname -s)"
    case "$UNAME_S" in
      Linux) OS="linux" ;;
      Darwin) OS="darwin" ;;
      *) OS="linux" ;;
    esac
    UNAME_M="$(uname -m)"
    case "$UNAME_M" in
      x86_64|amd64) ARCH="amd64" ;;
      aarch64|arm64) ARCH="arm64" ;;
      *) ARCH="amd64" ;;
    esac
    mkdir -p "$target/bin/${OS}-${ARCH}"
    cp "$REPO_ROOT/bin/arw" "$target/bin/${OS}-${ARCH}/arw"
    chmod +x "$target/bin/${OS}-${ARCH}/arw"
  fi

  echo "Installed skill to: $target"
done

echo "ARW Skill installation complete."
