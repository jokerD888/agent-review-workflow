#!/usr/bin/env sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo 'Usage: ./scripts/start-task.sh <task-name> [base-branch]' >&2
  exit 2
fi

task_name=$1
base_branch=${2:-main}

git rev-parse --show-toplevel >/dev/null
if [ -n "$(git status --porcelain)" ]; then
  echo 'Working tree is not clean. Commit, stash, or review existing changes first.' >&2
  exit 1
fi

git show-ref --verify --quiet "refs/heads/$base_branch"
slug=$(printf '%s' "$task_name" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9][^a-z0-9]*/-/g; s/^-//; s/-$//')
if [ -z "$slug" ]; then
  echo 'Task name must contain letters or numbers.' >&2
  exit 2
fi

branch_name="ai/$slug"
if git show-ref --verify --quiet "refs/heads/$branch_name"; then
  echo "Branch already exists: $branch_name" >&2
  exit 1
fi

git switch "$base_branch"
git switch -c "$branch_name"
printf 'Ready for AI work on %s\n' "$branch_name"
