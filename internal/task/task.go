package task

import (
	"fmt"
	"strings"
	"time"
)

const SchemaVersion = 1

type Lifecycle string

const (
	Active         Lifecycle = "active"
	ReadyForReview Lifecycle = "ready_for_review"
	InReview       Lifecycle = "in_review"
	Approved       Lifecycle = "approved"
	Parked         Lifecycle = "parked"
	Merged         Lifecycle = "merged"
	Abandoned      Lifecycle = "abandoned"
)

type ReviewStatus string

const (
	ReviewNone             ReviewStatus = "none"
	ReviewChangesRequested ReviewStatus = "changes_requested"
	ReviewConditional      ReviewStatus = "conditional"
	ReviewApproved         ReviewStatus = "approved"
	ReviewStale            ReviewStatus = "stale"
)

type TestEvidence struct {
	Command    string `yaml:"command" json:"command"`
	Result     string `yaml:"result" json:"result"`
	ExitCode   *int   `yaml:"exit_code,omitempty" json:"exitCode,omitempty"`
	Summary    string `yaml:"summary,omitempty" json:"summary,omitempty"`
	RecordedAt string `yaml:"recorded_at,omitempty" json:"recordedAt,omitempty"`
}

type Base struct {
	Ref string `yaml:"ref" json:"ref"`
	SHA string `yaml:"sha" json:"sha"`
}

type Review struct {
	Status          ReviewStatus `yaml:"status" json:"status"`
	ReviewedBaseSHA string       `yaml:"reviewed_base_sha" json:"reviewedBaseSHA"`
	ReviewedHeadSHA string       `yaml:"reviewed_head_sha" json:"reviewedHeadSHA"`
}

type Task struct {
	SchemaVersion int            `yaml:"schema_version" json:"schemaVersion"`
	ID            string         `yaml:"id" json:"id"`
	Title         string         `yaml:"title" json:"title"`
	Kind          string         `yaml:"kind" json:"kind"`
	Branch        string         `yaml:"branch" json:"branch"`
	Base          Base           `yaml:"base" json:"base"`
	ParentTask    string         `yaml:"parent_task" json:"parentTask,omitempty"`
	Lifecycle     Lifecycle      `yaml:"lifecycle" json:"lifecycle"`
	Review        Review         `yaml:"review" json:"review"`
	Dependencies  []string       `yaml:"dependencies" json:"dependencies"`
	Tests         []TestEvidence `yaml:"tests" json:"tests"`
	CreatedAt     string         `yaml:"created_at" json:"createdAt"`
	UpdatedAt     string         `yaml:"updated_at" json:"updatedAt"`
}

func (t Task) Validate() error {
	if t.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported task schema version %d", t.SchemaVersion)
	}
	if !ValidID(t.ID) {
		return fmt.Errorf("invalid task id %q", t.ID)
	}
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("task title is required")
	}
	if !strings.HasPrefix(t.Branch, "arw/") {
		return fmt.Errorf("task branch must start with arw/")
	}
	if len(t.Base.SHA) != 40 {
		return fmt.Errorf("base SHA must be a full Git SHA")
	}
	if !validLifecycle(t.Lifecycle) {
		return fmt.Errorf("invalid lifecycle %q", t.Lifecycle)
	}
	if !validReview(t.Review.Status) {
		return fmt.Errorf("invalid review status %q", t.Review.Status)
	}
	return nil
}

func ValidID(id string) bool {
	if id == "" || strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func Touch(t *Task) { t.UpdatedAt = time.Now().Format(time.RFC3339) }

func validLifecycle(value Lifecycle) bool {
	switch value {
	case Active, ReadyForReview, InReview, Approved, Parked, Merged, Abandoned:
		return true
	}
	return false
}

func validReview(value ReviewStatus) bool {
	switch value {
	case ReviewNone, ReviewChangesRequested, ReviewConditional, ReviewApproved, ReviewStale:
		return true
	}
	return false
}
