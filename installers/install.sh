#!/usr/bin/env sh
set -eu

REPOSITORY='jokerD888/agent-review-workflow'
VERSION='latest'
INSTALL_EXTENSION=0
CONFIGURE_AGENTS=0
FORCE=0
while [ "$#" -gt 0 ]; do case "$1" in --version) VERSION=${2:?}; shift 2;; --with-extension) INSTALL_EXTENSION=1; shift;; --configure-agents) CONFIGURE_AGENTS=1; shift;; --force) FORCE=1; shift;; *) echo "Usage: install.sh [--version TAG] [--with-extension] [--configure-agents] [--force]" >&2; exit 2;; esac; done
case "$(uname -s)" in Linux) ;; *) echo 'This installer supports Linux only.' >&2; exit 2;; esac
case "$(uname -m)" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) echo "Unsupported architecture: $(uname -m)" >&2; exit 2;; esac
ROOT=${XDG_DATA_HOME:-"$HOME/.local/share"}/agent-review-workflow
BIN="$ROOT/bin"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM
if [ "$VERSION" = latest ]; then
  LATEST_URL=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPOSITORY/releases/latest")
  TAG=$(printf '%s' "$LATEST_URL" | sed -n 's#.*/releases/tag/##p')
  if [ -z "$TAG" ]; then TAG=$(curl -fsSL "https://github.com/$REPOSITORY/releases/latest" | grep -o '/releases/tag/v[^"?< ]*' | head -n1 | sed 's#.*/##'); fi
else
  TAG=$VERSION
fi
[ -n "$TAG" ] || { echo 'Could not resolve the latest ARW release tag.' >&2; exit 1; }
RELEASE_BASE="https://github.com/$REPOSITORY/releases/download/$TAG"
curl -fsSL "$RELEASE_BASE/checksums.txt" -o "$TMP/checksums.txt"
download() { name=$1; curl -fsSL "$RELEASE_BASE/$name" -o "$TMP/$name"; (cd "$TMP" && awk -v name="$name" '{ sub(/\r$/, "", $2); if (length($1) == 64 && $2 == name) print }' checksums.txt | sha256sum -c -); }
set_managed_rules() {
  target=$1
  mkdir -p "$(dirname -- "$target")"
  cleaned="$TMP/cleaned-$(basename -- "$target")"
  if [ -f "$target" ]; then
    awk -v start='<!-- agent-review-workflow:begin -->' -v end='<!-- agent-review-workflow:end -->' '$0 == start { inside=1; next } $0 == end { inside=0; next } !inside { print }' "$target" > "$cleaned"
  else
    : > "$cleaned"
  fi
  { cat "$cleaned"; printf '\n<!-- agent-review-workflow:begin -->\n'; cat "$TMP/arw-global-rules.md"; printf '\n<!-- agent-review-workflow:end -->\n'; } > "$target"
}
mkdir -p "$BIN"
for name in "arw_linux_$ARCH" "arw-mcp_linux_$ARCH"; do [ "$FORCE" = 1 ] || [ ! -e "$BIN/${name%%_linux_*}" ] || { echo "Use --force to replace existing installation." >&2; exit 1; }; download "$name"; install -m 0755 "$TMP/$name" "$BIN/${name%%_linux_*}"; done
if [ "$INSTALL_EXTENSION" = 1 ]; then vsix="agent-review-workflow-$TAG.vsix"; download "$vsix"; code --install-extension "$TMP/$vsix" --force; fi
if [ "$CONFIGURE_AGENTS" = 1 ]; then
  download arw-global-rules.md
  set_managed_rules "${CODEX_HOME:-"$HOME/.codex"}/AGENTS.md"
  set_managed_rules "$HOME/.claude/CLAUDE.md"
  set_managed_rules "${XDG_CONFIG_HOME:-"$HOME/.config"}/opencode/AGENTS.md"
  command -v codex >/dev/null 2>&1 && codex mcp add arw -- "$BIN/arw-mcp"
  command -v claude >/dev/null 2>&1 && claude mcp add --scope user arw -- "$BIN/arw-mcp"
  if command -v python3 >/dev/null 2>&1; then
    ARW_OPENCODE_CONFIG="${XDG_CONFIG_HOME:-"$HOME/.config"}/opencode/opencode.json" ARW_MCP_COMMAND="$BIN/arw-mcp" python3 - <<'PY'
import json, os
path = os.environ["ARW_OPENCODE_CONFIG"]
os.makedirs(os.path.dirname(path), exist_ok=True)
try:
    with open(path, encoding="utf-8") as f: config = json.load(f)
except FileNotFoundError: config = {}
version = os.popen("opencode --version").read().strip()
server = {"type": "local", "command": [os.environ["ARW_MCP_COMMAND"]]}
mcp = config.setdefault("mcp", {})
if int(version.split(".", 1)[0]) >= 2:
    server["timeout"] = {"startup": 30000}
    mcp.setdefault("servers", {})["arw"] = server
else:
    mcp.pop("servers", None)
    server["enabled"] = True
    server["timeout"] = 30000
    mcp["arw"] = server
with open(path, "w", encoding="utf-8") as f: json.dump(config, f, indent=2); f.write("\n")
PY
  else
    echo 'OpenCode MCP configuration requires python3; configure mcp.servers.arw manually.' >&2
  fi
fi
echo "Installed ARW in $BIN. Ensure $BIN is on PATH, then run: arw doctor"
