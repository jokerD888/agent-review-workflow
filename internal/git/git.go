package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Client struct{ Root string }

func Discover(dir string) (Client, error) {
	output, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Client{}, fmt.Errorf("not inside a Git worktree: %w", err)
	}
	return Client{Root: strings.TrimSpace(output)}, nil
}

func (c Client) Run(args ...string) (string, error) { return run(c.Root, args...) }

// RunWithInput runs Git without invoking a shell. It is used by the ledger's
// plumbing path so registry writes never need to check out product files.
func (c Client) RunWithInput(input string, environment []string, args ...string) (string, error) {
	return runWithInput(c.Root, input, environment, args...)
}

func (c Client) RequireClean() error {
	output, err := c.Run("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) != "" {
		return fmt.Errorf("working tree is not clean; commit, stash, or explicitly resolve it before creating a task")
	}
	return nil
}

func (c Client) Resolve(ref string) (string, error) {
	output, err := c.Run("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("cannot resolve Git ref %q: %w", ref, err)
	}
	return strings.TrimSpace(output), nil
}

func (c Client) BranchExists(branch string) bool {
	_, err := c.Run("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}
func (c Client) CommonDir() (string, error) {
	output, err := c.Run("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(strings.TrimSpace(output)) {
		return strings.TrimSpace(output), nil
	}
	return filepath.Clean(filepath.Join(c.Root, strings.TrimSpace(output))), nil
}

func run(dir string, args ...string) (string, error) {
	return runWithInput(dir, "", nil, args...)
}

func runWithInput(dir, input string, environment []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, environment...)
	// Supplying an explicit EOF prevents plumbing commands such as update-index
	// from inheriting a terminal and waiting for interactive input on Windows.
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("git %s: timed out after 30 seconds", strings.Join(args, " "))
		}
		return "", fmt.Errorf("git %s: %w%s", strings.Join(args, " "), err, formatStderr(stderr.String()))
	}
	return stdout.String(), nil
}

func formatStderr(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return ": " + strings.TrimSpace(value)
}
