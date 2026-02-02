/**
 * Design doc picker view
 * Lists files from docs/designs/ for selection when creating plans
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView, ViewState } from "../app.js";
import { runPlanCreate } from "../../cli/plan-create.js";

interface FilePickerContext {
  nextView: ViewState | "plan-create";
  phase?: "prd" | "design";
}

interface FileItem {
  name: string;
  path: string;
  modifiedAt: Date;
}

const DESIGNS_DIR = "docs/designs";

export class FilePickerView implements RafaView {
  private app: RafaApp;
  private files: FileItem[] = [];
  private selectedIndex = 0;
  private nextView: ViewState | "plan-create" = "conversation";
  private phase?: "prd" | "design";
  private isLoading = true;
  private error: string | null = null;

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(context?: unknown): void {
    this.selectedIndex = 0;
    this.isLoading = true;
    this.error = null;
    this.files = [];

    if (context && typeof context === "object") {
      const ctx = context as FilePickerContext;
      this.nextView = ctx.nextView;
      this.phase = ctx.phase;
    }

    // Load files asynchronously
    void this.loadFiles();
  }

  /**
   * Load markdown files from the docs/designs/ directory
   */
  private async loadFiles(): Promise<void> {
    try {
      const designsPath = path.resolve(process.cwd(), DESIGNS_DIR);

      // Check if directory exists
      try {
        await fs.access(designsPath);
      } catch {
        // Directory doesn't exist - show empty state
        this.files = [];
        this.isLoading = false;
        this.app.requestRender();
        return;
      }

      // Read directory contents
      const entries = await fs.readdir(designsPath, { withFileTypes: true });

      // Filter for markdown files and get their stats
      const filePromises = entries
        .filter((entry) => entry.isFile() && entry.name.endsWith(".md"))
        .map(async (entry) => {
          const filePath = path.join(designsPath, entry.name);
          const stat = await fs.stat(filePath);
          return {
            name: entry.name,
            path: filePath,
            modifiedAt: stat.mtime,
          };
        });

      const files = await Promise.all(filePromises);

      // Sort by modification date (most recent first)
      this.files = files.sort(
        (a, b) => b.modifiedAt.getTime() - a.modifiedAt.getTime(),
      );

      this.isLoading = false;
      this.app.requestRender();
    } catch (err) {
      this.error =
        err instanceof Error ? err.message : "Failed to load files";
      this.isLoading = false;
      this.app.requestRender();
    }
  }

  /**
   * Format a date for display (e.g., "Jan 15" or "2024-01-15")
   */
  private formatDate(date: Date): string {
    const now = new Date();
    const isCurrentYear = date.getFullYear() === now.getFullYear();

    if (isCurrentYear) {
      // Show "Jan 15" format for current year
      const months = [
        "Jan",
        "Feb",
        "Mar",
        "Apr",
        "May",
        "Jun",
        "Jul",
        "Aug",
        "Sep",
        "Oct",
        "Nov",
        "Dec",
      ];
      return `${months[date.getMonth()]} ${date.getDate()}`;
    } else {
      // Show "2024-01-15" format for older files
      const month = String(date.getMonth() + 1).padStart(2, "0");
      const day = String(date.getDate()).padStart(2, "0");
      return `${date.getFullYear()}-${month}-${day}`;
    }
  }

  render(width: number): string[] {
    const lines: string[] = [];
    const title =
      this.nextView === "plan-create"
        ? "Select Design Document"
        : "Select Source File";

    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth(`  ${title}`, width));
    lines.push(truncateToWidth("  " + "─".repeat(Math.min(30, width - 4)), width));
    lines.push(truncateToWidth("", width));

    if (this.isLoading) {
      lines.push(truncateToWidth("  Loading...", width));
    } else if (this.error) {
      lines.push(truncateToWidth(`  Error: ${this.error}`, width));
      lines.push(truncateToWidth("", width));
      lines.push(truncateToWidth("  [Esc] Back", width));
    } else if (this.files.length === 0) {
      lines.push(truncateToWidth("  No design documents found.", width));
      lines.push(truncateToWidth("", width));
      lines.push(truncateToWidth(`  Expected location: ${DESIGNS_DIR}/`, width));
      lines.push(
        truncateToWidth(
          "  Create a design doc first, then run plan create.",
          width,
        ),
      );
    } else {
      // Calculate column widths
      const dateWidth = 12; // "Jan 15" or "2024-01-15"
      const separatorWidth = 3; // " │ "
      const prefixWidth = 4; // "  > " or "    "
      const availableNameWidth =
        width - prefixWidth - separatorWidth - dateWidth - 2;

      for (let i = 0; i < this.files.length; i++) {
        const file = this.files[i];
        const isSelected = i === this.selectedIndex;
        const prefix = isSelected ? "  > " : "    ";
        const dateStr = this.formatDate(file.modifiedAt);

        // Truncate name if needed
        let displayName = file.name;
        if (displayName.length > availableNameWidth) {
          displayName = displayName.slice(0, availableNameWidth - 1) + "…";
        }

        // Pad name to align dates
        const paddedName = displayName.padEnd(availableNameWidth);

        const line = `${prefix}${paddedName} │ ${dateStr}`;
        lines.push(truncateToWidth(line, width));
      }
    }

    lines.push(truncateToWidth("", width));
    if (this.files.length > 0) {
      lines.push(
        truncateToWidth("  [↑/↓] Navigate  [Enter] Select  [Esc] Back", width),
      );
    } else {
      lines.push(truncateToWidth("  [Esc] Back", width));
    }

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
        // Run plan creation with selected file
        void this.handlePlanCreate(file.path);
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

  /**
   * Handle plan creation when a file is selected
   */
  private async handlePlanCreate(filePath: string): Promise<void> {
    // Stop TUI to allow CLI output
    this.app.stop();

    // Run plan creation
    const result = await runPlanCreate({ filePath });
    if (!result.success) {
      console.error(`Error: ${result.message}`);
      process.exit(1);
    }
    process.exit(0);
  }

  invalidate(): void {
    // No cached state to invalidate
  }
}
