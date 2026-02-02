#!/usr/bin/env node
/**
 * Rafa - Task loop runner for AI coding agents
 * Entry point and CLI routing
 */

import { RafaApp } from "./tui/app.js";

export async function main(): Promise<void> {
  const app = new RafaApp();
  await app.run();
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
