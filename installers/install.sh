#!/usr/bin/env sh
set -eu

REPOSITORY='jokerD888/agent-review-workflow'
VERSION='latest'
CONFIGURE_AGENTS=0
FORCE=0
while [ "$#" -gt 0 ]; do case "$1" in --version) VERSION=${2:?}; shift 2;; --configure-agents) CONFIGURE_AGENTS=1; shift;; --force) FORCE=1; shift;; *) echo "Usage: install.sh [--version TAG] [--configure-agents] [--force]" >&2; exit 2;; esac; done
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
ensure_bin_on_path() {
  profile_path="$HOME/.profile"
  profile_start='# agent-review-workflow:path:begin'
  profile_end='# agent-review-workflow:path:end'
  cleaned="$TMP/cleaned-profile"
  if [ -f "$profile_path" ]; then
    awk -v start="$profile_start" -v end="$profile_end" '$0 == start { inside=1; next } $0 == end { inside=0; next } !inside { print }' "$profile_path" > "$cleaned"
  else
    : > "$cleaned"
  fi
  escaped_bin=$(printf '%s' "$BIN" | sed 's/[\\"]/\\&/g')
  {
    cat "$cleaned"
    printf '\n%s\n' "$profile_start"
    printf 'export PATH="%s:$PATH"\n' "$escaped_bin"
    printf '%s\n' "$profile_end"
  } > "$profile_path"
}
set_managed_rules() {
  target=$1
  mkdir -p "$(dirname -- "$target")"
  cleaned="$TMP/cleaned-$(basename -- "$target")"
  if [ -f "$target" ]; then
    if ! awk -v start='<!-- agent-review-workflow:begin -->' -v end='<!-- agent-review-workflow:end -->' '
      $0 == start { if (inside) { bad = 1; exit }; inside = 1; next }
      $0 == end { if (!inside) { bad = 1; exit }; inside = 0; next }
      END { if (inside) { bad = 1 }; exit bad }
    ' "$target"; then
      echo "Refusing to update malformed agent-review-workflow markers in $target. Repair the marker block first." >&2
      return 1
    fi
    awk -v start='<!-- agent-review-workflow:begin -->' -v end='<!-- agent-review-workflow:end -->' '$0 == start { inside=1; next } $0 == end { inside=0; next } !inside { print }' "$target" > "$cleaned"
  else
    : > "$cleaned"
  fi
  { cat "$cleaned"; printf '\n<!-- agent-review-workflow:begin -->\n'; cat "$TMP/arw-global-rules.md"; printf '\n<!-- agent-review-workflow:end -->\n'; } > "$target"
}
mkdir -p "$BIN"
for name in "arw_linux_$ARCH" "arw-mcp_linux_$ARCH"; do [ "$FORCE" = 1 ] || [ ! -e "$BIN/${name%%_linux_*}" ] || { echo "Use --force to replace existing installation." >&2; exit 1; }; done
for name in "arw_linux_$ARCH" "arw-mcp_linux_$ARCH"; do download "$name"; done
for name in "arw_linux_$ARCH" "arw-mcp_linux_$ARCH"; do install -m 0755 "$TMP/$name" "$BIN/${name%%_linux_*}"; done
ensure_bin_on_path
if [ "$CONFIGURE_AGENTS" = 1 ]; then
  download arw-global-rules.md
  set_managed_rules "${CODEX_HOME:-"$HOME/.codex"}/AGENTS.md"
  set_managed_rules "$HOME/.claude/CLAUDE.md"
  set_managed_rules "${XDG_CONFIG_HOME:-"$HOME/.config"}/opencode/AGENTS.md"
  command -v codex >/dev/null 2>&1 && codex mcp add arw -- "$BIN/arw-mcp"
  command -v claude >/dev/null 2>&1 && claude mcp add --scope user arw -- "$BIN/arw-mcp"
  if command -v opencode >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
    ARW_OPENCODE_CONFIG="${XDG_CONFIG_HOME:-"$HOME/.config"}/opencode/opencode.json" ARW_MCP_COMMAND="$BIN/arw-mcp" ARW_OPENCODE_VERSION="$(opencode --version)" python3 - <<'PY'
import json, os, re
path = os.environ["ARW_OPENCODE_CONFIG"]
os.makedirs(os.path.dirname(path), exist_ok=True)
try:
    with open(path, encoding="utf-8") as f: config = json.load(f)
except FileNotFoundError: config = {}
if not isinstance(config, dict): raise SystemExit("OpenCode configuration root must be an object.")
match = re.search(r"\d+", os.environ["ARW_OPENCODE_VERSION"])
if not match: raise SystemExit("Could not determine the OpenCode major version.")
major = int(match.group())
server = {"type": "local", "command": [os.environ["ARW_MCP_COMMAND"]]}
mcp = config.setdefault("mcp", {})
if not isinstance(mcp, dict): raise SystemExit("OpenCode mcp configuration must be an object.")
if major >= 2:
    server["timeout"] = {"startup": 30000}
    mcp.setdefault("servers", {})["arw"] = server
else:
    servers = mcp.get("servers")
    if servers:
        if not isinstance(servers, dict) or set(servers) != {"arw"}:
            raise SystemExit("Refusing to remove unrelated mcp.servers entries for legacy OpenCode configuration; migrate them manually.")
        mcp.pop("servers")
    server["enabled"] = True
    server["timeout"] = 30000
    mcp["arw"] = server
with open(path, "w", encoding="utf-8") as f: json.dump(config, f, indent=2); f.write("\n")
PY
  elif command -v opencode >/dev/null 2>&1; then
    echo 'OpenCode MCP configuration requires python3; configure it manually.' >&2
  else
    echo 'OpenCode is not installed; skipped its MCP configuration.' >&2
  fi
fi
echo "Installed ARW in $BIN. Open a new shell, then run: arw doctor"
