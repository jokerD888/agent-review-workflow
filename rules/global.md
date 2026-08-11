# Personal AI development workflow

## Git safety

- Before editing a Git repository, inspect the current branch and working tree.
- Work only on the current task branch. Do not edit `main`, `master`, or a
  protected/release branch.
- Do not switch, create, delete, merge, rebase, or rename branches unless the
  user explicitly asks.
- Do not push, force-push, open a pull request, or change remote configuration
  unless the user explicitly asks.
- Do not run commands that discard work, including `git reset --hard`,
  `git clean`, or `git checkout --`, unless the user explicitly asks.
- Preserve unrelated changes already present in the working tree. Stage only
  files and hunks created for the current task; ask when the boundary is unclear.

## Change scope and checkpoints

- Make the smallest reasonable change that satisfies the request.
- Do not reformat unrelated code or update unrelated dependencies.
- Create a local commit only after an independently understandable, buildable or
  testable unit of work is complete. Do not commit merely because one response
  ended.
- Use explicit staging. Never use `git commit -a` for an AI checkpoint.
- Do not amend, squash, or rewrite earlier AI checkpoint commits. Add a follow-up
  commit when a correction is needed so the review trail remains visible.
- If the user requests no commits, leave work uncommitted and still provide the
  review summary.

## Verification and reporting

- Run the narrowest relevant formatter, build, type-check, lint, and/or tests.
- Never claim a check passed unless it was actually run; state skipped checks and
  the reason.
- Call out public API, authentication, authorization, security, database/data
  migration, concurrency, dependency, and release-impacting changes.
- At task completion report: purpose, files changed, verification results, local
  commit hash(es), remaining uncommitted changes, and known risks or follow-ups.

## Review boundary

- Do not interrupt the user for review after every small edit.
- Expect the user to review the accumulated task-branch diff before merge.
- The user retains final approval for merge, push, release, credential changes,
  destructive operations, and externally visible changes.

## ARW v2 task workflow

- When `arw` and its typed `arw-mcp` tools are available in a Git repository,
  read `workflow_context` before starting AI work on a named task.
- Map a clear user intent such as “开始修复登录跳转” only to
  `workflow_start_task`; map review requests to `workflow_prepare_review`.
  Never pass arbitrary shell text to ARW.
- For “在 VS Code 中审查 …”, prepare the review first, then use
  `workflow_open_review` with a new window. Do not change the current VS Code
  window's branch or worktree.
- If a task depends on an unapproved parent, explain that reviewing it is
  conditional and recommend reviewing the parent first. The user may still
  choose to review the child.
- A user saying “搁置” or “恢复” is sufficient intent for the corresponding
  typed task operation. A user saying “审查通过” must be treated as a human
  conclusion only; it never authorizes merge or push.
