#!/usr/bin/env sh
set -eu

base_branch=${1:-}
git rev-parse --show-toplevel >/dev/null

if [ -z "$base_branch" ]; then
  for candidate in main master; do
    if git show-ref --verify --quiet "refs/heads/$candidate"; then
      base_branch=$candidate
      break
    fi
  done
fi

if [ -z "$base_branch" ]; then
  echo 'Could not find main or master. Pass the base branch as the first argument.' >&2
  exit 2
fi

git merge-base "$base_branch" HEAD >/dev/null
printf 'Reviewing changes introduced by HEAD against %s\n' "$base_branch"
printf '\n=== Diff stat ===\n'
git diff --stat "$base_branch...HEAD"
printf '\n=== Checkpoint commits ===\n'
git log --reverse --oneline "$base_branch..HEAD"
printf '\n=== Whitespace check ===\n'
git diff --check "$base_branch...HEAD" || {
  echo 'Whitespace errors found. Review them before merge.' >&2
}
