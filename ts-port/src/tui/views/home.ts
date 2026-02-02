/**
 * Home menu view - main entry point with navigation options
 */

import { matchesKey, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";
import { listPlans } from "../../storage/plans.js";
import type { Plan } from "../../core/plan.js";

interface MenuItem {
  key: string;
  label: string;
  description: string;
  section: "define" | "execute" | "other";
}

const MENU_ITEMS: MenuItem[] = [
  {
    key: "p",
    label: "Create PRD",
    description: "Define the problem and requirements",
    section: "define",
  },
  {
    key: "d",
    label: "Create Design Doc",
    description: "Plan the technical approach",
    section: "define",
  },
  {
    key: "c",
    label: "Create Plan",
    description: "Break design into executable tasks",
    section: "execute",
  },
  {
    key: "r",
    label: "Run Plan",
    description: "Execute tasks with AI agents",
    section: "execute",
  },
  { key: "q", label: "Quit", description: "", section: "other" },
];

export class HomeView implements RafaView {
  private app: RafaApp;
  private currentPlan: Plan | null = null;

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(_context?: unknown): void {
    void this.loadCurrentPlan();
  }

  private async loadCurrentPlan(): Promise<void> {
    try {
      const plans = await listPlans();
      // Find the most recent in-progress or not-started plan
      const activePlan = plans.find(
        (p) => p.status === "in_progress" || p.status === "not_started",
      );
      this.currentPlan = activePlan || null;
    } catch {
      this.currentPlan = null;
    }
    this.app.requestRender();
  }

  render(width: number): string[] {
    const lines: string[] = [];
    const contentWidth = Math.min(60, width - 4);
    const padding = "   ";

    // Empty line at top
    lines.push("");

    // Header box
    const boxTop = `╭${"─".repeat(contentWidth - 2)}╮`;
    const boxBottom = `╰${"─".repeat(contentWidth - 2)}╯`;
    const boxEmpty = `│${" ".repeat(contentWidth - 2)}│`;

    lines.push(truncateToWidth(`${padding}${boxTop}`, width));
    lines.push(truncateToWidth(`${padding}${boxEmpty}`, width));

    const title = "Rafa";
    const subtitle = "AI Workflow Orchestrator";
    const titlePadding = Math.floor((contentWidth - 2 - title.length) / 2);
    const subtitlePadding = Math.floor(
      (contentWidth - 2 - subtitle.length) / 2,
    );
    lines.push(
      truncateToWidth(
        `${padding}│${" ".repeat(titlePadding)}${title}${" ".repeat(contentWidth - 2 - titlePadding - title.length)}│`,
        width,
      ),
    );
    lines.push(
      truncateToWidth(
        `${padding}│${" ".repeat(subtitlePadding)}${subtitle}${" ".repeat(contentWidth - 2 - subtitlePadding - subtitle.length)}│`,
        width,
      ),
    );
    lines.push(truncateToWidth(`${padding}${boxEmpty}`, width));
    lines.push(truncateToWidth(`${padding}${boxBottom}`, width));
    lines.push("");

    // Define section
    lines.push(truncateToWidth(`${padding}Define`, width));
    lines.push(truncateToWidth(`${padding}${"─".repeat(6)}`, width));
    const defineItems = MENU_ITEMS.filter((item) => item.section === "define");
    for (const item of defineItems) {
      const line = `${padding}[${item.key}] ${item.label.padEnd(20)} ${item.description}`;
      lines.push(truncateToWidth(line, width));
    }
    lines.push("");

    // Execute section
    lines.push(truncateToWidth(`${padding}Execute`, width));
    lines.push(truncateToWidth(`${padding}${"─".repeat(7)}`, width));
    const executeItems = MENU_ITEMS.filter(
      (item) => item.section === "execute",
    );
    for (const item of executeItems) {
      const line = `${padding}[${item.key}] ${item.label.padEnd(20)} ${item.description}`;
      lines.push(truncateToWidth(line, width));
    }
    lines.push("");

    // Current plan status (if one exists)
    if (this.currentPlan) {
      const completed = this.currentPlan.tasks.filter(
        (t) => t.status === "completed",
      ).length;
      const total = this.currentPlan.tasks.length;
      const statusText =
        this.currentPlan.status === "in_progress"
          ? `In Progress (${completed}/${total} tasks)`
          : `Ready (${total} tasks)`;

      lines.push(truncateToWidth(`${padding}Current Plan`, width));
      lines.push(truncateToWidth(`${padding}${"─".repeat(12)}`, width));
      lines.push(
        truncateToWidth(
          `${padding}${this.currentPlan.name}: ${statusText}`,
          width,
        ),
      );
      lines.push("");
    }

    // Quit option
    lines.push(truncateToWidth(`${padding}[q] Quit`, width));
    lines.push("");

    return lines;
  }

  handleInput(data: string): void {
    // Hotkey-driven navigation
    if (matchesKey(data, "p")) {
      this.app.navigate("conversation", { phase: "prd" });
    } else if (matchesKey(data, "d")) {
      this.app.navigate("file-picker", {
        nextView: "conversation",
        phase: "design",
      });
    } else if (matchesKey(data, "c")) {
      this.app.navigate("file-picker", { nextView: "plan-create" });
    } else if (matchesKey(data, "r")) {
      this.app.navigate("plan-list");
    } else if (matchesKey(data, "q")) {
      this.app.stop();
      process.exit(0);
    }
  }

  invalidate(): void {
    // No cached state to invalidate
  }
}
