/**
 * Plan execution view
 * Shows real-time progress of plan/task execution
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";

export interface RunContext {
  planId?: string;
  planName?: string;
}

export class RunView implements RafaView {
  private app: RafaApp;
  private planId?: string;
  private planName?: string;

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(context?: unknown): void {
    if (context && typeof context === "object") {
      const ctx = context as RunContext;
      this.planId = ctx.planId;
      this.planName = ctx.planName;
    }
  }

  render(width: number): string[] {
    const lines: string[] = [];

    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  Plan Execution", width));
    lines.push(truncateToWidth("  ─".repeat(Math.min(30, width - 2)), width));
    lines.push(truncateToWidth("", width));

    if (this.planName) {
      lines.push(truncateToWidth(`  Plan: ${this.planName}`, width));
      if (this.planId) {
        lines.push(truncateToWidth(`  ID: ${this.planId}`, width));
      }
      lines.push(truncateToWidth("", width));
    }

    lines.push(truncateToWidth("  [Run View - placeholder]", width));
    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  This view will show:", width));
    lines.push(truncateToWidth("  - Task progress list (left)", width));
    lines.push(truncateToWidth("  - Activity log (center)", width));
    lines.push(truncateToWidth("  - Output stream (right)", width));
    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  [Esc] Back to Home  [Ctrl+C] Stop execution", width));

    return lines;
  }

  handleInput(data: string): void {
    if (matchesKey(data, Key.escape)) {
      this.app.navigate("home");
    }
  }

  invalidate(): void {
    // No cached state to invalidate
  }
}
