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

type StartOptions struct{ Title, ID, Kind, BaseRef, ParentTask, WorktreePath string }

type StartResult struct {
	Task     task.Task `json:"task"`
	Worktree string    `json:"worktree"`
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
	if options.Kind == "" {
		options.Kind = "other"
	}
	baseRef := options.BaseRef
	if baseRef == "" {
		baseRef = "main"
	}
	baseSHA, err := s.Git.Resolve(baseRef)
	if err != nil {
		return StartResult{}, err
	}
	dependencies := []string{}
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
		dependencies = append(dependencies, parent)
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
	entry := task.Task{SchemaVersion: task.SchemaVersion, ID: id, Title: options.Title, Kind: options.Kind, Branch: branch, Base: task.Base{Ref: baseRef, SHA: baseSHA}, ParentTask: parent, Lifecycle: task.Active, Review: task.Review{Status: task.ReviewNone}, Dependencies: dependencies, Tests: []task.TestEvidence{}, CreatedAt: now, UpdatedAt: now}
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
	review.MarkPrepared(&entry, snapshot)
	if err := s.Ledger.Save(entry, "chore(arw): prepare review "+id); err != nil {
		return review.Snapshot{}, err
	}
	if err := s.Ledger.SaveSnapshot(id, snapshot); err != nil {
		return review.Snapshot{}, err
	}
	return snapshot, nil
}

func (s Service) ReviewStatus(id string) (review.Snapshot, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return review.Snapshot{}, err
	}
	all, err := s.Ledger.List()
	if err != nil {
		return review.Snapshot{}, err
	}
	return review.Prepare(s.Git, entry, all)
}

func (s Service) Approve(id string) (task.Task, review.Snapshot, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	all, err := s.Ledger.List()
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	snapshot, err := review.Prepare(s.Git, entry, all)
	if err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	if snapshot.DependencyStatus != review.DependencyClear {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("cannot record final approval while dependency status is %s", snapshot.DependencyStatus)
	}
	if snapshot.WorkingTree != "clean" {
		return task.Task{}, review.Snapshot{}, fmt.Errorf("cannot record approval until the task worktree status is clean (current status: %s)", snapshot.WorkingTree)
	}
	entry.Lifecycle = task.Approved
	entry.Review = task.Review{Status: task.ReviewApproved, ReviewedBaseSHA: snapshot.Base.SHA, ReviewedHeadSHA: snapshot.Head.SHA}
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): approve task "+id); err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	if err := s.Ledger.SaveSnapshot(id, snapshot); err != nil {
		return task.Task{}, review.Snapshot{}, err
	}
	return entry, snapshot, nil
}

func (s Service) RequestChanges(id, reason string) (task.Task, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	entry.Lifecycle = task.Active
	entry.Review.Status = task.ReviewChangesRequested
	task.Touch(&entry)
	if err := s.Ledger.Save(entry, "chore(arw): request changes for "+id+reasonSuffix(reason)); err != nil {
		return task.Task{}, err
	}
	return entry, nil
}
func (s Service) OpenWorktree(id string) (string, error) {
	entry, err := s.Ledger.Get(id)
	if err != nil {
		return "", err
	}
	path, err := worktree.Find(s.Git, entry.Branch)
	if err != nil {
		return "", err
	}
	if err := worktree.Open(path); err != nil {
		return "", err
	}
	return path, nil
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
