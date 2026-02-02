/**
 * Design creation CLI command
 *
 * Launches the TUI in Design creation mode, allowing the user to
 * create a Technical Design document through a conversation with Claude.
 */

import { RafaApp } from "../tui/app.js";
import type { ConversationContext } from "../tui/views/conversation.js";

export interface DesignOptions {
  name?: string;
  from?: string; // Path to source PRD
}

/**
 * Run the Design creation flow
 *
 * This launches the TUI directly into the conversation view with Design mode active.
 */
export async function runDesign(options: DesignOptions): Promise<void> {
  const app = new RafaApp();

  // Create Design context with optional name and source PRD
  const context: ConversationContext = {
    phase: "design",
    name: options.name,
    sourceFile: options.from,
  };

  // Navigate directly to conversation view with Design context
  app.navigateOnStart("conversation", context);

  await app.run();
}
