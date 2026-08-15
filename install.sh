#!/usr/bin/env sh
set -eu

MARKER_START='<!-- agent-review-workflow:begin -->'
MARKER_END='<!-- agent-review-workflow:end -->'
RAW_BASE_URL='https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main'
PROJECT_PATH=''
GLOBAL_ONLY=0
INSTALL_OPENCODE_REVIEWER=0
USER_HOME_PATH="${HOME:?HOME must be set}"
UPDATE=0

usage() {
  cat <<'EOF'
Usage: ./install.sh [--project PATH] [--global-only] [--with-opencode-reviewer]
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --project)
      PROJECT_PATH=${2:?--project requires a path}
      shift 2
      ;;
    --global-only)
      GLOBAL_ONLY=1
      shift
      ;;
    --with-opencode-reviewer)
      INSTALL_OPENCODE_REVIEWER=1
      shift
      ;;
    --update)
      UPDATE=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ "$GLOBAL_ONLY" -eq 1 ] && [ -n "$PROJECT_PATH" ]; then
  echo 'Use either --global-only or --project, not both.' >&2
  exit 2
fi

SCRIPT_DIRECTORY=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd || true)
TEMP_DIRECTORY=$(mktemp -d)
trap 'rm -rf "$TEMP_DIRECTORY"' EXIT INT TERM

get_workflow_file() {
  relative_path=$1
  local_path="$SCRIPT_DIRECTORY/$relative_path"
  if [ "$UPDATE" -eq 0 ] && [ -n "$SCRIPT_DIRECTORY" ] && [ -f "$local_path" ]; then
    printf '%s\n' "$local_path"
    return
  fi

  downloaded_path="$TEMP_DIRECTORY/$(basename "$relative_path")"
  curl -fsSL "$RAW_BASE_URL/$relative_path" -o "$downloaded_path"
  printf '%s\n' "$downloaded_path"
}

install_command_line_tool() {
  runtime_root="$USER_HOME_PATH/.local/share/agent-review-workflow"
  runtime_files='install.sh arw.sh rules/global.md templates/AGENTS.md templates/CLAUDE.md templates/opencode-reviewer.md scripts/start-task.sh scripts/review.sh'
  for runtime_file in $runtime_files; do
    destination="$runtime_root/$runtime_file"
    mkdir -p "$(dirname -- "$destination")"
    source_file=$(get_workflow_file "$runtime_file")
    cp "$source_file" "$destination"
  done
  chmod +x "$runtime_root/install.sh" "$runtime_root/arw.sh" "$runtime_root/scripts/start-task.sh" "$runtime_root/scripts/review.sh"

  bin_directory="$USER_HOME_PATH/.local/bin"
  mkdir -p "$bin_directory"
  cat > "$bin_directory/arw" <<EOF
#!/usr/bin/env sh
exec "$runtime_root/arw.sh" "\$@"
EOF
  chmod +x "$bin_directory/arw"

  profile_path="$USER_HOME_PATH/.profile"
  profile_start='# agent-review-workflow:path:begin'
  profile_end='# agent-review-workflow:path:end'
  if ! grep -Fq "$profile_start" "$profile_path" 2>/dev/null; then
    {
      printf '\n%s\n' "$profile_start"
      printf 'export PATH="$HOME/.local/bin:$PATH"\n'
      printf '%s\n' "$profile_end"
    } >> "$profile_path"
  fi
  printf "Installed command-line tool in %s. Open a new shell to use 'arw'.\n" "$runtime_root"
}

set_managed_block() {
  target_path=$1
  content_path=$2
  target_directory=$(dirname -- "$target_path")
  mkdir -p "$target_directory"

  cleaned_path="$TEMP_DIRECTORY/cleaned-$(basename "$target_path")"
  if [ -f "$target_path" ]; then
    if ! awk -v start="$MARKER_START" -v end="$MARKER_END" '
      $0 == start { if (inside) { bad = 1; exit }; inside = 1; next }
      $0 == end { if (!inside) { bad = 1; exit }; inside = 0; next }
      END { if (inside) { bad = 1 }; exit bad }
    ' "$target_path"; then
      echo "Refusing to update malformed agent-review-workflow markers in $target_path. Repair the marker block first." >&2
      return 1
    fi
    awk -v start="$MARKER_START" -v end="$MARKER_END" '
      $0 == start { inside = 1; next }
      $0 == end { inside = 0; next }
      !inside { print }
    ' "$target_path" > "$cleaned_path"
  else
    : > "$cleaned_path"
  fi

  {
    cat "$cleaned_path"
    printf '\n%s\n' "$MARKER_START"
    cat "$content_path"
    printf '\n%s\n' "$MARKER_END"
  } > "$target_path"
  printf 'Updated %s\n' "$target_path"
}

global_rules=$(get_workflow_file 'rules/global.md')
codex_home_path=${CODEX_HOME:-"$USER_HOME_PATH/.codex"}
set_managed_block "$codex_home_path/AGENTS.md" "$global_rules"
set_managed_block "$USER_HOME_PATH/.claude/CLAUDE.md" "$global_rules"
set_managed_block "$USER_HOME_PATH/.config/opencode/AGENTS.md" "$global_rules"
install_command_line_tool

if [ -n "$PROJECT_PATH" ]; then
  if [ ! -d "$PROJECT_PATH" ]; then
    echo "Project path does not exist: $PROJECT_PATH" >&2
    exit 2
  fi

  project_rules=$(get_workflow_file 'templates/AGENTS.md')
  claude_shim=$(get_workflow_file 'templates/CLAUDE.md')
  set_managed_block "$PROJECT_PATH/AGENTS.md" "$project_rules"
  set_managed_block "$PROJECT_PATH/CLAUDE.md" "$claude_shim"

  if [ "$INSTALL_OPENCODE_REVIEWER" -eq 1 ]; then
    reviewer_path="$PROJECT_PATH/.opencode/agents/review.md"
    if [ -e "$reviewer_path" ]; then
      printf 'Skipped existing OpenCode reviewer: %s\n' "$reviewer_path" >&2
    else
      mkdir -p "$(dirname -- "$reviewer_path")"
      reviewer_template=$(get_workflow_file 'templates/opencode-reviewer.md')
      cp "$reviewer_template" "$reviewer_path"
      printf 'Created %s\n' "$reviewer_path"
    fi
  fi
fi

echo 'Agent Review Workflow installation complete.'
