package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	gitclient "github.com/jokerD888/agent-review-workflow/internal/git"
	"github.com/jokerD888/agent-review-workflow/internal/ledger"
	"github.com/jokerD888/agent-review-workflow/internal/review"
	"github.com/jokerD888/agent-review-workflow/internal/task"
	"github.com/jokerD888/agent-review-workflow/internal/worktree"
)

type Service struct {
	Git    gitclient.Client
	Ledger ledger.Store
}

func New(dir string) (Service, error) {
	client, err := gitclient.Discover(dir)
	if err != nil {
		return Service{}, err
	}
	return Service{Git: client, Ledger: ledger.Store{Git: client}}, nil
}

type StartOptions struct{ Title, ID, BaseRef, ParentTask, WorktreePath string }

type StartResult struct {
	Task     task.Task `json:"task"`
	Worktree string    `json:"worktree"`
}

type MergeResult struct {
	Task         task.Task `json:"task"`
	SourceBranch string    `json:"sourceBranch"`
	TargetBranch string    `json:"targetBranch"`
	HeadSHA      string    `json:"headSHA"`
}

func (s Service) Setup() error { return s.Ledger.Setup() }

func (s Service) Start(options StartOptions) (StartResult, error) {
	if err := s.Git.RequireClean(); err != nil {
		return StartResult{}, err
	}
	if err := s.Ledger.Setup(); err != nil {
		return StartResult{}, err
	}
	id := options.ID
	if id == "" {
		id = Slug(options.Title)
	}
	if !task.ValidID(id) {
		return StartResult{}, fmt.Errorf("task id %q is invalid; use lowercase letters, numbers, and hyphens", id)
	}
	if _, err := s.Ledger.Get(id); err == nil {
		return StartResult{}, fmt.Errorf("task %q already exists", id)
	}
	baseRef := options.BaseRef
	if baseRef == "" {
		baseRef = "main"
	}
	baseSHA, err := s.Git.Resolve(baseRef)
	if err != nil {
		return StartResult{}, err
	}
	parent := options.ParentTask
	if parent != "" {
		parentEntry, err := s.Ledger.Get(parent)
		if err != nil {
			return StartResult{}, fmt.Errorf("load parent task: %w", err)
		}
		baseRef = parentEntry.Branch
		baseSHA, err = s.Git.Resolve(baseRef)
		if err != nil {
			return StartResult{}, err
		}
	}
	branch := "arw/" + id
	path := options.WorktreePath
	if path == "" {
		path = worktree.DefaultPath(s.Git.Root, id)
	}
	if _, err := os.Stat(path); err == nil {
		return StartResult{}, fmt.Errorf("worktree path already exists: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return StartResult{}, err
	}
	if err := worktree.Create(s.Git, branch, baseSHA, path); err != nil {
		return StartResult{}, err
	}
	now := time.Now().Format(time.RFC3339)
	entry := task.Task{SchemaVersion: task.SchemaVersion, ID: id, Title: options.Title, Branch: branch, Base: task.Base{Ref: baseRef, SHA: baseSHA}, ParentTask: parent, Lifecycle: task.Active, Review: task.Review{Status: task.ReviewNone}, CreatedAt: now, UpdatedAt: now}
	if err := s.Ledger.Save(entry, "chore(arw): create task "+id); err != nil {
		return StartResult{}, fmt.Errorf("task worktree was created, but saving its registry record failed: %w", err)
	}
	return StartResult{Task: entry, Worktree: path}, nil
}

func (s Service) Tasks() ([]task.Task, error)       { return s.Ledger.List() }
func (s Service) Task(id string) (task.Task, error) { return s.Ledger.Get(id) }

func (s Service) Park(id string) (task.Task, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	if entry.Lifecycle == task.Merged || entry.Lifecycle == task.Abandoned {
		return task.Task{}, fmt.Errorf("cannot park %s task", entry.Lifecycle)
	}
	entry.Lifecycle = task.Parked
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): park task "+id); err != nil {
		return task.Task{}, err
	}
	return entry, nil
}
func (s Service) Resume(id string) (task.Task, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	if entry.Lifecycle != task.Parked {
		return task.Task{}, fmt.Errorf("task %q is not parked", id)
	}
	entry.Lifecycle = task.Active
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): resume task "+id); err != nil {
		return task.Task{}, err
	}
	return entry, nil
}

func (s Service) MarkReady(id string) (task.Task, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	if entry.Lifecycle != task.Active {
		return task.Task{}, fmt.Errorf("task %q cannot be marked ready from %s", id, entry.Lifecycle)
	}
	entry.Lifecycle = task.ReadyForReview
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): mark task ready "+id); err != nil {
		return task.Task{}, err
	}
	return entry, nil
}

// Merge fast-forwards an approved task into its recorded parent/base branch.
// The target is derived from task metadata so an agent cannot choose an
// arbitrary branch. A non-fast-forward merge is refused because it would
// create an integration result that was not part of the approved snapshot.
func (s Service) Merge(id string) (MergeResult, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return MergeResult{}, err
	}
	if entry.Lifecycle != task.ReadyForReview || entry.Review.Status != task.ReviewApproved {
		return MergeResult{}, fmt.Errorf("task %q must have current human approval before merge", id)
	}
	all, err := s.Ledger.List()
	if err != nil {
		return MergeResult{}, err
	}
	snapshot, err := review.Prepare(s.Git, entry, all)
	if err != nil {
		return MergeResult{}, err
	}
	if snapshot.ApprovalValidity != review.ApprovalCurrent || snapshot.DependencyStatus != review.DependencyClear {
		return MergeResult{}, fmt.Errorf("task %q approval is no longer current; prepare and complete review again", id)
	}
	if snapshot.WorkingTree != "clean" {
		return MergeResult{}, fmt.Errorf("cannot merge until the task worktree is clean (current status: %s)", snapshot.WorkingTree)
	}
	targetBranch, err := s.mergeTarget(entry)
	if err != nil {
		return MergeResult{}, err
	}
	targetHead, err := s.Git.Resolve(targetBranch)
	if err != nil {
		return MergeResult{}, err
	}
	if targetHead != entry.Review.ReviewedBaseSHA {
		return MergeResult{}, fmt.Errorf("target branch %q moved from reviewed base %s to %s; update the task and review again", targetBranch, entry.Review.ReviewedBaseSHA, targetHead)
	}
	targetPath, err := worktree.Find(s.Git, targetBranch)
	if err != nil {
		return MergeResult{}, err
	}
	if targetPath == "" {
		return MergeResult{}, fmt.Errorf("target branch %q has no local worktree; merge was not attempted", targetBranch)
	}
	targetGit := gitclient.Client{Root: targetPath}
	if err := targetGit.RequireClean(); err != nil {
		return MergeResult{}, fmt.Errorf("target worktree is not clean: %w", err)
	}
	if _, err := targetGit.Run("merge", "--ff-only", entry.Branch); err != nil {
		return MergeResult{}, fmt.Errorf("fast-forward task %q into %q: %w", id, targetBranch, err)
	}
	mergedHead, err := targetGit.Resolve(targetBranch)
	if err != nil {
		return MergeResult{}, fmt.Errorf("verify merged target: %w", err)
	}
	if mergedHead != snapshot.Head.SHA {
		return MergeResult{}, fmt.Errorf("target %q ended at %s instead of approved head %s", targetBranch, mergedHead, snapshot.Head.SHA)
	}
	entry.Lifecycle = task.Merged
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): merge task "+id+" into "+targetBranch); err != nil {
		return MergeResult{}, fmt.Errorf("task was merged into %q, but updating the registry failed: %w", targetBranch, err)
	}
	return MergeResult{Task: entry, SourceBranch: entry.Branch, TargetBranch: targetBranch, HeadSHA: mergedHead}, nil
}

func (s Service) mergeTarget(entry task.Task) (string, error) {
	if entry.ParentTask != "" {
		parent, err := s.Ledger.Get(entry.ParentTask)
		if err != nil {
			return "", fmt.Errorf("load parent task %q: %w", entry.ParentTask, err)
		}
		return parent.Branch, nil
	}
	if !s.Git.BranchExists(entry.Base.Ref) {
		return "", fmt.Errorf("recorded base %q is not a local branch and cannot be used as a merge target", entry.Base.Ref)
	}
	return entry.Base.Ref, nil
}

func (s Service) Abandon(id string) (task.Task, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	if entry.Lifecycle == task.Merged || entry.Lifecycle == task.Abandoned {
		return task.Task{}, fmt.Errorf("cannot abandon %s task", entry.Lifecycle)
	}
	entry.Lifecycle = task.Abandoned
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): abandon task "+id); err != nil {
		return task.Task{}, err
	}
	return entry, nil
}

func (s Service) PrepareReview(id string) (review.Snapshot, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return review.Snapshot{}, err
	}
	all, err := s.Ledger.List()
	if err != nil {
		return review.Snapshot{}, err
	}
	snapshot, err := review.Prepare(s.Git, entry, all)
	if err != nil {
		return review.Snapshot{}, err
	}
	return snapshot, nil
}

func (s Service) Approve(id, expectedBaseSHA, expectedHeadSHA string) (task.Task, review.Snapshot, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	if entry.Lifecycle == task.Merged || entry.Lifecycle == task.Abandoned {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("cannot approve %s task", entry.Lifecycle)
	}
	if !task.ValidSHA(expectedBaseSHA) || !task.ValidSHA(expectedHeadSHA) {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("approval requires the full base and HEAD SHA from the review the user actually completed")
	}
	all, err := s.Ledger.List()
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	snapshot, err := review.Prepare(s.Git, entry, all)
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	if snapshot.Base.SHA != expectedBaseSHA || snapshot.Head.SHA != expectedHeadSHA {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("reviewed version changed: user reviewed %s...%s, current range is %s...%s", expectedBaseSHA, expectedHeadSHA, snapshot.Base.SHA, snapshot.Head.SHA)
	}
	if entry.Lifecycle != task.ReadyForReview {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("task %q must be marked ready for review before approval", id)
	}
	if snapshot.DependencyStatus != review.DependencyClear {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("cannot record final approval while dependency status is %s", snapshot.DependencyStatus)
	}
	if snapshot.WorkingTree != "clean" {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("cannot record approval until the task worktree status is clean (current status: %s)", snapshot.WorkingTree)
	}
	entry.Review = task.Review{Status: task.ReviewApproved, ReviewedBaseSHA: snapshot.Base.SHA, ReviewedHeadSHA: snapshot.Head.SHA}
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): approve task "+id); err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	return entry, snapshot, nil
}

func (s Service) RequestChanges(id, reason string) (task.Task, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	if entry.Lifecycle == task.Merged || entry.Lifecycle == task.Abandoned {
		return task.Task{}, fmt.Errorf("cannot request changes for %s task", entry.Lifecycle)
	}
	entry.Lifecycle = task.Active
	entry.Review = task.Review{Status: task.ReviewChangesRequested}
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): request changes for "+id+reasonSuffix(reason)); err != nil {
		return task.Task{}, err
	}
	return entry, nil
}
func (s Service) WorktreePath(id string) (string, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return "", err
	}
	return worktree.Find(s.Git, entry.Branch)
}

func Slug(value string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '_' {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		return "task-" + time.Now().Format("20060102-150405")
	}
	return id
}
func reasonSuffix(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	return ": " + strings.ReplaceAll(strings.TrimSpace(reason), "\n", " ")
}
