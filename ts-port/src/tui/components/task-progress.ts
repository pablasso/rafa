/**
 * Task list with status component
 *
 * Displays a list of tasks with their status indicators:
 * ✓ t01: Set up project
 * ▶ t02: Implement core (attempt 2/3)
 * ○ t03: Add tests
 * ○ t04: Documentation
 */

import { truncateToWidth } from "@mariozechner/pi-tui";
import type { Task, TaskStatus } from "../../core/task.js";

// Maximum number of attempts per task (matches executor logic)
export const MAX_TASK_ATTEMPTS = 3;

export class TaskProgressComponent {
  private tasks: Task[] = [];
  private currentTaskId: string | null = null;

  /**
   * Update the task list
   */
  setTasks(tasks: Task[]): void {
    this.tasks = tasks;
  }

  /**
   * Set the currently executing task
   */
  setCurrentTask(taskId: string | null): void {
    this.currentTaskId = taskId;
  }

  /**
   * Get tasks (for testing)
   */
  getTasks(): Task[] {
    return this.tasks;
  }

  /**
   * Get current task ID (for testing)
   */
  getCurrentTaskId(): string | null {
    return this.currentTaskId;
  }

  /**
   * Get status icon for a task status
   */
  private getStatusIcon(status: TaskStatus): string {
    switch (status) {
      case "completed":
        return "✓";
      case "in_progress":
        return "▶";
      case "failed":
        return "✗";
      case "pending":
      default:
        return "○";
    }
  }

  /**
   * Format attempt info for display
   */
  private formatAttempts(task: Task): string {
    if (task.status === "in_progress" && task.attempts > 0) {
      return ` (attempt ${task.attempts}/${MAX_TASK_ATTEMPTS})`;
    }
    if (task.status === "failed") {
      return ` (failed after ${task.attempts})`;
    }
    return "";
  }

  /**
   * Render the task progress list
   */
  render(width: number): string[] {
    const lines: string[] = [];

    if (this.tasks.length === 0) {
      lines.push(truncateToWidth("  (no tasks)", width));
      return lines;
    }

    for (const task of this.tasks) {
      const icon = this.getStatusIcon(task.status);
      const isCurrent = task.id === this.currentTaskId;
      const prefix = isCurrent ? "> " : "  ";
      const attempts = this.formatAttempts(task);

      // Format: > ✓ t01: Title (attempt 1/3)
      const line = `${prefix}${icon} ${task.id}: ${task.title}${attempts}`;
      lines.push(truncateToWidth(line, width));
    }

    return lines;
  }
}
