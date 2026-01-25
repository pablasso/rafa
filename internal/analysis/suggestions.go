package analysis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pablasso/rafa/internal/plan"
)

// Suggestion represents a suggested AGENTS.md addition.
type Suggestion struct {
	Category    string // e.g., "Testing", "Formatting", "Common Issues"
	Title       string
	Description string
	Example     string // Optional code/command example
}

// Analyzer generates AGENTS.md suggestions from plan execution data.
type Analyzer struct {
	planDir string
	plan    *plan.Plan
}

// NewAnalyzer creates a new analyzer for the given plan.
func NewAnalyzer(planDir string, p *plan.Plan) *Analyzer {
	return &Analyzer{
		planDir: planDir,
		plan:    p,
	}
}

// Analyze examines the plan execution and generates suggestions.
func (a *Analyzer) Analyze() ([]Suggestion, error) {
	events, err := a.loadProgressEvents()
	if err != nil {
		return nil, err
	}

	outputLines, err := a.loadOutputLog()
	if err != nil {
		// Output log is optional
		outputLines = nil
	}

	var suggestions []Suggestion

	// Analyze retry patterns
	retrySuggestions := a.analyzeRetries(events)
	suggestions = append(suggestions, retrySuggestions...)

	// Analyze failure patterns in output
	if outputLines != nil {
		failureSuggestions := a.analyzeFailurePatterns(outputLines)
		suggestions = append(suggestions, failureSuggestions...)
	}

	// Analyze successful patterns
	successSuggestions := a.analyzeSuccessPatterns(events, outputLines)
	suggestions = append(suggestions, successSuggestions...)

	// Deduplicate similar suggestions
	suggestions = deduplicate(suggestions)

	return suggestions, nil
}

// loadProgressEvents reads and parses the progress log.
func (a *Analyzer) loadProgressEvents() ([]plan.ProgressEvent, error) {
	path := filepath.Join(a.planDir, "progress.log")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []plan.ProgressEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event plan.ProgressEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // Skip malformed lines
		}
		events = append(events, event)
	}

	return events, scanner.Err()
}

// loadOutputLog reads the output log.
func (a *Analyzer) loadOutputLog() ([]string, error) {
	path := filepath.Join(a.planDir, "output.log")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

// analyzeRetries looks for tasks that required multiple attempts.
func (a *Analyzer) analyzeRetries(events []plan.ProgressEvent) []Suggestion {
	taskAttempts := make(map[string]int)

	for _, event := range events {
		if event.Event == plan.EventTaskStarted {
			taskID, _ := event.Data["task_id"].(string)
			attempt, _ := event.Data["attempt"].(float64)
			if int(attempt) > taskAttempts[taskID] {
				taskAttempts[taskID] = int(attempt)
			}
		}
	}

	var suggestions []Suggestion

	// Find tasks with >1 attempt
	for taskID, attempts := range taskAttempts {
		if attempts > 1 {
			task := a.findTask(taskID)
			if task != nil {
				suggestions = append(suggestions, Suggestion{
					Category:    "Common Issues",
					Title:       fmt.Sprintf("Task '%s' required %d attempts", task.Title, attempts),
					Description: "Consider adding more specific acceptance criteria or breaking this task into smaller pieces.",
				})
			}
		}
	}

	return suggestions
}

// analyzeFailurePatterns looks for common failure patterns in output.
func (a *Analyzer) analyzeFailurePatterns(lines []string) []Suggestion {
	var suggestions []Suggestion

	// Pattern: test failures
	testFailures := 0
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "fail") && strings.Contains(lower, "test") {
			testFailures++
		}
	}
	if testFailures > 2 {
		suggestions = append(suggestions, Suggestion{
			Category:    "Testing",
			Title:       "Multiple test failures observed",
			Description: "Agents encountered test failures during execution. Consider documenting test requirements.",
			Example:     "## Testing\n\nRun tests before committing:\n\n```bash\nmake test\n```",
		})
	}

	// Pattern: formatting issues
	fmtIssues := 0
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "fmt") || strings.Contains(lower, "format") || strings.Contains(lower, "lint") {
			if strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(lower, "differ") {
				fmtIssues++
			}
		}
	}
	if fmtIssues > 0 {
		suggestions = append(suggestions, Suggestion{
			Category:    "Formatting",
			Title:       "Formatting issues detected",
			Description: "Code formatting checks failed. Document the formatting command.",
			Example:     "## Formatting\n\nAlways run the formatter after modifying code:\n\n```bash\nmake fmt\n```",
		})
	}

	// Pattern: import/dependency issues
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "cannot find module") || strings.Contains(lower, "module not found") {
			suggestions = append(suggestions, Suggestion{
				Category:    "Dependencies",
				Title:       "Module/dependency issues",
				Description: "Dependency resolution issues occurred. Document how to install dependencies.",
			})
			break
		}
	}

	return suggestions
}

// analyzeSuccessPatterns looks for patterns in successful execution.
func (a *Analyzer) analyzeSuccessPatterns(events []plan.ProgressEvent, outputLines []string) []Suggestion {
	var suggestions []Suggestion

	// If plan completed successfully with tasks that had verification commands
	planCompleted := false
	for _, event := range events {
		if event.Event == plan.EventPlanCompleted {
			planCompleted = true
			break
		}
	}

	if planCompleted && len(a.plan.Tasks) > 0 {
		// Look for common verification patterns in acceptance criteria
		verifyCommands := make(map[string]int)
		for _, task := range a.plan.Tasks {
			for _, criterion := range task.AcceptanceCriteria {
				lower := strings.ToLower(criterion)
				if strings.Contains(lower, "make test") {
					verifyCommands["make test"]++
				}
				if strings.Contains(lower, "make fmt") {
					verifyCommands["make fmt"]++
				}
				if strings.Contains(lower, "go build") {
					verifyCommands["go build"]++
				}
			}
		}

		// Suggest documenting frequently used commands
		for cmd, count := range verifyCommands {
			if count >= 2 {
				suggestions = append(suggestions, Suggestion{
					Category:    "Verification",
					Title:       fmt.Sprintf("'%s' used in %d tasks", cmd, count),
					Description: "This command was used for verification across multiple tasks. Document it prominently.",
				})
			}
		}
	}

	return suggestions
}

// findTask finds a task by ID.
func (a *Analyzer) findTask(taskID string) *plan.Task {
	for i := range a.plan.Tasks {
		if a.plan.Tasks[i].ID == taskID {
			return &a.plan.Tasks[i]
		}
	}
	return nil
}

// deduplicate removes similar suggestions.
func deduplicate(suggestions []Suggestion) []Suggestion {
	seen := make(map[string]bool)
	var result []Suggestion

	for _, s := range suggestions {
		key := s.Category + ":" + s.Title
		if !seen[key] {
			seen[key] = true
			result = append(result, s)
		}
	}

	return result
}

// FormatSuggestions formats suggestions for display.
func FormatSuggestions(suggestions []Suggestion) string {
	if len(suggestions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("================================================================\n")
	sb.WriteString("  SUGGESTED AGENTS.md ADDITIONS\n")
	sb.WriteString("================================================================\n")
	sb.WriteString("\n")
	sb.WriteString("Based on this plan execution, consider adding to AGENTS.md:\n\n")

	// Group by category
	byCategory := make(map[string][]Suggestion)
	for _, s := range suggestions {
		byCategory[s.Category] = append(byCategory[s.Category], s)
	}

	// Sort categories for consistent output
	var categories []string
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		sb.WriteString(fmt.Sprintf("## %s\n\n", cat))
		for _, s := range byCategory[cat] {
			sb.WriteString(fmt.Sprintf("- %s\n", s.Title))
			sb.WriteString(fmt.Sprintf("  %s\n", s.Description))
			if s.Example != "" {
				sb.WriteString("\n  Suggested content:\n")
				for _, line := range strings.Split(s.Example, "\n") {
					sb.WriteString(fmt.Sprintf("  %s\n", line))
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("================================================================\n")
	sb.WriteString("Note: Review these suggestions and add relevant ones to AGENTS.md\n")

	return sb.String()
}
