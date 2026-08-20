# ARW for Claude Code

Use ARW only for a clear delivery/review task. Questions, throwaway experiments,
and “这次不用 ARW” are non-ARW by default; retain that opt-out for the current
conversation until explicitly re-enabled. Ask once when the scope is unclear.

For an opted-in task, use `workflow_get_task` for a known task before changing
its code, or `workflow_list_tasks` to discover tasks. Map natural-language
task, review, park, and resume requests to the corresponding structured
`workflow_*` MCP tool. Record approval only when the
user explicitly says the named task passed, using the exact base and HEAD from
the snapshot the user reviewed; never substitute a newer snapshot. Record
requested changes only from the user's explicit conclusion. A separate explicit merge request may use
`workflow_merge_task`, which performs only a local fast-forward into the recorded
parent/base branch. Do not invoke arbitrary shell text through ARW; never infer
approval, push as part of merge, resolve conflicts, rebase, reset, or delete
project state without separate explicit authorization.
