# ARW for OpenCode

Use the local `arw-mcp` server's typed `workflow_*` tools only for clear ARW
intents. Questions, throwaway work, and a user statement such as "这次不用 ARW"
are non-ARW for the current conversation; ask once only if delivery scope is
unclear. For an opted-in task, read `workflow_get_task`; before calling
`workflow_start_task`, briefly propose the intended task id, branch
(`arw/<id>`), and worktree path and let the user confirm or rename; do not
create a task silently. For a dependent child task, explain that it can be
inspected but cannot receive final approval until the parent is approved. Never
treat the agent's own judgment as approval. Use `workflow_approve_task` only for
the user's explicit approval and the exact reviewed base/HEAD; never substitute a
newer snapshot. Use `workflow_request_changes` only for an explicit
negative conclusion, and `workflow_merge_task` only for a separate explicit
merge request. A merge is local and fast-forward-only; it never implies push
permission or permission to resolve conflicts or perform destructive Git actions.
When the parent has already been merged, the merge target walks up the parent
chain. A user saying "clear" for a merged or abandoned task maps to
`workflow_clear_task`; "clear all merged" maps to `workflow_clear_merged`;
clearing preserves the registry record.
