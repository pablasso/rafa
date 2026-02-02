/**
 * Main TUI container and view routing
 */

import {
  TUI,
  ProcessTerminal,
  type Component,
  matchesKey,
  Key,
  truncateToWidth,
} from "@mariozechner/pi-tui";
import { HomeView } from "./views/home.js";
import { ConversationView } from "./views/conversation.js";
import { RunView } from "./views/run.js";
import { PlanListView } from "./views/plan-list.js";
import { FilePickerView } from "./views/file-picker.js";
import { checkClaudeCli } from "../utils/claude-check.js";

export type ViewState =
  | "home"
  | "conversation"
  | "run"
  | "plan-list"
  | "file-picker";

/**
 * Interface for Rafa views that can be navigated to and activated
 */
export interface RafaView extends Component {
  /**
   * Called when the view becomes active
   * @param context Optional context data passed during navigation
   */
  activate(context?: unknown): void;
}

/**
 * Main Rafa application - manages TUI and view navigation
 */
export class RafaApp {
  private tui: TUI;
  private currentView: ViewState = "home";
  private views: Map<ViewState, RafaView>;
  private viewContainer: ViewContainer;

  constructor() {
    const terminal = new ProcessTerminal();
    this.tui = new TUI(terminal);

    // Initialize all views with reference to app for navigation
    this.views = new Map<ViewState, RafaView>([
      ["home", new HomeView(this)],
      ["conversation", new ConversationView(this)],
      ["run", new RunView(this)],
      ["plan-list", new PlanListView(this)],
      ["file-picker", new FilePickerView(this)],
    ]);

    // Create a container that renders the current view
    this.viewContainer = new ViewContainer(this);
    this.tui.addChild(this.viewContainer);
    this.tui.setFocus(this.viewContainer);
  }

  /**
   * Navigate to a different view
   */
  navigate(view: ViewState, context?: unknown): void {
    this.currentView = view;
    const viewInstance = this.views.get(view);
    if (viewInstance) {
      viewInstance.activate(context);
    }
    this.tui.requestRender();
  }

  /**
   * Get the current view state
   */
  getCurrentView(): ViewState {
    return this.currentView;
  }

  /**
   * Get the current view instance
   */
  getCurrentViewInstance(): RafaView | undefined {
    return this.views.get(this.currentView);
  }

  /**
   * Request a re-render of the TUI
   */
  requestRender(): void {
    this.tui.requestRender();
  }

  /**
   * Run the TUI application
   */
  async run(): Promise<void> {
    // Check Claude CLI availability before starting
    const cliCheck = await checkClaudeCli();
    if (!cliCheck.available) {
      console.error(cliCheck.message);
      process.exit(1);
    }

    // Activate the initial view
    const homeView = this.views.get("home");
    if (homeView) {
      homeView.activate();
    }

    // Start the TUI
    this.tui.start();
  }

  /**
   * Stop the TUI application
   */
  stop(): void {
    this.tui.stop();
  }
}

/**
 * Container component that delegates rendering and input to the current view
 */
class ViewContainer implements Component {
  private app: RafaApp;

  constructor(app: RafaApp) {
    this.app = app;
  }

  render(width: number): string[] {
    const view = this.app.getCurrentViewInstance();
    if (view) {
      return view.render(width);
    }
    return [truncateToWidth("No view active", width)];
  }

  handleInput(data: string): void {
    // Check if run view is running - if so, let it handle Ctrl+C for abort
    if (matchesKey(data, Key.ctrl("c"))) {
      const currentView = this.app.getCurrentView();
      if (currentView === "run") {
        const runView = this.app.getCurrentViewInstance();
        // Use a type guard to check if the view has the getIsRunning method
        if (runView && "getIsRunning" in runView) {
          const isRunning = (runView as { getIsRunning: () => boolean }).getIsRunning();
          if (isRunning && runView.handleInput) {
            // Let the run view handle Ctrl+C for aborting execution
            runView.handleInput(data);
            return;
          }
        }
      }
      // Not running - do global quit
      this.app.stop();
      process.exit(0);
    }

    // Delegate to current view
    const view = this.app.getCurrentViewInstance();
    if (view && view.handleInput) {
      view.handleInput(data);
    }
  }

  invalidate(): void {
    const view = this.app.getCurrentViewInstance();
    if (view) {
      view.invalidate();
    }
  }
}
