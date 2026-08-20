package review

import (
	"fmt"
	"strings"
	"time"

	gitclient "github.com/jokerD888/agent-review-workflow/internal/git"
	"github.com/jokerD888/agent-review-workflow/internal/task"
)

type DependencyStatus string

const (
	DependencyClear      DependencyStatus = "clear"
	AwaitingPrerequisite DependencyStatus = "awaiting_prerequisite_review"
	ParentChanged        DependencyStatus = "parent_changed"
	DependencyBlocked    DependencyStatus = "blocked"
)

type Revision struct {
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`
	SHA string `yaml:"sha" json:"sha"`
}
type File struct {
	Path      string `yaml:"path" json:"path"`
	Status    string `yaml:"status" json:"status"`
	Additions int    `yaml:"additions" json:"additions"`
	Deletions int    `yaml:"deletions" json:"deletions"`
}
type Commit struct {
	SHA     string `yaml:"sha" json:"sha"`
	Subject string `yaml:"subject" json:"subject"`
}
type Snapshot struct {
	SchemaVersion    int                 `yaml:"schema_version" json:"schemaVersion"`
	TaskID           string              `yaml:"task_id" json:"taskId"`
	CreatedAt        string              `yaml:"created_at" json:"createdAt"`
	Base             Revision            `yaml:"base" json:"base"`
	Head             Revision            `yaml:"head" json:"head"`
	Comparison       string              `yaml:"comparison" json:"comparison"`
	Files            []File              `yaml:"files" json:"files"`
	Commits          []Commit            `yaml:"commits" json:"commits"`
	WorkingTree      string              `yaml:"working_tree" json:"workingTree"`
	DependencyStatus DependencyStatus    `yaml:"dependency_status" json:"dependencyStatus"`
	ReviewStatus     task.ReviewStatus   `yaml:"review_status" json:"reviewStatus"`
	Risks            []string            `yaml:"risks" json:"risks"`
}

func Prepare(git gitclient.Client, current task.Task, all []task.Task) (Snapshot, error) {
	head, err := git.Resolve(current.Branch)
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve task branch %q: %w", current.Branch, err)
	}
	base, baseRef, dependency, err := comparisonBase(git, current, all)
	if err != nil {
		return Snapshot{}, err
	}
	files, err := changedFiles(git, base, head)
	if err != nil {
		return Snapshot{}, err
	}
	commits, err := commits(git, base, head)
	if err != nil {
		return Snapshot{}, err
	}
	workingTree := "unknown"
	if worktree, err := worktreeForBranch(git, current.Branch); err == nil && worktree != "" {
		probe := git
		probe.Root = worktree
		if output, err := probe.Run("-c", "core.fsmonitor=false", "status", "--porcelain", "--untracked-files=all"); err == nil {
			if strings.TrimSpace(output) == "" {
				workingTree = "clean"
			} else {
				workingTree = "dirty"
			}
		}
	}
	status := effectiveReviewStatus(current, base, head, dependency)
	risks := []string{}
	if workingTree == "dirty" {
		risks = append(risks, "Task worktree contains uncommitted changes; the committed review range does not include them.")
	}
	if workingTree == "unknown" {
		risks = append(risks, "Task worktree status could not be determined; final approval is unavailable until it can be checked.")
	}
	if dependency == AwaitingPrerequisite {
		risks = append(risks, "Parent task has not received final approval; this is a conditional review.")
	}
	if dependency == ParentChanged {
		risks = append(risks, "Parent task changed after this task's review context; review must be refreshed.")
	}
	if status == task.ReviewStale {
		risks = append(risks, "The prior review conclusion no longer matches this base and HEAD.")
	}
	return Snapshot{SchemaVersion: 1, TaskID: current.ID, CreatedAt: time.Now().Format(time.RFC3339), Base: Revision{Ref: baseRef, SHA: base}, Head: Revision{SHA: head}, Comparison: baseRef + "@" + base + "...HEAD@" + head, Files: files, Commits: commits, WorkingTree: workingTree, DependencyStatus: dependency, ReviewStatus: status, Risks: risks}, nil
}

func MarkPrepared(entry *task.Task, snapshot Snapshot) {
	if entry.Lifecycle == task.Active || entry.Lifecycle == task.ReadyForReview {
		entry.Lifecycle = task.InReview
	}
	if entry.Review.Status == task.ReviewApproved || entry.Review.Status == task.ReviewConditional {
		if snapshot.ReviewStatus == task.ReviewStale {
			entry.Review.Status = task.ReviewStale
		}
	}
	task.Touch(entry)
}

func comparisonBase(git gitclient.Client, current task.Task, all []task.Task) (string, string, DependencyStatus, error) {
	if current.ParentTask == "" {
		return current.Base.SHA, current.Base.Ref, DependencyClear, nil
	}
	parent, ok := find(all, current.ParentTask)
	if !ok {
		return "", "", DependencyBlocked, fmt.Errorf("parent task %q is missing", current.ParentTask)
	}
	parentHead, err := git.Resolve(parent.Branch)
	if err != nil {
		return "", "", DependencyBlocked, fmt.Errorf("resolve parent branch: %w", err)
	}
	if parent.Review.Status != task.ReviewApproved || parent.Review.ReviewedHeadSHA != parentHead {
		return parentHead, parent.Branch, AwaitingPrerequisite, nil
	}
	if current.Review.Status == task.ReviewConditional && current.Review.ReviewedBaseSHA != "" && current.Review.ReviewedBaseSHA != parentHead {
		return parentHead, parent.Branch, ParentChanged, nil
	}
	return parentHead, parent.Branch, DependencyClear, nil
}

func effectiveReviewStatus(current task.Task, base, head string, dependency DependencyStatus) task.ReviewStatus {
	if current.Review.Status == task.ReviewApproved || current.Review.Status == task.ReviewConditional {
		if current.Review.ReviewedBaseSHA != base || current.Review.ReviewedHeadSHA != head || dependency == ParentChanged {
			return task.ReviewStale
		}
	}
	if dependency == AwaitingPrerequisite && current.Review.Status == task.ReviewApproved {
		return task.ReviewConditional
	}
	return current.Review.Status
}

func find(entries []task.Task, id string) (task.Task, bool) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return task.Task{}, false
}

func changedFiles(git gitclient.Client, base, head string) ([]File, error) {
	statusOutput, err := git.Run("diff", "--name-status", base+"..."+head)
	if err != nil {
		return nil, err
	}
	numstatOutput, err := git.Run("diff", "--numstat", base+"..."+head)
	if err != nil {
		return nil, err
	}
	counts := map[string][2]int{}
	for _, line := range strings.Split(strings.TrimSpace(numstatOutput), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		var add, del int
		fmt.Sscanf(fields[0], "%d", &add)
		fmt.Sscanf(fields[1], "%d", &del)
		counts[fields[len(fields)-1]] = [2]int{add, del}
	}
	files := []File{}
	for _, line := range strings.Split(strings.TrimSpace(statusOutput), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		path := fields[len(fields)-1]
		count := counts[path]
		files = append(files, File{Path: path, Status: fields[0], Additions: count[0], Deletions: count[1]})
	}
	return files, nil
}

func commits(git gitclient.Client, base, head string) ([]Commit, error) {
	output, err := git.Run("log", "--format=%H%x09%s", base+".."+head)
	if err != nil {
		return nil, err
	}
	entries := []Commit{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) == 2 {
			entries = append(entries, Commit{SHA: fields[0], Subject: fields[1]})
		}
	}
	return entries, nil
}

func worktreeForBranch(git gitclient.Client, branch string) (string, error) {
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
