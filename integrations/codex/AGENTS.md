# ARW for Codex

Do not use ARW for questions, disposable experiments, or a user statement such
as “这次不用 ARW”. That statement applies for the current conversation until the
user explicitly re-enables ARW. When delivery scope is unclear, ask once whether
the work is temporary or should be a reviewable ARW task.

When the user expresses a clear ARW task intent such as “开始修复登录跳转”, first
call `workflow_context`, then call the narrowest ARW tool. For review intents, use
`workflow_prepare_review`; only call `workflow_open_review` with `new_window:
true` when the user explicitly asks to open VS Code.

Never represent a task tool as permission to merge, push, rebase, reset, delete
branches, or delete worktrees. Human approval is recorded only after the user
expressly says it passed or clicks the VS Code confirmation.
