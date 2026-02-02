/**
 * Main TUI container and view routing
 */

export type ViewState =
  | "home"
  | "conversation"
  | "run"
  | "plan-list"
  | "file-picker";

export class RafaApp {
  private currentView: ViewState = "home";

  navigate(view: ViewState, _context?: unknown): void {
    this.currentView = view;
  }

  getCurrentView(): ViewState {
    return this.currentView;
  }

  async run(): Promise<void> {
    // TUI initialization will be implemented in Task 2
  }
}
