package task

import "testing"

func TestValidateAcceptsMinimalTask(t *testing.T) {
	entry := Task{SchemaVersion: SchemaVersion, ID: "fix-login", Title: "Fix login", Kind: "bugfix", Branch: "arw/fix-login", Base: Base{Ref: "main", SHA: "0123456789012345678901234567890123456789"}, Lifecycle: Active, Review: Review{Status: ReviewNone}}
	if err := entry.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsUnsafeBranch(t *testing.T) {
	entry := Task{SchemaVersion: SchemaVersion, ID: "fix-login", Title: "Fix login", Kind: "bugfix", Branch: "arw/another-task", Base: Base{Ref: "main", SHA: "0123456789012345678901234567890123456789"}, Lifecycle: Active, Review: Review{Status: ReviewNone}}
	if err := entry.Validate(); err == nil {
		t.Fatal("Validate() accepted a branch for another task")
	}
}

func TestValidateRejectsUnsupportedKind(t *testing.T) {
	entry := Task{SchemaVersion: SchemaVersion, ID: "fix-login", Title: "Fix login", Kind: "urgent", Branch: "arw/fix-login", Base: Base{Ref: "main", SHA: "0123456789012345678901234567890123456789"}, Lifecycle: Active, Review: Review{Status: ReviewNone}}
	if err := entry.Validate(); err == nil {
		t.Fatal("Validate() accepted an unsupported kind")
	}
}

func TestValidID(t *testing.T) {
	for _, id := range []string{"a", "fix-login-2"} {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false", id)
		}
	}
	for _, id := range []string{"", "-start", "end-", "contains--dash", "Upper", "contains/slash"} {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true", id)
		}
	}
}

func TestTestEvidenceValidation(t *testing.T) {
	for _, evidence := range []TestEvidence{{Command: "go test ./...", Result: "passed"}, {Command: "manual review", Result: "skipped"}} {
		if err := evidence.Validate(); err != nil {
			t.Errorf("Validate(%#v) error = %v", evidence, err)
		}
	}
	for _, evidence := range []TestEvidence{{Result: "passed"}, {Command: "go test ./...", Result: "complete"}} {
		if err := evidence.Validate(); err == nil {
			t.Errorf("Validate(%#v) succeeded", evidence)
		}
	}
}
