package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jokerD888/agent-review-workflow/internal/app"
	"github.com/jokerD888/agent-review-workflow/internal/ledger"
	"github.com/jokerD888/agent-review-workflow/internal/task"
)

var version = "0.2.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "arw:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp()
		return nil
	}
	if args[0] == "version" {
		fmt.Println(version)
		return nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	svc, err := app.New(dir)
	if err != nil {
		return err
	}
	switch args[0] {
	case "setup":
		return svc.Setup()
	case "doctor":
		return doctor(svc, hasJSON(args))
	case "refresh":
		return refresh(svc, hasJSON(args))
	case "task":
		return taskCommand(svc, args[1:])
	case "review":
		return reviewCommand(svc, args[1:])
	case "worktree":
		return worktreeCommand(svc, args[1:])
	default:
		return fmt.Errorf("unknown command %q; run 'arw help'", args[0])
	}
}

func taskCommand(svc app.Service, args []string) error {
	args, jsonOutput := withoutFormat(args)
	if len(args) == 0 {
		return errors.New("usage: arw task <start|list|show|park|resume> ...")
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("task start", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "task id")
		kind := fs.String("kind", "other", "task kind")
		base := fs.String("base", "main", "base Git ref")
		parent := fs.String("parent", "", "parent task id")
		worktreePath := fs.String("worktree", "", "worktree path")
		format := fs.String("format", "", "output format")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		title := strings.Join(fs.Args(), " ")
		if title == "" {
			return errors.New("usage: arw task start [--id id] [--parent task-id] <title>")
		}
		result, err := svc.Start(app.StartOptions{Title: title, ID: *id, Kind: *kind, BaseRef: *base, ParentTask: *parent, WorktreePath: *worktreePath})
		if err != nil {
			return err
		}
		return output(result, *format == "json" || jsonOutput)
	case "list":
		fs := flag.NewFlagSet("task list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		view := fs.String("view", "", "reviewable|active|parked|blocked")
		format := fs.String("format", "", "output format")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		entries, err := svc.Tasks()
		if err != nil {
			return err
		}
		entries = filter(entries, *view)
		return output(entries, *format == "json" || jsonOutput)
	case "show":
		if len(args) < 2 {
			return errors.New("usage: arw task show <task-id> [--format json]")
		}
		entry, err := svc.Task(args[1])
		if err != nil {
			return err
		}
		return output(entry, jsonOutput)
	case "park":
		if len(args) < 2 {
			return errors.New("usage: arw task park <task-id> [--format json]")
		}
		entry, err := svc.Park(args[1])
		if err != nil {
			return err
		}
		return output(entry, jsonOutput)
	case "resume":
		if len(args) < 2 {
			return errors.New("usage: arw task resume <task-id> [--format json]")
		}
		entry, err := svc.Resume(args[1])
		if err != nil {
			return err
		}
		return output(entry, jsonOutput)
	default:
		return fmt.Errorf("unknown task command %q", args[0])
	}
}

func reviewCommand(svc app.Service, args []string) error {
	args, jsonOutput := withoutFormat(args)
	if len(args) == 0 {
		return errors.New("usage: arw review <prepare|status|approve|request-changes> <task-id>")
	}
	switch args[0] {
	case "prepare":
		if len(args) < 2 {
			return errors.New("usage: arw review prepare <task-id> [--format json]")
		}
		snapshot, err := svc.PrepareReview(args[1])
		if err != nil {
			return err
		}
		return output(snapshot, jsonOutput)
	case "status":
		if len(args) < 2 {
			return errors.New("usage: arw review status <task-id> [--format json]")
		}
		snapshot, err := svc.ReviewStatus(args[1])
		if err != nil {
			return err
		}
		return output(snapshot, jsonOutput)
	case "approve":
		approveArgs, confirm := stripBool(args[1:], "--confirm")
		if len(approveArgs) != 1 {
			return errors.New("usage: arw review approve --confirm <task-id>")
		}
		if !confirm {
			return errors.New("approval changes the task audit record; repeat with --confirm after human review")
		}
		entry, snapshot, err := svc.Approve(approveArgs[0])
		if err != nil {
			return err
		}
		return output(struct {
			Task     task.Task `json:"task"`
			Snapshot any       `json:"snapshot"`
		}{entry, snapshot}, jsonOutput)
	case "request-changes":
		fs := flag.NewFlagSet("review request-changes", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		reason := fs.String("reason", "", "reason")
		format := fs.String("format", "", "output format")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(fs.Args()) != 1 {
			return errors.New("usage: arw review request-changes <task-id> [--reason text]")
		}
		entry, err := svc.RequestChanges(fs.Args()[0], *reason)
		if err != nil {
			return err
		}
		return output(entry, *format == "json" || jsonOutput)
	default:
		return fmt.Errorf("unknown review command %q", args[0])
	}
}

func worktreeCommand(svc app.Service, args []string) error {
	args, jsonOutput := withoutFormat(args)
	if len(args) < 2 || args[0] != "open" {
		return errors.New("usage: arw worktree open <task-id> [--format json]")
	}
	path, err := svc.OpenWorktree(args[1])
	if err != nil {
		return err
	}
	return output(map[string]string{"taskId": args[1], "worktree": path}, jsonOutput)
}

func doctor(svc app.Service, jsonOutput bool) error {
	entries, err := svc.Tasks()
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	data := map[string]any{"version": version, "repository": svc.Git.Root, "registryBranch": ledger.RegistryBranch, "registryExists": svc.Git.BranchExists(ledger.RegistryBranch), "tasks": len(entries)}
	return output(data, jsonOutput)
}
func refresh(svc app.Service, jsonOutput bool) error {
	entries, err := svc.Tasks()
	if err != nil {
		return err
	}
	return output(map[string]any{"tasks": len(entries), "registryBranch": ledger.RegistryBranch}, jsonOutput)
}

func filter(entries []task.Task, view string) []task.Task {
	if view == "" {
		return entries
	}
	filtered := []task.Task{}
	for _, entry := range entries {
		keep := false
		switch view {
		case "active":
			keep = entry.Lifecycle == task.Active || entry.Lifecycle == task.InReview || entry.Lifecycle == task.ReadyForReview
		case "parked":
			keep = entry.Lifecycle == task.Parked
		case "reviewable":
			keep = entry.Lifecycle == task.ReadyForReview || entry.Lifecycle == task.InReview || entry.Lifecycle == task.Approved
		case "blocked":
			keep = entry.ParentTask != "" && entry.Review.Status != task.ReviewApproved
		default:
			return entries
		}
		if keep {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func hasJSON(args []string) bool {
	_, jsonOutput := withoutFormat(args)
	return jsonOutput
}

func withoutFormat(args []string) ([]string, bool) {
	clean := make([]string, 0, len(args))
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		value := args[i]
		if value == "--format=json" || value == "--json" {
			jsonOutput = true
			continue
		}
		if value == "--format" && i+1 < len(args) && args[i+1] == "json" {
			jsonOutput = true
			i++
			continue
		}
		clean = append(clean, value)
	}
	return clean, jsonOutput
}

func stripBool(args []string, name string) ([]string, bool) {
	clean := make([]string, 0, len(args))
	found := false
	for _, value := range args {
		if value == name {
			found = true
			continue
		}
		clean = append(clean, value)
	}
	return clean, found
}
func output(value any, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	switch v := value.(type) {
	case []task.Task:
		for _, entry := range v {
			fmt.Printf("%-28s %-18s %-18s %s\n", entry.ID, entry.Lifecycle, entry.Review.Status, entry.Title)
		}
		return nil
	default:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
}
func printHelp() {
	fmt.Print(`ARW v2 (development)

Usage:
  arw setup | doctor | refresh
  arw task start [--id id] [--kind kind] [--base ref] [--parent task-id] <title>
  arw task list [--view reviewable|active|parked|blocked]
  arw task show|park|resume <task-id>
  arw review prepare|status <task-id>
  arw review approve --confirm <task-id>
  arw review request-changes <task-id> [--reason text]
  arw worktree open <task-id>

Add --format json to receive the stable machine interface.
`)
}
