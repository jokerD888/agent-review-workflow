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
	Parked         Lifecycle = "parked"
	Merged         Lifecycle = "merged"
	Abandoned      Lifecycle = "abandoned"
)

type ReviewStatus string

const (
	ReviewNone             ReviewStatus = "none"
	ReviewChangesRequested ReviewStatus = "changes_requested"
	ReviewApproved         ReviewStatus = "approved"
)

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
	SchemaVersion int       `yaml:"schema_version" json:"schemaVersion"`
	ID            string    `yaml:"id" json:"id"`
	Title         string    `yaml:"title" json:"title"`
	Branch        string    `yaml:"branch" json:"branch"`
	Base          Base      `yaml:"base" json:"base"`
	ParentTask    string    `yaml:"parent_task" json:"parentTask,omitempty"`
	Lifecycle     Lifecycle `yaml:"lifecycle" json:"lifecycle"`
	Review        Review    `yaml:"review" json:"review"`
	CreatedAt     string    `yaml:"created_at" json:"createdAt"`
	UpdatedAt     string    `yaml:"updated_at" json:"updatedAt"`
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
	if t.Branch != "arw/"+t.ID {
		return fmt.Errorf("task branch must be arw/%s", t.ID)
	}
	if strings.TrimSpace(t.Base.Ref) == "" {
		return fmt.Errorf("base ref is required")
	}
	if !ValidSHA(t.Base.SHA) {
		return fmt.Errorf("base SHA must be a full lowercase Git SHA")
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
	if id == "" || strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") || strings.Contains(id, "--") {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func ValidSHA(sha string) bool {
	if len(sha) != 40 {
		return false
	}
	for _, r := range sha {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func Touch(t *Task) { t.UpdatedAt = time.Now().Format(time.RFC3339) }

func validLifecycle(value Lifecycle) bool {
	switch value {
	case Active, ReadyForReview, Parked, Merged, Abandoned:
		return true
	}
	return false
}

func validReview(value ReviewStatus) bool {
	switch value {
	case ReviewNone, ReviewChangesRequested, ReviewApproved:
		return true
	}
	return false
}
