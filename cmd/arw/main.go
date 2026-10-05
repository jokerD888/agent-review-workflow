package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jokerD888/agent-review-workflow/internal/app"
	"github.com/jokerD888/agent-review-workflow/internal/ledger"
	"github.com/jokerD888/agent-review-workflow/internal/task"
)

var version = "0.2.0-dev"

func main() {
	if err := runWithIO(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if hasJSON(os.Args[1:]) {
			payload, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Fprintln(os.Stderr, string(payload))
		} else {
			fmt.Fprintln(os.Stderr, "arw:", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	return runWithIO(args, os.Stdout, os.Stderr)
}

func runWithIO(args []string, stdout, stderr io.Writer) error {
	cleanArgs, globalJSON := withoutFormat(args)
	if len(cleanArgs) == 0 || cleanArgs[0] == "help" || cleanArgs[0] == "--help" || cleanArgs[0] == "-h" {
		printHelp(stdout)
		return nil
	}
	if cleanArgs[0] == "version" {
		if globalJSON {
			return outputWriter(map[string]string{"version": version}, true, stdout)
		}
		fmt.Fprintln(stdout, version)
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
	switch cleanArgs[0] {
	case "setup":
		if err := svc.Setup(); err != nil {
			return err
		}
		if globalJSON {
			return outputWriter(map[string]any{"status": "ok", "message": "registry initialized"}, true, stdout)
		}
		return nil
	case "doctor":
		return doctor(svc, globalJSON || hasJSON(args), stdout)
	case "task":
		return taskCommand(svc, cleanArgs[1:], globalJSON, stdout, stderr)
	case "review":
		return reviewCommand(svc, cleanArgs[1:], globalJSON, stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q; run 'arw help'", cleanArgs[0])
	}
}

func taskCommand(svc app.Service, args []string, globalJSON bool, stdout, stderr io.Writer) error {
	args, localJSON := withoutFormat(args)
	jsonOutput := globalJSON || localJSON
	if len(args) == 0 {
		return errors.New("usage: arw task <start|list|show|park|resume|ready|merge|abandon|clear> ...")
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("task start", flag.ContinueOnError)
		fs.SetOutput(stderr)
		id := fs.String("id", "", "task id")
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
		result, err := svc.Start(app.StartOptions{Title: title, ID: *id, BaseRef: *base, ParentTask: *parent, WorktreePath: *worktreePath})
		if err != nil {
			return err
		}
		return outputWriter(result, *format == "json" || jsonOutput, stdout)
	case "list":
		fs := flag.NewFlagSet("task list", flag.ContinueOnError)
		fs.SetOutput(stderr)
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
		return outputWriter(entries, *format == "json" || jsonOutput, stdout)
	case "show":
		if len(args) < 2 {
			return errors.New("usage: arw task show <task-id> [--format json]")
		}
		entry, err := svc.Task(args[1])
		if err != nil {
			return err
		}
		return outputWriter(entry, jsonOutput, stdout)
	case "park":
		if len(args) < 2 {
			return errors.New("usage: arw task park <task-id> [--format json]")
		}
		entry, err := svc.Park(args[1])
		if err != nil {
			return err
		}
		return outputWriter(entry, jsonOutput, stdout)
	case "resume":
		if len(args) < 2 {
			return errors.New("usage: arw task resume <task-id> [--format json]")
		}
		entry, err := svc.Resume(args[1])
		if err != nil {
			return err
		}
		return outputWriter(entry, jsonOutput, stdout)
	case "ready":
		if len(args) != 2 {
			return errors.New("usage: arw task ready <task-id> [--format json]")
		}
		entry, err := svc.MarkReady(args[1])
		if err != nil {
			return err
		}
		return outputWriter(entry, jsonOutput, stdout)
	case "merge":
		mergeArgs, confirm := stripBool(args[1:], "--confirm")
		if len(mergeArgs) != 1 || !confirm {
			return errors.New("usage: arw task merge --confirm <task-id>")
		}
		result, err := svc.Merge(mergeArgs[0])
		if err != nil {
			return err
		}
		return outputWriter(result, jsonOutput, stdout)
	case "abandon":
		abandonArgs, confirm := stripBool(args[1:], "--confirm")
		if len(abandonArgs) != 1 || !confirm {
			return errors.New("usage: arw task abandon --confirm <task-id>")
		}
		entry, err := svc.Abandon(abandonArgs[0])
		if err != nil {
			return err
		}
		return outputWriter(entry, jsonOutput, stdout)
	case "clear":
		clearArgs, confirm := stripBool(args[1:], "--confirm")
		if !confirm {
			return errors.New("clear deletes branches and worktrees; repeat with --confirm")
		}
		if len(clearArgs) == 1 {
			result, err := svc.Clear(clearArgs[0])
			if err != nil {
				return err
			}
			return outputWriter(result, jsonOutput, stdout)
		}
		if len(clearArgs) == 0 {
			results, err := svc.ClearMerged()
			if err != nil {
				return err
			}
			return outputWriter(results, jsonOutput, stdout)
		}
		return errors.New("usage: arw task clear --confirm <task-id> | arw task clear --all-merged --confirm")
	default:
		return fmt.Errorf("unknown task command %q", args[0])
	}
}

func reviewCommand(svc app.Service, args []string, globalJSON bool, stdout, stderr io.Writer) error {
	args, localJSON := withoutFormat(args)
	jsonOutput := globalJSON || localJSON
	if len(args) == 0 {
		return errors.New("usage: arw review <prepare|approve|request-changes> <task-id>")
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
		return outputWriter(snapshot, jsonOutput, stdout)
	case "approve":
		approveArgs, confirm := stripBool(args[1:], "--confirm")
		if !confirm {
			return errors.New("approval changes the task audit record; repeat with --confirm after human review")
		}
		fs := flag.NewFlagSet("review approve", flag.ContinueOnError)
		fs.SetOutput(stderr)
		expectedBase := fs.String("base", "", "reviewed base SHA")
		expectedHead := fs.String("head", "", "reviewed HEAD SHA")
		if err := fs.Parse(approveArgs); err != nil {
			return err
		}
		if len(fs.Args()) != 1 || *expectedBase == "" || *expectedHead == "" {
			return errors.New("usage: arw review approve --confirm --base <sha> --head <sha> <task-id>")
		}
		entry, snapshot, err := svc.Approve(fs.Args()[0], *expectedBase, *expectedHead)
		if err != nil {
			return err
		}
		return outputWriter(struct {
			Task     task.Task `json:"task"`
			Snapshot any       `json:"snapshot"`
		}{entry, snapshot}, jsonOutput, stdout)
	case "request-changes":
		fs := flag.NewFlagSet("review request-changes", flag.ContinueOnError)
		fs.SetOutput(stderr)
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
		return outputWriter(entry, *format == "json" || jsonOutput, stdout)
	default:
		return fmt.Errorf("unknown review command %q", args[0])
	}
}

func doctor(svc app.Service, jsonOutput bool, stdout io.Writer) error {
	entries, err := svc.Tasks()
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	data := map[string]any{"version": version, "repository": svc.Git.Root, "registryBranch": ledger.RegistryBranch, "registryExists": svc.Git.BranchExists(ledger.RegistryBranch), "tasks": len(entries)}
	return outputWriter(data, jsonOutput, stdout)
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
			keep = entry.Lifecycle == task.Active || entry.Lifecycle == task.ReadyForReview
		case "parked":
			keep = entry.Lifecycle == task.Parked
		case "reviewable":
			keep = entry.Lifecycle == task.ReadyForReview
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
	return outputWriter(value, jsonOutput, os.Stdout)
}

func outputWriter(value any, jsonOutput bool, stdout io.Writer) error {
	if jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	switch v := value.(type) {
	case []task.Task:
		for _, entry := range v {
			fmt.Fprintf(stdout, "%-28s %-18s %-18s %s\n", entry.ID, entry.Lifecycle, entry.Review.Status, entry.Title)
		}
		return nil
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `ARW v2 (development)

Usage:
  arw setup | doctor [--json]
  arw task start [--id id] [--base ref] [--parent task-id] [--json] <title>
  arw task list [--view reviewable|active|parked|blocked] [--json]
  arw task show|park|resume|ready <task-id> [--json]
  arw task merge --confirm <task-id> [--json]
  arw task abandon --confirm <task-id> [--json]
  arw task clear --confirm <task-id> [--json]
  arw task clear --confirm [--json]         # clear all merged/abandoned
  arw review prepare <task-id> [--json]
  arw review approve --confirm --base <sha> --head <sha> <task-id> [--json]
  arw review request-changes <task-id> [--reason text] [--json]

Add --json or --format json to receive the stable machine interface.
`)
}
