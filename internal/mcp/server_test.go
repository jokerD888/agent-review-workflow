package mcp

import "testing"

func TestDefinitionsExposeOnlyConstrainedTools(t *testing.T) {
	forbidden := map[string]bool{"shell": true, "run_arw": true, "git_push": true, "git_merge": true, "git_rebase": true, "git_reset": true, "delete_worktree": true}
	seen := map[string]bool{}
	for _, definition := range definitions() {
		name, ok := definition["name"].(string)
		if !ok || name == "" {
			t.Fatal("tool definition has no name")
		}
		if forbidden[name] {
			t.Fatalf("forbidden tool %q is exposed", name)
		}
		seen[name] = true
	}
	for _, required := range []string{"workflow_context", "workflow_list_tasks", "workflow_start_task", "workflow_prepare_review", "workflow_open_review", "workflow_park_task", "workflow_resume_task", "workflow_mark_ready", "workflow_approve_task", "workflow_request_changes", "workflow_merge_task", "workflow_mark_merged", "workflow_abandon_task", "workflow_refresh"} {
		if !seen[required] {
			t.Errorf("required tool %q is absent", required)
		}
	}
}
