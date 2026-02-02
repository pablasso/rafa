/**
 * Tests for init and deinit commands
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import {
  runInit,
  isInitialized,
  getRafaDir,
  getSettingsPath,
  addToGitignore,
  removeFromGitignore,
} from "../cli/init.js";
import { runDeinit } from "../cli/deinit.js";

describe("init command", () => {
  let testDir: string;

  beforeEach(async () => {
    // Create a temporary test directory
    testDir = await fs.mkdtemp(path.join(process.cwd(), ".test-init-"));
  });

  afterEach(async () => {
    // Clean up test directory
    await fs.rm(testDir, { recursive: true, force: true });
  });

  it("should create .rafa directory structure", async () => {
    const result = await runInit({ workDir: testDir, skipSkills: true });

    expect(result.success).toBe(true);
    expect(result.message).toContain("Initialized Rafa");

    // Verify directory structure
    const rafaDir = getRafaDir(testDir);
    const stat = await fs.stat(rafaDir);
    expect(stat.isDirectory()).toBe(true);

    const plansDir = path.join(rafaDir, "plans");
    const plansStat = await fs.stat(plansDir);
    expect(plansStat.isDirectory()).toBe(true);

    const sessionsDir = path.join(rafaDir, "sessions");
    const sessionsStat = await fs.stat(sessionsDir);
    expect(sessionsStat.isDirectory()).toBe(true);
  });

  it("should create settings.json with defaults", async () => {
    await runInit({ workDir: testDir, skipSkills: true });

    const settingsPath = getSettingsPath(testDir);
    const content = await fs.readFile(settingsPath, "utf-8");
    const settings = JSON.parse(content);

    expect(settings.defaultMaxAttempts).toBe(5);
  });

  it("should create version file marking TypeScript version", async () => {
    await runInit({ workDir: testDir, skipSkills: true });

    const versionPath = path.join(getRafaDir(testDir), "version");
    const content = await fs.readFile(versionPath, "utf-8");
    const version = JSON.parse(content);

    expect(version.version).toBe(2);
    expect(version.runtime).toBe("typescript");
  });

  it("should add gitignore entries", async () => {
    await runInit({ workDir: testDir, skipSkills: true });

    const gitignorePath = path.join(testDir, ".gitignore");
    const content = await fs.readFile(gitignorePath, "utf-8");

    expect(content).toContain(".rafa/**/*.lock");
    expect(content).toContain(".rafa/sessions/");
  });

  it("should fail if already initialized", async () => {
    // Initialize first
    await runInit({ workDir: testDir, skipSkills: true });

    // Try again
    const result = await runInit({ workDir: testDir, skipSkills: true });

    expect(result.success).toBe(false);
    expect(result.message).toContain("already initialized");
  });

  it("isInitialized should return false for empty directory", async () => {
    expect(await isInitialized(testDir)).toBe(false);
  });

  it("isInitialized should return true after init", async () => {
    await runInit({ workDir: testDir, skipSkills: true });
    expect(await isInitialized(testDir)).toBe(true);
  });
});

describe("deinit command", () => {
  let testDir: string;

  beforeEach(async () => {
    // Create a temporary test directory and initialize
    testDir = await fs.mkdtemp(path.join(process.cwd(), ".test-deinit-"));
    await runInit({ workDir: testDir, skipSkills: true });
  });

  afterEach(async () => {
    // Clean up test directory
    await fs.rm(testDir, { recursive: true, force: true });
  });

  it("should remove .rafa directory with --force", async () => {
    const result = await runDeinit({ workDir: testDir, force: true });

    expect(result.success).toBe(true);
    expect(result.message).toContain("has been removed");

    // Verify directory is gone
    const rafaDir = getRafaDir(testDir);
    await expect(fs.stat(rafaDir)).rejects.toThrow();
  });

  it("should remove gitignore entries", async () => {
    // Add some extra content to gitignore to verify we don't remove everything
    const gitignorePath = path.join(testDir, ".gitignore");
    await fs.appendFile(gitignorePath, "node_modules/\n");

    await runDeinit({ workDir: testDir, force: true });

    const content = await fs.readFile(gitignorePath, "utf-8");
    expect(content).not.toContain(".rafa/**/*.lock");
    expect(content).not.toContain(".rafa/sessions/");
    expect(content).toContain("node_modules/");
  });

  it("should abort if user declines confirmation", async () => {
    const result = await runDeinit({
      workDir: testDir,
      confirm: async () => false,
    });

    expect(result.success).toBe(true);
    expect(result.message).toBe("Aborted.");

    // Verify directory still exists
    const rafaDir = getRafaDir(testDir);
    const stat = await fs.stat(rafaDir);
    expect(stat.isDirectory()).toBe(true);
  });

  it("should proceed if user confirms", async () => {
    const result = await runDeinit({
      workDir: testDir,
      confirm: async () => true,
    });

    expect(result.success).toBe(true);
    expect(result.message).toContain("has been removed");
  });

  it("should fail if not initialized", async () => {
    // Remove .rafa first
    const rafaDir = getRafaDir(testDir);
    await fs.rm(rafaDir, { recursive: true, force: true });

    const result = await runDeinit({ workDir: testDir, force: true });

    expect(result.success).toBe(false);
    expect(result.message).toContain("not initialized");
  });
});

describe("gitignore helpers", () => {
  let testDir: string;
  let gitignorePath: string;

  beforeEach(async () => {
    testDir = await fs.mkdtemp(path.join(process.cwd(), ".test-gitignore-"));
    gitignorePath = path.join(testDir, ".gitignore");
  });

  afterEach(async () => {
    await fs.rm(testDir, { recursive: true, force: true });
  });

  it("addToGitignore should create file if missing", async () => {
    await addToGitignore("test-entry", testDir);

    const content = await fs.readFile(gitignorePath, "utf-8");
    expect(content).toBe("test-entry\n");
  });

  it("addToGitignore should append to existing file", async () => {
    await fs.writeFile(gitignorePath, "existing-entry\n");

    await addToGitignore("new-entry", testDir);

    const content = await fs.readFile(gitignorePath, "utf-8");
    expect(content).toBe("existing-entry\nnew-entry\n");
  });

  it("addToGitignore should not duplicate entries", async () => {
    await fs.writeFile(gitignorePath, "test-entry\n");

    await addToGitignore("test-entry", testDir);

    const content = await fs.readFile(gitignorePath, "utf-8");
    expect(content).toBe("test-entry\n");
  });

  it("addToGitignore should handle files without trailing newline", async () => {
    await fs.writeFile(gitignorePath, "existing-entry");

    await addToGitignore("new-entry", testDir);

    const content = await fs.readFile(gitignorePath, "utf-8");
    expect(content).toBe("existing-entry\nnew-entry\n");
  });

  it("removeFromGitignore should remove entry", async () => {
    await fs.writeFile(gitignorePath, "entry1\nentry2\nentry3\n");

    await removeFromGitignore("entry2", testDir);

    const content = await fs.readFile(gitignorePath, "utf-8");
    expect(content).toBe("entry1\nentry3\n");
  });

  it("removeFromGitignore should handle missing file", async () => {
    // Should not throw
    await removeFromGitignore("entry", testDir);
  });

  it("removeFromGitignore should not modify if entry not found", async () => {
    await fs.writeFile(gitignorePath, "entry1\nentry2\n");

    await removeFromGitignore("nonexistent", testDir);

    const content = await fs.readFile(gitignorePath, "utf-8");
    expect(content).toBe("entry1\nentry2\n");
  });
});
