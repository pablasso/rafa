# Technical Design: `rafa plan create` Command

## Overview

This document details the implementation of the `rafa plan create <file>` command, which converts a technical design or PRD (markdown) into an executable JSON plan. This is the entry point for users to transform their design documents into structured task sequences that Rafa can execute.

**PRD Reference**: [docs/prds/rafa-core.md](../prds/rafa-core.md)

## Goals

- Parse markdown design documents and extract discrete, well-scoped tasks
- Generate task acceptance criteria that are specific and verifiable
- Create self-contained plan folders with proper structure
- Provide clear feedback on success with actionable next steps

## Non-Goals

- Validating the quality of acceptance criteria (future: `rafa doctor`)
- Supporting non-markdown input formats
- Interactive task editing or refinement
- Parallel plan creation

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│              rafa plan create <file> [--name] [--dry-run]       │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      1. Input Validation                         │
│  - Check .rafa/ exists (initialized) - skip if --dry-run       │
│  - Check file exists and is readable                            │
│  - Check file is markdown (.md extension)                       │
│  - Check file is not empty                                      │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                    2. Read Design Document                       │
│  - Read file contents                                           │
│  - Normalize path to relative (from repo root)                  │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                  3. AI Task Extraction                           │
│  - Call Claude Code CLI with structured prompt                  │
│  - Use --dangerously-skip-permissions flag                      │
│  - Parse JSON from response (with defensive extraction)         │
│  - Validate response structure                                  │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                   4. Generate Plan Structure                     │
│  - Generate short plan ID (6 chars, crypto/rand)                │
│  - Use --name flag, or AI-extracted name, or filename           │
│  - Assign sequential task IDs (t01, t02, ...)                   │
│  - Set initial statuses                                         │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
                        ┌──────────────┐
                        │  --dry-run?  │
                        └──────────────┘
                         │            │
                        yes           no
                         │            │
                         ▼            ▼
┌───────────────────────────┐  ┌─────────────────────────────────┐
│   5a. Display Preview     │  │      5b. Create Plan Folder     │
│  - Show extracted tasks   │  │  - Check for name collision     │
│  - Show what would be     │  │  - Create .rafa/plans/<id>-name/│
│    created                │  │  - Write plan.json              │
│  - Exit without saving    │  │  - Create empty log files       │
└───────────────────────────┘  └─────────────────────────────────┘
                                          │
                                          ▼
                               ┌─────────────────────────────────┐
                               │      6. Display Success         │
                               │  - Show plan ID, name, count    │
                               │  - Show task summary            │
                               │  - Show: rafa plan run <name>   │
                               └─────────────────────────────────┘
```

## Technical Details

### CLI Interface

```
rafa plan create <file> [flags]

Arguments:
  file    Path to markdown design document (required)

Flags:
  --name <name>    Override the plan name (default: extracted from document or filename)
  --dry-run        Preview extracted tasks without creating the plan

Examples:
  rafa plan create docs/designs/user-auth.md
  rafa plan create docs/designs/user-auth.md --name my-auth-feature
  rafa plan create docs/designs/user-auth.md --dry-run
```

### Project Structure (Go)

```
rafa/
├── cmd/
│   └── rafa/
│       └── main.go           # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go           # Root command setup
│   │   └── plan/
│   │       ├── plan.go       # plan subcommand
│   │       ├── create.go     # plan create implementation
│   │       └── create_test.go
│   ├── plan/
│   │   ├── plan.go           # Plan struct and methods
│   │   ├── plan_test.go
│   │   ├── task.go           # Task struct
│   │   ├── storage.go        # Plan folder operations
│   │   └── storage_test.go
│   ├── ai/
│   │   ├── claude.go         # Claude Code CLI integration
│   │   └── claude_test.go
│   └── util/
│       ├── id.go             # Short ID generation
│       └── id_test.go
├── go.mod
└── go.sum
```

### Data Model

**Plan struct** (`internal/plan/plan.go`):

```go
type Plan struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    SourceFile  string    `json:"sourceFile"`
    CreatedAt   time.Time `json:"createdAt"`
    Status      string    `json:"status"`
    Tasks       []Task    `json:"tasks"`
}

type Task struct {
    ID                 string   `json:"id"`
    Title              string   `json:"title"`
    Description        string   `json:"description"`
    AcceptanceCriteria []string `json:"acceptanceCriteria"`
    Status             string   `json:"status"`
    Attempts           int      `json:"attempts"`
}
```

**Status values**:
- Plan: `not_started`, `in_progress`, `completed`, `failed`
- Task: `pending`, `in_progress`, `completed`, `failed`

### Key Components

#### 1. Input Validator (`internal/cli/plan/create.go`)

```go
type CreateOptions struct {
    FilePath string
    Name     string // --name flag, empty if not provided
    DryRun   bool   // --dry-run flag
}

func validateInputs(opts CreateOptions) error {
    // Check .rafa/ exists (skip for dry-run)
    if !opts.DryRun {
        if _, err := os.Stat(".rafa"); os.IsNotExist(err) {
            return fmt.Errorf("rafa not initialized. Run `rafa init` first")
        }
    }

    // Check file exists
    info, err := os.Stat(opts.FilePath)
    if os.IsNotExist(err) {
        return fmt.Errorf("file not found: %s", opts.FilePath)
    }

    // Check markdown extension
    if !strings.HasSuffix(opts.FilePath, ".md") {
        return fmt.Errorf("file must be markdown (.md): %s", opts.FilePath)
    }

    // Check file is not empty
    if info.Size() == 0 {
        return fmt.Errorf("design document is empty: %s", opts.FilePath)
    }

    return nil
}

// normalizeSourcePath converts absolute path to relative from repo root
func normalizeSourcePath(filePath string) (string, error) {
    absPath, err := filepath.Abs(filePath)
    if err != nil {
        return "", err
    }

    // Get repo root (where .rafa/ lives)
    repoRoot, err := findRepoRoot()
    if err != nil {
        return filePath, nil // Fall back to original path
    }

    relPath, err := filepath.Rel(repoRoot, absPath)
    if err != nil {
        return filePath, nil // Fall back to original path
    }

    return relPath, nil
}
```

#### 2. Claude Code Integration (`internal/ai/claude.go`)

```go
func ExtractTasks(designContent string) (*TaskExtractionResult, error) {
    prompt := buildExtractionPrompt(designContent)

    // Call Claude Code CLI with required flags per PRD
    cmd := exec.Command("claude",
        "-p", prompt,
        "--output-format", "json",
        "--dangerously-skip-permissions",
    )

    output, err := cmd.Output()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return nil, fmt.Errorf("claude cli failed: %s", string(exitErr.Stderr))
        }
        return nil, fmt.Errorf("claude cli failed: %w", err)
    }

    // Parse JSON response with defensive extraction
    jsonData, err := extractJSON(output)
    if err != nil {
        return nil, fmt.Errorf("failed to extract JSON from response: %w", err)
    }

    var result TaskExtractionResult
    if err := json.Unmarshal(jsonData, &result); err != nil {
        return nil, fmt.Errorf("failed to parse plan from design: %w", err)
    }

    if err := result.Validate(); err != nil {
        return nil, fmt.Errorf("invalid plan structure: %w", err)
    }

    return &result, nil
}

// extractJSON finds and extracts JSON object from Claude's response.
// Handles cases where response might include additional text or formatting.
func extractJSON(data []byte) ([]byte, error) {
    // Try direct parse first
    if json.Valid(data) {
        return data, nil
    }

    // Look for JSON object boundaries
    str := string(data)
    start := strings.Index(str, "{")
    end := strings.LastIndex(str, "}")

    if start == -1 || end == -1 || end <= start {
        return nil, errors.New("no JSON object found in response")
    }

    jsonStr := str[start : end+1]
    if !json.Valid([]byte(jsonStr)) {
        return nil, errors.New("extracted content is not valid JSON")
    }

    return []byte(jsonStr), nil
}
```

**Extraction Prompt** (key design element):

```go
func buildExtractionPrompt(designContent string) string {
    return fmt.Sprintf(`You are a technical project planner. Analyze this design document and extract discrete implementation tasks.

DESIGN DOCUMENT:
%s

OUTPUT REQUIREMENTS:
Return a JSON object with this exact structure:
{
  "name": "kebab-case-name-from-document",
  "description": "One sentence describing the overall goal",
  "tasks": [
    {
      "title": "Short imperative title (e.g., 'Implement user login endpoint')",
      "description": "Detailed description of what needs to be done. Include relevant context from the design.",
      "acceptanceCriteria": [
        "Specific, verifiable criterion (e.g., 'npm test passes')",
        "Another measurable criterion",
        "Prefer runnable checks over prose"
      ]
    }
  ]
}

TASK GUIDELINES:
- Each task should use roughly 50-60%% of an AI agent's context window
- Tasks must be completable in sequence (later tasks can depend on earlier ones)
- Acceptance criteria must be verifiable by the agent itself
- Prefer criteria that can be verified with commands (tests, type checks, lint)
- Include 2-5 acceptance criteria per task
- Order tasks by implementation dependency

Return ONLY the JSON, no markdown formatting or explanation.`, designContent)
}
```

#### 3. ID Generator (`internal/util/id.go`)

```go
import (
    "crypto/rand"
    "fmt"
    "math/big"
)

const idChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateShortID creates a 6-character alphanumeric ID using crypto/rand
// for proper randomness (avoids duplicate IDs across runs).
func GenerateShortID() (string, error) {
    b := make([]byte, 6)
    max := big.NewInt(int64(len(idChars)))

    for i := range b {
        n, err := rand.Int(rand.Reader, max)
        if err != nil {
            return "", fmt.Errorf("failed to generate random ID: %w", err)
        }
        b[i] = idChars[n.Int64()]
    }
    return string(b), nil
}

func GenerateTaskID(index int) string {
    return fmt.Sprintf("t%02d", index+1)
}
```

#### 4. Plan Storage (`internal/plan/storage.go`)

```go
// ResolvePlanName checks for name collisions and returns a unique name.
// If "feature-auth" exists, returns "feature-auth-2", "feature-auth-3", etc.
func ResolvePlanName(baseName string) (string, error) {
    plansDir := filepath.Join(".rafa", "plans")

    entries, err := os.ReadDir(plansDir)
    if err != nil {
        if os.IsNotExist(err) {
            return baseName, nil // No plans yet, use as-is
        }
        return "", err
    }

    // Collect existing names (strip ID prefix)
    existingNames := make(map[string]bool)
    for _, entry := range entries {
        if entry.IsDir() {
            // Folder format: <id>-<name>
            parts := strings.SplitN(entry.Name(), "-", 2)
            if len(parts) == 2 {
                existingNames[parts[1]] = true
            }
        }
    }

    // If base name is available, use it
    if !existingNames[baseName] {
        return baseName, nil
    }

    // Find next available suffix
    for i := 2; ; i++ {
        candidate := fmt.Sprintf("%s-%d", baseName, i)
        if !existingNames[candidate] {
            return candidate, nil
        }
    }
}

func CreatePlanFolder(plan *Plan) error {
    folderName := fmt.Sprintf("%s-%s", plan.ID, plan.Name)
    folderPath := filepath.Join(".rafa", "plans", folderName)

    // Create directory
    if err := os.MkdirAll(folderPath, 0755); err != nil {
        return fmt.Errorf("failed to create plan folder: %w", err)
    }

    // Write plan.json
    planJSON, err := json.MarshalIndent(plan, "", "  ")
    if err != nil {
        return err
    }
    if err := os.WriteFile(filepath.Join(folderPath, "plan.json"), planJSON, 0644); err != nil {
        return err
    }

    // Create empty log files
    if err := os.WriteFile(filepath.Join(folderPath, "progress.log"), []byte{}, 0644); err != nil {
        return err
    }
    if err := os.WriteFile(filepath.Join(folderPath, "output.log"), []byte{}, 0644); err != nil {
        return err
    }

    return nil
}
```

### AI Response Handling

The Claude Code CLI response needs careful handling:

```go
type TaskExtractionResult struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Tasks       []ExtractedTask `json:"tasks"`
}

type ExtractedTask struct {
    Title              string   `json:"title"`
    Description        string   `json:"description"`
    AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

func (r *TaskExtractionResult) Validate() error {
    // Name is optional - will fall back to filename if empty
    if len(r.Tasks) == 0 {
        return errors.New("no tasks extracted")
    }
    for i, task := range r.Tasks {
        if task.Title == "" {
            return fmt.Errorf("task %d missing title", i+1)
        }
        if len(task.AcceptanceCriteria) == 0 {
            return fmt.Errorf("task %d (%s) missing acceptance criteria", i+1, task.Title)
        }
    }
    return nil
}
```

### Plan Name Resolution

The plan name is determined in this order of precedence:

```go
func resolvePlanName(opts CreateOptions, extracted *TaskExtractionResult) string {
    // 1. Use --name flag if provided
    if opts.Name != "" {
        return toKebabCase(opts.Name)
    }

    // 2. Use AI-extracted name if available
    if extracted.Name != "" {
        return toKebabCase(extracted.Name)
    }

    // 3. Fall back to filename without extension
    base := filepath.Base(opts.FilePath)
    return strings.TrimSuffix(base, ".md")
}

func toKebabCase(s string) string {
    // Convert spaces and underscores to hyphens, lowercase
    s = strings.ToLower(s)
    s = strings.ReplaceAll(s, " ", "-")
    s = strings.ReplaceAll(s, "_", "-")
    // Remove any non-alphanumeric characters except hyphens
    // ... (regex or manual filtering)
    return s
}
```

## Edge Cases

| Case | How it's handled |
|------|------------------|
| `.rafa/` doesn't exist | Error: "rafa not initialized. Run `rafa init` first" |
| File doesn't exist | Error: "file not found: <path>" |
| File not markdown | Error: "file must be markdown (.md): <path>" |
| Claude Code not installed | Error: "Claude Code CLI not found. Install it: https://claude.ai/code" |
| Claude Code not authenticated | Error: "Claude Code not authenticated. Run `claude auth` first" |
| Empty design document | Error: "design document is empty" |
| AI returns invalid JSON | Error: "failed to parse plan from design: <parse error>" |
| AI returns no tasks | Error: "could not extract tasks from design. Ensure document has clear requirements." |
| Plan name collision | Append numeric suffix: `feature-auth`, `feature-auth-2` |
| Very large design doc | No chunking in v1; Claude Code handles context limits |

## Security

- **File access**: Only reads the specified design file, writes only to `.rafa/`
- **Command execution**: Only executes `claude` CLI, no user-provided commands
- **No secrets**: Plan files contain no credentials (source file path is relative)
- **Permissions**: Uses standard file permissions (0755 dirs, 0644 files)

## Performance

- **Expected latency**: 5-30 seconds (dominated by Claude Code CLI call)
- **No streaming**: Wait for complete response, then process
- **Single file I/O**: Minimal disk operations

## Testing

### Unit Tests (`*_test.go` files)

**`internal/util/id_test.go`**:
```go
func TestGenerateShortID(t *testing.T) {
    // Test ID length is always 6
    // Test ID contains only valid characters (a-zA-Z0-9)
    // Test uniqueness: generate 1000 IDs, verify no duplicates
}

func TestGenerateTaskID(t *testing.T) {
    // Test sequential numbering: t01, t02, ... t99
    // Test zero-padding: index 0 -> "t01", index 9 -> "t10"
}
```

**`internal/plan/plan_test.go`**:
```go
func TestPlanSerialization(t *testing.T) {
    // Test Plan struct marshals to expected JSON format
    // Test Plan struct unmarshals from valid JSON
    // Test timestamps serialize in ISO-8601 format
}

func TestTaskExtractionResultValidate(t *testing.T) {
    // Test valid result passes validation
    // Test empty tasks returns error
    // Test task without title returns error
    // Test task without acceptance criteria returns error
    // Test empty name is allowed (falls back to filename)
}
```

**`internal/plan/storage_test.go`**:
```go
func TestResolvePlanName(t *testing.T) {
    // Test new name returns unchanged
    // Test existing name returns "name-2"
    // Test existing "name" and "name-2" returns "name-3"
    // Test with empty plans directory
}

func TestCreatePlanFolder(t *testing.T) {
    // Test creates directory structure
    // Test writes valid plan.json
    // Test creates empty log files
    // Test handles filesystem errors
}
```

**`internal/ai/claude_test.go`**:
```go
func TestExtractJSON(t *testing.T) {
    // Test clean JSON passes through
    // Test JSON with leading text is extracted
    // Test JSON with trailing text is extracted
    // Test JSON wrapped in markdown code block is extracted
    // Test invalid JSON returns error
    // Test no JSON found returns error
}

func TestBuildExtractionPrompt(t *testing.T) {
    // Test prompt includes design content
    // Test prompt specifies JSON output format
    // Test prompt includes task guidelines
}
```

**`internal/cli/plan/create_test.go`**:
```go
func TestValidateInputs(t *testing.T) {
    // Test missing .rafa/ returns error (unless dry-run)
    // Test dry-run skips .rafa/ check
    // Test missing file returns error
    // Test non-markdown file returns error
    // Test empty file returns error
    // Test valid inputs pass
}

func TestResolvePlanNamePrecedence(t *testing.T) {
    // Test --name flag takes precedence
    // Test AI-extracted name used if no flag
    // Test filename used as fallback
}

func TestNormalizeSourcePath(t *testing.T) {
    // Test absolute path converted to relative
    // Test already-relative path unchanged
    // Test path outside repo returns original
}
```

### Integration Tests

```go
func TestCreateCommandE2E(t *testing.T) {
    // Setup: create temp dir with .rafa/ and sample design.md
    // Mock Claude CLI with canned JSON response
    // Run: execute plan create command
    // Verify: plan folder created with correct structure
    // Verify: plan.json contains expected data
    // Verify: log files exist
    // Verify: stdout shows success message
}

func TestCreateCommandDryRun(t *testing.T) {
    // Setup: create temp dir with sample design.md (no .rafa/)
    // Mock Claude CLI with canned response
    // Run: execute plan create --dry-run
    // Verify: no files created
    // Verify: stdout shows preview
}

func TestCreateCommandErrors(t *testing.T) {
    // Test Claude CLI not found
    // Test Claude CLI returns error
    // Test Claude CLI returns invalid JSON
    // Test plan name collision handling
}
```

### Test Utilities

```go
// MockClaudeCLI replaces the claude command for testing
type MockClaudeCLI struct {
    Response string
    Error    error
}

func (m *MockClaudeCLI) Setup(t *testing.T) {
    // Create mock claude binary in PATH
    // Configure it to return m.Response or m.Error
}
```

### Manual Testing Checklist

- [ ] Test with small design document (~100 lines)
- [ ] Test with large design document (~1000 lines)
- [ ] Test with poorly structured document (no clear sections)
- [ ] Test --dry-run shows correct preview
- [ ] Test --name overrides AI-extracted name
- [ ] Test creating multiple plans with same source file
- [ ] Verify error messages are helpful for common mistakes

## Rollout

1. Implement core structs and storage layer
2. Implement Claude Code CLI integration
3. Implement `plan create` command
4. Add error handling and edge cases
5. Write tests
6. Document usage in README

No feature flags needed - this is greenfield development.

## Trade-offs

### Considered: Interactive task refinement
- **Rejected**: Adds complexity, user can edit plan.json directly
- **Chosen**: Single-shot extraction, simpler UX

### Considered: Multiple AI calls for large documents
- **Rejected**: Chunking adds complexity and risks inconsistent task ordering
- **Chosen**: Single call, rely on Claude Code's context management

### Considered: Task validation/scoring before save
- **Rejected**: Out of scope per PRD ("Rafa doesn't validate designs")
- **Chosen**: Trust AI output, user reviews plan.json (use `--dry-run` to preview)

### Considered: Prompt user for plan name
- **Rejected**: Adds friction in default flow
- **Chosen**: AI extracts name, with `--name` flag for override when needed

### Considered: Fail on name collision
- **Rejected**: Poor UX, forces user to manually resolve
- **Chosen**: Auto-append suffix (`-2`, `-3`), keeps workflow smooth

## Decisions

1. **No retry on invalid JSON**: If Claude returns unparseable output, fail immediately with error. Retrying won't change the result.

2. **Name resolution order**: `--name` flag → AI-extracted name → filename without extension.

3. **Use crypto/rand for IDs**: Ensures unique IDs across runs without requiring manual seeding.

4. **Defensive JSON parsing**: Extract JSON from response even if wrapped in text or markdown formatting.

5. **Relative source paths**: Store `sourceFile` as relative to repo root for portability.

6. **Name collision handling**: Append numeric suffix (`-2`, `-3`) rather than fail.

## Dependencies

- Go 1.21+ (for standard library improvements)
- [cobra](https://github.com/spf13/cobra) - CLI framework
- Claude Code CLI - installed and authenticated

## Output Examples

### Standard Create

```
$ rafa plan create docs/designs/user-auth.md

Creating plan from: docs/designs/user-auth.md
Extracting tasks...

Plan created: xK9pQ2-user-auth

  8 tasks extracted:

  t01: Set up authentication middleware
  t02: Implement user registration endpoint
  t03: Implement login endpoint
  t04: Add JWT token generation
  t05: Implement token refresh endpoint
  t06: Add password reset flow
  t07: Write authentication tests
  t08: Update API documentation

Run `rafa plan run user-auth` to start execution.
```

### With --name Flag

```
$ rafa plan create docs/designs/user-auth.md --name auth-v2

Creating plan from: docs/designs/user-auth.md
Extracting tasks...

Plan created: mN3xK7-auth-v2

  8 tasks extracted:
  ...

Run `rafa plan run auth-v2` to start execution.
```

### Dry Run

```
$ rafa plan create docs/designs/user-auth.md --dry-run

Creating plan from: docs/designs/user-auth.md
Extracting tasks...

Plan preview (dry run - nothing saved):

  Name: user-auth
  Source: docs/designs/user-auth.md
  Tasks: 8

  t01: Set up authentication middleware
       ✓ Middleware function exists in src/middleware/auth.ts
       ✓ Middleware validates JWT tokens
       ✓ npm test passes

  t02: Implement user registration endpoint
       ✓ POST /api/register endpoint exists
       ✓ Validates email and password format
       ✓ Returns 201 on success
       ✓ npm test passes

  ... (remaining tasks)

To create this plan, run without --dry-run.
```
