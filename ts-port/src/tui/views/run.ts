/**
 * Plan execution view with split-pane layout
 *
 * Shows real-time progress of plan/task execution with:
 * - Task progress (top-left): List of all tasks with status
 * - Activity log (bottom-left, 25% height): Tool use timeline
 * - Output stream (right, 75% width): Claude's text output
 *
 * Layout:
 * ┌────────────────────────┬───────────────────────────────────────────┐
 * │  Task Progress         │  Output Stream                            │
 * │  ○ t01: Setup          │  [Claude's text output streams here...]   │
 * │  ▶ t02: Implement      │                                           │
 * │  ○ t03: Test           │                                           │
 * ├────────────────────────┤                                           │
 * │  Activity Log          │                                           │
 * │  ├─ Read auth.go       │                                           │
 * │  └─ Write login.go     │                                           │
 * └────────────────────────┴───────────────────────────────────────────┘
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";
import type { Plan } from "../../core/plan.js";
import type { Task } from "../../core/task.js";
import type { ClaudeEvent } from "../../core/claude-runner.js";
import { ActivityLogComponent } from "../components/activity-log.js";
import { TaskProgressComponent } from "../components/task-progress.js";
import { OutputStreamComponent } from "../components/output-stream.js";

// Layout constants
const LEFT_PANE_RATIO = 0.25; // Left pane is 25% of width
const ACTIVITY_LOG_RATIO = 0.25; // Activity log is 25% of left pane height
const MIN_LEFT_PANE_WIDTH = 30;
const MIN_RIGHT_PANE_WIDTH = 40;
const BORDER_WIDTH = 1; // Vertical separator

export interface RunContext {
  planId?: string;
  planName?: string;
  planDir?: string;
  plan?: Plan;
}

/**
 * Callback for aborting execution
 */
export type AbortCallback = () => void;

export class RunView implements RafaView {
  private app: RafaApp;
  private planId?: string;
  private planName?: string;
  private planDir?: string;
  private plan?: Plan;
  private currentTaskId: string | null = null;
  private isRunning = false;
  private statusMessage = "Ready";
  private abortCallback: AbortCallback | null = null;

  // Sub-components
  private taskProgress: TaskProgressComponent;
  private activityLog: ActivityLogComponent;
  private outputStream: OutputStreamComponent;

  constructor(app: RafaApp) {
    this.app = app;
    this.taskProgress = new TaskProgressComponent();
    this.activityLog = new ActivityLogComponent();
    this.outputStream = new OutputStreamComponent();
  }

  activate(context?: unknown): void {
    if (context && typeof context === "object") {
      const ctx = context as RunContext;
      this.planDir = ctx.planDir;

      if (ctx.plan) {
        this.plan = ctx.plan;
        // Derive planId and planName from plan if not explicitly provided
        this.planId = ctx.planId || ctx.plan.id;
        this.planName = ctx.planName || ctx.plan.name;
        this.taskProgress.setTasks(ctx.plan.tasks);
      } else {
        this.planId = ctx.planId;
        this.planName = ctx.planName;
      }
    }

    // Clear previous state for fresh run
    this.activityLog.clear();
    this.outputStream.clear();
    this.currentTaskId = null;
    this.isRunning = false;
    this.statusMessage = "Ready";
  }

  /**
   * Process a Claude event - updates activity log and output stream
   * Call this from the executor when events arrive
   */
  processEvent(event: ClaudeEvent): void {
    this.activityLog.processEvent(event);
    this.outputStream.processEvent(event);
  }

  /**
   * Set the current task being executed
   */
  setCurrentTask(taskId: string | null): void {
    this.currentTaskId = taskId;
    this.taskProgress.setCurrentTask(taskId);
  }

  /**
   * Update the task list (e.g., after status changes)
   */
  updateTasks(tasks: Task[]): void {
    this.taskProgress.setTasks(tasks);
  }

  /**
   * Set the running state
   */
  setRunning(running: boolean): void {
    this.isRunning = running;
    this.statusMessage = running ? "Running..." : "Stopped";
  }

  /**
   * Set the abort callback for Ctrl+C handling during execution
   */
  setAbortCallback(callback: AbortCallback | null): void {
    this.abortCallback = callback;
  }

  /**
   * Check if the view is currently running
   */
  getIsRunning(): boolean {
    return this.isRunning;
  }

  /**
   * Abort the current execution (called when Ctrl+C is pressed)
   */
  abortExecution(): void {
    if (this.isRunning && this.abortCallback) {
      this.abortCallback();
      this.statusMessage = "Stopping...";
      this.app.requestRender();
    }
  }

  /**
   * Set a status message
   */
  setStatus(message: string): void {
    this.statusMessage = message;
  }

  /**
   * Clear activity log for a new task attempt
   */
  clearActivityLog(): void {
    this.activityLog.clear();
  }

  /**
   * Request a TUI re-render
   */
  requestRender(): void {
    this.app.requestRender();
  }

  /**
   * Get access to activity log (for executor)
   */
  getActivityLog(): ActivityLogComponent {
    return this.activityLog;
  }

  /**
   * Get access to output stream (for executor)
   */
  getOutputStream(): OutputStreamComponent {
    return this.outputStream;
  }

  render(width: number): string[] {
    const lines: string[] = [];

    // Calculate pane dimensions
    const leftPaneWidth = Math.max(
      MIN_LEFT_PANE_WIDTH,
      Math.floor(width * LEFT_PANE_RATIO)
    );
    const rightPaneWidth = Math.max(
      MIN_RIGHT_PANE_WIDTH,
      width - leftPaneWidth - BORDER_WIDTH
    );

    // Header
    lines.push(this.renderHeader(width));
    lines.push(this.renderSeparator(width, "─"));

    // Calculate content height (terminal typically has ~24 rows, header takes ~2)
    // We'll dynamically determine based on the available lines
    const contentHeight = 20; // Reasonable default for terminal height
    const activityLogHeight = Math.max(4, Math.floor(contentHeight * ACTIVITY_LOG_RATIO));
    const taskProgressHeight = contentHeight - activityLogHeight;

    // Render sub-components to get their lines
    const taskLines = this.taskProgress.render(leftPaneWidth - 2);
    const activityLines = this.activityLog.render(leftPaneWidth - 2);
    const outputLines = this.outputStream.renderWithHeight(
      rightPaneWidth - 2,
      contentHeight
    );

    // Pad task lines to fill task progress section
    while (taskLines.length < taskProgressHeight) {
      taskLines.push("");
    }

    // Pad activity lines to fill activity log section
    while (activityLines.length < activityLogHeight) {
      activityLines.push("");
    }

    // Pad output lines to fill content area
    while (outputLines.length < contentHeight) {
      outputLines.push("");
    }

    // Render the task progress section (top-left)
    lines.push(this.renderPaneHeader(leftPaneWidth, "Tasks", rightPaneWidth, "Output"));

    for (let i = 0; i < taskProgressHeight; i++) {
      const leftContent = taskLines[i] || "";
      const rightContent = outputLines[i] || "";
      lines.push(this.composeLine(leftContent, leftPaneWidth, rightContent, rightPaneWidth));
    }

    // Render divider between task progress and activity log
    lines.push(this.renderLeftDivider(leftPaneWidth, "Activity", rightPaneWidth));

    // Render the activity log section (bottom-left) with output continuation
    for (let i = 0; i < activityLogHeight; i++) {
      const leftContent = activityLines[i] || "";
      const outputIndex = taskProgressHeight + 1 + i; // +1 for divider
      const rightContent = outputLines[outputIndex] || "";
      lines.push(this.composeLine(leftContent, leftPaneWidth, rightContent, rightPaneWidth));
    }

    // Footer
    lines.push(this.renderSeparator(width, "─"));
    lines.push(this.renderFooter(width));

    return lines;
  }

  private renderHeader(width: number): string {
    const title = this.planName
      ? `Plan: ${this.planName} (${this.planId || ""})`
      : "Plan Execution";
    const status = `[${this.statusMessage}]`;
    const padding = width - title.length - status.length - 4;
    const padStr = padding > 0 ? " ".repeat(padding) : " ";
    return truncateToWidth(`  ${title}${padStr}${status}`, width);
  }

  private renderSeparator(width: number, char: string): string {
    return truncateToWidth(char.repeat(width), width);
  }

  private renderPaneHeader(
    leftWidth: number,
    leftTitle: string,
    rightWidth: number,
    rightTitle: string
  ): string {
    const leftHeader = ` ${leftTitle} `.padEnd(leftWidth - 1, "─");
    const rightHeader = `┬─ ${rightTitle} `.padEnd(rightWidth, "─");
    return truncateToWidth(leftHeader + rightHeader, leftWidth + rightWidth + BORDER_WIDTH);
  }

  private renderLeftDivider(
    leftWidth: number,
    sectionTitle: string,
    rightWidth: number
  ): string {
    const leftPart = `├─ ${sectionTitle} `.padEnd(leftWidth - 1, "─");
    const rightPart = "│" + " ".repeat(rightWidth);
    return truncateToWidth(leftPart + rightPart, leftWidth + rightWidth + BORDER_WIDTH);
  }

  private composeLine(
    leftContent: string,
    leftWidth: number,
    rightContent: string,
    rightWidth: number
  ): string {
    // Pad left content
    const leftPadded = truncateToWidth(` ${leftContent}`, leftWidth - 1, "", true);
    // Add separator and right content
    const rightPadded = truncateToWidth(` ${rightContent}`, rightWidth, "", true);
    return `${leftPadded}│${rightPadded}`;
  }

  private renderFooter(width: number): string {
    const controls = this.isRunning
      ? "[Ctrl+C] Stop execution"
      : "[Esc] Back  [r] Retry  [Ctrl+C] Quit";
    return truncateToWidth(`  ${controls}`, width);
  }

  handleInput(data: string): void {
    // Ctrl+C during execution triggers abort
    if (matchesKey(data, Key.ctrl("c")) && this.isRunning) {
      this.abortExecution();
      return;
    }

    if (matchesKey(data, Key.escape) && !this.isRunning) {
      this.app.navigate("home");
    }

    // 'r' to retry - placeholder for future implementation
    if (matchesKey(data, "r") && !this.isRunning) {
      // Retry logic would go here
      this.setStatus("Retry not yet implemented");
      this.app.requestRender();
    }
  }

  invalidate(): void {
    // Clear any cached render state
    this.taskProgress.setTasks(this.plan?.tasks || []);
    this.taskProgress.setCurrentTask(this.currentTaskId);
  }
}
