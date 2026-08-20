package worktree

import (
	"fmt"
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
