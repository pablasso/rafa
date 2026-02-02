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
import {
  runConversation,
  createAbortController,
  type ClaudeAbortController,
  type ClaudeEvent,
} from "../../core/claude-runner.js";
import type { TextEventData, DoneEventData } from "../../core/stream-parser.js";
import {
  createSessionFile,
  updateSessionMetadata,
  appendUserMessage,
} from "../../storage/sessions.js";
import { createSession } from "../../core/session.js";
import { generateId } from "../../utils/id.js";

// Layout constants
const LEFT_PANE_RATIO = 0.25; // Activity log is 25% of width
const MIN_LEFT_PANE_WIDTH = 25;
const MIN_RIGHT_PANE_WIDTH = 40;
const BORDER_WIDTH = 1; // Vertical separator
const EDITOR_HEIGHT = 5; // Height of input editor

// Conversation phase states
type ConversationPhase =
  | "initial" // Just started, about to send /prd prompt
  | "drafting" // Claude is working on the PRD
  | "reviewing" // Auto-review triggered, Claude reviewing
  | "awaiting_input" // Waiting for user to revise, approve, or cancel
  | "saving"; // User approved, saving the PRD

export interface ConversationContext {
  phase: "prd" | "design";
  sourceFile?: string;
  name?: string;
}

// Export PrdContext as alias for use in CLI
export type PrdContext = ConversationContext;

export class ConversationView implements RafaView {
  private app: RafaApp;
  private phase: "prd" | "design" = "prd";
  private sourceFile?: string;
  private documentName?: string;

  // Conversation state
  private conversationPhase: ConversationPhase = "initial";
  private isClaudeResponding = false;
  private claudeSessionId?: string;
  private sessionPath?: string;
  private abortController?: ClaudeAbortController;
  private accumulatedText = "";
  private hasDraftBeenGenerated = false;
  private hasReviewBeenTriggered = false;

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
      this.documentName = ctx.name;
    }

    // Reset state for fresh conversation
    this.activityLog.clear();
    this.outputPane.clear();
    this.inputEditor.clear();
    this.isClaudeResponding = false;
    this.claudeSessionId = undefined;
    this.sessionPath = undefined;
    this.abortController = undefined;
    this.accumulatedText = "";
    this.hasDraftBeenGenerated = false;
    this.hasReviewBeenTriggered = false;
    this.conversationPhase = "initial";
    this.inputEditor.setFocused(true);
    this.inputEditor.setDisabled(false);

    // Start the conversation automatically
    this.startConversation();
  }

  /**
   * Start the PRD/Design conversation
   */
  private async startConversation(): Promise<void> {
    // Generate a name if not provided
    if (!this.documentName) {
      this.documentName = `draft-${generateId()}`;
    }

    // Create session file
    const session = createSession(this.phase, this.documentName);
    this.sessionPath = await createSessionFile(session, this.documentName);

    // Add activity event
    this.activityLog.addCustomEvent("Session started");
    this.conversationPhase = "drafting";
    this.app.requestRender();

    // Build initial prompt
    const initialPrompt = this.buildInitialPrompt();

    // Send to Claude
    await this.sendToClaude(initialPrompt);
  }

  /**
   * Build the initial prompt for PRD creation
   */
  private buildInitialPrompt(): string {
    if (this.phase === "prd") {
      return `Use the /prd skill to help me create a Product Requirements Document (PRD).

Start by asking me about the problem I want to solve and the users who will benefit from the solution. Guide me through the PRD creation process step by step.`;
    } else {
      let prompt = `Use the /technical-design skill to help me create a Technical Design document.`;
      if (this.sourceFile) {
        prompt += `\n\nBase the design on this PRD: ${this.sourceFile}`;
      } else {
        prompt += `\n\nStart by asking me about the technical approach I want to take.`;
      }
      return prompt;
    }
  }

  /**
   * Build the review prompt
   */
  private buildReviewPrompt(): string {
    if (this.phase === "prd") {
      return `Now use the /prd-review skill to review the PRD you just created. Identify any issues that should be addressed and suggest improvements. After the review, decide which findings are worth addressing vs acceptable trade-offs, and make any necessary updates to the PRD.`;
    } else {
      return `Now use the /technical-design-review skill to review the technical design you just created. Identify any issues that should be addressed and suggest improvements. After the review, decide which findings are worth addressing vs acceptable trade-offs, and make any necessary updates to the design.`;
    }
  }

  /**
   * Send a message to Claude and stream the response
   */
  private async sendToClaude(prompt: string): Promise<void> {
    this.isClaudeResponding = true;
    this.inputEditor.setDisabled(true);
    this.accumulatedText = "";
    this.abortController = createAbortController();
    this.app.requestRender();

    try {
      // Log the user message to session
      if (this.sessionPath) {
        await appendUserMessage(this.sessionPath, prompt);
      }

      const sessionId = await runConversation({
        prompt,
        sessionId: this.claudeSessionId,
        onEvent: (event) => this.handleClaudeEvent(event),
        abortController: this.abortController,
      });

      // Store session ID for resume
      this.claudeSessionId = sessionId;
      if (this.sessionPath) {
        await updateSessionMetadata(this.sessionPath, {
          claudeSessionId: sessionId,
        });
      }

      // Check if we should trigger review
      await this.checkAndTriggerReview();
    } catch (error) {
      if ((error as Error).name === "ClaudeAbortError") {
        this.activityLog.addCustomEvent("Aborted by user");
      } else {
        this.activityLog.addCustomEvent(`Error: ${(error as Error).message}`);
      }
    } finally {
      this.isClaudeResponding = false;
      this.inputEditor.setDisabled(false);
      this.abortController = undefined;
      this.conversationPhase = "awaiting_input";
      this.app.requestRender();
    }
  }

  /**
   * Handle a Claude event - updates activity log and output pane
   */
  private handleClaudeEvent(event: ClaudeEvent): void {
    this.activityLog.processEvent(event);
    this.outputPane.processEvent(event);

    // Accumulate text for draft detection
    if (event.type === "text") {
      const textData = event.data as TextEventData;
      this.accumulatedText += textData.text;
    }

    // Check for done event to detect draft completion
    if (event.type === "done") {
      const doneData = event.data as DoneEventData;
      if (doneData.result && !this.hasDraftBeenGenerated) {
        this.hasDraftBeenGenerated = true;
      }
    }

    this.app.requestRender();
  }

  /**
   * Check if we should trigger the review phase
   */
  private async checkAndTriggerReview(): Promise<void> {
    // Only trigger review once, after initial drafting
    if (this.hasReviewBeenTriggered) {
      return;
    }

    // Check if a draft has been generated by looking for common PRD markers
    const hasPrdContent = this.detectPrdDraft();

    if (hasPrdContent && this.conversationPhase !== "reviewing") {
      this.hasReviewBeenTriggered = true;
      this.conversationPhase = "reviewing";
      this.activityLog.addCustomEvent("Auto-triggering review...");
      this.app.requestRender();

      // Brief delay to let user see the draft before review
      await new Promise((resolve) => setTimeout(resolve, 500));

      // Trigger review
      await this.sendToClaude(this.buildReviewPrompt());
    }
  }

  /**
   * Detect if a PRD draft has been generated based on content markers
   */
  private detectPrdDraft(): boolean {
    const text = this.accumulatedText.toLowerCase();

    // Look for common PRD section headers
    const prdMarkers = [
      "## problem",
      "## users",
      "## requirements",
      "## user journey",
      "## success metrics",
      "# prd:",
      "# product requirements",
    ];

    // Check if at least 2 markers are present (indicating a draft structure)
    const markerCount = prdMarkers.filter((marker) =>
      text.includes(marker),
    ).length;
    return markerCount >= 2;
  }

  /**
   * Process a Claude event - updates activity log and output pane
   * Public API for external control
   */
  processEvent(event: ClaudeEvent): void {
    this.handleClaudeEvent(event);
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
      Math.floor(width * LEFT_PANE_RATIO),
    );
    const rightPaneWidth = Math.max(
      MIN_RIGHT_PANE_WIDTH,
      width - leftPaneWidth - BORDER_WIDTH,
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
      mainPaneHeight,
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
      this.renderPaneHeader(
        leftPaneWidth,
        "Activity",
        rightPaneWidth,
        "Output",
      ),
    );

    // Render split panes
    for (let i = 0; i < mainPaneHeight; i++) {
      const leftContent = activityLines[i] || "";
      const rightContent = outputLines[i] || "";
      lines.push(
        this.composeLine(
          leftContent,
          leftPaneWidth,
          rightContent,
          rightPaneWidth,
        ),
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
    if (this.documentName) {
      subtitle = ` - ${this.documentName}`;
    }
    const status = this.getStatusText();
    const titlePart = `  ${title}${subtitle}`;
    const padding = width - titlePart.length - status.length - 2;
    const padStr = padding > 0 ? " ".repeat(padding) : " ";
    return truncateToWidth(`${titlePart}${padStr}${status}`, width);
  }

  private getStatusText(): string {
    switch (this.conversationPhase) {
      case "initial":
        return "[Starting...]";
      case "drafting":
        return "[Drafting...]";
      case "reviewing":
        return "[Reviewing...]";
      case "awaiting_input":
        return "[Ready]";
      case "saving":
        return "[Saving...]";
      default:
        return this.isClaudeResponding ? "[Thinking...]" : "[Ready]";
    }
  }

  private renderSeparator(width: number, char: string): string {
    return truncateToWidth(char.repeat(width), width);
  }

  private renderPaneHeader(
    leftWidth: number,
    leftTitle: string,
    rightWidth: number,
    rightTitle: string,
  ): string {
    const leftHeader = ` ${leftTitle} `.padEnd(leftWidth - 1, "─");
    const rightHeader = `┬─ ${rightTitle} `.padEnd(rightWidth, "─");
    return truncateToWidth(
      leftHeader + rightHeader,
      leftWidth + rightWidth + BORDER_WIDTH,
    );
  }

  private composeLine(
    leftContent: string,
    leftWidth: number,
    rightContent: string,
    rightWidth: number,
  ): string {
    const leftPadded = truncateToWidth(
      ` ${leftContent}`,
      leftWidth - 1,
      "",
      true,
    );
    const rightPadded = truncateToWidth(
      ` ${rightContent}`,
      rightWidth,
      "",
      true,
    );
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
    } else if (!this.inputEditor.isEmpty()) {
      controls = "[Alt+Enter] Send  [a] Approve  [c] Cancel  [Esc] Back";
    } else {
      controls = "[a] Approve  [c] Cancel  [Esc] Back";
    }
    return truncateToWidth(`  ${controls}`, width);
  }

  handleInput(data: string): void {
    // Always handle Escape - go back
    if (matchesKey(data, Key.escape)) {
      if (this.abortController) {
        this.abortController.abort();
      }
      this.app.navigate("home");
      return;
    }

    // Ctrl+C during Claude response - abort
    if (matchesKey(data, Key.ctrl("c")) && this.isClaudeResponding) {
      if (this.abortController) {
        this.abortController.abort();
        this.activityLog.addCustomEvent("Aborting...");
        this.app.requestRender();
      }
      return;
    }

    // When Claude is responding, only escape and Ctrl+C work
    if (this.isClaudeResponding) {
      return;
    }

    // Handle [a] Approve hotkey - only when editor is empty
    if (matchesKey(data, "a") && this.inputEditor.isEmpty()) {
      this.handleApprove();
      return;
    }

    // Handle [c] Cancel hotkey - only when editor is empty
    if (matchesKey(data, "c") && this.inputEditor.isEmpty()) {
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
   * Handle approve action - save the PRD and exit
   */
  private async handleApprove(): Promise<void> {
    this.conversationPhase = "saving";
    this.app.requestRender();

    try {
      // Ask Claude to save the PRD
      const savePath = await this.savePrdDocument();

      if (savePath) {
        this.activityLog.addCustomEvent(`Saved to ${savePath}`);
        this.app.requestRender();

        // Brief delay to show success message
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    } catch (error) {
      this.activityLog.addCustomEvent(
        `Error saving: ${(error as Error).message}`,
      );
      this.conversationPhase = "awaiting_input";
      this.app.requestRender();
      return;
    }

    // Go back to home
    this.app.navigate("home");
  }

  /**
   * Save the PRD document to docs/prds/<name>.md
   */
  private async savePrdDocument(): Promise<string | null> {
    // Ask Claude to save the document
    const savePrompt = this.buildSavePrompt();

    this.activityLog.addCustomEvent("Saving document...");
    this.app.requestRender();

    let savedPath: string | null = null;

    try {
      await runConversation({
        prompt: savePrompt,
        sessionId: this.claudeSessionId,
        onEvent: (event) => {
          this.handleClaudeEvent(event);

          // Try to extract saved path from output
          if (event.type === "text") {
            const textData = event.data as TextEventData;
            const match = textData.text.match(
              /docs\/(prds|designs)\/[a-zA-Z0-9-_]+\.md/,
            );
            if (match) {
              savedPath = match[0];
            }
          }
        },
      });

      return savedPath;
    } catch (error) {
      throw new Error(`Failed to save document: ${(error as Error).message}`);
    }
  }

  /**
   * Build the prompt to save the document
   */
  private buildSavePrompt(): string {
    const dir = this.phase === "prd" ? "docs/prds" : "docs/designs";
    const suggestedName = this.documentName || "document";

    return `Save the ${this.phase === "prd" ? "PRD" : "technical design"} document we just created to ${dir}/${suggestedName}.md.

Create the directory if it doesn't exist. If a file with that name already exists, append a number to make it unique (e.g., ${suggestedName}-2.md).

After saving, confirm the exact path where the document was saved.`;
  }

  /**
   * Handle cancel action
   */
  private handleCancel(): void {
    if (this.abortController) {
      this.abortController.abort();
    }
    // Go back without saving
    this.app.navigate("home");
  }

  /**
   * Handle sending a message to Claude
   */
  private async handleSendMessage(text: string): Promise<void> {
    this.inputEditor.clear();
    this.app.requestRender();

    // Send the revision to Claude
    await this.sendToClaude(text);
  }

  invalidate(): void {
    // Reset any cached state
  }
}
