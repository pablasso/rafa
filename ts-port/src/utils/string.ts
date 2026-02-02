/**
 * String utility functions
 */

/**
 * Converts a string to kebab-case.
 *
 * - Lowercases the string
 * - Replaces spaces and underscores with hyphens
 * - Removes non-alphanumeric characters (except hyphens)
 * - Collapses multiple consecutive hyphens
 * - Trims leading/trailing hyphens
 */
export function toKebabCase(s: string): string {
  let result = "";

  for (const char of s) {
    if (/[a-zA-Z0-9]/.test(char)) {
      result += char.toLowerCase();
    } else if (char === " " || char === "_" || char === "-") {
      result += "-";
    }
    // Other characters are dropped
  }

  // Collapse multiple consecutive hyphens
  while (result.includes("--")) {
    result = result.replace(/--/g, "-");
  }

  // Trim leading/trailing hyphens
  result = result.replace(/^-+|-+$/g, "");

  return result;
}

/**
 * Generates a task ID from a zero-based index.
 * Format: t01, t02, ..., t99, t100, etc.
 */
export function generateTaskId(index: number): string {
  return `t${String(index + 1).padStart(2, "0")}`;
}
