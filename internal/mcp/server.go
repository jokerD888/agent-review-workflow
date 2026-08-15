package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jokerD888/agent-review-workflow/internal/app"
	"github.com/jokerD888/agent-review-workflow/internal/task"
)

const ProtocolVersion = "2025-03-26"

type Server struct {
	In      io.Reader
	Out     io.Writer
	Err     io.Writer
	Version string
}
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type toolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s Server) Run() error {
	if s.In == nil {
		s.In = os.Stdin
	}
	if s.Out == nil {
		s.Out = os.Stdout
	}
	if s.Err == nil {
		s.Err = os.Stderr
	}
	scanner := bufio.NewScanner(s.In)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	encoder := json.NewEncoder(s.Out)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = encoder.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		if len(req.ID) == 0 {
			continue
		} // MCP notifications intentionally do not receive a response.
		result, err := s.handle(req)
		if err != nil {
			_ = encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
			continue
		}
		if err := encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Result: result}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s Server) handle(req request) (any, error) {
	switch req.Method {
	case "initialize":
		version := s.Version
		if version == "" {
			version = "0.2.0-dev"
		}
		return map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "arw-mcp", "version": version}, "instructions": "Use only the ARW task tools. High-risk Git operations are intentionally unavailable."}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": definitions()}, nil
	case "tools/call":
		var call toolCall
		if err := json.Unmarshal(req.Params, &call); err != nil {
			return nil, fmt.Errorf("invalid tools/call parameters: %w", err)
		}
		value, err := execute(call)
		if err != nil {
			return map[string]any{"content": []map[string]string{{"type": "text", "text": err.Error()}}, "isError": true}, nil
		}
		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return map[string]any{"content": []map[string]string{{"type": "text", "text": string(data)}}, "structuredContent": value}, nil
	default:
		return nil, fmt.Errorf("method %q is not supported", req.Method)
	}
}

func execute(call toolCall) (any, error) {
	var args map[string]any
	if len(call.Arguments) == 0 {
		args = map[string]any{}
	} else if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid tool arguments: %w", err)
	}
	cwd, _ := args["cwd"].(string)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	svc, err := app.New(cwd)
	if err != nil {
		return nil, err
	}
	switch call.Name {
	case "workflow_context":
		entries, err := svc.Tasks()
		if err != nil {
			return nil, err
		}
		return map[string]any{"repository": svc.Git.Root, "tasks": entries}, nil
	case "workflow_list_tasks":
		entries, err := svc.Tasks()
		if err != nil {
			return nil, err
		}
		filter, _ := args["filter"].(string)
		return filterTasks(entries, filter), nil
	case "workflow_start_task":
		title, ok := args["title"].(string)
		if !ok || strings.TrimSpace(title) == "" {
			return nil, fmt.Errorf("title is required")
		}
		parent, _ := args["parent_task"].(string)
		id, _ := args["id"].(string)
		kind, _ := args["kind"].(string)
		return svc.Start(app.StartOptions{Title: title, ParentTask: parent, ID: id, Kind: kind})
	case "workflow_prepare_review":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		return svc.PrepareReview(id)
	case "workflow_open_review":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		newWindow, _ := args["new_window"].(bool)
		if !newWindow {
			path, err := svc.WorktreePath(id)
			if err != nil {
				return nil, err
			}
			return map[string]any{"taskId": id, "worktree": path, "opened": false}, nil
		}
		path, err := svc.OpenWorktree(id)
		if err != nil {
			return nil, err
		}
		return map[string]any{"taskId": id, "worktree": path, "opened": true}, nil
	case "workflow_park_task":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		return svc.Park(id)
	case "workflow_resume_task":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		return svc.Resume(id)
	case "workflow_mark_ready":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		return svc.MarkReady(id)
	case "workflow_record_test":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		command, err := requiredString(args, "command")
		if err != nil {
			return nil, err
		}
		result, err := requiredString(args, "result")
		if err != nil {
			return nil, err
		}
		evidence := task.TestEvidence{Command: command, Result: result}
		if summary, ok := args["summary"].(string); ok {
			evidence.Summary = summary
		}
		if code, ok := args["exit_code"].(float64); ok {
			if code != float64(int(code)) {
				return nil, fmt.Errorf("exit_code must be an integer")
			}
			value := int(code)
			evidence.ExitCode = &value
		}
		return svc.RecordTest(id, evidence)
	case "workflow_mark_merged":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		if confirmed, _ := args["confirm"].(bool); !confirmed {
			return nil, fmt.Errorf("confirm must be true before recording a merge")
		}
		return svc.MarkMerged(id)
	case "workflow_abandon_task":
		id, err := requiredString(args, "task_id")
		if err != nil {
			return nil, err
		}
		if confirmed, _ := args["confirm"].(bool); !confirmed {
			return nil, fmt.Errorf("confirm must be true before abandoning a task")
		}
		return svc.Abandon(id)
	case "workflow_refresh":
		entries, err := svc.Tasks()
		if err != nil {
			return nil, err
		}
		return map[string]any{"repository": svc.Git.Root, "tasks": len(entries)}, nil
	default:
		return nil, fmt.Errorf("tool %q is not exposed", call.Name)
	}
}

func requiredString(args map[string]any, name string) (string, error) {
	value, ok := args[name].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}
func filterTasks(entries []task.Task, filter string) []task.Task {
	if filter == "" {
		return entries
	}
	result := []task.Task{}
	for _, entry := range entries {
		if filter == "active" && (entry.Lifecycle == task.Active || entry.Lifecycle == task.InReview || entry.Lifecycle == task.ReadyForReview) || filter == "parked" && entry.Lifecycle == task.Parked || filter == "reviewable" && (entry.Lifecycle == task.ReadyForReview || entry.Lifecycle == task.InReview || entry.Lifecycle == task.Approved) || filter == "blocked" && entry.ParentTask != "" && entry.Review.Status != task.ReviewApproved {
			result = append(result, entry)
		}
	}
	return result
}

func definitions() []map[string]any {
	cwd := map[string]any{"cwd": map[string]any{"type": "string", "description": "Repository worktree to inspect. Defaults to the MCP process working directory."}}
	withTask := func(extra map[string]any) map[string]any {
		properties := map[string]any{"task_id": map[string]any{"type": "string"}, "cwd": cwd["cwd"]}
		for key, value := range extra {
			properties[key] = value
		}
		return properties
	}
	return []map[string]any{
		{"name": "workflow_context", "description": "Read the current repository and known ARW tasks before taking task actions.", "inputSchema": map[string]any{"type": "object", "properties": cwd}},
		{"name": "workflow_list_tasks", "description": "List tasks tracked by ARW. This is read-only.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"filter": map[string]any{"type": "string", "enum": []string{"active", "reviewable", "parked", "blocked"}}, "cwd": cwd["cwd"]}}},
		{"name": "workflow_start_task", "description": "Create a task branch, task worktree, and ledger record. Does not touch protected branches or remotes.", "inputSchema": map[string]any{"type": "object", "required": []string{"title"}, "properties": map[string]any{"title": map[string]any{"type": "string"}, "id": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string"}, "parent_task": map[string]any{"type": "string"}, "cwd": cwd["cwd"]}}},
		{"name": "workflow_prepare_review", "description": "Create a review snapshot using the task's recorded base or parent task. Read-only with respect to source code.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id"}, "properties": withTask(nil)}},
		{"name": "workflow_open_review", "description": "Locate a task worktree; set new_window true only when the user explicitly asks to open VS Code.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id"}, "properties": withTask(map[string]any{"new_window": map[string]any{"type": "boolean"}})}},
		{"name": "workflow_park_task", "description": "Park a task while preserving its branch, worktree, and review history.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id"}, "properties": withTask(nil)}},
		{"name": "workflow_resume_task", "description": "Resume a parked task.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id"}, "properties": withTask(nil)}},
		{"name": "workflow_mark_ready", "description": "Record that a task is ready for review. This does not merge, push, or modify source code.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id"}, "properties": withTask(nil)}},
		{"name": "workflow_record_test", "description": "Record existing test evidence only; this tool never executes the command.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id", "command", "result"}, "properties": withTask(map[string]any{"command": map[string]any{"type": "string"}, "result": map[string]any{"type": "string", "enum": []string{"passed", "failed", "skipped", "unknown"}}, "exit_code": map[string]any{"type": "integer"}, "summary": map[string]any{"type": "string"}})}},
		{"name": "workflow_mark_merged", "description": "Record a completed merge after the user confirms it occurred. This never performs a merge or remote action.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id", "confirm"}, "properties": withTask(map[string]any{"confirm": map[string]any{"type": "boolean"}})}},
		{"name": "workflow_abandon_task", "description": "Record that a task was abandoned after the user confirms it. This retains its branch and worktree.", "inputSchema": map[string]any{"type": "object", "required": []string{"task_id", "confirm"}, "properties": withTask(map[string]any{"confirm": map[string]any{"type": "boolean"}})}},
		{"name": "workflow_refresh", "description": "Refresh local task context without modifying Git history.", "inputSchema": map[string]any{"type": "object", "properties": cwd}},
	}
}
