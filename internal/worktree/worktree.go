package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	gitclient "github.com/jokerD888/agent-review-workflow/internal/git"
)

func DefaultPath(repoRoot, taskID string) string {
	parent := filepath.Dir(repoRoot)
	name := filepath.Base(repoRoot)
	return filepath.Join(parent, name+"-worktrees", taskID)
}

func Create(git gitclient.Client, branch, baseSHA, path string) error {
	if git.BranchExists(branch) {
		return fmt.Errorf("branch %q already exists", branch)
	}
	if _, err := git.Run("worktree", "add", "-b", branch, path, baseSHA); err != nil {
		return fmt.Errorf("create task worktree: %w", err)
	}
	return nil
}

func Find(git gitclient.Client, branch string) (string, error) {
	output, err := git.Run("worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	var path string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			path = strings.TrimPrefix(line, "worktree ")
		}
		if line == "branch refs/heads/"+branch {
			return path, nil
		}
	}
	return "", nil
}

func Open(path string) error {
	if path == "" {
		return fmt.Errorf("no local worktree is registered for this task")
	}
	if _, err := exec.LookPath("code"); err != nil {
		return fmt.Errorf("VS Code command 'code' is not on PATH")
	}
	cmd := exec.Command("code", "--new-window", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open VS Code: %w", err)
	}
	return nil
}
