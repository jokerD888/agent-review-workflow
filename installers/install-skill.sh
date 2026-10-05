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

# 1. If script is in install/ inside release package
if [ -f "$SCRIPT_DIR/../SKILL.md" ]; then
  SOURCE_SKILL="$(cd -- "$SCRIPT_DIR/.." && pwd)"
  REPO_ROOT="$SOURCE_SKILL"
elif [ -f "$SCRIPT_DIR/SKILL.md" ]; then
  SOURCE_SKILL="$SCRIPT_DIR"
  REPO_ROOT="$SOURCE_SKILL"
else
  REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
  if [ -f "$REPO_ROOT/dist/agent-review-workflow/SKILL.md" ]; then
    SOURCE_SKILL="$REPO_ROOT/dist/agent-review-workflow"
  elif [ -f "$REPO_ROOT/skills/agent-review-workflow/SKILL.md" ]; then
    SOURCE_SKILL="$REPO_ROOT/skills/agent-review-workflow"
  else
    echo "Error: Could not locate agent-review-workflow skill source files." >&2
    exit 1
  fi
fi

# Detect platform to verify binary presence
UNAME_S="$(uname -s)"
case "$UNAME_S" in
  Linux) OS="linux" ;;
  Darwin) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) OS="linux" ;;
esac

UNAME_M="$(uname -m)"
case "$UNAME_M" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) ARCH="amd64" ;;
esac

PLATFORM="${OS}-${ARCH}"
BIN_NAME="arw"
if [ "$OS" = "windows" ]; then
  BIN_NAME="arw.exe"
fi

HAS_PREBUILT_BIN=0
if [ -d "$SOURCE_SKILL/bin/$PLATFORM" ] || [ -f "$SOURCE_SKILL/bin/$PLATFORM/$BIN_NAME" ]; then
  HAS_PREBUILT_BIN=1
fi

HAS_LOCAL_BUILT_BIN=0
if [ -f "$REPO_ROOT/bin/$BIN_NAME" ]; then
  HAS_LOCAL_BUILT_BIN=1
fi

if [ "$HAS_PREBUILT_BIN" -eq 0 ] && [ "$HAS_LOCAL_BUILT_BIN" -eq 0 ]; then
  echo "Error: No arw binary found for platform '$PLATFORM'." >&2
  echo "If installing from source, compile the CLI binary first:" >&2
  echo "  go build -o ./bin/$BIN_NAME ./cmd/arw" >&2
  echo "Or install using a pre-packaged release package containing bin/." >&2
  exit 1
fi

get_target_paths() {
  scope=$1
  host=$2
  repo_dir=$3

  if [ "$scope" = "global" ]; then
    anti_official="$HOME/.gemini/config/skills/agent-review-workflow"
    anti_legacy="$HOME/.gemini/antigravity/skills/agent-review-workflow"
    codex_home="${CODEX_HOME:-"$HOME/.codex"}/skills/agent-review-workflow"
    claude_home="$HOME/.claude/skills/agent-review-workflow"
    opencode_home="${XDG_CONFIG_HOME:-"$HOME/.config"}/opencode/skills/agent-review-workflow"
    generic_home="$HOME/.agents/skills/agent-review-workflow"

    case "$host" in
      antigravity)
        echo "$anti_official"
        echo "$anti_legacy"
        ;;
      codex) echo "$codex_home" ;;
      claude) echo "$claude_home" ;;
      opencode) echo "$opencode_home" ;;
      generic) echo "$generic_home" ;;
      all)
        echo "$anti_official"
        echo "$anti_legacy"
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
    anti_repo="$resolved_repo/.agents/skills/agent-review-workflow"
    claude_repo="$resolved_repo/.claude/skills/agent-review-workflow"
    codex_repo="$resolved_repo/.codex/skills/agent-review-workflow"
    opencode_repo="$resolved_repo/.opencode/skills/agent-review-workflow"
    generic_repo="$resolved_repo/.agents/skills/agent-review-workflow"

    case "$host" in
      antigravity) echo "$anti_repo" ;;
      claude) echo "$claude_repo" ;;
      codex) echo "$codex_repo" ;;
      opencode) echo "$opencode_repo" ;;
      generic) echo "$generic_repo" ;;
      all)
        echo "$anti_repo"
        echo "$claude_repo"
        echo "$codex_repo"
        echo "$opencode_repo"
        ;;
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

  # Copy bin
  dest_bin="$target/bin"
  if [ "$HAS_PREBUILT_BIN" -eq 1 ]; then
    mkdir -p "$dest_bin"
    cp -R "$SOURCE_SKILL/bin/"* "$dest_bin/"
    find "$dest_bin" -type f -exec chmod +x {} + 2>/dev/null || true
  elif [ "$HAS_LOCAL_BUILT_BIN" -eq 1 ]; then
    mkdir -p "$dest_bin/$PLATFORM"
    cp "$REPO_ROOT/bin/$BIN_NAME" "$dest_bin/$PLATFORM/$BIN_NAME"
    chmod +x "$dest_bin/$PLATFORM/$BIN_NAME" 2>/dev/null || true
  fi

  echo "Installed skill to: $target"
done

echo "ARW Skill installation complete."
