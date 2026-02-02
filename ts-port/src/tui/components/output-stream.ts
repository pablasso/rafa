/**
 * Claude output display component with streaming support
 *
 * Displays Claude's text output with:
 * - Real-time streaming (text appended incrementally)
 * - Auto-scroll (always shows most recent output)
 * - History truncation (prevents memory issues)
 * - Line wrapping for display width
 */

import { truncateToWidth } from "@mariozechner/pi-tui";
import type { ClaudeEvent, TextEventData } from "../../core/stream-parser.js";

// Maximum characters to keep in history
const MAX_CONTENT_LENGTH = 50000;

// Number of characters to remove when truncating
const TRUNCATE_AMOUNT = 10000;

// Maximum distance to search for a line break after truncation
const MAX_LINE_BOUNDARY_SEARCH = 100;

export class OutputStreamComponent {
  private content: string = "";
  private visibleLines: number = 20; // Default, updated on render

  /**
   * Process a Claude event and extract text output
   */
  processEvent(claudeEvent: ClaudeEvent): void {
    if (claudeEvent.type === "text") {
      const data = claudeEvent.data as TextEventData;
      if (data.text) {
        this.append(data.text);
      }
    }
  }

  /**
   * Append text to the output stream
   */
  append(text: string): void {
    this.content += text;

    // Truncate history if too large
    if (this.content.length > MAX_CONTENT_LENGTH) {
      // Remove from the beginning, keeping recent output
      this.content = this.content.substring(TRUNCATE_AMOUNT);

      // Find the first complete line to avoid partial line at start
      const firstNewline = this.content.indexOf("\n");
      if (firstNewline !== -1 && firstNewline < MAX_LINE_BOUNDARY_SEARCH) {
        this.content = this.content.substring(firstNewline + 1);
      }
    }
  }

  /**
   * Clear the output stream
   */
  clear(): void {
    this.content = "";
  }

  /**
   * Get the raw content (for testing)
   */
  getContent(): string {
    return this.content;
  }

  /**
   * Set the number of visible lines (for scrolling)
   */
  setVisibleLines(lines: number): void {
    this.visibleLines = Math.max(1, lines);
  }

  /**
   * Wrap text to fit within width, respecting word boundaries when possible
   */
  private wrapText(text: string, width: number): string[] {
    if (width <= 0) return [];

    const lines: string[] = [];
    const inputLines = text.split("\n");

    for (const line of inputLines) {
      if (line.length === 0) {
        lines.push("");
        continue;
      }

      if (line.length <= width) {
        lines.push(line);
        continue;
      }

      // Wrap long lines
      let remaining = line;
      while (remaining.length > 0) {
        if (remaining.length <= width) {
          lines.push(remaining);
          break;
        }

        // Try to break at a word boundary
        let breakPoint = width;
        const spaceIndex = remaining.lastIndexOf(" ", width);
        if (spaceIndex > width * 0.5) {
          // Found a reasonable break point
          breakPoint = spaceIndex;
        }

        lines.push(remaining.substring(0, breakPoint));
        remaining = remaining.substring(breakPoint).trimStart();
      }
    }

    return lines;
  }

  /**
   * Render the output stream with auto-scroll
   * Returns the most recent lines that fit in the visible area
   */
  render(width: number): string[] {
    if (this.content.length === 0) {
      return [truncateToWidth("  (waiting for output...)", width)];
    }

    // Wrap content to width
    const wrappedLines = this.wrapText(this.content, width);

    // Auto-scroll: show the most recent lines
    const startIndex = Math.max(0, wrappedLines.length - this.visibleLines);
    const visibleContent = wrappedLines.slice(startIndex);

    // Ensure all lines are truncated to width (safety)
    return visibleContent.map((line) => truncateToWidth(line, width));
  }

  /**
   * Render with a specific number of lines (for layout management)
   */
  renderWithHeight(width: number, height: number): string[] {
    this.setVisibleLines(height);
    return this.render(width);
  }
}
