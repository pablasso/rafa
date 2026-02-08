//go:build ignore

// Command gen_demo_create_fixture generates the embedded create-demo fixture.
//
// Usage:
//
//	go run ./scripts/gen_demo_create_fixture.go \
//	  --source-doc docs/designs/plan-create-command.md \
//	  --stream-log /path/to/create-stream.jsonl
//
// The script enforces that source-doc does not already have a plan in .rafa/plans/*/plan.json.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/demo/createfixture"
)

type createFixtureV1 struct {
	Version    int              `json:"version"`
	SourceFile string           `json:"sourceFile"`
	Events     []ai.StreamEvent `json:"events"`
}

func main() {
	var (
		sourceDoc         string
		streamLog         string
		outPath           string
		repoRoot          string
		maxEvents         int
		maxTextBytes      int
		maxToolTargetByte int
	)

	flag.StringVar(&sourceDoc, "source-doc", "", "Source design document used for create-plan capture")
	flag.StringVar(&streamLog, "stream-log", "", "Path to captured Claude stream-json log for create-plan")
	flag.StringVar(&outPath, "out", "internal/demo/fixtures/create.default.v1.json", "Output fixture path")
	flag.StringVar(&repoRoot, "repo-root", "", "Repo root (auto-detected if empty)")
	flag.IntVar(&maxEvents, "max-events", 1200, "Maximum events to include")
	flag.IntVar(&maxTextBytes, "max-text-bytes", 8192, "Maximum bytes per text event")
	flag.IntVar(&maxToolTargetByte, "max-tool-target-bytes", 2048, "Maximum bytes for tool targets")
	flag.Parse()

	if sourceDoc == "" {
		exitf("--source-doc is required")
	}
	if streamLog == "" {
		exitf("--stream-log is required")
	}
	if repoRoot == "" {
		var err error
		repoRoot, err = findRepoRoot()
		if err != nil {
			exitf("find repo root: %v", err)
		}
	}

	if err := createfixture.EnsureSourceFileHasNoPlan(repoRoot, sourceDoc); err != nil {
		exitf("%v", err)
	}
	normalizedSource, err := createfixture.NormalizeSourceFile(repoRoot, sourceDoc)
	if err != nil {
		exitf("normalize source doc: %v", err)
	}

	events, err := parseCreateStreamLog(streamLog, repoRoot, maxEvents, maxTextBytes, maxToolTargetByte)
	if err != nil {
		exitf("parse stream log: %v", err)
	}
	if len(events) == 0 {
		exitf("no replay events parsed")
	}
	if !hasPlanApprovedMarker(events) {
		exitf("captured stream does not include PLAN_APPROVED_JSON marker")
	}

	fixture := createFixtureV1{
		Version:    1,
		SourceFile: normalizedSource,
		Events:     events,
	}

	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		exitf("marshal fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		exitf("mkdir output dir: %v", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		exitf("write fixture: %v", err)
	}

	fmt.Printf("Wrote %s (%d bytes)\n", outPath, len(data))
}

func parseCreateStreamLog(path, repoRoot string, maxEvents, maxTextBytes, maxToolTargetBytes int) ([]ai.StreamEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	home, _ := os.UserHomeDir()
	repoRootAbs, _ := filepath.Abs(repoRoot)
	repoRootSlash := filepath.ToSlash(repoRootAbs)
	homeSlash := filepath.ToSlash(home)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var events []ai.StreamEvent
	for scanner.Scan() {
		line := scanner.Text()
		parsed := parseRawStreamEvent(line)
		for i := range parsed {
			parsed[i].Text = sanitizeText(parsed[i].Text, repoRootSlash, homeSlash, maxTextBytes)
			parsed[i].ToolTarget = sanitizeText(parsed[i].ToolTarget, repoRootSlash, homeSlash, maxToolTargetBytes)
			if parsed[i].Type == "" {
				continue
			}
			events = append(events, parsed[i])
			if maxEvents > 0 && len(events) >= maxEvents {
				return events, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func parseRawStreamEvent(line string) []ai.StreamEvent {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil
	}

	eventType, _ := raw["type"].(string)
	switch eventType {
	case "system":
		subtype, _ := raw["subtype"].(string)
		if subtype == "init" {
			sessionID, _ := raw["session_id"].(string)
			return []ai.StreamEvent{{Type: "init", SessionID: sessionID}}
		}
	case "stream_event":
		return parseNestedStreamEvent(raw)
	case "assistant":
		return parseAssistantEvent(raw)
	case "user":
		return parseUserEvent(raw)
	case "result":
		return parseResultEvent(raw)
	}
	return nil
}

func parseNestedStreamEvent(raw map[string]interface{}) []ai.StreamEvent {
	event, _ := raw["event"].(map[string]interface{})
	if event == nil {
		return nil
	}
	etype, _ := event["type"].(string)
	switch etype {
	case "content_block_delta":
		delta, _ := event["delta"].(map[string]interface{})
		if deltaType, _ := delta["type"].(string); deltaType == "text_delta" {
			text, _ := delta["text"].(string)
			if text != "" {
				return []ai.StreamEvent{{Type: "text", Text: text}}
			}
		}
	case "content_block_start":
		block, _ := event["content_block"].(map[string]interface{})
		if blockType, _ := block["type"].(string); blockType == "tool_use" {
			name, _ := block["name"].(string)
			target := extractToolTarget(name, asMap(block["input"]))
			return []ai.StreamEvent{{Type: "tool_use", ToolName: name, ToolTarget: target}}
		}
	}
	return nil
}

func parseAssistantEvent(raw map[string]interface{}) []ai.StreamEvent {
	message, _ := raw["message"].(map[string]interface{})
	if message == nil {
		return nil
	}
	contents, _ := message["content"].([]interface{})
	sessionID, _ := raw["session_id"].(string)

	var out []ai.StreamEvent
	for _, c := range contents {
		block, _ := c.(map[string]interface{})
		if block == nil {
			continue
		}
		blockType, _ := block["type"].(string)
		switch blockType {
		case "tool_use":
			name, _ := block["name"].(string)
			target := extractToolTarget(name, asMap(block["input"]))
			out = append(out, ai.StreamEvent{
				Type:       "tool_use",
				ToolName:   name,
				ToolTarget: target,
				SessionID:  sessionID,
			})
		case "text":
			text, _ := block["text"].(string)
			if text != "" {
				out = append(out, ai.StreamEvent{Type: "text", Text: text, SessionID: sessionID})
			}
		}
	}
	return out
}

func parseUserEvent(raw map[string]interface{}) []ai.StreamEvent {
	message, _ := raw["message"].(map[string]interface{})
	if message == nil {
		return nil
	}
	contents, _ := message["content"].([]interface{})
	for _, c := range contents {
		block, _ := c.(map[string]interface{})
		if block == nil {
			continue
		}
		blockType, _ := block["type"].(string)
		if blockType == "tool_result" {
			return []ai.StreamEvent{{Type: "tool_result"}}
		}
	}
	return nil
}

func parseResultEvent(raw map[string]interface{}) []ai.StreamEvent {
	sessionID, _ := raw["session_id"].(string)
	cost, _ := raw["total_cost_usd"].(float64)

	var inputTokens, outputTokens int64
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		if v, ok := usage["input_tokens"].(float64); ok {
			inputTokens = int64(v)
		}
		if v, ok := usage["output_tokens"].(float64); ok {
			outputTokens = int64(v)
		}
	}

	var out []ai.StreamEvent
	if inputTokens > 0 || outputTokens > 0 || cost > 0 {
		out = append(out, ai.StreamEvent{
			Type:         "usage",
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			CostUSD:      cost,
			SessionID:    sessionID,
		})
	}
	out = append(out, ai.StreamEvent{Type: "done", SessionID: sessionID})
	return out
}

func extractToolTarget(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "Read", "Write", "Edit", "MultiEdit":
		if v, _ := input["file_path"].(string); v != "" {
			return v
		}
	case "Glob", "Grep":
		if v, _ := input["pattern"].(string); v != "" {
			return v
		}
	case "Task":
		if v, _ := input["description"].(string); v != "" {
			return v
		}
		if v, _ := input["prompt"].(string); v != "" {
			return v
		}
	case "Bash":
		if v, _ := input["command"].(string); v != "" {
			return v
		}
	case "WebFetch":
		if v, _ := input["url"].(string); v != "" {
			return v
		}
	}
	return ""
}

func sanitizeText(text, repoRootSlash, homeSlash string, maxBytes int) string {
	if text == "" {
		return ""
	}
	if repoRootSlash != "" {
		text = strings.ReplaceAll(text, repoRootSlash+"/", "")
	}
	if homeSlash != "" {
		text = strings.ReplaceAll(text, homeSlash+"/", "<HOME>/")
		text = strings.ReplaceAll(text, homeSlash, "<HOME>")
	}
	return truncateUTF8(text, maxBytes)
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

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func hasPlanApprovedMarker(events []ai.StreamEvent) bool {
	for _, ev := range events {
		if ev.Type == "text" && strings.Contains(ev.Text, "PLAN_APPROVED_JSON:") {
			return true
		}
	}
	return false
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

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
