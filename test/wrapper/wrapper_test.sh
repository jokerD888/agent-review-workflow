#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
WRAPPER="$REPO_ROOT/skills/agent-review-workflow/scripts/arw.sh"

echo "Running POSIX wrapper tests..."

# Check shell syntax
sh -n "$WRAPPER"

# Setup temporary skill environment
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

mkdir -p "$TMP_DIR/scripts"
cp "$WRAPPER" "$TMP_DIR/scripts/arw.sh"
chmod +x "$TMP_DIR/scripts/arw.sh"

# Test 1: Missing binary exits with 127
set +e
"$TMP_DIR/scripts/arw.sh" version 2>"$TMP_DIR/err.txt"
EXIT_CODE=$?
set -e

if [ "$EXIT_CODE" -ne 127 ]; then
  echo "Test 1 failed: Expected exit code 127, got $EXIT_CODE" >&2
  exit 1
fi

if ! grep -q "not found at" "$TMP_DIR/err.txt"; then
  echo "Test 1 failed: Expected 'not found at' in stderr" >&2
  cat "$TMP_DIR/err.txt" >&2
  exit 1
fi
echo "PASS: Test 1 - Missing binary exits with 127"

# Test 2: Platform detection and execution
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

PLATFORM="${OS}-${ARCH}"
BIN_DIR="$TMP_DIR/bin/$PLATFORM"
mkdir -p "$BIN_DIR"

# Mock arw binary
cat <<'EOF' > "$BIN_DIR/arw"
#!/usr/bin/env sh
if [ "$1" = "version" ]; then
  echo '{"version":"0.2.0-dev"}'
  exit 0
fi
if [ "$1" = "fail" ]; then
  echo "error message" >&2
  exit 42
fi
echo "unknown" >&2
exit 1
EOF
chmod +x "$BIN_DIR/arw"

# Test argument forwarding and stdout
OUT="$("$TMP_DIR/scripts/arw.sh" version)"
if [ "$OUT" != '{"version":"0.2.0-dev"}' ]; then
  echo "Test 2 failed: Expected '{\"version\":\"0.2.0-dev\"}', got '$OUT'" >&2
  exit 1
fi
echo "PASS: Test 2 - Forward arguments and stdout"

# Test exit code preservation
set +e
"$TMP_DIR/scripts/arw.sh" fail 2>/dev/null
EXIT_CODE=$?
set -e

if [ "$EXIT_CODE" -ne 42 ]; then
  echo "Test 3 failed: Expected exit code 42, got $EXIT_CODE" >&2
  exit 1
fi
echo "PASS: Test 3 - Preserved non-zero exit code"

echo "ALL POSIX WRAPPER TESTS PASSED!"
