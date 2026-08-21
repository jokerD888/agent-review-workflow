# ARW for Codex

Do not use ARW for questions, disposable experiments, or a user statement such
as "这次不用 ARW". That statement applies for the current conversation until the
user explicitly re-enables ARW. When delivery scope is unclear, ask once whether
the work is temporary or should be a reviewable ARW task.

When the user expresses a clear ARW task intent such as "开始修复登录跳转",
briefly propose the intended task id, branch (`arw/<id>`), and worktree path,
let the user confirm or rename, then call `workflow_start_task`. Do not create
a task silently. Use `workflow_list_tasks` to discover tasks and
`workflow_get_task` for a known task. For review intents, use
`workflow_prepare_review`; it returns the worktree path and exact comparison
range, while the user chooses VS Code, GitLens, or another Git UI.

Record approval with `workflow_approve_task` only after the user expressly says
the named task passed or clicks the VS Code confirmation. Pass the exact base and
HEAD from the snapshot the user reviewed; never substitute a newer snapshot. Map
an explicit request for changes to `workflow_request_changes`. If the user
separately asks to merge that approved task, use `workflow_merge_task`; it
performs only a local fast-forward into the recorded parent/base branch. When
the parent has already been merged, the merge target walks up the parent chain
to the first non-merged ancestor or the recorded base. A user saying "clear"
for a merged or abandoned task maps to `workflow_clear_task`; "clear all merged"
maps to `workflow_clear_merged`; clearing preserves the registry record. Never
infer approval, merge merely because approval was given, push as part of merge,
resolve merge conflicts, rebase, reset, or delete branches/worktrees without
separate explicit authorization.
