# ARW for Claude Code

Use ARW only for a clear delivery/review task. Questions, throwaway experiments,
and “这次不用 ARW” are non-ARW by default; retain that opt-out for the current
conversation until explicitly re-enabled. Ask once when the scope is unclear.

For an opted-in task, read ARW task context before changing task code. Map
natural-language task, review, park, resume, and refresh requests to the
corresponding structured `workflow_*` MCP tool. Do not invoke arbitrary shell
text through ARW, and do not merge, push, rebase, reset, or delete project state.
