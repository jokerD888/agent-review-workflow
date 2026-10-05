package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runWithIO([]string{"version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWithIO(version) error = %v", err)
	}
	if strings.TrimSpace(stdout.String()) != version {
		t.Fatalf("version output = %q, want %q", stdout.String(), version)
	}

	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"version", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWithIO(version --json) error = %v", err)
	}
	var data map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}
	if data["version"] != version {
		t.Fatalf("version JSON = %q, want %q", data["version"], version)
	}

	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"--json", "version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWithIO(--json version) error = %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}
	if data["version"] != version {
		t.Fatalf("version JSON = %q, want %q", data["version"], version)
	}
}

func TestHelpOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runWithIO([]string{"help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWithIO(help) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help output missing Usage: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--json") {
		t.Fatalf("help output missing --json documentation: %q", stdout.String())
	}
}

func TestCLIInGitRepo(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "ARW Test")
	runGit(t, root, "config", "user.email", "arw-test@example.invalid")
	runGit(t, root, "commit", "--allow-empty", "-m", "initial")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	// 1. setup --json
	var stdout, stderr bytes.Buffer
	err = runWithIO([]string{"setup", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("setup --json error = %v", err)
	}
	var setupResp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &setupResp); err != nil {
		t.Fatalf("parse setup JSON error = %v", err)
	}
	if setupResp["status"] != "ok" {
		t.Fatalf("setup status = %v, want ok", setupResp["status"])
	}

	// 2. doctor --json
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"doctor", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("doctor --json error = %v", err)
	}
	var docResp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &docResp); err != nil {
		t.Fatalf("parse doctor JSON error = %v", err)
	}
	if docResp["registryExists"] != true {
		t.Fatalf("doctor registryExists = %v, want true", docResp["registryExists"])
	}

	// 3. task start --json
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"task", "start", "--id", "cli-test", "CLI Test Task", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("task start --json error = %v", err)
	}
	var startResp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &startResp); err != nil {
		t.Fatalf("parse task start JSON error = %v", err)
	}
	taskMap, ok := startResp["task"].(map[string]any)
	if !ok || taskMap["id"] != "cli-test" {
		t.Fatalf("unexpected task start JSON response: %v", startResp)
	}

	// 4. task list --json
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"task", "list", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("task list --json error = %v", err)
	}
	var listResp []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &listResp); err != nil {
		t.Fatalf("parse task list JSON error = %v", err)
	}
	if len(listResp) != 1 || listResp[0]["id"] != "cli-test" {
		t.Fatalf("task list = %v, want 1 task with id cli-test", listResp)
	}

	// 5. task show --json
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"task", "show", "cli-test", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("task show --json error = %v", err)
	}
	var showResp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &showResp); err != nil {
		t.Fatalf("parse task show JSON error = %v", err)
	}
	if showResp["id"] != "cli-test" || showResp["lifecycle"] != "active" {
		t.Fatalf("task show = %v", showResp)
	}

	// 6. task park --json & resume --json
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"task", "park", "cli-test", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("task park --json error = %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"task", "resume", "cli-test", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("task resume --json error = %v", err)
	}

	// 7. task ready --json
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"task", "ready", "cli-test", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("task ready --json error = %v", err)
	}

	// 8. review prepare --json
	stdout.Reset()
	stderr.Reset()
	err = runWithIO([]string{"review", "prepare", "cli-test", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("review prepare --json error = %v", err)
	}
	var snapResp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &snapResp); err != nil {
		t.Fatalf("parse review prepare JSON error = %v", err)
	}
	if snapResp["taskId"] != "cli-test" {
		t.Fatalf("snapshot taskId = %v, want cli-test", snapResp["taskId"])
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
