/**
 * User settings - stored in .rafa/settings.json
 */

import * as fs from "node:fs/promises";

export interface Settings {
  defaultMaxAttempts: number;
}

const DEFAULT_SETTINGS: Settings = {
  defaultMaxAttempts: 5,
};

/**
 * Loads settings from a file. Returns defaults if file doesn't exist.
 */
export async function loadSettings(settingsPath: string): Promise<Settings> {
  try {
    const content = await fs.readFile(settingsPath, "utf-8");
    const parsed = JSON.parse(content);

    // Merge with defaults to handle missing fields
    return {
      ...DEFAULT_SETTINGS,
      ...parsed,
    };
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return DEFAULT_SETTINGS;
    }
    throw err;
  }
}

/**
 * Saves settings to a file
 */
export async function saveSettings(
  settings: Settings,
  settingsPath: string
): Promise<void> {
  const content = JSON.stringify(settings, null, 2) + "\n";
  await fs.writeFile(settingsPath, content, "utf-8");
}

/**
 * Returns a copy of the default settings
 */
export function getDefaultSettings(): Settings {
  return { ...DEFAULT_SETTINGS };
}
