/**
 * Tests for settings module
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import { loadSettings, saveSettings, getDefaultSettings } from "../storage/settings.js";

describe("settings", () => {
  let testDir: string;
  let settingsPath: string;

  beforeEach(async () => {
    testDir = await fs.mkdtemp(path.join(process.cwd(), ".test-settings-"));
    settingsPath = path.join(testDir, "settings.json");
  });

  afterEach(async () => {
    await fs.rm(testDir, { recursive: true, force: true });
  });

  describe("loadSettings", () => {
    it("should return defaults if file does not exist", async () => {
      const settings = await loadSettings(settingsPath);

      expect(settings).toEqual({
        defaultMaxAttempts: 5,
      });
    });

    it("should load settings from file", async () => {
      await fs.writeFile(
        settingsPath,
        JSON.stringify({ defaultMaxAttempts: 10 })
      );

      const settings = await loadSettings(settingsPath);

      expect(settings.defaultMaxAttempts).toBe(10);
    });

    it("should merge with defaults for missing fields", async () => {
      await fs.writeFile(settingsPath, JSON.stringify({}));

      const settings = await loadSettings(settingsPath);

      expect(settings.defaultMaxAttempts).toBe(5);
    });

    it("should throw on invalid JSON", async () => {
      await fs.writeFile(settingsPath, "not json");

      await expect(loadSettings(settingsPath)).rejects.toThrow();
    });
  });

  describe("saveSettings", () => {
    it("should save settings to file", async () => {
      const settings = { defaultMaxAttempts: 10 };

      await saveSettings(settings, settingsPath);

      const content = await fs.readFile(settingsPath, "utf-8");
      const parsed = JSON.parse(content);
      expect(parsed.defaultMaxAttempts).toBe(10);
    });

    it("should format with indentation", async () => {
      const settings = { defaultMaxAttempts: 10 };

      await saveSettings(settings, settingsPath);

      const content = await fs.readFile(settingsPath, "utf-8");
      expect(content).toContain("  ");
    });

    it("should end with newline", async () => {
      const settings = { defaultMaxAttempts: 10 };

      await saveSettings(settings, settingsPath);

      const content = await fs.readFile(settingsPath, "utf-8");
      expect(content.endsWith("\n")).toBe(true);
    });
  });

  describe("getDefaultSettings", () => {
    it("should return a copy of default settings", () => {
      const defaults1 = getDefaultSettings();
      const defaults2 = getDefaultSettings();

      expect(defaults1).toEqual(defaults2);
      expect(defaults1).not.toBe(defaults2); // Should be different objects
    });

    it("should return correct defaults", () => {
      const defaults = getDefaultSettings();

      expect(defaults.defaultMaxAttempts).toBe(5);
    });
  });
});
