package review

import (
	"fmt"
	"strings"
	"time"

	gitclient "github.com/jokerD888/agent-review-workflow/internal/git"
	"github.com/jokerD888/agent-review-workflow/internal/task"
)

type DependencyStatus string

type ApprovalValidity string

const (
	DependencyClear      DependencyStatus = "clear"
	AwaitingPrerequisite DependencyStatus = "awaiting_prerequisite_review"
	ParentChanged        DependencyStatus = "parent_changed"
	DependencyBlocked    DependencyStatus = "blocked"
	ApprovalNotApproved  ApprovalValidity = "not_approved"
	ApprovalCurrent      ApprovalValidity = "current"
	ApprovalStale        ApprovalValidity = "stale"
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
	SchemaVersion    int               `yaml:"schema_version" json:"schemaVersion"`
	TaskID           string            `yaml:"task_id" json:"taskId"`
	CreatedAt        string            `yaml:"created_at" json:"createdAt"`
	Base             Revision          `yaml:"base" json:"base"`
	Head             Revision          `yaml:"head" json:"head"`
	Comparison       string            `yaml:"comparison" json:"comparison"`
	Files            []File            `yaml:"files" json:"files"`
	Commits          []Commit          `yaml:"commits" json:"commits"`
	WorkingTree      string            `yaml:"working_tree" json:"workingTree"`
	Worktree         string            `yaml:"worktree" json:"worktree"`
	DependencyStatus DependencyStatus  `yaml:"dependency_status" json:"dependencyStatus"`
	ReviewStatus     task.ReviewStatus `yaml:"review_status" json:"reviewStatus"`
	ApprovalValidity ApprovalValidity  `yaml:"approval_validity" json:"approvalValidity"`
	Risks            []string          `yaml:"risks" json:"risks"`
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
	worktree := ""
	workingTree := "unknown"
	if path, err := worktreeForBranch(git, current.Branch); err == nil && path != "" {
		worktree = path
		probe := git
		probe.Root = path
		if output, err := probe.Run("-c", "core.fsmonitor=false", "status", "--porcelain", "--untracked-files=all"); err == nil {
			if strings.TrimSpace(output) == "" {
				workingTree = "clean"
			} else {
				workingTree = "dirty"
			}
		}
	}
	approvalValidity := approvalValidity(current, base, head, dependency)
	risks := []string{}
	if workingTree == "dirty" {
		risks = append(risks, "Task worktree contains uncommitted changes; the committed review range does not include them.")
	}
	if workingTree == "unknown" {
		risks = append(risks, "Task worktree status could not be determined; final approval is unavailable until it can be checked.")
	}
	if dependency == AwaitingPrerequisite {
		risks = append(risks, "Parent task has not received final approval; this task cannot receive final approval yet.")
	}
	if dependency == ParentChanged {
		risks = append(risks, "Parent task changed after this task's review context; review must be refreshed.")
	}
	if approvalValidity == ApprovalStale {
		risks = append(risks, "The prior review conclusion no longer matches this base and HEAD.")
	}
	return Snapshot{SchemaVersion: 1, TaskID: current.ID, CreatedAt: time.Now().Format(time.RFC3339), Base: Revision{Ref: baseRef, SHA: base}, Head: Revision{SHA: head}, Comparison: baseRef + "@" + base + "...HEAD@" + head, Files: files, Commits: commits, WorkingTree: workingTree, Worktree: worktree, DependencyStatus: dependency, ReviewStatus: current.Review.Status, ApprovalValidity: approvalValidity, Risks: risks}, nil
}

// EffectiveTargetRef walks up the parent chain, skipping tasks whose lifecycle
// is Merged (their content has already landed further up the chain), and
// returns the branch name of the first non-merged ancestor. If the chain has
// no parent task, the task's recorded Base.Ref is returned. An Abandoned
// ancestor aborts the walk: a child of an abandoned task requires the user to
// decide how to proceed.
func EffectiveTargetRef(entry task.Task, all []task.Task) (string, error) {
	cur := entry
	for {
		if cur.ParentTask == "" {
			return cur.Base.Ref, nil
		}
		parent, ok := find(all, cur.ParentTask)
		if !ok {
			return "", fmt.Errorf("parent task %q is missing", cur.ParentTask)
		}
		switch parent.Lifecycle {
		case task.Merged:
			cur = parent
		case task.Abandoned:
			return "", fmt.Errorf("parent task %q was abandoned; resolve the dependency manually", cur.ParentTask)
		default:
			return parent.Branch, nil
		}
	}
}

func comparisonBase(git gitclient.Client, current task.Task, all []task.Task) (string, string, DependencyStatus, error) {
	if current.ParentTask == "" {
		base, err := git.Resolve(current.Base.Ref)
		if err != nil {
			return "", "", DependencyBlocked, fmt.Errorf("resolve recorded base %q: %w", current.Base.Ref, err)
		}
		return base, current.Base.Ref, DependencyClear, nil
	}
	parent, ok := find(all, current.ParentTask)
	if !ok {
		return "", "", DependencyBlocked, fmt.Errorf("parent task %q is missing", current.ParentTask)
	}
	// When the parent has already been merged, its content has landed further
	// up the chain. Walk up to the effective target branch and use its current
	// HEAD as this task's base. The parent's own approval validity is no longer
	// relevant — what matters is whether this task's reviewed base still
	// matches the effective target's current HEAD.
	if parent.Lifecycle == task.Merged {
		ref, err := EffectiveTargetRef(parent, all)
		if err != nil {
			return "", "", DependencyBlocked, err
		}
		mergedHead, err := git.Resolve(ref)
		if err != nil {
			return "", "", DependencyBlocked, fmt.Errorf("resolve effective target %q: %w", ref, err)
		}
		if current.Review.Status == task.ReviewApproved && current.Review.ReviewedBaseSHA != "" && current.Review.ReviewedBaseSHA != mergedHead {
			return mergedHead, ref, ParentChanged, nil
		}
		return mergedHead, ref, DependencyClear, nil
	}
	// An abandoned parent cannot serve as a base. The user must decide how to
	// resolve the child (rebase, abandon, or re-parent it).
	if parent.Lifecycle == task.Abandoned {
		return "", "", DependencyBlocked, fmt.Errorf("parent task %q was abandoned; resolve the dependency manually", current.ParentTask)
	}
	parentHead, err := git.Resolve(parent.Branch)
	if err != nil {
		return "", "", DependencyBlocked, fmt.Errorf("resolve parent branch: %w", err)
	}
	parentBase, _, parentDependency, err := comparisonBase(git, parent, all)
	if err != nil {
		return "", "", DependencyBlocked, err
	}
	if approvalValidity(parent, parentBase, parentHead, parentDependency) != ApprovalCurrent {
		return parentHead, parent.Branch, AwaitingPrerequisite, nil
	}
	if current.Review.Status == task.ReviewApproved && current.Review.ReviewedBaseSHA != "" && current.Review.ReviewedBaseSHA != parentHead {
		return parentHead, parent.Branch, ParentChanged, nil
	}
	return parentHead, parent.Branch, DependencyClear, nil
}

func approvalValidity(current task.Task, base, head string, dependency DependencyStatus) ApprovalValidity {
	if current.Review.Status != task.ReviewApproved {
		return ApprovalNotApproved
	}
	if current.Review.ReviewedBaseSHA != base || current.Review.ReviewedHeadSHA != head || dependency != DependencyClear {
		return ApprovalStale
	}
	return ApprovalCurrent
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
