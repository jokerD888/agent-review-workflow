#!/usr/bin/env sh
set -eu

REPOSITORY='jokerD888/agent-review-workflow'
VERSION='latest'
INSTALL_EXTENSION=0
FORCE=0
while [ "$#" -gt 0 ]; do case "$1" in --version) VERSION=${2:?}; shift 2;; --with-extension) INSTALL_EXTENSION=1; shift;; --force) FORCE=1; shift;; *) echo "Usage: install.sh [--version TAG] [--with-extension] [--force]" >&2; exit 2;; esac; done
case "$(uname -s)" in Linux) ;; *) echo 'This installer supports Linux only.' >&2; exit 2;; esac
case "$(uname -m)" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) echo "Unsupported architecture: $(uname -m)" >&2; exit 2;; esac
ROOT=${XDG_DATA_HOME:-"$HOME/.local/share"}/agent-review-workflow
BIN="$ROOT/bin"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM
API="https://api.github.com/repos/$REPOSITORY/releases"
if [ "$VERSION" = latest ]; then RELEASE="$API/latest"; else RELEASE="$API/tags/$VERSION"; fi
curl -fsSL "$RELEASE" -o "$TMP/release.json"
asset_url() { grep -o '"browser_download_url"[[:space:]]*:[[:space:]]*"[^"]*"' "$TMP/release.json" | sed -n "s/.*\/$1\"$//p" | head -n1; }
curl -fsSL "$(asset_url checksums.txt)" -o "$TMP/checksums.txt"
download() { name=$1; url=$(asset_url "$name"); [ -n "$url" ] || { echo "Missing asset: $name" >&2; exit 1; }; curl -fsSL "$url" -o "$TMP/$name"; grep -E "^[[:xdigit:]]{64}[[:space:]]+\*?$name$" "$TMP/checksums.txt" | sha256sum -c -; }
mkdir -p "$BIN"
for name in "arw_linux_$ARCH" "arw-mcp_linux_$ARCH"; do [ "$FORCE" = 1 ] || [ ! -e "$BIN/${name%%_linux_*}" ] || { echo "Use --force to replace existing installation." >&2; exit 1; }; download "$name"; install -m 0755 "$TMP/$name" "$BIN/${name%%_linux_*}"; done
if [ "$INSTALL_EXTENSION" = 1 ]; then vsix=$(grep -o '"name"[[:space:]]*:[[:space:]]*"agent-review-workflow-[^"]*\.vsix"' "$TMP/release.json" | head -n1 | sed 's/.*"\([^"]*\)"$/\1/'); [ -n "$vsix" ] && download "$vsix" && code --install-extension "$TMP/$vsix" --force; fi
echo "Installed ARW in $BIN. Ensure $BIN is on PATH, then run: arw doctor"
