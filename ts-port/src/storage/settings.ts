/**
 * User settings
 */

export interface Settings {
  defaultMaxAttempts: number;
}

const DEFAULT_SETTINGS: Settings = {
  defaultMaxAttempts: 5,
};

export async function loadSettings(_settingsPath: string): Promise<Settings> {
  // Implementation in Task 17
  return DEFAULT_SETTINGS;
}

export async function saveSettings(
  _settings: Settings,
  _settingsPath: string
): Promise<void> {
  // Implementation in Task 17
}
