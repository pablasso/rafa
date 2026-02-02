#!/usr/bin/env node
/**
 * Rafa - Task loop runner for AI coding agents
 * Entry point and CLI routing
 */

import { RafaApp } from "./tui/app.js";
import { runInit } from "./cli/init.js";
import { runDeinit } from "./cli/deinit.js";

/**
 * Parses command line arguments
 */
function parseArgs(): { command: string | null; force: boolean } {
  const args = process.argv.slice(2);
  const command = args[0] || null;
  const force = args.includes("--force") || args.includes("-f");
  return { command, force };
}

/**
 * Prints usage information
 */
function printUsage(): void {
  console.log(`Usage: rafa [command]

Commands:
  (no command)    Launch TUI (home screen)
  init            Initialize Rafa in the current repository
  deinit          Remove Rafa from the current repository

Options:
  --force, -f     Skip confirmation prompts (for deinit)
  --help, -h      Show this help message
  --version, -v   Show version information`);
}

/**
 * Main entry point
 */
export async function main(): Promise<void> {
  const { command, force } = parseArgs();

  switch (command) {
    case null: {
      // Launch TUI
      const app = new RafaApp();
      await app.run();
      break;
    }

    case "init": {
      const result = await runInit();
      if (result.success) {
        console.log(result.message);
      } else {
        console.error(`Error: ${result.message}`);
        process.exit(1);
      }
      break;
    }

    case "deinit": {
      const result = await runDeinit({ force });
      if (result.success) {
        console.log(result.message);
      } else {
        console.error(`Error: ${result.message}`);
        process.exit(1);
      }
      break;
    }

    case "--help":
    case "-h":
      printUsage();
      break;

    case "--version":
    case "-v":
      console.log("rafa version 0.1.0");
      break;

    default:
      console.error(`Unknown command: ${command}`);
      printUsage();
      process.exit(1);
  }
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
