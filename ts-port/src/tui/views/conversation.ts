/**
 * PRD/Design conversation view
 *
 * Handles interactive conversations with Claude for creating PRDs and designs.
 *
 * Layout:
 * ┌────────────────────────┬───────────────────────────────────────────┐
 * │  Activity Log (25%)    │  Output Pane (75%)                        │
 * │  ├─ Read file.ts       │  [Claude's response streams here...]      │
 * │  └─ Write file.ts      │                                           │
 * │                        │                                           │
 * ├────────────────────────┴───────────────────────────────────────────┤
 * │  Input Editor                                                      │
 * │  │ Type to revise, or press 'a' to approve...                      │
 * ├────────────────────────────────────────────────────────────────────┤
 * │  [a] Approve [c] Cancel [Esc] Back                                 │
 * └────────────────────────────────────────────────────────────────────┘
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";
import { ActivityLogComponent } from "../components/activity-log.js";
import { OutputStreamComponent } from "../components/output-stream.js";
import { MultilineEditorComponent } from "../components/multiline-editor.js";
import type { ClaudeEvent } from "../../core/stream-parser.js";

// Layout constants
const LEFT_PANE_RATIO = 0.25; // Activity log is 25% of width
const MIN_LEFT_PANE_WIDTH = 25;
const MIN_RIGHT_PANE_WIDTH = 40;
const BORDER_WIDTH = 1; // Vertical separator
const EDITOR_HEIGHT = 5; // Height of input editor

export interface ConversationContext {
  phase: "prd" | "design";
  sourceFile?: string;
}

export class ConversationView implements RafaView {
  private app: RafaApp;
  private phase: "prd" | "design" = "prd";
  private sourceFile?: string;

  // State
  private isClaudeResponding = false;
  private editorFocused = true;

  // Sub-components
  private activityLog: ActivityLogComponent;
  private outputPane: OutputStreamComponent;
  private inputEditor: MultilineEditorComponent;

  constructor(app: RafaApp) {
    this.app = app;
    this.activityLog = new ActivityLogComponent();
    this.outputPane = new OutputStreamComponent();
    this.inputEditor = new MultilineEditorComponent({
      placeholder: "Type to revise, or press 'a' to approve...",
      maxHeight: EDITOR_HEIGHT,
    });
    this.inputEditor.setFocused(true);
  }

  activate(context?: unknown): void {
    if (context && typeof context === "object") {
      const ctx = context as ConversationContext;
      this.phase = ctx.phase || "prd";
      this.sourceFile = ctx.sourceFile;
    }

    // Reset state for fresh conversation
    this.activityLog.clear();
    this.outputPane.clear();
    this.inputEditor.clear();
    this.isClaudeResponding = false;
    this.editorFocused = true;
    this.inputEditor.setFocused(true);
    this.inputEditor.setDisabled(false);
  }

  /**
   * Process a Claude event - updates activity log and output pane
   */
  processEvent(event: ClaudeEvent): void {
    this.activityLog.processEvent(event);
    this.outputPane.processEvent(event);
  }

  /**
   * Set whether Claude is currently responding (disables input)
   */
  setClaudeResponding(responding: boolean): void {
    this.isClaudeResponding = responding;
    this.inputEditor.setDisabled(responding);
  }

  /**
   * Check if Claude is responding
   */
  getIsClaudeResponding(): boolean {
    return this.isClaudeResponding;
  }

  /**
   * Get access to activity log (for external control)
   */
  getActivityLog(): ActivityLogComponent {
    return this.activityLog;
  }

  /**
   * Get access to output pane (for external control)
   */
  getOutputPane(): OutputStreamComponent {
    return this.outputPane;
  }

  /**
   * Request a TUI re-render
   */
  requestRender(): void {
    this.app.requestRender();
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

    // Calculate content height
    // Total height minus header, editor, and footer
    const contentHeight = 15; // Reasonable default
    const mainPaneHeight = contentHeight;

    // Render sub-components
    const activityLines = this.activityLog.render(leftPaneWidth - 2);
    const outputLines = this.outputPane.renderWithHeight(
      rightPaneWidth - 2,
      mainPaneHeight
    );

    // Pad activity lines to fill
    while (activityLines.length < mainPaneHeight) {
      activityLines.push("");
    }

    // Pad output lines to fill
    while (outputLines.length < mainPaneHeight) {
      outputLines.push("");
    }

    // Render pane headers
    lines.push(
      this.renderPaneHeader(leftPaneWidth, "Activity", rightPaneWidth, "Output")
    );

    // Render split panes
    for (let i = 0; i < mainPaneHeight; i++) {
      const leftContent = activityLines[i] || "";
      const rightContent = outputLines[i] || "";
      lines.push(
        this.composeLine(leftContent, leftPaneWidth, rightContent, rightPaneWidth)
      );
    }

    // Separator before editor
    lines.push(this.renderSeparator(width, "─"));

    // Input editor
    lines.push(this.renderEditorHeader(width));
    const editorLines = this.inputEditor.render(width);
    lines.push(...editorLines);

    // Footer
    lines.push(this.renderSeparator(width, "─"));
    lines.push(this.renderFooter(width));

    return lines;
  }

  private renderHeader(width: number): string {
    const title = this.phase === "prd" ? "PRD Creation" : "Technical Design";
    let subtitle = "";
    if (this.sourceFile) {
      subtitle = ` - ${this.sourceFile}`;
    }
    const status = this.isClaudeResponding ? "[Thinking...]" : "[Ready]";
    const titlePart = `  ${title}${subtitle}`;
    const padding = width - titlePart.length - status.length - 2;
    const padStr = padding > 0 ? " ".repeat(padding) : " ";
    return truncateToWidth(`${titlePart}${padStr}${status}`, width);
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
    return truncateToWidth(
      leftHeader + rightHeader,
      leftWidth + rightWidth + BORDER_WIDTH
    );
  }

  private composeLine(
    leftContent: string,
    leftWidth: number,
    rightContent: string,
    rightWidth: number
  ): string {
    const leftPadded = truncateToWidth(` ${leftContent}`, leftWidth - 1, "", true);
    const rightPadded = truncateToWidth(` ${rightContent}`, rightWidth, "", true);
    return `${leftPadded}│${rightPadded}`;
  }

  private renderEditorHeader(width: number): string {
    const label = this.isClaudeResponding
      ? " Input (disabled while Claude responds)"
      : " Input";
    return truncateToWidth(label, width);
  }

  private renderFooter(width: number): string {
    let controls: string;
    if (this.isClaudeResponding) {
      controls = "[Ctrl+C] Stop  [Esc] Back";
    } else if (this.editorFocused && !this.inputEditor.isEmpty()) {
      controls = "[Enter] Send  [a] Approve  [c] Cancel  [Esc] Back";
    } else {
      controls = "[a] Approve  [c] Cancel  [Esc] Back";
    }
    return truncateToWidth(`  ${controls}`, width);
  }

  handleInput(data: string): void {
    // Always handle Escape - go back
    if (matchesKey(data, Key.escape)) {
      this.app.navigate("home");
      return;
    }

    // Ctrl+C during Claude response - abort
    if (matchesKey(data, Key.ctrl("c")) && this.isClaudeResponding) {
      // TODO: Implement abort callback similar to RunView
      return;
    }

    // When Claude is responding, only escape and Ctrl+C work
    if (this.isClaudeResponding) {
      return;
    }

    // Handle [a] Approve hotkey - only when editor not focused or empty
    if (matchesKey(data, "a") && (!this.editorFocused || this.inputEditor.isEmpty())) {
      this.handleApprove();
      return;
    }

    // Handle [c] Cancel hotkey - only when editor not focused or empty
    if (matchesKey(data, "c") && (!this.editorFocused || this.inputEditor.isEmpty())) {
      this.handleCancel();
      return;
    }

    // Handle Alt+Enter to submit from editor
    if (data === "\x1b\r" || data === "\x1b\n") {
      const text = this.inputEditor.getText().trim();
      if (text) {
        this.handleSendMessage(text);
      }
      return;
    }

    // Forward to editor
    this.inputEditor.handleInput(data);
    this.app.requestRender();
  }

  /**
   * Handle approve action
   */
  private handleApprove(): void {
    // TODO: Implement approve logic
    // This would finalize the PRD/Design and proceed
    this.app.navigate("home");
  }

  /**
   * Handle cancel action
   */
  private handleCancel(): void {
    // Go back without saving
    this.app.navigate("home");
  }

  /**
   * Handle sending a message to Claude
   */
  private handleSendMessage(text: string): void {
    // TODO: Implement message sending with Claude
    // This would:
    // 1. Add message to activity log
    // 2. Set isClaudeResponding = true
    // 3. Run claude with --resume
    // 4. Stream response to outputPane
    // 5. Set isClaudeResponding = false when done

    // For now, just clear the editor
    this.inputEditor.clear();
    this.app.requestRender();
  }

  invalidate(): void {
    // Reset any cached state
  }
}
