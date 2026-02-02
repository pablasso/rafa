/**
 * PRD/Design conversation view
 * Handles interactive conversations with Claude for creating PRDs and designs
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";

export interface ConversationContext {
  phase: "prd" | "design";
  sourceFile?: string;
}

export class ConversationView implements RafaView {
  private app: RafaApp;
  private phase: "prd" | "design" = "prd";
  private sourceFile?: string;

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(context?: unknown): void {
    if (context && typeof context === "object") {
      const ctx = context as ConversationContext;
      this.phase = ctx.phase || "prd";
      this.sourceFile = ctx.sourceFile;
    }
  }

  render(width: number): string[] {
    const lines: string[] = [];
    const title = this.phase === "prd" ? "PRD Creation" : "Technical Design";

    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth(`  ${title}`, width));
    lines.push(truncateToWidth("  ─".repeat(Math.min(30, width - 2)), width));
    lines.push(truncateToWidth("", width));

    if (this.sourceFile) {
      lines.push(truncateToWidth(`  Source: ${this.sourceFile}`, width));
      lines.push(truncateToWidth("", width));
    }

    lines.push(truncateToWidth("  [Conversation View - placeholder]", width));
    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  This view will show:", width));
    lines.push(truncateToWidth("  - Activity log (left pane)", width));
    lines.push(truncateToWidth("  - Claude's response (right pane)", width));
    lines.push(truncateToWidth("  - Input editor (bottom)", width));
    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  [Esc] Back to Home", width));

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
