// task_tools.go — task_write and task_read tool implementations.
// Tasks are stored as a markdown file in the project directory (project-scoped).
// Layer: tool execution. Dependencies: internal/platform (project dir resolution).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"blazeai/internal/platform"
)

const tasksFileName = "tasks.md"

// TaskWriteArgs are the arguments for task_write.
//
// WHAT:  Parsed arguments for writing the project task list.
// PARAMS: Tasks — full markdown content to write (overwrite).
type TaskWriteArgs struct {
	Tasks string `json:"tasks"`
}

// TaskWriteTool writes the project task list to disk.
//
// WHAT:  Implements the task_write tool — overwrites tasks.md with markdown content.
// WHY:   The LLM uses this to persist and update its task list for the current project.
// PARAMS: workDir — function returning the current working directory.
type TaskWriteTool struct {
	workDir func() string
}

// NewTaskWriteTool creates a TaskWriteTool bound to the given workDir getter.
//
// PARAMS: workDir — closure returning the current working directory.
// RETURNS: *TaskWriteTool — ready to execute.
func NewTaskWriteTool(workDir func() string) *TaskWriteTool {
	return &TaskWriteTool{workDir: workDir}
}

func (t *TaskWriteTool) Name() string { return "task_write" }

func (t *TaskWriteTool) Description() string {
	return "Write project task list (markdown). Full overwrite."
}

func (t *TaskWriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"tasks": {
				"type": "string",
				"description": "Full task list in markdown format."
			}
		},
		"required": ["tasks"]
	}`)
}

func (t *TaskWriteTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[TaskWriteArgs](args)
	if err != nil {
		return ""
	}
	return truncateDisplay(parsed.Tasks, 80)
}

func (t *TaskWriteTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[TaskWriteArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Tasks == "" {
		return "error: tasks is required"
	}
	projectDir, err := platform.EnsureProjectDir(t.workDir())
	if err != nil {
		return fmt.Sprintf("error: cannot resolve project dir: %v", err)
	}
	path := filepath.Join(projectDir, tasksFileName)
	if err := os.WriteFile(path, []byte(parsed.Tasks), 0644); err != nil {
		return fmt.Sprintf("error: cannot write tasks: %v", err)
	}
	return "ok"
}

// TaskReadTool reads the project task list from disk.
//
// WHAT:  Implements the task_read tool — reads tasks.md content.
// WHY:   The LLM uses this to review its current task list after resume or /cd.
// PARAMS: workDir — function returning the current working directory.
type TaskReadTool struct {
	workDir func() string
}

// NewTaskReadTool creates a TaskReadTool bound to the given workDir getter.
//
// PARAMS: workDir — closure returning the current working directory.
// RETURNS: *TaskReadTool — ready to execute.
func NewTaskReadTool(workDir func() string) *TaskReadTool {
	return &TaskReadTool{workDir: workDir}
}

func (t *TaskReadTool) Name() string { return "task_read" }

func (t *TaskReadTool) Description() string {
	return "Read project task list."
}

func (t *TaskReadTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *TaskReadTool) FormatArgs(args json.RawMessage) string {
	return ""
}

func (t *TaskReadTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	projectDir, err := platform.EnsureProjectDir(t.workDir())
	if err != nil {
		return fmt.Sprintf("error: cannot resolve project dir: %v", err)
	}
	path := filepath.Join(projectDir, tasksFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "no tasks"
		}
		return fmt.Sprintf("error: cannot read tasks: %v", err)
	}
	if len(data) == 0 {
		return "no tasks"
	}
	return string(data)
}
