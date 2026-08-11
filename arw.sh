#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
command_name=${1:-help}
shift || true

case "$command_name" in
  init)
    exec "$root/install.sh" --project "$(pwd)" "$@"
    ;;
  start)
    exec "$root/scripts/start-task.sh" "$@"
    ;;
  review)
    exec "$root/scripts/review.sh" "$@"
    ;;
  update)
    exec "$root/install.sh" --update
    ;;
  doctor)
    printf 'Git: '
    command -v git || true
    printf 'VS Code: '
    command -v code || true
    for file in "${CODEX_HOME:-$HOME/.codex}/AGENTS.md" "$HOME/.claude/CLAUDE.md" "$HOME/.config/opencode/AGENTS.md"; do
      if [ -f "$file" ]; then printf 'OK\t%s\n' "$file"; else printf 'MISSING\t%s\n' "$file"; fi
    done
    ;;
  help|--help|-h)
    cat <<'EOF'
Usage: arw <command>

  arw init [--with-opencode-reviewer]  Configure the current project.
  arw start <task name> [base-branch]  Create an AI task branch.
  arw review [base-branch]             Summarize the current task diff.
  arw update                            Download the latest workflow files.
  arw doctor                            Check prerequisites and rule files.
EOF
    ;;
  *)
    echo "Unknown command: $command_name. Run 'arw help'." >&2
    exit 2
    ;;
esac
