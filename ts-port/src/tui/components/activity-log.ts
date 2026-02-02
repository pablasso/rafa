/**
 * Tool use timeline component with tree-style formatting
 *
 * Displays a real-time log of Claude's tool usage with a tree structure:
 * ├─ Read auth.go
 * │  └─ done 2s
 * ├─ Spawned Explore
 * │  └─ running
 * └─ Write login.go
 *    └─ done 1s
 */

import { truncateToWidth } from "@mariozechner/pi-tui";
import type {
  ClaudeEvent,
  ToolUseEventData,
  ToolResultEventData,
} from "../../core/stream-parser.js";

export interface ActivityEvent {
  id: string; // tool_use ID for matching results
  name: string; // Tool name (Read, Write, etc.)
  target: string; // File path or description
  status: "running" | "done" | "error";
  startTime: number; // timestamp for duration calculation
  duration?: number; // ms, set when done
}

// Maximum number of events to keep in history to prevent memory issues
const MAX_EVENTS = 100;

// Maximum path length before shortening
const MAX_PATH_LENGTH = 40;

export class ActivityLogComponent {
  private events: ActivityEvent[] = [];
  private pendingToolUses: Map<string, number> = new Map(); // id -> event index

  /**
   * Process a Claude event and update the activity log
   */
  processEvent(claudeEvent: ClaudeEvent): void {
    if (claudeEvent.type === "tool_use") {
      const data = claudeEvent.data as ToolUseEventData;
      this.addToolUse(data.id, data.name, data.input);
    } else if (claudeEvent.type === "tool_result") {
      const data = claudeEvent.data as ToolResultEventData;
      this.completeToolUse(data.toolUseId, data.isError);
    }
  }

  /**
   * Add a new tool use event (when tool starts)
   */
  private addToolUse(
    id: string,
    name: string,
    input: Record<string, unknown>,
  ): void {
    // Extract a meaningful target from the input
    const target = this.extractTarget(name, input);

    const event: ActivityEvent = {
      id,
      name,
      target,
      status: "running",
      startTime: Date.now(),
    };

    // Trim history if needed
    if (this.events.length >= MAX_EVENTS) {
      // Remove oldest events, keeping recent ones
      const removeCount = this.events.length - MAX_EVENTS + 1;
      this.events.splice(0, removeCount);

      // Update pending map indices
      this.rebuildPendingMap();
    }

    this.events.push(event);
    this.pendingToolUses.set(id, this.events.length - 1);
  }

  /**
   * Mark a tool use as complete
   */
  private completeToolUse(toolUseId: string, isError: boolean): void {
    const index = this.pendingToolUses.get(toolUseId);
    if (index !== undefined && this.events[index]) {
      const event = this.events[index];
      event.status = isError ? "error" : "done";
      event.duration = Date.now() - event.startTime;
      this.pendingToolUses.delete(toolUseId);
    }
  }

  /**
   * Extract a display-friendly target from tool input
   */
  private extractTarget(
    toolName: string,
    input: Record<string, unknown>,
  ): string {
    // Common file path parameters
    if (input.file_path && typeof input.file_path === "string") {
      return this.shortenPath(input.file_path);
    }
    if (input.path && typeof input.path === "string") {
      return this.shortenPath(input.path);
    }

    // Command for Bash
    if (toolName.toLowerCase() === "bash" && input.command) {
      const cmd = String(input.command);
      // Show first 30 chars of command
      return cmd.length > 30 ? cmd.substring(0, 30) + "…" : cmd;
    }

    // Pattern for Glob/Grep
    if (input.pattern && typeof input.pattern === "string") {
      return input.pattern;
    }

    // Task/Agent spawning
    if (input.description && typeof input.description === "string") {
      return String(input.description);
    }

    // URL for web fetch
    if (input.url && typeof input.url === "string") {
      return String(input.url);
    }

    return "";
  }

  /**
   * Shorten a file path for display
   */
  private shortenPath(path: string): string {
    // Get just the filename or last path segment
    const parts = path.split("/");
    const filename = parts[parts.length - 1];

    // If the path is short enough, show it
    if (path.length <= MAX_PATH_LENGTH) {
      return path;
    }

    // Show parent/filename if needed
    if (parts.length >= 2) {
      const parent = parts[parts.length - 2];
      return `${parent}/${filename}`;
    }

    return filename;
  }

  /**
   * Rebuild the pending map after trimming events
   */
  private rebuildPendingMap(): void {
    this.pendingToolUses.clear();
    for (let i = 0; i < this.events.length; i++) {
      if (this.events[i].status === "running") {
        this.pendingToolUses.set(this.events[i].id, i);
      }
    }
  }

  /**
   * Clear all events
   */
  clear(): void {
    this.events = [];
    this.pendingToolUses.clear();
  }

  /**
   * Get all events (for testing)
   */
  getEvents(): ActivityEvent[] {
    return this.events;
  }

  /**
   * Add a raw event directly (for testing or manual events)
   */
  addEvent(event: ActivityEvent): void {
    if (this.events.length >= MAX_EVENTS) {
      this.events.splice(0, 1);
      this.rebuildPendingMap();
    }
    this.events.push(event);
    if (event.status === "running") {
      this.pendingToolUses.set(event.id, this.events.length - 1);
    }
  }

  /**
   * Add a custom status event (not a tool use)
   * Used for system messages like "Session started", "Auto-triggering review..."
   */
  addCustomEvent(message: string): void {
    const event: ActivityEvent = {
      id: `custom-${Date.now()}`,
      name: message,
      target: "",
      status: "done",
      startTime: Date.now(),
      duration: 0,
    };
    this.addEvent(event);
  }

  /**
   * Render the activity log as tree-style formatted lines
   */
  render(width: number): string[] {
    const lines: string[] = [];

    if (this.events.length === 0) {
      lines.push(truncateToWidth("  (no activity)", width));
      return lines;
    }

    for (let i = 0; i < this.events.length; i++) {
      const event = this.events[i];
      const isLast = i === this.events.length - 1;

      // Main tool line
      const prefix = isLast ? "└─" : "├─";
      const statusIcon = this.getStatusIcon(event.status);
      const mainLine = `${prefix} ${statusIcon} ${event.name}`;
      lines.push(truncateToWidth(mainLine, width));

      // Detail line (target/path)
      if (event.target) {
        const detailPrefix = isLast ? "   " : "│  ";
        const detailLine = `${detailPrefix}${event.target}`;
        lines.push(truncateToWidth(detailLine, width));
      }

      // Status line with duration
      const statusPrefix = isLast ? "   └─" : "│  └─";
      const statusText = this.formatStatus(event);
      const statusLine = `${statusPrefix} ${statusText}`;
      lines.push(truncateToWidth(statusLine, width));
    }

    return lines;
  }

  /**
   * Get status icon for display
   */
  private getStatusIcon(status: ActivityEvent["status"]): string {
    switch (status) {
      case "running":
        return "◐"; // Half-filled circle for in-progress
      case "done":
        return "✓";
      case "error":
        return "✗";
    }
  }

  /**
   * Format status with duration if available
   */
  private formatStatus(event: ActivityEvent): string {
    if (event.status === "running") {
      return "running";
    }

    const statusText = event.status === "error" ? "error" : "done";
    if (event.duration !== undefined) {
      const durationStr = this.formatDuration(event.duration);
      return `${statusText} ${durationStr}`;
    }
    return statusText;
  }

  /**
   * Format duration in human-readable form
   */
  private formatDuration(ms: number): string {
    if (ms < 1000) {
      return `${ms}ms`;
    }
    const seconds = Math.round(ms / 1000);
    if (seconds < 60) {
      return `${seconds}s`;
    }
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    return `${minutes}m${remainingSeconds}s`;
  }
}
