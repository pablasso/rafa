/**
 * Design doc picker view
 * Lists files from docs/designs/ for selection
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView, ViewState } from "../app.js";

interface FilePickerContext {
  nextView: ViewState | "plan-create";
  phase?: "prd" | "design";
}

interface FileItem {
  name: string;
  path: string;
}

export class FilePickerView implements RafaView {
  private app: RafaApp;
  private files: FileItem[] = [];
  private selectedIndex = 0;
  private nextView: ViewState | "plan-create" = "conversation";
  private phase?: "prd" | "design";

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(context?: unknown): void {
    this.selectedIndex = 0;
    if (context && typeof context === "object") {
      const ctx = context as FilePickerContext;
      this.nextView = ctx.nextView;
      this.phase = ctx.phase;
    }
    // Files will be loaded from filesystem in future implementation
    this.files = [];
  }

  render(width: number): string[] {
    const lines: string[] = [];
    const title =
      this.nextView === "plan-create"
        ? "Select Design Document"
        : "Select Source File";

    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth(`  ${title}`, width));
    lines.push(truncateToWidth("  ─".repeat(Math.min(30, width - 2)), width));
    lines.push(truncateToWidth("", width));

    if (this.files.length === 0) {
      lines.push(truncateToWidth("  No design documents found.", width));
      lines.push(truncateToWidth("", width));
      lines.push(truncateToWidth("  Expected location: docs/designs/", width));
      lines.push(
        truncateToWidth(
          "  Create a design doc first, then run plan create.",
          width,
        ),
      );
    } else {
      for (let i = 0; i < this.files.length; i++) {
        const file = this.files[i];
        const isSelected = i === this.selectedIndex;
        const prefix = isSelected ? "  > " : "    ";
        lines.push(truncateToWidth(`${prefix}${file.name}`, width));
      }
    }

    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  [Enter] Select  [Esc] Back", width));

    return lines;
  }

  handleInput(data: string): void {
    if (matchesKey(data, Key.escape)) {
      this.app.navigate("home");
    } else if (matchesKey(data, Key.up) && this.files.length > 0) {
      this.selectedIndex = Math.max(0, this.selectedIndex - 1);
      this.app.requestRender();
    } else if (matchesKey(data, Key.down) && this.files.length > 0) {
      this.selectedIndex = Math.min(
        this.files.length - 1,
        this.selectedIndex + 1,
      );
      this.app.requestRender();
    } else if (matchesKey(data, Key.enter) && this.files.length > 0) {
      const file = this.files[this.selectedIndex];
      if (this.nextView === "plan-create") {
        // Plan creation would be handled by a separate flow
        // For now, just go back home
        this.app.navigate("home");
      } else if (this.nextView === "conversation") {
        this.app.navigate("conversation", {
          phase: this.phase,
          sourceFile: file.path,
        });
      } else {
        this.app.navigate(this.nextView);
      }
    }
  }

  invalidate(): void {
    // No cached state to invalidate
  }
}
