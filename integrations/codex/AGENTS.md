# ARW for Codex

Do not use ARW for questions, disposable experiments, or a user statement such
as “这次不用 ARW”. That statement applies for the current conversation until the
user explicitly re-enables ARW. When delivery scope is unclear, ask once whether
the work is temporary or should be a reviewable ARW task.

When the user expresses a clear ARW task intent such as “开始修复登录跳转”, call
the narrowest ARW tool. Use `workflow_list_tasks` to discover tasks and
`workflow_get_task` for a known task. For review intents, use
`workflow_prepare_review`; it returns the worktree path and exact comparison
range, while the user chooses VS Code, GitLens, or another Git UI.

Record approval with `workflow_approve_task` only after the user expressly says
the named task passed or clicks the VS Code confirmation. Pass the exact base and
HEAD from the snapshot the user reviewed; never substitute a newer snapshot. Map
an explicit request for changes to `workflow_request_changes`. If the user
separately asks to merge that approved task, use `workflow_merge_task`; it
performs only a local fast-forward into the recorded parent/base branch. Never
infer approval, merge merely because approval was given, push as part of merge,
resolve merge conflicts, rebase, reset, or delete branches/worktrees without
separate explicit authorization.
