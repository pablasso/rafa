/**
 * PRD creation CLI command
 *
 * Launches the TUI in PRD creation mode, allowing the user to
 * create a Product Requirements Document through a conversation with Claude.
 */

import { RafaApp } from "../tui/app.js";
import type { PrdContext } from "../tui/views/conversation.js";

export interface PrdOptions {
  name?: string;
}

/**
 * Run the PRD creation flow
 *
 * This launches the TUI directly into the conversation view with PRD mode active.
 */
export async function runPrd(options: PrdOptions): Promise<void> {
  const app = new RafaApp();

  // Create PRD context with optional name
  const context: PrdContext = {
    phase: "prd",
    name: options.name,
  };

  // Navigate directly to conversation view with PRD context
  app.navigateOnStart("conversation", context);

  await app.run();
}
