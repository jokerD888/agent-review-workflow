//go:build integration

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jokerD888/agent-review-workflow/internal/review"
	"github.com/jokerD888/agent-review-workflow/internal/task"
)

func TestStackedTasksRequireParentApproval(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "--initial-branch=main")
	runGit(t, repo, "config", "user.name", "ARW Test")
	runGit(t, repo, "config", "user.email", "arw-test@example.invalid")
	runGit(t, repo, "commit", "--allow-empty", "-m", "initial")
	svc, err := New(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Setup(); err != nil {
		t.Fatal(err)
	}
	a, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", Kind: "feature", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, a.Worktree, "commit", "--allow-empty", "-m", "feature a")
	b, err := svc.Start(StartOptions{Title: "Feature B", ID: "feature-b", ParentTask: "feature-a", WorktreePath: filepath.Join(root, "feature-b")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, b.Worktree, "commit", "--allow-empty", "-m", "feature b")
	before, err := svc.PrepareReview("feature-b")
	if err != nil {
		t.Fatal(err)
	}
	if before.DependencyStatus != review.AwaitingPrerequisite {
		t.Fatalf("dependency before parent approval = %s", before.DependencyStatus)
	}
	if _, _, err := svc.Approve("feature-b"); err == nil {
		t.Fatal("Approve(feature-b) succeeded before parent approval")
	}
	if _, _, err := svc.Approve("feature-a"); err != nil {
		t.Fatalf("Approve(feature-a) error = %v", err)
	}
	after, err := svc.PrepareReview("feature-b")
	if err != nil {
		t.Fatal(err)
	}
	if after.DependencyStatus != review.DependencyClear {
		t.Fatalf("dependency after parent approval = %s", after.DependencyStatus)
	}
	if _, _, err := svc.Approve("feature-b"); err != nil {
		t.Fatalf("Approve(feature-b) error = %v", err)
	}
	entry, err := svc.Task("feature-b")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Lifecycle != task.Approved || entry.Review.Status != task.ReviewApproved {
		t.Fatalf("approved task = lifecycle %s review %s", entry.Lifecycle, entry.Review.Status)
	}
}

func TestApproveRequiresKnownCleanWorktree(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "--initial-branch=main")
	runGit(t, repo, "config", "user.name", "ARW Test")
	runGit(t, repo, "config", "user.email", "arw-test@example.invalid")
	runGit(t, repo, "commit", "--allow-empty", "-m", "initial")
	svc, err := New(repo)
	if err != nil {
		t.Fatal(err)
	}
	started, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", Kind: "feature", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(started.Worktree); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Approve("feature-a"); err == nil || !strings.Contains(err.Error(), "current status: unknown") {
		t.Fatalf("Approve() error = %v, want unavailable worktree status", err)
	}
}

func TestLifecycleAndTestEvidence(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "--initial-branch=main")
	runGit(t, repo, "config", "user.name", "ARW Test")
	runGit(t, repo, "config", "user.email", "arw-test@example.invalid")
	runGit(t, repo, "commit", "--allow-empty", "-m", "initial")
	svc, err := New(repo)
	if err != nil {
		t.Fatal(err)
	}
	started, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", Kind: "feature", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := svc.RecordTest(started.Task.ID, task.TestEvidence{Command: "go test ./...", Result: "passed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Tests) != 1 || entry.Tests[0].RecordedAt == "" {
		t.Fatalf("recorded test evidence = %#v", entry.Tests)
	}
	entry, err = svc.MarkReady(started.Task.ID)
	if err != nil || entry.Lifecycle != task.ReadyForReview {
		t.Fatalf("MarkReady() = %#v, %v", entry, err)
	}
	if _, err := svc.MarkMerged(started.Task.ID); err == nil {
		t.Fatal("MarkMerged() succeeded before approval")
	}
	if _, _, err := svc.Approve(started.Task.ID); err != nil {
		t.Fatal(err)
	}
	entry, err = svc.MarkMerged(started.Task.ID)
	if err != nil || entry.Lifecycle != task.Merged {
		t.Fatalf("MarkMerged() = %#v, %v", entry, err)
	}

	abandoned, err := svc.Start(StartOptions{Title: "Feature B", ID: "feature-b", Kind: "feature", WorktreePath: filepath.Join(root, "feature-b")})
	if err != nil {
		t.Fatal(err)
	}
	entry, err = svc.Abandon(abandoned.Task.ID)
	if err != nil || entry.Lifecycle != task.Abandoned {
		t.Fatalf("Abandon() = %#v, %v", entry, err)
	}
	if _, err := svc.RecordTest(abandoned.Task.ID, task.TestEvidence{Command: "go test ./...", Result: "passed"}); err == nil {
		t.Fatal("RecordTest() succeeded for an abandoned task")
	}
}

func TestChildReviewRequiresReapprovalWhenParentChanges(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "--initial-branch=main")
	runGit(t, repo, "config", "user.name", "ARW Test")
	runGit(t, repo, "config", "user.email", "arw-test@example.invalid")
	runGit(t, repo, "commit", "--allow-empty", "-m", "initial")
	svc, err := New(repo)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := svc.Start(StartOptions{Title: "Parent", ID: "parent", Kind: "feature", WorktreePath: filepath.Join(root, "parent")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, parent.Worktree, "commit", "--allow-empty", "-m", "parent work")
	child, err := svc.Start(StartOptions{Title: "Child", ID: "child", Kind: "feature", ParentTask: "parent", WorktreePath: filepath.Join(root, "child")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, child.Worktree, "commit", "--allow-empty", "-m", "child work")
	if _, _, err := svc.Approve("parent"); err != nil {
		t.Fatal(err)
	}
	runGit(t, parent.Worktree, "commit", "--allow-empty", "-m", "parent follow-up")
	snapshot, err := svc.PrepareReview("child")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.DependencyStatus != review.AwaitingPrerequisite {
		t.Fatalf("dependency after parent changed = %s", snapshot.DependencyStatus)
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
