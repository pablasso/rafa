package demo

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
)

// Dataset holds the demo plan and parsed output events.
type Dataset struct {
	Plan     *plan.Plan
	Attempts []TaskAttempt
}

// TaskAttempt captures the output events for a single task attempt.
type TaskAttempt struct {
	TaskID  string
	Attempt int
	Success bool
	Events  []Event
}

// EventType identifies demo playback event kinds.
type EventType string

const (
	EventOutput     EventType = "output"
	EventToolUse    EventType = "tool_use"
	EventToolResult EventType = "tool_result"
	EventUsage      EventType = "usage"
)

// Event represents a single output or activity event in the demo playback.
type Event struct {
	Type         EventType
	Text         string
	ToolName     string
	ToolTarget   string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

var (
	headerPattern = regexp.MustCompile(`^=== Task ([^,]+), Attempt ([0-9]+) ===$`)
	footerPattern = regexp.MustCompile(`^=== Task ([^:]+): (SUCCESS|FAILED) ===$`)
)

// LoadDefaultDataset loads the default demo plan/output from the repo.
func LoadDefaultDataset(repoRoot string) (*Dataset, error) {
	planPath := filepath.Join(repoRoot, ".rafa", "plans", "KG8JBy-rafa-workflow-orchestration", "plan.json")
	outputPath := filepath.Join(repoRoot, ".rafa", "plans", "KG8JBy-rafa-workflow-orchestration", "output.log")
	return LoadDataset(planPath, outputPath)
}

// LoadDataset loads the plan and output log into a demo dataset.
func LoadDataset(planPath, outputPath string) (*Dataset, error) {
	p, err := plan.LoadPlan(filepath.Dir(planPath))
	if err != nil {
		return nil, fmt.Errorf("load plan: %w", err)
	}

	attempts, err := parseOutputLog(outputPath)
	if err != nil {
		return nil, err
	}

	return &Dataset{
		Plan:     p,
		Attempts: attempts,
	}, nil
}

func parseOutputLog(path string) ([]TaskAttempt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open output log: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var attempts []TaskAttempt
	var current *TaskAttempt

	for scanner.Scan() {
		line := scanner.Text()
		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			if current != nil {
				attempts = append(attempts, *current)
			}
			attempt, parseErr := parseInt(matches[2])
			if parseErr != nil {
				return nil, parseErr
			}
			current = &TaskAttempt{
				TaskID:  matches[1],
				Attempt: attempt,
			}
			continue
		}

		if matches := footerPattern.FindStringSubmatch(line); matches != nil {
			if current != nil {
				current.Success = matches[2] == "SUCCESS"
				attempts = append(attempts, *current)
				current = nil
			}
			continue
		}

		if current == nil {
			continue
		}

		current.Events = append(current.Events, parseOutputLine(line)...)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan output log: %w", err)
	}

	if current != nil {
		attempts = append(attempts, *current)
	}

	if len(attempts) == 0 {
		return nil, errors.New("no task attempts found in output log")
	}

	return attempts, nil
}

func parseOutputLine(line string) []Event {
	var events []Event

	if text := executor.FormatStreamLine(line); text != "" {
		events = append(events, Event{
			Type: EventOutput,
			Text: text,
		})
	}

	var raw logLine
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return events
	}

	if toolName, toolTarget, ok := raw.toolUse(); ok {
		events = append(events, Event{
			Type:       EventToolUse,
			ToolName:   toolName,
			ToolTarget: toolTarget,
		})
	}

	if raw.isToolResult() {
		events = append(events, Event{Type: EventToolResult})
	}

	if raw.Type == "result" && raw.Usage != nil {
		events = append(events, Event{
			Type:         EventUsage,
			InputTokens:  raw.Usage.InputTokens,
			OutputTokens: raw.Usage.OutputTokens,
			CostUSD:      raw.TotalCostUSD,
		})
	}

	return events
}

type logLine struct {
	Type  string `json:"type"`
	Event *struct {
		Type         string `json:"type"`
		ContentBlock *struct {
			Type  string                 `json:"type"`
			Name  string                 `json:"name,omitempty"`
			Input map[string]interface{} `json:"input,omitempty"`
		} `json:"content_block,omitempty"`
	} `json:"event,omitempty"`
	Message *struct {
		Content []struct {
			Type  string                 `json:"type"`
			Name  string                 `json:"name,omitempty"`
			Input map[string]interface{} `json:"input,omitempty"`
		} `json:"content"`
	} `json:"message,omitempty"`
	Usage *struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage,omitempty"`
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
}

func (l logLine) toolUse() (string, string, bool) {
	if l.Type == "stream_event" && l.Event != nil && l.Event.Type == "content_block_start" && l.Event.ContentBlock != nil {
		if l.Event.ContentBlock.Type == "tool_use" && l.Event.ContentBlock.Name != "" {
			name := l.Event.ContentBlock.Name
			return name, extractToolTarget(name, l.Event.ContentBlock.Input), true
		}
	}

	if l.Type == "assistant" && l.Message != nil {
		for _, content := range l.Message.Content {
			if content.Type == "tool_use" && content.Name != "" {
				name := content.Name
				return name, extractToolTarget(name, content.Input), true
			}
		}
	}

	return "", "", false
}

func (l logLine) isToolResult() bool {
	if l.Type != "user" || l.Message == nil {
		return false
	}

	for _, content := range l.Message.Content {
		if content.Type == "tool_result" {
			return true
		}
	}

	return false
}

func extractToolTarget(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "Read", "Write", "Edit":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Glob", "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "Task":
		if desc, ok := input["description"].(string); ok {
			return desc
		}
	case "Bash":
		if command, ok := input["command"].(string); ok {
			return command
		}
	case "WebFetch":
		if url, ok := input["url"].(string); ok {
			return url
		}
	}
	return ""
}

func parseInt(value string) (int, error) {
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return 0, fmt.Errorf("parse int %q: %w", value, err)
	}
	return parsed, nil
}
