/**
 * Home menu view - main entry point with navigation options
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";

interface MenuItem {
  key: string;
  label: string;
  description: string;
}

const MENU_ITEMS: MenuItem[] = [
  {
    key: "p",
    label: "PRD",
    description: "Create a Product Requirements Document",
  },
  { key: "d", label: "Design", description: "Create a Technical Design" },
  {
    key: "c",
    label: "Create Plan",
    description: "Create a plan from a design doc",
  },
  { key: "r", label: "Run Plan", description: "Execute an existing plan" },
  { key: "l", label: "List Plans", description: "View all plans" },
  { key: "q", label: "Quit", description: "Exit Rafa" },
];

export class HomeView implements RafaView {
  private app: RafaApp;
  private selectedIndex = 0;

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(_context?: unknown): void {
    this.selectedIndex = 0;
  }

  render(width: number): string[] {
    const lines: string[] = [];

    // Header
    lines.push(truncateToWidth("", width));
    lines.push(
      truncateToWidth("  Rafa - Task Loop Runner for AI Coding Agents", width),
    );
    lines.push(truncateToWidth("  ═".repeat(Math.min(45, width - 2)), width));
    lines.push(truncateToWidth("", width));

    // Menu items
    for (let i = 0; i < MENU_ITEMS.length; i++) {
      const item = MENU_ITEMS[i];
      const isSelected = i === this.selectedIndex;
      const prefix = isSelected ? "  > " : "    ";
      const keyDisplay = `[${item.key}]`;
      const line = `${prefix}${keyDisplay} ${item.label}`;
      lines.push(truncateToWidth(line, width));

      // Show description for selected item
      if (isSelected) {
        lines.push(truncateToWidth(`      ${item.description}`, width));
      }
    }

    lines.push(truncateToWidth("", width));
    lines.push(
      truncateToWidth(
        "  Use arrow keys or hotkeys to navigate, Enter to select",
        width,
      ),
    );

    return lines;
  }

  handleInput(data: string): void {
    // Navigation
    if (matchesKey(data, Key.up)) {
      this.selectedIndex = Math.max(0, this.selectedIndex - 1);
      this.app.requestRender();
    } else if (matchesKey(data, Key.down)) {
      this.selectedIndex = Math.min(
        MENU_ITEMS.length - 1,
        this.selectedIndex + 1,
      );
      this.app.requestRender();
    } else if (matchesKey(data, Key.enter)) {
      this.selectItem(this.selectedIndex);
    }

    // Hotkeys - check for single letter keys directly
    if (matchesKey(data, "p")) {
      this.selectItem(0);
    } else if (matchesKey(data, "d")) {
      this.selectItem(1);
    } else if (matchesKey(data, "c")) {
      this.selectItem(2);
    } else if (matchesKey(data, "r")) {
      this.selectItem(3);
    } else if (matchesKey(data, "l")) {
      this.selectItem(4);
    } else if (matchesKey(data, "q")) {
      this.selectItem(5);
    }
  }

  private selectItem(index: number): void {
    const item = MENU_ITEMS[index];
    switch (item.key) {
      case "p":
        this.app.navigate("conversation", { phase: "prd" });
        break;
      case "d":
        this.app.navigate("file-picker", {
          nextView: "conversation",
          phase: "design",
        });
        break;
      case "c":
        this.app.navigate("file-picker", { nextView: "plan-create" });
        break;
      case "r":
        this.app.navigate("plan-list");
        break;
      case "l":
        this.app.navigate("plan-list");
        break;
      case "q":
        this.app.stop();
        process.exit(0);
    }
  }

  invalidate(): void {
    // No cached state to invalidate
  }
}
