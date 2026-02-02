/**
 * Plan selection view
 * Shows all plans with their status, allows selection for viewing/running
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";

interface PlanSummary {
  id: string;
  name: string;
  status: "not_started" | "in_progress" | "completed" | "failed";
  taskCount: number;
  completedTasks: number;
}

export class PlanListView implements RafaView {
  private app: RafaApp;
  private plans: PlanSummary[] = [];
  private selectedIndex = 0;

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(_context?: unknown): void {
    this.selectedIndex = 0;
    // Plans will be loaded from storage in future implementation
    this.plans = [];
  }

  render(width: number): string[] {
    const lines: string[] = [];

    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  Plans", width));
    lines.push(truncateToWidth("  ─".repeat(Math.min(30, width - 2)), width));
    lines.push(truncateToWidth("", width));

    if (this.plans.length === 0) {
      lines.push(truncateToWidth("  No plans found.", width));
      lines.push(truncateToWidth("", width));
      lines.push(
        truncateToWidth(
          "  Create a plan from a design doc to get started:",
          width,
        ),
      );
      lines.push(truncateToWidth("    [c] Create Plan", width));
    } else {
      for (let i = 0; i < this.plans.length; i++) {
        const plan = this.plans[i];
        const isSelected = i === this.selectedIndex;
        const prefix = isSelected ? "  > " : "    ";
        const statusIcon = this.getStatusIcon(plan.status);
        const progress = `${plan.completedTasks}/${plan.taskCount}`;
        const line = `${prefix}${statusIcon} ${plan.name} (${progress})`;
        lines.push(truncateToWidth(line, width));
      }
    }

    lines.push(truncateToWidth("", width));
    lines.push(
      truncateToWidth("  [Enter] Run plan  [Esc] Back to Home", width),
    );

    return lines;
  }

  private getStatusIcon(status: PlanSummary["status"]): string {
    switch (status) {
      case "completed":
        return "✓";
      case "in_progress":
        return "▶";
      case "failed":
        return "✗";
      default:
        return "○";
    }
  }

  handleInput(data: string): void {
    if (matchesKey(data, Key.escape)) {
      this.app.navigate("home");
    } else if (matchesKey(data, Key.up) && this.plans.length > 0) {
      this.selectedIndex = Math.max(0, this.selectedIndex - 1);
      this.app.requestRender();
    } else if (matchesKey(data, Key.down) && this.plans.length > 0) {
      this.selectedIndex = Math.min(
        this.plans.length - 1,
        this.selectedIndex + 1,
      );
      this.app.requestRender();
    } else if (matchesKey(data, Key.enter) && this.plans.length > 0) {
      const plan = this.plans[this.selectedIndex];
      this.app.navigate("run", { planId: plan.id, planName: plan.name });
    } else if (matchesKey(data, "c")) {
      this.app.navigate("file-picker", { nextView: "plan-create" });
    }
  }

  invalidate(): void {
    // No cached state to invalidate
  }
}
