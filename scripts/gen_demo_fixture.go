// Command gen_demo_fixture generates the embedded demo dataset fixture.
//
// Usage:
//
//	go run ./scripts/gen_demo_fixture.go
//
// It reads from a real `.rafa/plans/<id>-<name>/` directory, parses `output.log`
// into demo Events, applies basic redaction/truncation, and writes a curated
// fixture to `internal/demo/fixtures/default.v1.json`.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pablasso/rafa/internal/demo"
	"github.com/pablasso/rafa/internal/plan"
)

type fixtureV1 struct {
	Version int `json:"version"`
	Plan    struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Tasks []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"tasks"`
	} `json:"plan"`
	Attempts []demo.TaskAttempt `json:"attempts"`
}

var (
	headerPattern = regexp.MustCompile(`^=== Task ([^,]+), Attempt ([0-9]+) ===$`)
	footerPattern = regexp.MustCompile(`^=== Task ([^:]+): (SUCCESS|FAILED) ===$`)
)

type attemptRecord struct {
	Attempt int
	Success bool
	Events  []demo.Event
}

type attemptBuilder struct {
	repoRoot            string
	repoRootSlash       string
	home                string
	homeSlash           string
	maxEvents           int
	maxOutputChunkBytes int
	maxTextBytes        int
	maxToolTargetBytes  int

	events     []demo.Event
	outputBuf  strings.Builder
	lastUsage  *demo.Event
	reachedCap bool
}

func newAttemptBuilder(repoRoot, home string, maxEvents, maxOutputChunkBytes, maxTextBytes, maxToolTargetBytes int) *attemptBuilder {
	b := &attemptBuilder{
		repoRoot:            repoRoot,
		repoRootSlash:       filepath.ToSlash(repoRoot),
		home:                home,
		homeSlash:           filepath.ToSlash(home),
		maxEvents:           maxEvents,
		maxOutputChunkBytes: maxOutputChunkBytes,
		maxTextBytes:        maxTextBytes,
		maxToolTargetBytes:  maxToolTargetBytes,
	}
	return b
}

func (b *attemptBuilder) addParsed(events []demo.Event) {
	for _, ev := range events {
		switch ev.Type {
		case demo.EventOutput:
			b.appendOutput(ev.Text)
		case demo.EventToolUse:
			b.flushOutput()
			ev.ToolTarget = b.normalizeToolTarget(ev.ToolTarget)
			ev.ToolTarget = truncateUTF8(ev.ToolTarget, b.maxToolTargetBytes)
			b.appendEvent(ev)
		case demo.EventUsage:
			b.flushOutput()
			tmp := ev
			b.lastUsage = &tmp
			b.appendEvent(ev)
		default:
			b.flushOutput()
			b.appendEvent(ev)
		}
	}
}

func (b *attemptBuilder) appendOutput(text string) {
	if b.reachedCap || text == "" {
		return
	}
	text = b.normalizeText(text)
	b.outputBuf.WriteString(text)

	if strings.Contains(text, "\n") || (b.maxOutputChunkBytes > 0 && b.outputBuf.Len() >= b.maxOutputChunkBytes) {
		b.flushOutput()
	}
}

func (b *attemptBuilder) flushOutput() {
	if b.reachedCap || b.outputBuf.Len() == 0 {
		b.outputBuf.Reset()
		return
	}
	text := b.outputBuf.String()
	b.outputBuf.Reset()

	text = truncateUTF8(text, b.maxTextBytes)
	b.appendEvent(demo.Event{Type: demo.EventOutput, Text: text})
}

func (b *attemptBuilder) appendEvent(ev demo.Event) {
	if b.reachedCap {
		return
	}
	if b.maxEvents > 0 && len(b.events) >= b.maxEvents {
		b.reachedCap = true
		return
	}
	switch ev.Type {
	case demo.EventOutput:
		ev.Text = truncateUTF8(b.normalizeText(ev.Text), b.maxTextBytes)
	}
	b.events = append(b.events, ev)
}

func (b *attemptBuilder) finalize() []demo.Event {
	b.flushOutput()

	events := b.events
	// Ensure a usage event is present at the end if it was seen.
	if b.lastUsage != nil && (len(events) == 0 || events[len(events)-1].Type != demo.EventUsage) {
		events = append(events, *b.lastUsage)
	}
	return events
}

func (b *attemptBuilder) normalizeText(text string) string {
	if b.repoRootSlash != "" {
		text = strings.ReplaceAll(text, b.repoRootSlash+`/`, "")
		text = strings.ReplaceAll(text, b.repoRootSlash+string(os.PathSeparator), "")
	}
	if b.homeSlash != "" {
		text = strings.ReplaceAll(text, b.homeSlash+`/`, "<HOME>/")
		text = strings.ReplaceAll(text, b.homeSlash, "<HOME>")
	}
	return text
}

func (b *attemptBuilder) normalizeToolTarget(target string) string {
	if target == "" {
		return ""
	}

	// Convert absolute paths to repo-relative when possible.
	if filepath.IsAbs(target) && b.repoRoot != "" {
		if rel, err := filepath.Rel(b.repoRoot, target); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}

	// Replace home prefix to avoid user-specific paths.
	if b.home != "" && strings.HasPrefix(target, b.home) {
		return "<HOME>" + filepath.ToSlash(strings.TrimPrefix(target, b.home))
	}

	return b.normalizeText(filepath.ToSlash(target))
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	if maxBytes <= 3 {
		return s[:maxBytes]
	}
	target := maxBytes - 3
	i := 0
	for i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		if i+size > target {
			break
		}
		i += size
	}
	if i == 0 {
		return s[:target] + "..."
	}
	return s[:i] + "..."
}

func main() {
	var (
		sourceDir           string
		outPath             string
		maxTasks            int
		maxEventsPerTask    int
		maxOutputChunkBytes int
		maxTextBytes        int
		maxToolTargetBytes  int
	)

	flag.StringVar(&sourceDir, "source", ".rafa/plans/KG8JBy-rafa-workflow-orchestration", "Source .rafa plan directory")
	flag.StringVar(&outPath, "out", "internal/demo/fixtures/default.v1.json", "Output fixture path")
	flag.IntVar(&maxTasks, "max-tasks", 0, "Max tasks to include (0 = all)")
	flag.IntVar(&maxEventsPerTask, "max-events", 450, "Max events per task attempt to include in the fixture")
	flag.IntVar(&maxOutputChunkBytes, "chunk-bytes", 768, "Max bytes per output event chunk before flushing")
	flag.IntVar(&maxTextBytes, "max-text-bytes", 4096, "Max bytes for output text/tool target strings")
	flag.IntVar(&maxToolTargetBytes, "max-tool-target-bytes", 2048, "Max bytes for tool targets")
	flag.Parse()

	planDir, err := filepath.Abs(sourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve source dir: %v\n", err)
		os.Exit(1)
	}

	planObj, err := plan.LoadPlan(planDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load plan: %v\n", err)
		os.Exit(1)
	}

	taskLimit := maxTasks
	if taskLimit <= 0 || taskLimit > len(planObj.Tasks) {
		taskLimit = len(planObj.Tasks)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)
		os.Exit(1)
	}
	home, _ := os.UserHomeDir()

	bestSuccess := make(map[string]attemptRecord)
	bestAny := make(map[string]attemptRecord)

	outputLog := filepath.Join(planDir, "output.log")
	if err := scanOutputLog(outputLog, repoRoot, home, maxEventsPerTask, maxOutputChunkBytes, maxTextBytes, maxToolTargetBytes, func(taskID string, record attemptRecord) {
		any, ok := bestAny[taskID]
		if !ok || record.Attempt > any.Attempt {
			bestAny[taskID] = record
		}

		if !record.Success {
			return
		}
		best, ok := bestSuccess[taskID]
		if !ok || record.Attempt > best.Attempt {
			bestSuccess[taskID] = record
		}
	}); err != nil {
		fmt.Fprintf(os.Stderr, "parse output log: %v\n", err)
		os.Exit(1)
	}

	var fx fixtureV1
	fx.Version = 1
	fx.Plan.ID = "DEMO"
	fx.Plan.Name = "demo"
	for _, task := range planObj.Tasks[:taskLimit] {
		fx.Plan.Tasks = append(fx.Plan.Tasks, struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}{
			ID:    task.ID,
			Title: task.Title,
		})
	}

	for _, task := range planObj.Tasks[:taskLimit] {
		record, ok := bestSuccess[task.ID]
		if !ok {
			record, ok = bestAny[task.ID]
		}
		if !ok {
			continue
		}
		fx.Attempts = append(fx.Attempts, demo.TaskAttempt{
			TaskID:  task.ID,
			Attempt: 1,
			Success: true,
			Events:  record.Events,
		})
	}

	// Ensure stable output ordering.
	sort.SliceStable(fx.Attempts, func(i, j int) bool {
		return fx.Attempts[i].TaskID < fx.Attempts[j].TaskID
	})

	data, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal fixture: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write fixture: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %s (%d bytes)\n", outPath, len(data))
}

func scanOutputLog(
	path string,
	repoRoot string,
	home string,
	maxEventsPerTask int,
	maxOutputChunkBytes int,
	maxTextBytes int,
	maxToolTargetBytes int,
	onAttempt func(taskID string, record attemptRecord),
) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var (
		currentTaskID  string
		currentAttempt int
		currentSuccess bool
		builder        *attemptBuilder
	)

	flushCurrent := func() {
		if currentTaskID == "" || builder == nil {
			return
		}
		record := attemptRecord{
			Attempt: currentAttempt,
			Success: currentSuccess,
			Events:  builder.finalize(),
		}
		onAttempt(currentTaskID, record)
		currentTaskID = ""
		currentAttempt = 0
		currentSuccess = false
		builder = nil
	}

	for scanner.Scan() {
		line := scanner.Text()

		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			flushCurrent()
			currentTaskID = matches[1]
			currentAttempt = atoi(matches[2])
			builder = newAttemptBuilder(repoRoot, home, maxEventsPerTask, maxOutputChunkBytes, maxTextBytes, maxToolTargetBytes)
			continue
		}

		if matches := footerPattern.FindStringSubmatch(line); matches != nil {
			if currentTaskID != "" && matches[1] == currentTaskID {
				currentSuccess = matches[2] == "SUCCESS"
			}
			flushCurrent()
			continue
		}

		if builder == nil {
			continue
		}

		events, result := demo.ParseOutputLine(line)
		if result != nil {
			currentSuccess = *result
		}
		builder.addParsed(events)
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	flushCurrent()
	return nil
}

func atoi(value string) int {
	var parsed int
	fmt.Sscanf(value, "%d", &parsed)
	return parsed
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repo root from %s", cwd)
		}
		dir = parent
	}
}
