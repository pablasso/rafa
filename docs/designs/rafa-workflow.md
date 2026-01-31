# Technical Design: Rafa Workflow Orchestration

## Overview

This design adds end-to-end workflow orchestration to Rafa, enabling users to create PRDs and technical design documents through AI-guided conversations, with built-in reviews and session persistence. The goal is to provide a seamless flow from problem definition through implementation, while maintaining the existing plan execution infrastructure.

**PRD**: [docs/prds/rafa-workflow.md](../prds/rafa-workflow.md)

## Goals

- Enable conversational document creation (PRD, Design) via Claude Code CLI
- Provide activity visibility into what Claude is doing during conversations
- Support session persistence and resume via Claude's `--resume` flag
- Integrate automatic reviews at each phase
- Maintain backward compatibility with existing plan execution

## Non-Goals

- Direct API calls to Claude (CLI only)
- Parallel phase execution
- Auto-progression between phases
- External integrations (Jira, Linear, etc.)
- Collaborative/multi-user sessions

## Architecture

### High-Level Flow

```
┌──────────────────────────────────────────────────────────────────┐
│                         User Interface                            │
├──────────────┬───────────────────────────────────────────────────┤
│   CLI        │                    TUI                             │
│  rafa prd    │   HomeView → ConversationView → CompletionView    │
│  rafa design │                                                    │
│  rafa plan   │                                                    │
└──────┬───────┴───────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Conversation Engine                            │
│  - Claude CLI invocation with stream-json                         │
│  - Session management (--resume)                                  │
│  - Activity parsing (tool_use events)                             │
│  - State machine (drafting → reviewing → complete)                │
└──────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Skills Integration                             │
│  - Downloaded from github.com/pablasso/skills                     │
│  - Installed to .claude/skills/                                   │
│  - Invoked via "Use the /prd skill" prompts                       │
└──────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Session Storage                                │
│  - .rafa/sessions/*.json                                          │
│  - Claude session ID for resume                                   │
│  - Phase metadata                                                 │
└──────────────────────────────────────────────────────────────────┘
```

### Component Diagram

```
internal/
├── ai/
│   ├── claude.go           # Existing task extraction
│   └── conversation.go     # NEW: Streaming conversation handler
├── session/                # NEW: Session management package
│   ├── session.go          # Session struct and persistence
│   └── storage.go          # File-based session storage
├── skills/                 # NEW: Skills management package
│   └── installer.go        # Download and install skills from GitHub
├── cli/
│   ├── init.go             # MODIFIED: Add skills installation
│   ├── prd.go              # NEW: PRD command
│   ├── design.go           # NEW: Design command
│   └── sessions.go         # NEW: List sessions command
└── tui/
    ├── app.go              # MODIFIED: Add new views
    └── views/
        ├── home.go         # MODIFIED: New menu structure
        ├── conversation.go # NEW: Conversational mode view
        └── run.go          # MODIFIED: Add activity timeline
```

## Technical Details

### Claude CLI Integration for Conversations

The existing `internal/ai/claude.go` handles one-shot task extraction. For conversations, we need streaming with `--resume` support.

**New file: `internal/ai/conversation.go`**

```go
package ai

import (
    "bufio"
    "context"
    "encoding/json"
    "io"
    "os/exec"
)

// ConversationConfig holds settings for a conversation session.
type ConversationConfig struct {
    SessionID     string   // Claude session ID for --resume
    InitialPrompt string   // First message to send
    SkillName     string   // e.g., "prd", "technical-design"
}

// StreamEvent represents a parsed event from Claude's stream-json output.
type StreamEvent struct {
    Type         string  // "init", "text", "tool_use", "tool_result", "error", "done"
    Text         string  // For text events
    ToolName     string  // For tool_use events
    ToolTarget   string  // File or resource being accessed
    SessionID    string  // Available from init, assistant, and result events
    InputTokens  int64   // Token usage (from result event)
    OutputTokens int64
    CostUSD      float64 // Total cost (from result event)
}

// Conversation manages a multi-turn conversation with Claude CLI.
// Each message is a separate `claude -p` invocation with --resume.
type Conversation struct {
    sessionID string
    ctx       context.Context
    cancel    context.CancelFunc

    // Current invocation state
    cmd    *exec.Cmd
    stdout io.ReadCloser
}

// StartConversation begins a new conversation with the initial prompt.
// Returns after Claude finishes the first response.
func StartConversation(ctx context.Context, config ConversationConfig) (*Conversation, <-chan StreamEvent, error) {
    ctx, cancel := context.WithCancel(ctx)

    conv := &Conversation{
        sessionID: config.SessionID,
        ctx:       ctx,
        cancel:    cancel,
    }

    events, err := conv.invoke(config.InitialPrompt)
    if err != nil {
        cancel()
        return nil, nil, err
    }

    return conv, events, nil
}

// SendMessage sends a follow-up message in the conversation.
// Uses --resume with the session ID from previous responses.
// Returns a channel of events for this response.
func (c *Conversation) SendMessage(message string) (<-chan StreamEvent, error) {
    if c.sessionID == "" {
        return nil, fmt.Errorf("no session ID available for resume")
    }
    return c.invoke(message)
}

// invoke runs a single claude -p invocation and returns the event stream.
func (c *Conversation) invoke(prompt string) (<-chan StreamEvent, error) {
    args := []string{
        "-p", prompt,
        "--output-format", "stream-json",
        "--verbose",
        "--include-partial-messages",
        "--dangerously-skip-permissions",
    }

    if c.sessionID != "" {
        args = append(args, "--resume", c.sessionID)
    }

    c.cmd = CommandContext(c.ctx, "claude", args...)

    stdout, err := c.cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }
    c.stdout = stdout

    // Capture stderr for error messages
    c.cmd.Stderr = os.Stderr

    if err := c.cmd.Start(); err != nil {
        return nil, err
    }

    events := make(chan StreamEvent, 100)

    go func() {
        defer close(events)
        scanner := bufio.NewScanner(stdout)

        for scanner.Scan() {
            event := parseStreamEvent(scanner.Text())
            if event.Type != "" {
                // Capture session ID for future --resume calls
                if event.SessionID != "" {
                    c.sessionID = event.SessionID
                }
                events <- event
            }
        }

        // Wait for command to finish
        c.cmd.Wait()
    }()

    return events, nil
}

// SessionID returns the current session ID for persistence.
func (c *Conversation) SessionID() string {
    return c.sessionID
}

// Stop terminates the current invocation.
func (c *Conversation) Stop() error {
    c.cancel()
    if c.cmd != nil && c.cmd.Process != nil {
        return c.cmd.Process.Kill()
    }
    return nil
}

// parseStreamEvent converts a JSON line to a StreamEvent.
// Stream-json format (verified against Claude CLI v2.1.27):
// - {"type":"system","subtype":"init","session_id":"uuid",...} - session start
// - {"type":"assistant","message":{...},"session_id":"uuid"} - responses
// - {"type":"result","session_id":"uuid","total_cost_usd":0.01,...} - completion
func parseStreamEvent(line string) StreamEvent {
    var raw map[string]interface{}
    if err := json.Unmarshal([]byte(line), &raw); err != nil {
        return StreamEvent{}
    }

    eventType, _ := raw["type"].(string)

    switch eventType {
    case "system":
        // Init event with session ID
        sessionID, _ := raw["session_id"].(string)
        return StreamEvent{Type: "init", SessionID: sessionID}

    case "stream_event":
        return parseStreamEventNested(raw)

    case "assistant":
        return parseAssistantMessage(raw)

    case "result":
        // Final event with session ID, usage, and cost
        sessionID, _ := raw["session_id"].(string)
        costUSD, _ := raw["total_cost_usd"].(float64)
        var inputTokens, outputTokens int64
        if usage, ok := raw["usage"].(map[string]interface{}); ok {
            if v, ok := usage["input_tokens"].(float64); ok {
                inputTokens = int64(v)
            }
            if v, ok := usage["output_tokens"].(float64); ok {
                outputTokens = int64(v)
            }
        }
        return StreamEvent{
            Type:         "done",
            SessionID:    sessionID,
            CostUSD:      costUSD,
            InputTokens:  inputTokens,
            OutputTokens: outputTokens,
        }
    }

    return StreamEvent{}
}

// parseStreamEventNested handles nested stream_event structures.
func parseStreamEventNested(raw map[string]interface{}) StreamEvent {
    event, ok := raw["event"].(map[string]interface{})
    if !ok {
        return StreamEvent{}
    }

    eventType, _ := event["type"].(string)

    switch eventType {
    case "content_block_delta":
        delta, _ := event["delta"].(map[string]interface{})
        if deltaType, _ := delta["type"].(string); deltaType == "text_delta" {
            text, _ := delta["text"].(string)
            return StreamEvent{Type: "text", Text: text}
        }
    case "content_block_start":
        // Check for tool_use
        block, _ := event["content_block"].(map[string]interface{})
        if blockType, _ := block["type"].(string); blockType == "tool_use" {
            name, _ := block["name"].(string)
            return StreamEvent{Type: "tool_use", ToolName: name}
        }
    }

    return StreamEvent{}
}

// parseAssistantMessage extracts tool uses from complete messages.
func parseAssistantMessage(raw map[string]interface{}) StreamEvent {
    message, ok := raw["message"].(map[string]interface{})
    if !ok {
        return StreamEvent{}
    }

    content, ok := message["content"].([]interface{})
    if !ok {
        return StreamEvent{}
    }

    for _, c := range content {
        block, _ := c.(map[string]interface{})
        if blockType, _ := block["type"].(string); blockType == "tool_use" {
            name, _ := block["name"].(string)
            // Extract target from input if available
            input, _ := block["input"].(map[string]interface{})
            target := extractToolTarget(name, input)
            return StreamEvent{Type: "tool_use", ToolName: name, ToolTarget: target}
        }
    }

    return StreamEvent{}
}

// extractToolTarget gets the relevant target from tool input.
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
    }
    return ""
}

```

**Key design decisions:**
1. **Streaming stdin/stdout**: Unlike the existing one-shot approach, conversations need bidirectional communication
2. **Event parsing**: Transform raw stream-json into structured events for the TUI
3. **Session ID extraction**: Claude CLI outputs session info that we capture for `--resume`

### Session Management

**New file: `internal/session/session.go`**

```go
package session

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

// Phase represents the document creation phase.
type Phase string

const (
    PhasePRD        Phase = "prd"
    PhaseDesign     Phase = "design"
    PhasePlanCreate Phase = "plan-create"
)

// Status represents the session status.
type Status string

const (
    StatusInProgress Status = "in_progress"
    StatusCompleted  Status = "completed"
    StatusCancelled  Status = "cancelled"
)

// Session tracks a document creation conversation.
type Session struct {
    SessionID    string    `json:"sessionId"`    // Claude's session ID for --resume
    Phase        Phase     `json:"phase"`
    Name         string    `json:"name"`         // User-friendly name (e.g., "user-auth")
    DocumentPath string    `json:"documentPath"` // Path to output document
    Status       Status    `json:"status"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
    FromDocument string    `json:"fromDocument,omitempty"` // For designs created from PRD
}

// Storage manages session persistence.
type Storage struct {
    dir string
}

// NewStorage creates a storage instance for the given sessions directory.
func NewStorage(sessionsDir string) *Storage {
    return &Storage{dir: sessionsDir}
}

// Save persists a session to disk.
func (s *Storage) Save(session *Session) error {
    session.UpdatedAt = time.Now()

    filename := s.sessionFilename(session.Phase, session.Name)
    data, err := json.MarshalIndent(session, "", "  ")
    if err != nil {
        return err
    }

    // Atomic write
    tmpFile := filename + ".tmp"
    if err := os.WriteFile(tmpFile, data, 0644); err != nil {
        return err
    }
    return os.Rename(tmpFile, filename)
}

// Load retrieves a session by phase and name.
func (s *Storage) Load(phase Phase, name string) (*Session, error) {
    filename := s.sessionFilename(phase, name)
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    var session Session
    if err := json.Unmarshal(data, &session); err != nil {
        return nil, err
    }
    return &session, nil
}

// LoadByPhase retrieves the most recent session for a phase.
func (s *Storage) LoadByPhase(phase Phase) (*Session, error) {
    pattern := filepath.Join(s.dir, string(phase)+"-*.json")
    matches, err := filepath.Glob(pattern)
    if err != nil {
        return nil, err
    }

    if len(matches) == 0 {
        return nil, os.ErrNotExist
    }

    // Find most recently updated
    var latest *Session
    var latestTime time.Time

    for _, match := range matches {
        data, err := os.ReadFile(match)
        if err != nil {
            continue
        }

        var sess Session
        if err := json.Unmarshal(data, &sess); err != nil {
            continue
        }

        if sess.UpdatedAt.After(latestTime) {
            latestTime = sess.UpdatedAt
            latest = &sess
        }
    }

    if latest == nil {
        return nil, os.ErrNotExist
    }
    return latest, nil
}

// List returns all sessions.
func (s *Storage) List() ([]*Session, error) {
    pattern := filepath.Join(s.dir, "*.json")
    matches, err := filepath.Glob(pattern)
    if err != nil {
        return nil, err
    }

    var sessions []*Session
    for _, match := range matches {
        data, err := os.ReadFile(match)
        if err != nil {
            continue
        }

        var sess Session
        if err := json.Unmarshal(data, &sess); err != nil {
            continue
        }
        sessions = append(sessions, &sess)
    }

    return sessions, nil
}

// Delete removes a session file.
func (s *Storage) Delete(phase Phase, name string) error {
    filename := s.sessionFilename(phase, name)
    return os.Remove(filename)
}

func (s *Storage) sessionFilename(phase Phase, name string) string {
    return filepath.Join(s.dir, string(phase)+"-"+name+".json")
}
```

### Skills Installation

**Verified repo structure** (from github.com/pablasso/skills):
```
pablasso/skills/
├── code-review/
│   └── SKILL.md          # Required file per Claude Code docs
├── prd/
│   └── SKILL.md
├── prd-review/
│   └── SKILL.md
├── technical-design/
│   └── SKILL.md
├── technical-design-review/
│   └── SKILL.md
├── .claude/              # Settings (not copied)
├── CLAUDE.md             # Repo docs (not copied)
└── README.md             # Repo docs (not copied)
```

Claude Code discovers skills by looking for `<skill-name>/SKILL.md` in `.claude/skills/`. The installer must:
1. Download the tarball from GitHub
2. Extract only the skill directories (containing SKILL.md)
3. Install to `.claude/skills/` in the target project
4. Verify each skill has a SKILL.md file

**New file: `internal/skills/installer.go`**

```go
package skills

import (
    "archive/tar"
    "compress/gzip"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
)

const (
    skillsRepoOwner = "pablasso"
    skillsRepoName  = "skills"
    skillsURL       = "https://api.github.com/repos/pablasso/skills/tarball/main"
)

// RequiredSkills lists the skills that must be installed.
// Each skill directory must contain a SKILL.md file.
var RequiredSkills = []string{
    "prd",
    "prd-review",
    "technical-design",
    "technical-design-review",
    "code-review",
}

// Installer handles downloading and installing skills.
type Installer struct {
    targetDir string // .claude/skills/
}

// NewInstaller creates an installer targeting the given directory.
func NewInstaller(targetDir string) *Installer {
    return &Installer{targetDir: targetDir}
}

// Install downloads and extracts skills from GitHub.
func (i *Installer) Install() error {
    // Create target directory
    if err := os.MkdirAll(i.targetDir, 0755); err != nil {
        return fmt.Errorf("failed to create skills directory: %w", err)
    }

    // Download tarball
    resp, err := http.Get(skillsURL)
    if err != nil {
        return fmt.Errorf("failed to download skills: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("failed to download skills: HTTP %d", resp.StatusCode)
    }

    // Extract tarball
    if err := i.extractTarball(resp.Body); err != nil {
        // Clean up partial installation
        i.Uninstall()
        return fmt.Errorf("failed to extract skills: %w", err)
    }

    // Verify required skills are present with SKILL.md files
    return i.verify()
}

// extractTarball extracts skills from the GitHub tarball.
// Only extracts directories that match RequiredSkills.
func (i *Installer) extractTarball(r io.Reader) error {
    gzr, err := gzip.NewReader(r)
    if err != nil {
        return err
    }
    defer gzr.Close()

    tr := tar.NewReader(gzr)

    // GitHub tarballs have a root directory like "pablasso-skills-abc123/"
    // We need to strip this prefix
    var rootPrefix string

    for {
        header, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        // Detect root prefix from first entry
        if rootPrefix == "" {
            parts := strings.SplitN(header.Name, "/", 2)
            if len(parts) > 0 {
                rootPrefix = parts[0] + "/"
            }
        }

        // Strip root prefix
        relPath := strings.TrimPrefix(header.Name, rootPrefix)
        if relPath == "" {
            continue
        }

        // Parse path: first component is potential skill name
        parts := strings.SplitN(relPath, "/", 2)
        skillName := parts[0]

        // Only extract files inside required skill directories
        if !i.isRequiredSkill(skillName) {
            continue
        }

        targetPath := filepath.Join(i.targetDir, relPath)

        switch header.Typeflag {
        case tar.TypeDir:
            if err := os.MkdirAll(targetPath, 0755); err != nil {
                return err
            }
        case tar.TypeReg:
            if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
                return err
            }
            f, err := os.Create(targetPath)
            if err != nil {
                return err
            }
            if _, err := io.Copy(f, tr); err != nil {
                f.Close()
                return err
            }
            f.Close()
        }
    }

    return nil
}

// isRequiredSkill checks if a name matches a required skill.
func (i *Installer) isRequiredSkill(name string) bool {
    for _, skill := range RequiredSkills {
        if name == skill {
            return true
        }
    }
    return false
}

// verify checks that all required skills are installed with SKILL.md files.
func (i *Installer) verify() error {
    for _, skill := range RequiredSkills {
        skillFile := filepath.Join(i.targetDir, skill, "SKILL.md")
        if _, err := os.Stat(skillFile); os.IsNotExist(err) {
            return fmt.Errorf("required skill %q missing SKILL.md file", skill)
        }
    }
    return nil
}

// IsInstalled checks if skills are already installed with SKILL.md files.
func (i *Installer) IsInstalled() bool {
    for _, skill := range RequiredSkills {
        skillFile := filepath.Join(i.targetDir, skill, "SKILL.md")
        if _, err := os.Stat(skillFile); os.IsNotExist(err) {
            return false
        }
    }
    return true
}

// Uninstall removes installed skills.
func (i *Installer) Uninstall() error {
    for _, skill := range RequiredSkills {
        skillDir := filepath.Join(i.targetDir, skill)
        if err := os.RemoveAll(skillDir); err != nil && !os.IsNotExist(err) {
            return err
        }
    }
    return nil
}
```

### Modified Init Command

**File: `internal/cli/init.go` (modifications)**

```go
// Add to runInit function:

func runInit(cmd *cobra.Command, args []string) error {
    // ... existing prerequisite checks ...

    // Check if already initialized
    if IsInitialized() {
        return fmt.Errorf("rafa is already initialized in this repository")
    }

    // Create .rafa directory structure
    dirs := []string{
        rafaDir,
        filepath.Join(rafaDir, "plans"),
        filepath.Join(rafaDir, "sessions"), // NEW
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %w", dir, err)
        }
    }

    // NEW: Install skills
    fmt.Println("Installing skills from github.com/pablasso/skills...")
    skillsDir := filepath.Join(repoRoot, ".claude", "skills")
    installer := skills.NewInstaller(skillsDir)
    if err := installer.Install(); err != nil {
        // Clean up on failure - don't leave partial state
        // Skills cleanup is handled inside installer.Install() on extraction failure
        // Also clean up .rafa directory
        os.RemoveAll(rafaDir)
        // Clean up any partially installed skills
        installer.Uninstall()
        return fmt.Errorf("failed to install skills: %w", err)
    }
    fmt.Println("Skills installed successfully.")

    // Add gitignore entries
    gitignoreEntries := []string{
        ".rafa/**/*.lock",
        ".rafa/sessions/", // NEW: Session files are gitignored
    }
    for _, entry := range gitignoreEntries {
        if err := addToGitignore(entry); err != nil {
            // Non-fatal: log warning but continue
            fmt.Printf("Warning: failed to update .gitignore: %v\n", err)
        }
    }

    fmt.Println("Initialized Rafa in", rafaDir)
    fmt.Println("\nNext steps:")
    fmt.Println("  1. Run: rafa prd             # Create a PRD")
    fmt.Println("  2. Run: rafa design          # Create a technical design")
    fmt.Println("  3. Run: rafa plan create     # Create an execution plan")
    return nil
}
```

### TUI Conversational View

**New file: `internal/tui/views/conversation.go`**

```go
package views

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/textarea"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/pablasso/rafa/internal/ai"
    "github.com/pablasso/rafa/internal/session"
    "github.com/pablasso/rafa/internal/tui/components"
    "github.com/pablasso/rafa/internal/tui/styles"
)

// ConversationState represents the current state of the conversation.
type ConversationState int

const (
    StateConversing ConversationState = iota
    StateReviewing
    StateWaitingApproval
    StateCompleted
    StateCancelled
)

// ActivityEntry represents a single item in the activity timeline.
type ActivityEntry struct {
    Text      string
    Timestamp time.Time
    Indent    int  // Nesting level for tree display
    IsDone    bool // Whether this activity is complete
}

// ConversationModel handles the conversational document creation UI.
type ConversationModel struct {
    phase        session.Phase
    session      *session.Session
    conversation *ai.Conversation

    state       ConversationState

    // Activity timeline (left pane)
    activities   []ActivityEntry
    activityView components.OutputViewport

    // Response content (main pane)
    responseText strings.Builder
    responseView components.OutputViewport

    // Track Write tool targets for auto-review detection
    lastWritePath string

    // Input field
    input      textarea.Model
    inputFocus bool

    // UI state
    spinner     spinner.Model
    isThinking  bool

    // Event channels
    eventChan chan ai.StreamEvent

    // Context for cancellation
    ctx    context.Context
    cancel context.CancelFunc

    width  int
    height int
}

// ConversationConfig holds initialization parameters.
type ConversationConfig struct {
    Phase       session.Phase
    Name        string
    FromDoc     string // For design docs created from PRD
    ResumeFrom  *session.Session
}

// NewConversationModel creates a new conversation view.
func NewConversationModel(config ConversationConfig) ConversationModel {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = styles.SelectedStyle

    ta := textarea.New()
    ta.Placeholder = "Type your message..."
    ta.SetHeight(3)
    ta.ShowLineNumbers = false
    ta.Focus()

    ctx, cancel := context.WithCancel(context.Background())

    m := ConversationModel{
        phase:      config.Phase,
        state:      StateConversing,
        spinner:    s,
        input:      ta,
        inputFocus: true,
        eventChan:  make(chan ai.StreamEvent, 100),
        ctx:        ctx,
        cancel:     cancel,
    }

    // Initialize session
    if config.ResumeFrom != nil {
        m.session = config.ResumeFrom
        m.addActivity("Resuming session...", 0)
    } else {
        m.session = &session.Session{
            Phase:        config.Phase,
            Name:         config.Name,
            Status:       session.StatusInProgress,
            CreatedAt:    time.Now(),
            FromDocument: config.FromDoc,
        }
        m.addActivity("Starting session", 0)
    }

    return m
}

// Init implements tea.Model.
func (m ConversationModel) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        textarea.Blink,
        m.startConversation(),
        m.listenForEvents(),
    )
}

// startConversation initiates the Claude conversation.
func (m *ConversationModel) startConversation() tea.Cmd {
    return func() tea.Msg {
        prompt := m.buildInitialPrompt()

        config := ai.ConversationConfig{
            SessionID:     m.session.SessionID,
            InitialPrompt: prompt,
            SkillName:     string(m.phase),
        }

        conv, events, err := ai.StartConversation(m.ctx, config)
        if err != nil {
            return ConversationErrorMsg{Err: err}
        }

        m.conversation = conv

        // Forward events to our channel, handling backpressure
        go m.forwardEvents(events)

        return nil
    }
}

// forwardEvents reads from a response channel and forwards to the main event channel.
// Critical events (init, done) are never dropped; other events may be dropped if buffer is full.
func (m *ConversationModel) forwardEvents(events <-chan ai.StreamEvent) {
    for event := range events {
        // Critical events must not be dropped
        if event.Type == "init" || event.Type == "done" {
            m.eventChan <- event // Blocking send for critical events
            continue
        }

        // Non-critical events: drop if buffer full
        select {
        case m.eventChan <- event:
        default:
            // Log dropped events for debugging
        }
    }
}

// buildInitialPrompt creates the prompt based on phase.
func (m *ConversationModel) buildInitialPrompt() string {
    switch m.phase {
    case session.PhasePRD:
        return "Use the /prd skill to help the user create a PRD. Guide them through defining the problem, users, and requirements."
    case session.PhaseDesign:
        if m.session.FromDocument != "" {
            return fmt.Sprintf("Use the /technical-design skill to create a technical design document based on the PRD at %s.", m.session.FromDocument)
        }
        return "Use the /technical-design skill to help the user create a technical design document."
    case session.PhasePlanCreate:
        return "Help the user create an execution plan from their design document."
    default:
        return ""
    }
}

// listenForEvents returns a command that waits for stream events.
func (m ConversationModel) listenForEvents() tea.Cmd {
    return func() tea.Msg {
        event, ok := <-m.eventChan
        if !ok {
            return nil
        }
        return StreamEventMsg{Event: event}
    }
}

// StreamEventMsg wraps a stream event for the Update loop.
type StreamEventMsg struct {
    Event ai.StreamEvent
}

// ConversationErrorMsg indicates an error occurred.
type ConversationErrorMsg struct {
    Err error
}

// Update implements tea.Model.
func (m ConversationModel) Update(msg tea.Msg) (ConversationModel, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.updateLayout()
        return m, nil

    case spinner.TickMsg:
        if m.isThinking {
            var cmd tea.Cmd
            m.spinner, cmd = m.spinner.Update(msg)
            cmds = append(cmds, cmd)
        }

    case StreamEventMsg:
        if cmd := m.handleStreamEvent(msg.Event); cmd != nil {
            cmds = append(cmds, cmd)
        }
        cmds = append(cmds, m.listenForEvents())

    case ConversationErrorMsg:
        m.addActivity(fmt.Sprintf("Error: %v", msg.Err), 0)
        m.state = StateCancelled

    case tea.KeyMsg:
        return m.handleKeyPress(msg)
    }

    // Update textarea
    if m.inputFocus && !m.isThinking {
        var cmd tea.Cmd
        m.input, cmd = m.input.Update(msg)
        cmds = append(cmds, cmd)
    }

    return m, tea.Batch(cmds...)
}

// handleStreamEvent processes events from Claude.
// Returns a tea.Cmd if an async action is needed.
func (m *ConversationModel) handleStreamEvent(event ai.StreamEvent) tea.Cmd {
    switch event.Type {
    case "init":
        // Capture session ID immediately for persistence
        if event.SessionID != "" {
            m.session.SessionID = event.SessionID
        }

    case "text":
        m.responseText.WriteString(event.Text)
        m.responseView.AddLine(event.Text)

    case "tool_use":
        entry := fmt.Sprintf("Using %s", event.ToolName)
        if event.ToolTarget != "" {
            entry += fmt.Sprintf(": %s", shortenPath(event.ToolTarget))
        }
        m.addActivity(entry, 1)
        m.isThinking = true

        // Track Write targets for auto-review detection
        if event.ToolName == "Write" {
            m.lastWritePath = event.ToolTarget
        }

    case "tool_result":
        // Mark last activity as done
        if len(m.activities) > 0 {
            m.activities[len(m.activities)-1].IsDone = true
        }

    case "done":
        m.isThinking = false
        if event.SessionID != "" {
            m.session.SessionID = event.SessionID
        }

        // Check if this was a review phase
        if m.state == StateReviewing {
            m.state = StateWaitingApproval
            m.addActivity("Review complete", 0)
        } else if m.shouldAutoReview() {
            return m.triggerAutoReview()
        }
    }
    return nil
}

// handleKeyPress processes keyboard input.
func (m ConversationModel) handleKeyPress(msg tea.KeyMsg) (ConversationModel, tea.Cmd) {
    switch msg.String() {
    case "ctrl+c":
        m.cancel()
        m.state = StateCancelled
        return m, nil

    case "a":
        if m.state == StateWaitingApproval {
            return m.handleApprove()
        }

    case "c":
        if m.state == StateWaitingApproval {
            m.cancel()
            m.state = StateCancelled
            return m, nil
        }

    case "ctrl+enter", "cmd+enter":
        if !m.isThinking && m.input.Value() != "" {
            return m.sendMessage()
        }
    }

    return m, nil
}

// sendMessage sends user input to Claude.
func (m ConversationModel) sendMessage() (ConversationModel, tea.Cmd) {
    message := m.input.Value()
    m.input.Reset()

    m.addActivity(fmt.Sprintf("You: %s", truncate(message, 40)), 0)
    m.isThinking = true

    return m, func() tea.Msg {
        events, err := m.conversation.SendMessage(message)
        if err != nil {
            return ConversationErrorMsg{Err: err}
        }
        // Forward events from this response
        go m.forwardEvents(events)
        return nil
    }
}

// shouldAutoReview determines if auto-review should trigger.
// Uses Write tool detection instead of fragile text matching.
func (m *ConversationModel) shouldAutoReview() bool {
    // Check if we detected a Write to docs/prds/ or docs/designs/
    return m.lastWritePath != "" && (
        strings.HasPrefix(m.lastWritePath, "docs/prds/") ||
        strings.HasPrefix(m.lastWritePath, "docs/designs/"))
}

// triggerAutoReview starts the automatic review phase.
// Returns a tea.Cmd to properly handle the async operation.
func (m *ConversationModel) triggerAutoReview() tea.Cmd {
    m.state = StateReviewing
    m.addActivity("Starting auto-review...", 0)
    m.isThinking = true

    var reviewPrompt string
    switch m.phase {
    case session.PhasePRD:
        reviewPrompt = "Now use /prd-review to review what you created. Address any critical issues you find."
    case session.PhaseDesign:
        reviewPrompt = "Now use /technical-design-review to review what you created. Address any critical issues you find."
    }

    return func() tea.Msg {
        events, err := m.conversation.SendMessage(reviewPrompt)
        if err != nil {
            return ConversationErrorMsg{Err: err}
        }
        go m.forwardEvents(events)
        return nil
    }
}

// handleApprove processes the approval action.
func (m ConversationModel) handleApprove() (ConversationModel, tea.Cmd) {
    m.state = StateCompleted
    m.session.Status = session.StatusCompleted

    // Session persistence will be handled by the parent view
    return m, nil
}

// addActivity adds an entry to the activity timeline.
func (m *ConversationModel) addActivity(text string, indent int) {
    m.activities = append(m.activities, ActivityEntry{
        Text:      text,
        Timestamp: time.Now(),
        Indent:    indent,
    })
}

// updateLayout recalculates component sizes.
func (m *ConversationModel) updateLayout() {
    if m.width == 0 || m.height == 0 {
        return
    }

    leftWidth := (m.width * 25 / 100) - 2
    rightWidth := (m.width * 75 / 100) - 2

    // Height: total - title(2) - input(5) - action bar(1) - borders
    panelHeight := m.height - 10

    m.activityView.SetSize(leftWidth, panelHeight)
    m.responseView.SetSize(rightWidth, panelHeight)
    m.input.SetWidth(m.width - 4)
}

// View implements tea.Model.
func (m ConversationModel) View() string {
    if m.width == 0 || m.height == 0 {
        return ""
    }

    var b strings.Builder

    // Title
    title := m.renderTitle()
    b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title))
    b.WriteString("\n")

    // Main panels
    leftPanel := m.renderActivityPanel()
    rightPanel := m.renderResponsePanel()

    leftStyle := styles.BoxStyle.Copy().Width((m.width * 25 / 100) - 2)
    rightStyle := styles.BoxStyle.Copy().Width((m.width * 75 / 100) - 2)

    panels := lipgloss.JoinHorizontal(lipgloss.Top,
        leftStyle.Render(leftPanel),
        rightStyle.Render(rightPanel),
    )
    b.WriteString(panels)
    b.WriteString("\n")

    // Input field
    b.WriteString(m.renderInput())
    b.WriteString("\n")

    // Action bar
    b.WriteString(m.renderActionBar())

    return b.String()
}

// renderTitle returns the title bar.
func (m ConversationModel) renderTitle() string {
    var phase string
    switch m.phase {
    case session.PhasePRD:
        phase = "Creating PRD"
    case session.PhaseDesign:
        phase = "Creating Design"
    case session.PhasePlanCreate:
        phase = "Creating Plan"
    }

    if m.session.Name != "" {
        phase += ": " + m.session.Name
    }

    return styles.TitleStyle.Render("Rafa - " + phase)
}

// renderActivityPanel returns the activity timeline view.
func (m ConversationModel) renderActivityPanel() string {
    var lines []string
    lines = append(lines, styles.SubtleStyle.Render("Activity"))
    lines = append(lines, "")

    for _, entry := range m.activities {
        prefix := strings.Repeat("  ", entry.Indent)
        if entry.Indent > 0 {
            prefix = "├─ "
        }

        indicator := "○"
        if entry.IsDone {
            indicator = styles.SuccessStyle.Render("✓")
        } else if entry == m.activities[len(m.activities)-1] && m.isThinking {
            indicator = m.spinner.View()
        }

        line := fmt.Sprintf("%s%s %s", prefix, indicator, entry.Text)
        lines = append(lines, line)
    }

    return strings.Join(lines, "\n")
}

// renderResponsePanel returns the Claude response view.
func (m ConversationModel) renderResponsePanel() string {
    return m.responseView.View()
}

// renderInput returns the input field.
func (m ConversationModel) renderInput() string {
    if m.isThinking {
        return styles.SubtleStyle.Render("Waiting for Claude...")
    }

    if m.state == StateWaitingApproval {
        return styles.SubtleStyle.Render("Review complete. Approve or revise.")
    }

    return m.input.View()
}

// renderActionBar returns the bottom action bar.
func (m ConversationModel) renderActionBar() string {
    var items []string

    switch m.state {
    case StateWaitingApproval:
        items = []string{"[a] Approve", "[c] Cancel", "(type to revise)"}
    case StateCompleted:
        items = []string{"Complete!", "[Enter] Continue"}
    case StateCancelled:
        items = []string{"Cancelled", "[Enter] Return"}
    default:
        items = []string{"Ctrl+Enter Submit", "Ctrl+C Cancel"}
    }

    return components.NewStatusBar().Render(m.width, items)
}

// SetSize updates dimensions.
func (m *ConversationModel) SetSize(width, height int) {
    m.width = width
    m.height = height
    m.updateLayout()
}

// Helper functions

func shortenPath(path string) string {
    if len(path) <= 30 {
        return path
    }
    parts := strings.Split(path, "/")
    if len(parts) > 2 {
        return ".../" + strings.Join(parts[len(parts)-2:], "/")
    }
    return path
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max-3] + "..."
}
```

### Updated Home View

The home view needs to be updated to match the new menu structure from the PRD.

**File: `internal/tui/views/home.go` (modifications)**

```go
// Update menu items:
menuItems: []MenuItem{
    {Label: "Create PRD", Shortcut: "p", Description: "Define the problem and requirements"},
    {Label: "Create Design Doc", Shortcut: "d", Description: "Plan the technical approach"},
    {Label: "Create Plan", Shortcut: "c", Description: "Break design into executable tasks"},
    {Label: "Run Plan", Shortcut: "r", Description: "Execute tasks with AI agents"},
    {Label: "Quit", Shortcut: "q", Description: ""},
},

// Update key handlers:
case "p":
    return m, func() tea.Msg { return msgs.GoToConversationMsg{Phase: session.PhasePRD} }
case "d":
    return m, func() tea.Msg { return msgs.GoToConversationMsg{Phase: session.PhaseDesign} }
case "c":
    return m, func() tea.Msg { return msgs.GoToFilePickerMsg{} } // Design doc picker
case "r":
    return m, func() tea.Msg { return msgs.GoToPlanListMsg{} }
```

### Updated Running View with Activity Timeline

The running view needs to be updated to include the activity timeline and token/cost tracking.

**File: `internal/tui/views/run.go` (modifications)**

The existing `RunningModel` already has a left panel for progress. We need to:

1. Add an activity timeline similar to the conversation view
2. Add token/cost tracking display

```go
// Add to RunningModel struct:
type RunningModel struct {
    // ... existing fields ...

    // Activity timeline
    activities   []ActivityEntry

    // Token/cost tracking
    taskTokens  int64
    totalTokens int64
    estimatedCost float64
}

// Add activity tracking to handleStreamEvent:
func (m *RunningModel) handleStreamEvent(event ai.StreamEvent) {
    switch event.Type {
    case "tool_use":
        m.activities = append(m.activities, ActivityEntry{
            Text:      fmt.Sprintf("%s: %s", event.ToolName, shortenPath(event.ToolTarget)),
            Timestamp: time.Now(),
        })
    case "tool_result":
        if len(m.activities) > 0 {
            m.activities[len(m.activities)-1].IsDone = true
        }
    case "usage":
        m.taskTokens = event.InputTokens + event.OutputTokens
        m.totalTokens += m.taskTokens
        m.estimatedCost = estimateCost(m.totalTokens)
    }
}

// Update renderLeftPanel to include activity and usage:
func (m RunningModel) renderLeftPanel(width, height int) string {
    var lines []string

    // Header with task info
    lines = append(lines, fmt.Sprintf("Task %d/%d", m.currentTask, m.totalTasks))
    lines = append(lines, fmt.Sprintf("Attempt %d/%d", m.attempt, m.maxAttempts))
    lines = append(lines, fmt.Sprintf("%s", m.formatDuration(time.Since(m.startTime))))
    lines = append(lines, "")

    // Activity timeline
    lines = append(lines, styles.SubtleStyle.Render("Activity"))
    lines = append(lines, "─────")
    for _, entry := range m.activities {
        indicator := "├─"
        if entry.IsDone {
            indicator = styles.SuccessStyle.Render("✓")
        } else if entry == m.activities[len(m.activities)-1] {
            indicator = m.spinner.View()
        }
        line := fmt.Sprintf("%s %s", indicator, truncate(entry.Text, width-4))
        lines = append(lines, line)
    }
    lines = append(lines, "")

    // Usage stats
    lines = append(lines, styles.SubtleStyle.Render("Usage"))
    lines = append(lines, "─────")
    lines = append(lines, fmt.Sprintf("Task: %s", formatTokens(m.taskTokens)))
    lines = append(lines, fmt.Sprintf("Plan: %s", formatTokens(m.totalTokens)))
    lines = append(lines, fmt.Sprintf("Cost: $%.2f", m.estimatedCost))
    lines = append(lines, "")

    // Compact task list
    lines = append(lines, styles.SubtleStyle.Render("Tasks"))
    lines = append(lines, "─────")
    lines = append(lines, m.renderCompactTaskList())

    return strings.Join(lines, "\n")
}

// renderCompactTaskList shows tasks in a compact format.
func (m RunningModel) renderCompactTaskList() string {
    var parts []string
    for _, task := range m.tasks {
        switch task.Status {
        case "completed":
            parts = append(parts, styles.SuccessStyle.Render("✓"))
        case "running":
            parts = append(parts, "▶")
        case "failed":
            parts = append(parts, styles.ErrorStyle.Render("✗"))
        default:
            parts = append(parts, "○")
        }
    }
    return strings.Join(parts, " ")
}

func formatTokens(tokens int64) string {
    if tokens >= 1000 {
        return fmt.Sprintf("%.1fk", float64(tokens)/1000)
    }
    return fmt.Sprintf("%d", tokens)
}

func estimateCost(tokens int64) float64 {
    // Rough estimate based on Claude pricing
    // Adjust based on actual model used
    return float64(tokens) * 0.00001
}
```

### CLI Commands

**New file: `internal/cli/prd.go`**

```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
    "github.com/pablasso/rafa/internal/session"
    "github.com/pablasso/rafa/internal/tui"
)

var prdCmd = &cobra.Command{
    Use:   "prd",
    Short: "Create or resume a PRD",
    Long:  "Start a conversational PRD creation session with Claude.",
    RunE:  runPRD,
}

var prdResumeCmd = &cobra.Command{
    Use:   "resume [name]",
    Short: "Resume a PRD session",
    RunE:  runPRDResume,
}

var prdName string

func init() {
    prdCmd.Flags().StringVar(&prdName, "name", "", "Name for the PRD")
    prdCmd.AddCommand(prdResumeCmd)
}

func runPRD(cmd *cobra.Command, args []string) error {
    if !IsInitialized() {
        return fmt.Errorf("rafa is not initialized. Run 'rafa init' first")
    }

    if !skillsInstalled() {
        return fmt.Errorf("skills not found. Run 'rafa init' to install")
    }

    // Launch TUI with PRD phase
    return tui.RunWithConversation(tui.ConversationOpts{
        Phase: session.PhasePRD,
        Name:  prdName,
    })
}

func runPRDResume(cmd *cobra.Command, args []string) error {
    if !IsInitialized() {
        return fmt.Errorf("rafa is not initialized. Run 'rafa init' first")
    }

    sessionsDir := filepath.Join(rafaDir, "sessions")
    storage := session.NewStorage(sessionsDir)

    var sess *session.Session
    var err error

    if len(args) > 0 {
        sess, err = storage.Load(session.PhasePRD, args[0])
    } else {
        sess, err = storage.LoadByPhase(session.PhasePRD)
    }

    if err != nil {
        if os.IsNotExist(err) {
            return fmt.Errorf("no PRD session found to resume")
        }
        return err
    }

    return tui.RunWithConversation(tui.ConversationOpts{
        Phase:      session.PhasePRD,
        ResumeFrom: sess,
    })
}

func skillsInstalled() bool {
    repoRoot := findRepoRoot()
    skillsDir := filepath.Join(repoRoot, ".claude", "skills")

    for _, skill := range []string{"prd", "technical-design"} {
        if _, err := os.Stat(filepath.Join(skillsDir, skill)); os.IsNotExist(err) {
            return false
        }
    }
    return true
}
```

**New file: `internal/cli/design.go`**

```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
    "github.com/pablasso/rafa/internal/session"
    "github.com/pablasso/rafa/internal/tui"
)

var designCmd = &cobra.Command{
    Use:   "design",
    Short: "Create or resume a technical design",
    Long:  "Start a conversational design document creation session with Claude.",
    RunE:  runDesign,
}

var designResumeCmd = &cobra.Command{
    Use:   "resume [name]",
    Short: "Resume a design session",
    RunE:  runDesignResume,
}

var (
    designName string
    designFrom string
)

func init() {
    designCmd.Flags().StringVar(&designName, "name", "", "Name for the design doc")
    designCmd.Flags().StringVar(&designFrom, "from", "", "Path to PRD to base design on")
    designCmd.AddCommand(designResumeCmd)
}

func runDesign(cmd *cobra.Command, args []string) error {
    if !IsInitialized() {
        return fmt.Errorf("rafa is not initialized. Run 'rafa init' first")
    }

    if !skillsInstalled() {
        return fmt.Errorf("skills not found. Run 'rafa init' to install")
    }

    // Validate --from path if provided
    if designFrom != "" {
        if _, err := os.Stat(designFrom); os.IsNotExist(err) {
            return fmt.Errorf("PRD file not found: %s", designFrom)
        }
    }

    return tui.RunWithConversation(tui.ConversationOpts{
        Phase:   session.PhaseDesign,
        Name:    designName,
        FromDoc: designFrom,
    })
}

func runDesignResume(cmd *cobra.Command, args []string) error {
    if !IsInitialized() {
        return fmt.Errorf("rafa is not initialized. Run 'rafa init' first")
    }

    sessionsDir := filepath.Join(rafaDir, "sessions")
    storage := session.NewStorage(sessionsDir)

    var sess *session.Session
    var err error

    if len(args) > 0 {
        sess, err = storage.Load(session.PhaseDesign, args[0])
    } else {
        sess, err = storage.LoadByPhase(session.PhaseDesign)
    }

    if err != nil {
        if os.IsNotExist(err) {
            return fmt.Errorf("no design session found to resume")
        }
        return err
    }

    return tui.RunWithConversation(tui.ConversationOpts{
        Phase:      session.PhaseDesign,
        ResumeFrom: sess,
    })
}
```

**New file: `internal/cli/sessions.go`**

```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"
    "text/tabwriter"
    "time"

    "github.com/spf13/cobra"
    "github.com/pablasso/rafa/internal/session"
)

var sessionsCmd = &cobra.Command{
    Use:   "sessions",
    Short: "List active sessions",
    RunE:  runSessions,
}

func runSessions(cmd *cobra.Command, args []string) error {
    if !IsInitialized() {
        return fmt.Errorf("rafa is not initialized. Run 'rafa init' first")
    }

    sessionsDir := filepath.Join(rafaDir, "sessions")
    storage := session.NewStorage(sessionsDir)

    sessions, err := storage.List()
    if err != nil {
        return err
    }

    if len(sessions) == 0 {
        fmt.Println("No active sessions.")
        return nil
    }

    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "PHASE\tNAME\tSTATUS\tUPDATED")

    for _, s := range sessions {
        age := formatAge(s.UpdatedAt)
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Phase, s.Name, s.Status, age)
    }

    return w.Flush()
}

func formatAge(t time.Time) string {
    d := time.Since(t)
    switch {
    case d < time.Minute:
        return "just now"
    case d < time.Hour:
        return fmt.Sprintf("%dm ago", int(d.Minutes()))
    case d < 24*time.Hour:
        return fmt.Sprintf("%dh ago", int(d.Hours()))
    default:
        return fmt.Sprintf("%dd ago", int(d.Hours()/24))
    }
}
```

### Session Expiration Handling

Claude Code sessions have a limited lifetime. Rather than pre-checking (which would pollute the conversation), we detect expiration during the resume attempt.

**Add to `internal/ai/conversation.go`:**

```go
// ErrSessionExpired indicates the session cannot be resumed.
var ErrSessionExpired = errors.New("session expired or not found")

// Resume-related error detection
func isSessionExpiredError(output string) bool {
    // Error messages from Claude CLI when session is invalid
    return strings.Contains(output, "session not found") ||
           strings.Contains(output, "session expired") ||
           strings.Contains(output, "invalid session")
}
```

**Update `StartConversation` to detect expiration:**

```go
func StartConversation(ctx context.Context, config ConversationConfig) (*Conversation, <-chan StreamEvent, error) {
    // ... invocation code ...

    if err := c.cmd.Start(); err != nil {
        return nil, nil, err
    }

    // Read first event to check for session errors
    scanner := bufio.NewScanner(stdout)
    if scanner.Scan() {
        line := scanner.Text()
        // Check for session expiration in error response
        if strings.Contains(line, "error") && isSessionExpiredError(line) {
            c.cmd.Process.Kill()
            return nil, nil, ErrSessionExpired
        }
        // Process first line normally...
    }
    // ... rest of implementation
}
```

**Handle in CLI resume commands:**

```go
// In runPRDResume and runDesignResume:
err := tui.RunWithConversation(opts)
if errors.Is(err, ai.ErrSessionExpired) {
    fmt.Println("Session expired. Starting a new session...")
    opts.ResumeFrom = nil // Clear resume, start fresh
    return tui.RunWithConversation(opts)
}
return err
```

This approach:
- Doesn't pollute conversation with "ping" messages
- Detects expiration during actual resume attempt
- Offers graceful fallback to fresh session

## Security

- **Permissions**: Uses `--dangerously-skip-permissions` for all Claude invocations (required for non-interactive use)
- **File Access**: Skills are fetched from a trusted source (pablasso/skills repo)
- **Session Data**: Session files contain only metadata (session IDs, paths), no sensitive content
- **Gitignore**: Sessions directory is gitignored to prevent accidental commits

## Edge Cases

| Case | How it's handled |
|------|------------------|
| Skills repo unavailable | `rafa init` fails completely with error message |
| Session expired | Detected on resume, user prompted to start fresh |
| Document path exists | User warned and asked for alternative name |
| Claude CLI not installed | Prerequisite check fails with installation link |
| Git repo dirty during plan run | Existing check blocks execution |
| Terminal too small | Existing minimum size check |
| Multiple concurrent drafting sessions | Allowed - each session tracked independently |
| Concurrent plan execution | Existing lock file prevents |
| Network timeout during skills fetch | Install fails, partial state cleaned up |
| User cancels mid-conversation | Session saved with "cancelled" status |

## Performance

- **Streaming**: All Claude output is streamed in real-time (no buffering wait)
- **Event Parsing**: JSON parsing is done line-by-line, not buffered
- **Session Storage**: Atomic writes prevent corruption on crash
- **Activity Timeline**: Limited to last N entries to prevent memory growth
- **Output Viewport**: Uses virtual scrolling (existing component)

## Testing

### Unit Tests

#### `internal/session` - Session Management
| Test | Description |
|------|-------------|
| `TestSessionSave` | Atomic write creates valid JSON |
| `TestSessionLoad` | Load returns correct struct |
| `TestSessionLoadNotFound` | Returns os.ErrNotExist for missing |
| `TestSessionList` | Returns all sessions sorted by UpdatedAt |
| `TestSessionDelete` | Removes file, idempotent for missing |
| `TestSessionFilename` | Correct naming pattern `<phase>-<name>.json` |

#### `internal/skills` - Skills Installer
| Test | Description |
|------|-------------|
| `TestInstallExtractsTarball` | Mock HTTP, verify correct files extracted |
| `TestInstallSkipsNonSkillDirs` | `.claude/`, `README.md` not copied |
| `TestInstallVerifiesSKILLmd` | Fails if SKILL.md missing |
| `TestInstallCleansUpOnFailure` | Partial extraction removed on error |
| `TestIsInstalled` | True only when all skills have SKILL.md |
| `TestUninstall` | Removes only skill directories |

#### `internal/ai/conversation.go` - Stream Event Parsing
**Critical**: These tests cover the core parsing logic. Use table-driven tests with real captured output.

| Test | Input Event Type | Expected Output |
|------|------------------|-----------------|
| `TestParseInitEvent` | `{"type":"system","subtype":"init","session_id":"..."}` | `StreamEvent{Type:"init", SessionID:"..."}` |
| `TestParseTextDelta` | `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}` | `StreamEvent{Type:"text", Text:"Hello"}` |
| `TestParseToolUse` | `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"..."}}]}}` | `StreamEvent{Type:"tool_use", ToolName:"Read", ToolTarget:"..."}` |
| `TestParseTaskTool` | Tool use with `name:"Task"` | Correctly extracts subagent description |
| `TestParseToolResult` | `{"type":"user","message":{"content":[{"type":"tool_result"...}]}}` | `StreamEvent{Type:"tool_result"}` |
| `TestParseResultEvent` | `{"type":"result","session_id":"...","total_cost_usd":0.01,"usage":{...}}` | All fields populated |
| `TestParseResultError` | `{"type":"result","is_error":true,...}` | `StreamEvent{Type:"error"}` |
| `TestParseMalformedJSON` | Invalid JSON | Empty StreamEvent (no panic) |
| `TestParseUnknownType` | `{"type":"unknown"}` | Empty StreamEvent |

#### `internal/tui/views/conversation.go` - Conversation View
| Test | Description |
|------|-------------|
| `TestConversationModelInit` | Creates session, starts conversation |
| `TestConversationHandleTextEvent` | Appends to response view |
| `TestConversationHandleToolUse` | Adds to activity timeline |
| `TestConversationHandleDone` | Transitions state, triggers auto-review if needed |
| `TestConversationApprove` | Marks session complete |
| `TestConversationCancel` | Marks session cancelled, stops conversation |
| `TestConversationSendMessage` | Invokes Claude with --resume |
| `TestAutoReviewTrigger` | Detects Write to docs/ and triggers review |

#### `internal/tui/views/filenaming.go` - File Naming
| Test | Description |
|------|-------------|
| `TestFileNamingConfirm` | Enter saves file |
| `TestFileNamingEdit` | 'e' enables editing mode |
| `TestFileNamingOverwrite` | 'o' overwrites when file exists |
| `TestFileNamingCancel` | 'c' returns without saving |
| `TestFileNamingExistsWarning` | Shows warning when file exists |

#### Activity Timeline Formatting
| Test | Description |
|------|-------------|
| `TestActivityAddEntry` | Entry added with correct timestamp and indent |
| `TestActivityToolUseFormatting` | Tool name and shortened path displayed |
| `TestActivityLongPath` | Paths > 30 chars shortened to `.../last/two.go` |
| `TestActivityTaskTool` | Task tool shows subagent description |
| `TestActivityRapidEvents` | Multiple events in quick succession don't corrupt state |
| `TestActivityMarkDone` | Last entry marked done on tool_result |
| `TestActivityMaxEntries` | Old entries pruned when limit exceeded |

#### Error Recovery Tests
| Test | Description |
|------|-------------|
| `TestConversationNetworkDrop` | Context cancelled mid-stream, session persisted |
| `TestConversationCLICrash` | Claude process exits non-zero, error surfaced to user |
| `TestConversationSessionCorrupt` | Invalid JSON in session file, graceful error + offer fresh start |
| `TestConversationResumeExpired` | Session expired error detected, falls back to fresh session |
| `TestSkillsPartialExtract` | Network fails mid-tarball, cleanup removes partial files |
| `TestInitInterrupted` | Ctrl+C during init, no partial .rafa/ left behind |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestSkillsInstallFromGitHub` | Real HTTP to GitHub, verify all 5 skills |
| `TestConversationResumeSession` | Mock Claude CLI, verify --resume flag passed |
| `TestInitInstallsSkillsAndCreatesDirectories` | End-to-end init verification |
| `TestDeinitRemovesSkillsAndRafaDir` | End-to-end deinit verification |

### Manual Testing Checklist

- [ ] Complete PRD creation flow end-to-end
- [ ] Design doc creation from existing PRD
- [ ] Design doc creation without PRD
- [ ] Plan creation with conversational refinement
- [ ] Session resume after terminal close (within 5 hours)
- [ ] Session expiration handling (after 5+ hours)
- [ ] File exists conflict resolution
- [ ] Activity timeline shows tool uses correctly
- [ ] Token/cost tracking during plan execution
- [ ] Cancel mid-conversation preserves session

### Testing Risks

1. **Stream-json parsing**: High risk area. Claude CLI output format could change. Capture real output samples for regression tests.

2. **GitHub availability**: Skills integration tests depend on GitHub. Mock for unit tests, real only for CI integration suite.

3. **Session timing**: Hard to test 5-hour expiration. Document expected behavior, rely on manual testing.

## Rollout

### Phase 1: Foundation
1. Implement session package
2. Implement skills installer
3. Update `rafa init` with skills installation
4. Add `rafa sessions` command

### Phase 2: Conversation Engine
1. Implement conversation handler
2. Add activity parsing from stream-json
3. Implement session persistence

### Phase 3: TUI
1. Add conversation view
2. Update home view with new menu
3. Update running view with activity timeline

### Phase 4: CLI Commands
1. Add `rafa prd` command
2. Add `rafa design` command
3. Add resume subcommands

### Phase 5: Polish
1. Session expiration detection
2. File naming suggestions
3. Auto-review integration

## Trade-offs

### Considered: Direct API vs CLI

**Chose CLI** because:
- Simpler auth (reuses user's Claude Code login)
- Access to all Claude Code features (tools, skills)
- `--resume` provides session management for free
- Matches PRD requirement

### Considered: Persistent subprocess vs per-message invocation

**Chose per-message with `--resume`** because:
- More robust to crashes
- Matches Claude Code's design
- Easier to implement session persistence

**Clarification**: Each user message triggers a new `claude -p` invocation with `--resume <session-id>`. The `Conversation` struct manages the lifecycle of a single invocation, not a persistent subprocess. When the user sends a follow-up message, we:
1. Wait for the current invocation to complete (receive `result` event)
2. Persist the session ID
3. Start a new invocation with `--resume` for the next message

This is different from holding stdin/stdout pipes to a long-running process.

### Considered: Bundled skills vs fetched skills

**Chose fetched from GitHub** because:
- Skills can be updated independently
- Matches PRD requirement
- Smaller Rafa binary

### Considered: Real-time activity parsing vs post-hoc

**Chose real-time parsing** because:
- Better user experience (immediate feedback)
- Lower memory usage (don't buffer entire output)
- Matches PRD's activity timeline requirement

## Design Review Feedback Addressed

The following concerns from the technical design review have been addressed:

| Concern | Resolution |
|---------|------------|
| **Architecture contradiction**: Design said "per-message with --resume" but showed persistent subprocess | Clarified in Trade-offs section. Updated `Conversation` struct to use per-message invocation with `--resume`. Each user message spawns a new `claude -p` process. |
| **Error handling in triggerAutoReview** | Changed `triggerAutoReview()` to return `tea.Cmd` and properly propagate errors through the Bubble Tea event loop. |
| **Race condition in event channel** | Added `forwardEvents()` helper that blocks on critical events (`init`, `done`) to prevent losing session IDs or completion signals. |
| **Session expiration check pollutes conversation** | Removed pre-check "ping" approach. Now detect expiration during actual resume attempt and gracefully fall back to fresh session. |
| **Partial skills installation on failed init** | Added `installer.Uninstall()` call in init error path. Skills installer also cleans up internally on extraction failure. |
| **Auto-review detection fragile** | Changed from text matching to tracking Write tool targets. More reliable detection of document creation. |

## Open Questions

1. **Multi-line input handling**: The Bubble Tea textarea component may need custom handling for Ctrl+Enter vs Enter (Ctrl+Enter submits, Enter is newline).

---

## Additional Design Details

### Plan Creation Conversational Flow

The existing plan creation extracts tasks non-interactively. The new conversational flow:

1. User selects design doc via **existing FilePickerModel** (reuse `internal/tui/views/filepicker.go`)
   - Start picker in `docs/designs/` directory
   - If no `.md` files exist, show error: "No design docs found. Create one with 'rafa design'"

2. Optional: User provides initial instructions in a text input
   - "Any specific constraints or focus areas?"
   - Can skip with Enter

3. Rafa instructs Claude to extract tasks conversationally:
   ```
   Extract implementation tasks from this design document. After extraction,
   ask the user if they want to adjust scope, add constraints, or modify any tasks.
   ```

4. User can refine via conversation:
   - "Split task 3 into two smaller tasks"
   - "Add a task for database migrations"
   - "Remove the testing task, I'll handle that separately"

5. On approve: Save plan using existing `plan.SavePlan()` infrastructure

**Key difference from PRD/Design flows**: No auto-review step (tasks are self-reviewed during extraction).

### `rafa deinit` Command

**File: `internal/cli/deinit.go` (modifications)**

```go
func runDeinit(cmd *cobra.Command, args []string) error {
    // ... existing confirmation ...

    // Remove skills BEFORE removing .rafa
    skillsDir := filepath.Join(repoRoot, ".claude", "skills")
    installer := skills.NewInstaller(skillsDir)
    if err := installer.Uninstall(); err != nil {
        fmt.Printf("Warning: failed to remove skills: %v\n", err)
    }

    // Remove .rafa directory (existing behavior)
    if err := os.RemoveAll(rafaDir); err != nil {
        return fmt.Errorf("failed to remove %s: %w", rafaDir, err)
    }

    // Remove gitignore entries (existing behavior)
    // ...

    fmt.Println("Removed Rafa configuration and skills.")
    return nil
}
```

### Design Doc Selection for Plan Creation

Reuse existing `FilePickerModel` with modified starting directory:

```go
// In app.go, handle GoToFilePickerMsg with context
case msgs.GoToFilePickerMsg:
    startDir := m.repoRoot
    if msg.ForPlanCreation {
        startDir = filepath.Join(m.repoRoot, "docs", "designs")
    }
    m.filePicker = views.NewFilePickerModel(startDir)
    // ...
```

Add context to the message:
```go
// In msgs/messages.go
type GoToFilePickerMsg struct {
    CurrentDir      string
    ForPlanCreation bool  // If true, start in docs/designs/
}
```

### File Naming Interaction

When Claude finishes drafting a document, the flow is:

1. **Claude suggests a filename** based on content analysis
   - Included in Claude's response: "I suggest saving this as `user-authentication.md`"

2. **Rafa extracts the suggestion** from Claude's response
   - Parse for pattern: "saving this as `<filename>`" or "filename: <filename>"

3. **User sees confirmation prompt** in the TUI:
   ```
   Save as: docs/prds/user-authentication.md

   [Enter] Save  [e] Edit name  [c] Cancel
   ```

4. **If user presses `e`**: Show text input pre-filled with suggested name
   - User can edit and press Enter to confirm

5. **If file exists**: Show warning
   ```
   ⚠ docs/prds/user-authentication.md already exists

   [o] Overwrite  [e] Edit name  [c] Cancel
   ```

**Implementation**: Add `FileNamingModel` component in `internal/tui/views/`:

```go
type FileNamingModel struct {
    suggestedPath string
    editing       bool
    input         textinput.Model
    fileExists    bool
}

func (m FileNamingModel) Update(msg tea.Msg) (FileNamingModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if m.editing {
            // Handle text input
        } else {
            switch msg.String() {
            case "enter":
                if m.fileExists {
                    return m, nil // Require explicit overwrite
                }
                return m, m.confirmSave()
            case "e":
                m.editing = true
                m.input.Focus()
            case "o":
                if m.fileExists {
                    return m, m.confirmSave()
                }
            case "c":
                return m, m.cancel()
            }
        }
    }
    return m, nil
}
```

## Future Improvements

1. **Session refreshing**: Proactively refresh sessions approaching the 5-hour limit to avoid mid-conversation expiration. Could show a warning at 4.5 hours and offer to refresh.

## Verified Assumptions

The following were verified against Claude CLI v2.1.27 and official documentation:

### Session Lifetime
Sessions last **5 hours** from first message. OAuth tokens may expire in 2-4 hours unpredictably during long-running tasks. After ~3-4 days, context quality degrades significantly.

**Sources**: [Claude Code Session Management](https://stevekinney.com/courses/ai-development/claude-code-session-management), [OAuth token expiration issue](https://github.com/anthropics/claude-code/issues/12447)

### Task Tool (Subagent) Events
Task tool invocations appear as standard `tool_use` events:
```json
{"type":"assistant","message":{"content":[{
  "type":"tool_use",
  "name":"Task",
  "input":{"description":"...","prompt":"...","subagent_type":"Explore"}
}]}}
```
No special "subagent spawn" events - parse `name: "Task"` like any other tool.

### Error Event Format
Errors appear in the `result` event with `is_error: true`. For API errors, check `message.error` field.

**Source**: [CLI reference](https://code.claude.com/docs/en/cli-reference)

### Session ID Format
Session IDs are UUIDs available in multiple stream-json events:
- `system` (init): `{"type":"system","subtype":"init","session_id":"9f36fe2b-469c-4f0e-8df3-fc8ff1b2e297",...}`
- `assistant`: `{"type":"assistant","session_id":"...",...}`
- `result`: `{"type":"result","session_id":"...",...}`

The session ID is available immediately from the `system` init event, so we can persist it before the conversation even starts.

### Token and Cost Tracking
The `result` event includes comprehensive usage data:
```json
{
  "type": "result",
  "total_cost_usd": 0.01772025,
  "usage": {
    "input_tokens": 3,
    "output_tokens": 5,
    "cache_read_input_tokens": 18098,
    "cache_creation_input_tokens": 1365
  },
  "modelUsage": {
    "claude-opus-4-5-20251101": {
      "inputTokens": 3,
      "outputTokens": 5,
      "costUSD": 0.01772025
    }
  }
}
```

The `assistant` events also include per-message usage in `message.usage`.

### CLI Flags Required
For stream-json output with `--print` mode:
- `--verbose` is required (stream-json requires verbose)
- `--output-format stream-json`
- `--include-partial-messages` for token-by-token streaming
- `--dangerously-skip-permissions` for non-interactive use

### Resume Flag
`--resume <session-id>` accepts a UUID session ID for continuation.
