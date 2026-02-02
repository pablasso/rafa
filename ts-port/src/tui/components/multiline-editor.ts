/**
 * Multi-line text editor component
 *
 * A lightweight multi-line input editor that doesn't require a TUI instance.
 * Used for conversation input in PRD/Design creation.
 *
 * Features:
 * - Multi-line text input with cursor navigation
 * - Placeholder text when empty
 * - Visual cursor display with reverse video
 * - Disabled state support
 */

import { truncateToWidth, CURSOR_MARKER } from "@mariozechner/pi-tui";

// Maximum lines to keep in the editor
const MAX_LINES = 100;

// Maximum height for rendering (lines visible)
const DEFAULT_MAX_HEIGHT = 5;

export interface MultilineEditorOptions {
  placeholder?: string;
  maxHeight?: number;
}

export class MultilineEditorComponent {
  private lines: string[] = [""];
  private cursorLine = 0;
  private cursorCol = 0;
  private placeholder: string;
  private maxHeight: number;
  private disabled = false;
  private focused = false;

  // Bracketed paste mode buffering
  private pasteBuffer = "";
  private isInPaste = false;

  constructor(options: MultilineEditorOptions = {}) {
    this.placeholder = options.placeholder || "";
    this.maxHeight = options.maxHeight ?? DEFAULT_MAX_HEIGHT;
  }

  /**
   * Set the focused state (controls cursor display)
   */
  setFocused(focused: boolean): void {
    this.focused = focused;
  }

  /**
   * Get focused state
   */
  isFocused(): boolean {
    return this.focused;
  }

  /**
   * Set the disabled state
   */
  setDisabled(disabled: boolean): void {
    this.disabled = disabled;
  }

  /**
   * Check if editor is disabled
   */
  isDisabled(): boolean {
    return this.disabled;
  }

  /**
   * Get the text content
   */
  getText(): string {
    return this.lines.join("\n");
  }

  /**
   * Set the text content
   */
  setText(text: string): void {
    this.lines = text.split("\n");
    if (this.lines.length === 0) {
      this.lines = [""];
    }
    // Clamp cursor position
    this.cursorLine = Math.min(this.cursorLine, this.lines.length - 1);
    this.cursorCol = Math.min(
      this.cursorCol,
      this.lines[this.cursorLine].length,
    );
  }

  /**
   * Clear the editor
   */
  clear(): void {
    this.lines = [""];
    this.cursorLine = 0;
    this.cursorCol = 0;
  }

  /**
   * Check if editor is empty
   */
  isEmpty(): boolean {
    return this.lines.length === 1 && this.lines[0] === "";
  }

  /**
   * Handle keyboard input
   */
  handleInput(data: string): void {
    if (this.disabled) {
      return;
    }

    // Handle bracketed paste mode
    if (data.includes("\x1b[200~")) {
      this.isInPaste = true;
      this.pasteBuffer = "";
      data = data.replace("\x1b[200~", "");
    }

    if (this.isInPaste) {
      this.pasteBuffer += data;
      const endIndex = this.pasteBuffer.indexOf("\x1b[201~");
      if (endIndex !== -1) {
        const pasteContent = this.pasteBuffer.substring(0, endIndex);
        this.handlePaste(pasteContent);
        this.isInPaste = false;
        const remaining = this.pasteBuffer.substring(endIndex + 6);
        this.pasteBuffer = "";
        if (remaining) {
          this.handleInput(remaining);
        }
      }
      return;
    }

    // Handle special keys
    // Backspace
    if (data === "\x7f" || data === "\b") {
      this.handleBackspace();
      return;
    }

    // Delete (forward delete)
    if (data === "\x1b[3~") {
      this.handleDelete();
      return;
    }

    // Arrow keys
    if (data === "\x1b[A") {
      // Up
      this.moveCursorUp();
      return;
    }
    if (data === "\x1b[B") {
      // Down
      this.moveCursorDown();
      return;
    }
    if (data === "\x1b[C") {
      // Right
      this.moveCursorRight();
      return;
    }
    if (data === "\x1b[D") {
      // Left
      this.moveCursorLeft();
      return;
    }

    // Home (Ctrl+A or Home key)
    if (data === "\x01" || data === "\x1b[H") {
      this.cursorCol = 0;
      return;
    }

    // End (Ctrl+E or End key)
    if (data === "\x05" || data === "\x1b[F") {
      this.cursorCol = this.lines[this.cursorLine].length;
      return;
    }

    // Ctrl+K - delete to end of line
    if (data === "\x0b") {
      this.deleteToEndOfLine();
      return;
    }

    // Ctrl+U - delete to start of line
    if (data === "\x15") {
      this.deleteToStartOfLine();
      return;
    }

    // Enter - add new line or submit with Ctrl+Enter
    if (data === "\r" || data === "\n") {
      this.addNewLine();
      return;
    }

    // Tab - insert spaces
    if (data === "\t") {
      this.insertText("  ");
      return;
    }

    // Regular character input - reject control characters
    const hasControlChars = [...data].some((ch) => {
      const code = ch.charCodeAt(0);
      return code < 32 || code === 0x7f || (code >= 0x80 && code <= 0x9f);
    });

    if (!hasControlChars) {
      this.insertText(data);
    }
  }

  private handlePaste(pastedText: string): void {
    // Split paste into lines and insert
    const pasteLines = pastedText.split(/\r\n|\r|\n/);
    for (let i = 0; i < pasteLines.length; i++) {
      if (i > 0) {
        this.addNewLine();
      }
      this.insertText(pasteLines[i]);
    }
  }

  private insertText(text: string): void {
    const line = this.lines[this.cursorLine];
    this.lines[this.cursorLine] =
      line.slice(0, this.cursorCol) + text + line.slice(this.cursorCol);
    this.cursorCol += text.length;
  }

  private addNewLine(): void {
    if (this.lines.length >= MAX_LINES) {
      return;
    }
    const line = this.lines[this.cursorLine];
    const beforeCursor = line.slice(0, this.cursorCol);
    const afterCursor = line.slice(this.cursorCol);
    this.lines[this.cursorLine] = beforeCursor;
    this.lines.splice(this.cursorLine + 1, 0, afterCursor);
    this.cursorLine++;
    this.cursorCol = 0;
  }

  private handleBackspace(): void {
    if (this.cursorCol > 0) {
      const line = this.lines[this.cursorLine];
      this.lines[this.cursorLine] =
        line.slice(0, this.cursorCol - 1) + line.slice(this.cursorCol);
      this.cursorCol--;
    } else if (this.cursorLine > 0) {
      // Merge with previous line
      const currentLine = this.lines[this.cursorLine];
      const prevLine = this.lines[this.cursorLine - 1];
      this.cursorCol = prevLine.length;
      this.lines[this.cursorLine - 1] = prevLine + currentLine;
      this.lines.splice(this.cursorLine, 1);
      this.cursorLine--;
    }
  }

  private handleDelete(): void {
    const line = this.lines[this.cursorLine];
    if (this.cursorCol < line.length) {
      this.lines[this.cursorLine] =
        line.slice(0, this.cursorCol) + line.slice(this.cursorCol + 1);
    } else if (this.cursorLine < this.lines.length - 1) {
      // Merge with next line
      this.lines[this.cursorLine] = line + this.lines[this.cursorLine + 1];
      this.lines.splice(this.cursorLine + 1, 1);
    }
  }

  private moveCursorUp(): void {
    if (this.cursorLine > 0) {
      this.cursorLine--;
      this.cursorCol = Math.min(
        this.cursorCol,
        this.lines[this.cursorLine].length,
      );
    }
  }

  private moveCursorDown(): void {
    if (this.cursorLine < this.lines.length - 1) {
      this.cursorLine++;
      this.cursorCol = Math.min(
        this.cursorCol,
        this.lines[this.cursorLine].length,
      );
    }
  }

  private moveCursorLeft(): void {
    if (this.cursorCol > 0) {
      this.cursorCol--;
    } else if (this.cursorLine > 0) {
      this.cursorLine--;
      this.cursorCol = this.lines[this.cursorLine].length;
    }
  }

  private moveCursorRight(): void {
    const line = this.lines[this.cursorLine];
    if (this.cursorCol < line.length) {
      this.cursorCol++;
    } else if (this.cursorLine < this.lines.length - 1) {
      this.cursorLine++;
      this.cursorCol = 0;
    }
  }

  private deleteToEndOfLine(): void {
    const line = this.lines[this.cursorLine];
    this.lines[this.cursorLine] = line.slice(0, this.cursorCol);
  }

  private deleteToStartOfLine(): void {
    const line = this.lines[this.cursorLine];
    this.lines[this.cursorLine] = line.slice(this.cursorCol);
    this.cursorCol = 0;
  }

  /**
   * Render the editor
   */
  render(width: number): string[] {
    const result: string[] = [];

    // Show placeholder if empty
    if (this.isEmpty() && this.placeholder) {
      const placeholderLine = truncateToWidth(
        `│ \x1b[90m${this.placeholder}\x1b[0m`,
        width,
      );
      result.push(placeholderLine);
      // Pad to maxHeight
      for (let i = 1; i < this.maxHeight; i++) {
        result.push(truncateToWidth("│", width));
      }
      return result;
    }

    // Determine visible lines (scroll to keep cursor visible)
    const totalLines = this.lines.length;
    let startLine = 0;

    if (totalLines > this.maxHeight) {
      // Scroll to keep cursor visible
      if (this.cursorLine >= startLine + this.maxHeight) {
        startLine = this.cursorLine - this.maxHeight + 1;
      }
      if (this.cursorLine < startLine) {
        startLine = this.cursorLine;
      }
    }

    const endLine = Math.min(startLine + this.maxHeight, totalLines);

    // Render visible lines
    for (let i = startLine; i < endLine; i++) {
      const line = this.lines[i];
      let displayLine: string;

      if (i === this.cursorLine && this.focused && !this.disabled) {
        // Render line with cursor
        const beforeCursor = line.slice(0, this.cursorCol);
        const atCursor = line[this.cursorCol] || " ";
        const afterCursor = line.slice(this.cursorCol + 1);

        // Reverse video for cursor
        const cursorChar = `\x1b[7m${atCursor}\x1b[27m`;
        displayLine = `│ ${beforeCursor}${CURSOR_MARKER}${cursorChar}${afterCursor}`;
      } else {
        displayLine = `│ ${line}`;
      }

      result.push(truncateToWidth(displayLine, width, "", true));
    }

    // Pad to maxHeight if needed
    while (result.length < this.maxHeight) {
      result.push(truncateToWidth("│", width));
    }

    return result;
  }

  /**
   * Render with a specific height
   */
  renderWithHeight(width: number, height: number): string[] {
    const oldMaxHeight = this.maxHeight;
    this.maxHeight = height;
    const result = this.render(width);
    this.maxHeight = oldMaxHeight;
    return result;
  }
}
