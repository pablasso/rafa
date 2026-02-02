/**
 * Tests for migration from Go version to TypeScript version
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import {
  needsMigration,
  getVersionInfo,
  writeVersionFile,
  runMigration,
  migrateIfNeeded,
} from "../storage/migration.js";

describe("migration", () => {
  let testDir: string;
  let rafaDir: string;

  beforeEach(async () => {
    // Create a temporary test directory
    testDir = await fs.mkdtemp(path.join(process.cwd(), ".test-migration-"));
    rafaDir = path.join(testDir, ".rafa");
  });

  afterEach(async () => {
    // Clean up test directory
    await fs.rm(testDir, { recursive: true, force: true });
  });

  describe("needsMigration", () => {
    it("should return false if .rafa does not exist", async () => {
      expect(await needsMigration(rafaDir)).toBe(false);
    });

    it("should return true if .rafa exists but version file does not", async () => {
      // Create .rafa directory (simulating Go version state)
      await fs.mkdir(rafaDir, { recursive: true });

      expect(await needsMigration(rafaDir)).toBe(true);
    });

    it("should return false if version file exists", async () => {
      // Create .rafa directory with version file
      await fs.mkdir(rafaDir, { recursive: true });
      await fs.writeFile(
        path.join(rafaDir, "version"),
        JSON.stringify({ version: 2, runtime: "typescript" })
      );

      expect(await needsMigration(rafaDir)).toBe(false);
    });
  });

  describe("getVersionInfo", () => {
    it("should return null if version file does not exist", async () => {
      await fs.mkdir(rafaDir, { recursive: true });

      expect(await getVersionInfo(rafaDir)).toBeNull();
    });

    it("should return version info if file exists", async () => {
      await fs.mkdir(rafaDir, { recursive: true });
      await fs.writeFile(
        path.join(rafaDir, "version"),
        JSON.stringify({ version: 2, runtime: "typescript" })
      );

      const info = await getVersionInfo(rafaDir);
      expect(info).toEqual({ version: 2, runtime: "typescript" });
    });
  });

  describe("writeVersionFile", () => {
    it("should write version file with correct content", async () => {
      await fs.mkdir(rafaDir, { recursive: true });

      await writeVersionFile(rafaDir);

      const content = await fs.readFile(
        path.join(rafaDir, "version"),
        "utf-8"
      );
      const info = JSON.parse(content);

      expect(info.version).toBe(2);
      expect(info.runtime).toBe("typescript");
    });
  });

  describe("runMigration", () => {
    it("should rename progress.log to progress.jsonl in plan directories", async () => {
      // Create Go-style directory structure
      const planDir = path.join(rafaDir, "plans", "abc123-test-plan");
      await fs.mkdir(planDir, { recursive: true });
      await fs.writeFile(
        path.join(planDir, "progress.log"),
        '{"event":"test"}\n'
      );

      await runMigration(rafaDir);

      // Check that progress.log was renamed to progress.jsonl
      const progressJsonl = path.join(planDir, "progress.jsonl");
      const stat = await fs.stat(progressJsonl);
      expect(stat.isFile()).toBe(true);

      // Check that progress.log no longer exists
      await expect(
        fs.stat(path.join(planDir, "progress.log"))
      ).rejects.toThrow();

      // Content should be preserved
      const content = await fs.readFile(progressJsonl, "utf-8");
      expect(content).toBe('{"event":"test"}\n');
    });

    it("should handle multiple plan directories", async () => {
      // Create multiple Go-style plan directories
      const plan1Dir = path.join(rafaDir, "plans", "abc123-plan-one");
      const plan2Dir = path.join(rafaDir, "plans", "def456-plan-two");

      await fs.mkdir(plan1Dir, { recursive: true });
      await fs.mkdir(plan2Dir, { recursive: true });

      await fs.writeFile(path.join(plan1Dir, "progress.log"), "log1\n");
      await fs.writeFile(path.join(plan2Dir, "progress.log"), "log2\n");

      const result = await runMigration(rafaDir);

      expect(result.progressLogsMigrated).toBe(2);

      // Check both were renamed
      await fs.stat(path.join(plan1Dir, "progress.jsonl"));
      await fs.stat(path.join(plan2Dir, "progress.jsonl"));
    });

    it("should clear sessions directory", async () => {
      // Create sessions directory with old session files
      const sessionsDir = path.join(rafaDir, "sessions");
      await fs.mkdir(sessionsDir, { recursive: true });
      await fs.writeFile(
        path.join(sessionsDir, "session1.json"),
        '{"old":"session"}'
      );
      await fs.writeFile(
        path.join(sessionsDir, "session2.json"),
        '{"old":"session2"}'
      );

      const result = await runMigration(rafaDir);

      expect(result.sessionCleared).toBe(true);

      // Sessions directory should exist but be empty
      const stat = await fs.stat(sessionsDir);
      expect(stat.isDirectory()).toBe(true);

      const entries = await fs.readdir(sessionsDir);
      expect(entries).toHaveLength(0);
    });

    it("should write version file", async () => {
      await fs.mkdir(rafaDir, { recursive: true });

      await runMigration(rafaDir);

      const info = await getVersionInfo(rafaDir);
      expect(info).toEqual({ version: 2, runtime: "typescript" });
    });

    it("should handle missing plans directory", async () => {
      await fs.mkdir(rafaDir, { recursive: true });

      const result = await runMigration(rafaDir);

      expect(result.progressLogsMigrated).toBe(0);
      expect(result.migrated).toBe(true);
    });

    it("should handle missing sessions directory", async () => {
      await fs.mkdir(rafaDir, { recursive: true });

      const result = await runMigration(rafaDir);

      expect(result.sessionCleared).toBe(false);
      expect(result.migrated).toBe(true);
    });
  });

  describe("migrateIfNeeded", () => {
    it("should not migrate if .rafa does not exist", async () => {
      const result = await migrateIfNeeded(rafaDir);

      expect(result).toBe(false);
      // No version file should be created
      await expect(fs.stat(path.join(rafaDir, "version"))).rejects.toThrow();
    });

    it("should migrate if .rafa exists without version file", async () => {
      // Create Go-style .rafa directory
      await fs.mkdir(path.join(rafaDir, "plans"), { recursive: true });

      const result = await migrateIfNeeded(rafaDir);

      expect(result).toBe(true);
      // Version file should exist
      const info = await getVersionInfo(rafaDir);
      expect(info).toEqual({ version: 2, runtime: "typescript" });
    });

    it("should not migrate if version file exists", async () => {
      // Create TypeScript-style .rafa directory
      await fs.mkdir(rafaDir, { recursive: true });
      await writeVersionFile(rafaDir);

      const result = await migrateIfNeeded(rafaDir);

      expect(result).toBe(false);
    });

    it("should be idempotent (can be called multiple times)", async () => {
      // Create Go-style .rafa directory
      const planDir = path.join(rafaDir, "plans", "abc123-test");
      await fs.mkdir(planDir, { recursive: true });
      await fs.writeFile(path.join(planDir, "progress.log"), "test\n");

      // First migration
      await migrateIfNeeded(rafaDir);

      // Second call should not fail
      const result = await migrateIfNeeded(rafaDir);
      expect(result).toBe(false);

      // Check state is correct
      const info = await getVersionInfo(rafaDir);
      expect(info?.version).toBe(2);
    });
  });
});
