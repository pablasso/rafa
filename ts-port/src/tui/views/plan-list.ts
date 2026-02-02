/**
 * Plan selection view
 * Shows all plans with their status, allows selection for viewing/running
 */

import { matchesKey, Key, truncateToWidth } from "@mariozechner/pi-tui";
import type { RafaApp, RafaView } from "../app.js";
import { listPlans } from "../../storage/plans.js";
import type { Plan } from "../../core/plan.js";

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
  private isLoading = true;
  private error: string | null = null;

  constructor(app: RafaApp) {
    this.app = app;
  }

  activate(_context?: unknown): void {
    this.selectedIndex = 0;
    this.isLoading = true;
    this.error = null;
    this.plans = [];

    // Load plans asynchronously
    void this.loadPlans();
  }

  /**
   * Load plans from .rafa/plans/ directory
   */
  private async loadPlans(): Promise<void> {
    try {
      const plans = await listPlans();
      this.plans = this.convertToSummaries(plans);
      this.isLoading = false;
      this.app.requestRender();
    } catch (err) {
      this.error = err instanceof Error ? err.message : "Failed to load plans";
      this.isLoading = false;
      this.app.requestRender();
    }
  }

  /**
   * Convert Plan objects to PlanSummary and sort by priority
   * Priority: in_progress > not_started > completed > failed
   */
  private convertToSummaries(plans: Plan[]): PlanSummary[] {
    const summaries: PlanSummary[] = plans.map((plan) => ({
      id: plan.id,
      name: plan.name,
      status: plan.status,
      taskCount: plan.tasks.length,
      completedTasks: plan.tasks.filter((t) => t.status === "completed").length,
    }));

    // Sort by status priority, then by name
    const statusPriority: Record<PlanSummary["status"], number> = {
      in_progress: 0,
      not_started: 1,
      completed: 2,
      failed: 3,
    };

    return summaries.sort((a, b) => {
      const priorityDiff = statusPriority[a.status] - statusPriority[b.status];
      if (priorityDiff !== 0) return priorityDiff;
      return a.name.localeCompare(b.name);
    });
  }

  render(width: number): string[] {
    const lines: string[] = [];

    lines.push(truncateToWidth("", width));
    lines.push(truncateToWidth("  Plans", width));
    lines.push(
      truncateToWidth("  " + "─".repeat(Math.min(30, width - 4)), width),
    );
    lines.push(truncateToWidth("", width));

    if (this.isLoading) {
      lines.push(truncateToWidth("  Loading plans...", width));
    } else if (this.error) {
      lines.push(truncateToWidth(`  Error: ${this.error}`, width));
      lines.push(truncateToWidth("", width));
      lines.push(truncateToWidth("  [Esc] Back", width));
    } else if (this.plans.length === 0) {
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
      // Calculate column widths for alignment
      const prefixWidth = 4; // "  > " or "    "
      const iconWidth = 2; // "✓ " etc.
      const progressWidth = 10; // "(XX/XX)" with padding
      const availableNameWidth =
        width - prefixWidth - iconWidth - progressWidth - 2;

      for (let i = 0; i < this.plans.length; i++) {
        const plan = this.plans[i];
        const isSelected = i === this.selectedIndex;
        const prefix = isSelected ? "  > " : "    ";
        const statusIcon = this.getStatusIcon(plan.status);
        const progress = `(${plan.completedTasks}/${plan.taskCount})`;

        // Truncate name if needed
        let displayName = plan.name;
        if (displayName.length > availableNameWidth) {
          displayName = displayName.slice(0, availableNameWidth - 1) + "…";
        }

        const line = `${prefix}${statusIcon} ${displayName} ${progress}`;
        lines.push(truncateToWidth(line, width));
      }
    }

    lines.push(truncateToWidth("", width));
    if (this.plans.length > 0 && !this.isLoading && !this.error) {
      lines.push(
        truncateToWidth(
          "  [↑/↓] Navigate  [Enter] Run  [c] Create  [Esc] Back",
          width,
        ),
      );
    } else if (!this.isLoading) {
      lines.push(truncateToWidth("  [c] Create Plan  [Esc] Back", width));
    }

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
    } else if (
      matchesKey(data, Key.up) &&
      this.plans.length > 0 &&
      !this.isLoading &&
      !this.error
    ) {
      this.selectedIndex = Math.max(0, this.selectedIndex - 1);
      this.app.requestRender();
    } else if (
      matchesKey(data, Key.down) &&
      this.plans.length > 0 &&
      !this.isLoading &&
      !this.error
    ) {
      this.selectedIndex = Math.min(
        this.plans.length - 1,
        this.selectedIndex + 1,
      );
      this.app.requestRender();
    } else if (
      matchesKey(data, Key.enter) &&
      this.plans.length > 0 &&
      !this.isLoading &&
      !this.error
    ) {
      const plan = this.plans[this.selectedIndex];
      this.app.navigate("run", {
        planId: plan.id,
        planName: plan.name,
      });
    } else if (matchesKey(data, "c") && !this.isLoading) {
      this.app.navigate("file-picker", { nextView: "plan-create" });
    }
  }

  invalidate(): void {
    // No cached state to invalidate
  }
}
