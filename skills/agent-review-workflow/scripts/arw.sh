#!/usr/bin/env sh
set -e

# 1. Resolve Skill absolute directory
SCRIPT_DIR="$(cd -- "$(dirname -- "$0")" && pwd)"
SKILL_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

# 2. Determine OS
UNAME_S="$(uname -s 2>/dev/null || true)"
EXT=""
case "$UNAME_S" in
  Linux)
    OS="linux"
    ;;
  Darwin)
    OS="darwin"
    ;;
  MINGW*|MSYS*|CYGWIN*)
    OS="windows"
    EXT=".exe"
    ;;
  *)
    echo "arw: unsupported operating system: $UNAME_S" >&2
    exit 1
    ;;
esac

# 3. Determine Architecture
UNAME_M="$(uname -m 2>/dev/null || true)"
case "$UNAME_M" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "arw: unsupported architecture: $UNAME_M" >&2
    exit 1
    ;;
esac

PLATFORM="${OS}-${ARCH}"
BIN_NAME="arw${EXT}"
TARGET="${SKILL_ROOT}/bin/${PLATFORM}/${BIN_NAME}"

# 4. Check binary existence
if [ ! -f "$TARGET" ]; then
  echo "arw binary for platform '$PLATFORM' not found at: $TARGET" >&2
  exit 127
fi

# 5. Check and ensure Unix executable bit
if [ "$OS" != "windows" ] && [ ! -x "$TARGET" ]; then
  chmod +x "$TARGET" 2>/dev/null || true
fi

# 6. Execute binary with transparent argument and exit code passthrough
exec "$TARGET" "$@"
