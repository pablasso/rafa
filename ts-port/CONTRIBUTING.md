# Contributing to Rafa

## Development Setup

### Prerequisites

- Node.js 20+
- npm
- Git
- [Claude Code](https://claude.ai/code) (for integration testing)

### Getting Started

```bash
git clone https://github.com/pablasso/rafa.git
cd rafa/ts-port
npm install
npm run dev
```

The `npm run dev` command watches for changes and recompiles TypeScript automatically.

### Running the App

```bash
npm run build && npm start
```

Or for development with auto-rebuild:

```bash
# Terminal 1: Watch and rebuild
npm run dev

# Terminal 2: Run the app
npm start
```

## Architecture Overview

Rafa is built with TypeScript using the [pi-tui](https://github.com/badlogic/pi-mono) framework for the terminal UI. The architecture follows a clear separation between TUI components, core business logic, and storage.

### Package Structure

```
src/
├── index.ts              # Entry point, CLI routing
├── cli/                  # CLI command handlers
│   ├── init.ts           # rafa init
│   ├── deinit.ts         # rafa deinit
│   ├── plan-create.ts    # rafa plan create
│   ├── prd.ts            # rafa prd
│   └── design.ts         # rafa design
├── tui/
│   ├── app.ts            # Main TUI container, view routing
│   ├── views/
│   │   ├── home.ts       # Home menu view
│   │   ├── conversation.ts # PRD/Design conversation view
│   │   ├── run.ts        # Plan execution view
│   │   ├── plan-list.ts  # Plan selection view
│   │   └── file-picker.ts # Design doc picker
│   └── components/
│       ├── activity-log.ts   # Tool use timeline
│       ├── task-progress.ts  # Task list with status
│       ├── output-stream.ts  # Claude output display
│       └── multiline-editor.ts # Text input component
├── core/
│   ├── plan.ts           # Plan types and operations
│   ├── task.ts           # Task types
│   ├── session.ts        # Conversation session management
│   ├── claude-runner.ts  # Claude CLI invocation
│   ├── stream-parser.ts  # Parse stream-json output
│   ├── executor.ts       # Task execution loop
│   ├── extract-tasks.ts  # Extract tasks from design docs
│   ├── progress.ts       # Progress event logging
│   └── skills.ts         # Skills installation
├── storage/
│   ├── plans.ts          # Plan CRUD operations
│   ├── sessions.ts       # Session persistence
│   ├── settings.ts       # User settings
│   └── migration.ts      # Data migration from Go version
└── utils/
    ├── git.ts            # Git operations
    ├── lock.ts           # File locking
    ├── id.ts             # Short ID generation
    ├── string.ts         # String utilities
    └── claude-check.ts   # Claude CLI detection
```

### Key Layers

**TUI Layer (`src/tui/`)**
- `app.ts` - Main TUI container using pi-tui's TUI class, handles view routing
- `views/` - Full-screen views that handle user interaction
- `components/` - Reusable UI components for rendering and display

**Core Layer (`src/core/`)**
- Business logic independent of the UI
- `executor.ts` - The main task execution loop with retry logic
- `claude-runner.ts` - Spawns Claude CLI and handles stream-json output
- `stream-parser.ts` - Parses Claude's JSONL output into typed events

**Storage Layer (`src/storage/`)**
- File-based persistence for plans, sessions, and settings
- Atomic writes via temp file + rename pattern
- JSONL format for append-only logs

**Utils (`src/utils/`)**
- Shared utilities for git, locking, ID generation

### Data Flow

```
User Input → View → Core Service → Storage
                 ↓
            Claude CLI → Stream Parser → Events
                                      ↓
                              Activity Log → TUI Render
```

## Adding New Views

Views are full-screen components that handle a specific workflow. To add a new view:

1. **Create the view file** in `src/tui/views/`:

```typescript
import { Container, Box, Key } from "@mariozechner/pi-tui";

export class MyView {
  private container: Container;

  constructor(private app: RafaApp) {
    this.container = new Container("vertical");
  }

  // Called when view becomes active
  activate(context?: unknown) {
    // Initialize state based on context
  }

  // Handle keyboard input, return true if handled
  handleInput(key: Key): boolean {
    if (key.matches("escape")) {
      this.app.navigate("home");
      return true;
    }
    return false;
  }

  // Render the view
  render(width: number, height: number): string[] {
    // Return array of lines to display
    return this.container.render(width, height);
  }
}
```

2. **Register the view** in `src/tui/app.ts`:

```typescript
// In RafaApp constructor
this.views = new Map([
  ["home", new HomeView(this)],
  ["my-view", new MyView(this)],  // Add here
  // ...
]);
```

3. **Navigate to your view**:

```typescript
// From another view or component
this.app.navigate("my-view", { someContext: data });
```

## Adding New Components

Components are reusable UI elements. To add a new component:

1. **Create the component file** in `src/tui/components/`:

```typescript
export class MyComponent {
  private data: MyData[] = [];

  // Process incoming data
  addItem(item: MyData) {
    this.data.push(item);
  }

  // Clear state
  clear() {
    this.data = [];
  }

  // Render to string lines
  render(width: number): string[] {
    const lines: string[] = [];
    for (const item of this.data) {
      lines.push(this.formatItem(item, width));
    }
    return lines;
  }

  private formatItem(item: MyData, width: number): string {
    // Format with ANSI colors, truncation, etc.
    return `${item.name}`;
  }
}
```

2. **Use in a view**:

```typescript
class SomeView {
  private myComponent = new MyComponent();

  render(width: number, height: number): string[] {
    const componentLines = this.myComponent.render(width);
    // Compose with other components...
  }
}
```

## Testing

Rafa uses [Vitest](https://vitest.dev/) for testing with three levels of tests.

### Running Tests

```bash
# Run all tests once
npm test

# Run tests in watch mode
npm run test:watch

# Run a specific test file
npx vitest run src/__tests__/plan.test.ts

# Run tests matching a pattern
npx vitest run -t "executor"
```

### Test Types

**Unit Tests** - Test core logic in isolation:
- `src/__tests__/plan.test.ts` - Plan/task data structures
- `src/__tests__/stream-parser.test.ts` - Claude output parsing
- `src/__tests__/id.test.ts` - ID generation
- `src/__tests__/lock.test.ts` - File locking

```typescript
import { describe, it, expect } from "vitest";
import { createPlan } from "../core/plan.js";

describe("createPlan", () => {
  it("creates a plan with correct properties", () => {
    const plan = createPlan("abc123", "test", "desc", "source.md", []);
    expect(plan.id).toBe("abc123");
    expect(plan.status).toBe("not_started");
  });
});
```

**Component Tests** - Test TUI components rendering:
- `src/__tests__/activity-log.test.ts` - Activity timeline display
- `src/__tests__/run-view.test.ts` - Run view behavior

```typescript
import { describe, it, expect, beforeEach } from "vitest";
import { ActivityLogComponent } from "../tui/components/activity-log.js";

describe("ActivityLogComponent", () => {
  let component: ActivityLogComponent;

  beforeEach(() => {
    component = new ActivityLogComponent();
  });

  it("renders events with tree structure", () => {
    component.addEvent({ /* ... */ });
    const lines = component.render(80);
    expect(lines.some(l => l.includes("├─"))).toBe(true);
  });
});
```

**Integration Tests** - Test full flows with mocked Claude CLI:
- `src/__tests__/executor.test.ts` - Task execution with retries
- `src/__tests__/init.test.ts` - Repository initialization

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { Executor } from "../core/executor.js";

// Mock external dependencies
vi.mock("../core/claude-runner.js", () => ({
  runTask: vi.fn(),
}));

describe("Executor", () => {
  it("retries failed tasks up to MAX_ATTEMPTS", async () => {
    vi.mocked(runTask).mockRejectedValueOnce(new Error("fail"))
      .mockResolvedValueOnce(undefined);

    const executor = new Executor({ /* ... */ });
    await executor.run();

    expect(runTask).toHaveBeenCalledTimes(2);
  });
});
```

**E2E Tests** - Manual testing with real Claude CLI:
- Launch the TUI and test full workflows manually
- Verify Claude interactions work correctly

### Test Guidelines

1. **Mock external dependencies** - Always mock Claude CLI, file system for unit tests
2. **Use temp directories** - Create temp dirs in `beforeEach`, clean up in `afterEach`
3. **Test edge cases** - Empty inputs, max limits, error conditions
4. **Test rendering** - Verify component output fits within width constraints

## Code Style

### TypeScript

- Use explicit types for function parameters and return values
- Prefer interfaces over type aliases for object shapes
- Use `const` by default, `let` when reassignment is needed

### Formatting

The project doesn't enforce a specific formatter, but maintain consistency with existing code:
- 2-space indentation
- Semicolons
- Double quotes for strings

### Imports

- Use `.js` extension for local imports (required for ESM):
  ```typescript
  import { Plan } from "../core/plan.js";
  ```
- Group imports: external packages, then local modules

## Available Scripts

| Script | Description |
|--------|-------------|
| `npm run build` | Compile TypeScript to JavaScript |
| `npm run dev` | Watch mode, recompile on changes |
| `npm start` | Run the compiled application |
| `npm test` | Run all tests once |
| `npm run test:watch` | Run tests in watch mode |
| `npm run build:binary` | Compile to standalone binary (requires Bun) |

## Building Binaries

To create standalone binaries for distribution:

```bash
# Requires Bun installed
npm run build:binary
```

This creates a self-contained binary at `dist/rafa` using Bun's compile feature.

## Debugging

### TUI Issues

If the TUI doesn't render correctly:
1. Check terminal size - some views require minimum dimensions
2. Verify pi-tui is properly initialized
3. Check for ANSI escape code issues in your terminal

### Claude Runner Issues

To debug Claude CLI interaction:
```bash
# Run Claude directly with same flags
claude -p "test" --output-format stream-json --verbose
```

### Lock File Issues

If rafa reports "plan is already running":
```bash
# Check if another instance is running
ps aux | grep rafa

# Remove stale lock manually
rm .rafa/plans/<plan-id>/run.lock
```

## Pull Request Guidelines

1. **Keep PRs focused** - One feature or fix per PR
2. **Write tests** - Add tests for new functionality
3. **Run tests locally** - Ensure `npm test` passes
4. **Update docs** - Update README if adding new features
