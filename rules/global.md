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
- The user retains final decision authority for approval, merge, push, release,
  credential changes, destructive operations, and externally visible changes.
  The agent may carry out an approved local action only after that intent is
  explicit; decision authority does not require the user to operate the CLI.

## ARW v2 task workflow

- ARW is opt-in by intent, not mandatory for every repository or conversation.
  Do not create an ARW task merely because a Git repository is open or the user
  named a subject.
- Treat questions, explanation, investigation, throwaway experiments, and
  clearly disposable one-off tools as non-ARW work by default.
- Treat a clear delivery/review intent (for example “开始修复…”, “新增功能…”,
  “这个要审查”) as an ARW task intent. When the scope is genuinely unclear,
  ask one short question before creating a task: “这是临时处理，还是要作为可审查
  任务纳入 ARW？” Do not repeatedly ask after the user has answered.
- “这次不用 ARW”, “不建任务”, “临时处理”, or equivalent wording disables ARW
  for the current conversation until the user explicitly asks to use/start ARW.
  This only skips ARW bookkeeping; all Git safety rules still apply.
- When `arw` and its typed `arw-mcp` tools are available and the user has chosen
  ARW for a named task, read `workflow_get_task` before starting AI work. Use
  `workflow_list_tasks` only when the task is not yet known.
- Map a clear user intent such as “开始修复登录跳转” only to
  `workflow_start_task`; map review requests to `workflow_prepare_review`.
  Never pass arbitrary shell text to ARW.
- For “在 VS Code 中审查 …”, prepare the review first, then return its worktree
  path and exact base/head range. Do not change the current VS Code window's
  branch or worktree; the user chooses the editor or Git UI.
- If a task depends on an unapproved parent, explain that it may be inspected
  but cannot receive final approval; recommend reviewing the parent first.
- A user saying “搁置” or “恢复” is sufficient intent for the corresponding
  typed task operation. When the user explicitly says a named task “审查通过”,
  call `workflow_approve_task` with the exact base and HEAD from the snapshot the
  user reviewed; never substitute a newer snapshot or infer approval from the
  agent's own implementation or review. “审查完毕” without a result is ambiguous,
  so ask whether it passed or needs changes. If the user explicitly requests
  changes, call `workflow_request_changes` and preserve the user's reason.
- Approval does not itself authorize merge or push. When the user separately
  asks to merge an approved task, call `workflow_merge_task`; it may only perform
  the tool's local fast-forward merge. Never treat merge permission as push
  permission, and never resolve a merge conflict without a new explicit request.
