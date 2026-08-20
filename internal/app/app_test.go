//go:build integration

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jokerD888/agent-review-workflow/internal/ledger"
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
	a, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", WorktreePath: filepath.Join(root, "feature-a")})
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
	if _, _, err := approveCurrent(t, svc, "feature-b"); err == nil {
		t.Fatal("Approve(feature-b) succeeded before parent approval")
	}
	if _, _, err := approveCurrent(t, svc, "feature-a"); err != nil {
		t.Fatalf("Approve(feature-a) error = %v", err)
	}
	after, err := svc.PrepareReview("feature-b")
	if err != nil {
		t.Fatal(err)
	}
	if after.DependencyStatus != review.DependencyClear {
		t.Fatalf("dependency after parent approval = %s", after.DependencyStatus)
	}
	if _, _, err := approveCurrent(t, svc, "feature-b"); err != nil {
		t.Fatalf("Approve(feature-b) error = %v", err)
	}
	entry, err := svc.Task("feature-b")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Lifecycle != task.ReadyForReview || entry.Review.Status != task.ReviewApproved {
		t.Fatalf("approved task = lifecycle %s review %s", entry.Lifecycle, entry.Review.Status)
	}
	merged, err := svc.Merge("feature-b")
	if err != nil {
		t.Fatal(err)
	}
	if merged.TargetBranch != "arw/feature-a" || merged.Task.Lifecycle != task.Merged {
		t.Fatalf("child merge result = %#v", merged)
	}
	parentAfterMerge, err := svc.Task("feature-a")
	if err != nil {
		t.Fatal(err)
	}
	parentSnapshot, err := svc.PrepareReview("feature-a")
	if err != nil {
		t.Fatal(err)
	}
	if parentAfterMerge.Lifecycle != task.ReadyForReview || parentAfterMerge.Review.Status != task.ReviewApproved || parentSnapshot.ApprovalValidity != review.ApprovalStale {
		t.Fatalf("parent after child merge = lifecycle %s review %s", parentAfterMerge.Lifecycle, parentAfterMerge.Review.Status)
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
	started, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(started.Worktree); err != nil {
		t.Fatal(err)
	}
	if _, _, err := approveCurrent(t, svc, "feature-a"); err == nil || !strings.Contains(err.Error(), "current status: unknown") {
		t.Fatalf("Approve() error = %v, want unavailable worktree status", err)
	}
}

func TestPrepareReviewDoesNotWriteRegistry(t *testing.T) {
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
	if _, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", WorktreePath: filepath.Join(root, "feature-a")}); err != nil {
		t.Fatal(err)
	}
	before, err := svc.Git.Resolve(ledger.RegistryBranch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PrepareReview("feature-a"); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Git.Resolve(ledger.RegistryBranch)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("PrepareReview() moved registry from %s to %s", before, after)
	}
}

func TestApproveRejectsHeadChangedSinceReview(t *testing.T) {
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
	started, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, started.Worktree, "commit", "--allow-empty", "-m", "reviewed version")
	snapshot, err := svc.PrepareReview(started.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, started.Worktree, "commit", "--allow-empty", "-m", "unreviewed follow-up")
	if _, _, err := svc.Approve(started.Task.ID, snapshot.Base.SHA, snapshot.Head.SHA); err == nil || !strings.Contains(err.Error(), "reviewed version changed") {
		t.Fatalf("Approve() error = %v, want reviewed version mismatch", err)
	}
}

func TestTaskLifecycle(t *testing.T) {
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
	started, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := svc.MarkReady(started.Task.ID)
	if err != nil || entry.Lifecycle != task.ReadyForReview {
		t.Fatalf("MarkReady() = %#v, %v", entry, err)
	}
	if _, err := svc.Merge(started.Task.ID); err == nil {
		t.Fatal("Merge() succeeded before approval")
	}
	if _, _, err := approveCurrent(t, svc, started.Task.ID); err != nil {
		t.Fatal(err)
	}
	merged, err := svc.Merge(started.Task.ID)
	entry = merged.Task
	if err != nil || entry.Lifecycle != task.Merged {
		t.Fatalf("Merge() = %#v, %v", entry, err)
	}
	if _, _, err := approveCurrent(t, svc, started.Task.ID); err == nil {
		t.Fatal("Approve() succeeded for merged task")
	}
	if _, err := svc.RequestChanges(started.Task.ID, "too late"); err == nil {
		t.Fatal("RequestChanges() succeeded for merged task")
	}

	abandoned, err := svc.Start(StartOptions{Title: "Feature B", ID: "feature-b", WorktreePath: filepath.Join(root, "feature-b")})
	if err != nil {
		t.Fatal(err)
	}
	entry, err = svc.Abandon(abandoned.Task.ID)
	if err != nil || entry.Lifecycle != task.Abandoned {
		t.Fatalf("Abandon() = %#v, %v", entry, err)
	}
}

func TestMergeApprovedTaskFastForwardsRecordedBase(t *testing.T) {
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
	started, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, started.Worktree, "commit", "--allow-empty", "-m", "feature a")
	taskHead, err := svc.Git.Resolve(started.Task.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := approveCurrent(t, svc, started.Task.ID); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Merge(started.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	mainHead, err := svc.Git.Resolve("main")
	if err != nil {
		t.Fatal(err)
	}
	if mainHead != taskHead || result.HeadSHA != taskHead || result.TargetBranch != "main" {
		t.Fatalf("merge result = %#v, main = %s, task = %s", result, mainHead, taskHead)
	}
	if result.Task.Lifecycle != task.Merged {
		t.Fatalf("merged task lifecycle = %s", result.Task.Lifecycle)
	}
}

func TestMergeRejectsTargetThatMovedAfterApproval(t *testing.T) {
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
	started, err := svc.Start(StartOptions{Title: "Feature A", ID: "feature-a", WorktreePath: filepath.Join(root, "feature-a")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, started.Worktree, "commit", "--allow-empty", "-m", "feature a")
	if _, _, err := approveCurrent(t, svc, started.Task.ID); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "commit", "--allow-empty", "-m", "main advanced")
	if _, err := svc.Merge(started.Task.ID); err == nil || !strings.Contains(err.Error(), "approval is no longer current") {
		t.Fatalf("Merge() error = %v, want stale approval rejection", err)
	}
	entry, err := svc.Task(started.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Lifecycle != task.ReadyForReview {
		t.Fatalf("task lifecycle after rejected merge = %s", entry.Lifecycle)
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
	parent, err := svc.Start(StartOptions{Title: "Parent", ID: "parent", WorktreePath: filepath.Join(root, "parent")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, parent.Worktree, "commit", "--allow-empty", "-m", "parent work")
	child, err := svc.Start(StartOptions{Title: "Child", ID: "child", ParentTask: "parent", WorktreePath: filepath.Join(root, "child")})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, child.Worktree, "commit", "--allow-empty", "-m", "child work")
	if _, _, err := approveCurrent(t, svc, "parent"); err != nil {
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

func approveCurrent(t *testing.T, svc Service, id string) (task.Task, review.Snapshot, error) {
	t.Helper()
	entry, err := svc.Task(id)
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	if entry.Lifecycle == task.Active {
		if _, err := svc.MarkReady(id); err != nil {
			return task.Task{}, review.Snapshot{}, err
		}
	}
	snapshot, err := svc.PrepareReview(id)
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	return svc.Approve(id, snapshot.Base.SHA, snapshot.Head.SHA)
}
