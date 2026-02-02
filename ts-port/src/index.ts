#!/usr/bin/env node
/**
 * Rafa - Task loop runner for AI coding agents
 * Entry point and CLI routing
 */

import { RafaApp } from "./tui/app.js";
import { runInit } from "./cli/init.js";
import { runDeinit } from "./cli/deinit.js";
import { runPrd } from "./cli/prd.js";
import { runDesign } from "./cli/design.js";
import { runPlanCreate } from "./cli/plan-create.js";

/**
 * CLI parsed arguments
 */
interface ParsedArgs {
  command: string | null;
  subcommand: string | null;
  positionalArg: string | null;
  force: boolean;
  dryRun: boolean;
  name?: string;
  from?: string;
}

/**
 * Parses command line arguments
 */
function parseArgs(): ParsedArgs {
  const args = process.argv.slice(2);
  const command = args[0] || null;
  const force = args.includes("--force") || args.includes("-f");
  const dryRun = args.includes("--dry-run");

  // Parse subcommand (e.g., "plan create" -> subcommand is "create")
  let subcommand: string | null = null;
  let positionalArg: string | null = null;

  // For commands like "plan create <file>", args[1] is subcommand, args[2] is file
  if (args[1] && !args[1].startsWith("-")) {
    subcommand = args[1];
    // Find positional arg (first non-flag after subcommand)
    for (let i = 2; i < args.length; i++) {
      if (!args[i].startsWith("-")) {
        // Check it's not a value for a flag
        if (i === 2 || !["--name", "--from"].includes(args[i - 1])) {
          positionalArg = args[i];
          break;
        }
      }
    }
  }

  // Parse --name flag
  let name: string | undefined;
  const nameIndex = args.indexOf("--name");
  if (nameIndex !== -1 && args[nameIndex + 1]) {
    name = args[nameIndex + 1];
  }

  // Parse --from flag (for design command)
  let from: string | undefined;
  const fromIndex = args.indexOf("--from");
  if (fromIndex !== -1 && args[fromIndex + 1]) {
    from = args[fromIndex + 1];
  }

  return { command, subcommand, positionalArg, force, dryRun, name, from };
}

/**
 * Prints usage information
 */
function printUsage(): void {
  console.log(`Usage: rafa [command]

Commands:
  (no command)      Launch TUI (home screen)
  init              Initialize Rafa in the current repository
  deinit            Remove Rafa from the current repository
  prd               Create a Product Requirements Document
  design            Create a Technical Design document
  plan create [file]  Create plan from design doc (shows picker if no file)

Options:
  --name <name>   Specify document/plan name
  --from <prd>    Reference existing PRD (for design)
  --dry-run       Preview changes without saving (for plan create)
  --force, -f     Skip confirmation prompts (for deinit)
  --help, -h      Show this help message
  --version, -v   Show version information`);
}

/**
 * Main entry point
 */
export async function main(): Promise<void> {
  const { command, subcommand, positionalArg, force, dryRun, name, from } =
    parseArgs();

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

    case "prd": {
      await runPrd({ name });
      break;
    }

    case "design": {
      await runDesign({ name, from });
      break;
    }

    case "plan": {
      if (subcommand === "create") {
        if (!positionalArg) {
          // No file argument - launch TUI with file picker
          const app = new RafaApp();
          app.navigateOnStart("file-picker", { nextView: "plan-create" });
          await app.run();
        } else {
          const result = await runPlanCreate({
            filePath: positionalArg,
            name,
            dryRun,
          });
          if (!result.success) {
            console.error(`Error: ${result.message}`);
            process.exit(1);
          }
        }
      } else {
        console.error(`Unknown plan subcommand: ${subcommand}`);
        console.error("Usage: rafa plan create <file>");
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
