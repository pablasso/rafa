import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "fs";
import * as path from "path";
import * as os from "os";
import { PlanLock, processExists } from "../utils/lock.js";

describe("PlanLock", () => {
  let testDir: string;

  beforeEach(() => {
    testDir = fs.mkdtempSync(path.join(os.tmpdir(), "lock-test-"));
  });

  afterEach(() => {
    fs.rmSync(testDir, { recursive: true, force: true });
  });

  describe("acquire", () => {
    it("creates a lock file with the current PID", async () => {
      const lock = new PlanLock(testDir);
      await lock.acquire();

      const lockPath = path.join(testDir, "run.lock");
      expect(fs.existsSync(lockPath)).toBe(true);

      const content = fs.readFileSync(lockPath, "utf-8");
      expect(parseInt(content, 10)).toBe(process.pid);
    });

    it("throws when lock is held by another running process", async () => {
      const lockPath = path.join(testDir, "run.lock");
      // Write current process PID to simulate an active lock
      fs.writeFileSync(lockPath, String(process.pid));

      const lock = new PlanLock(testDir);
      await expect(lock.acquire()).rejects.toThrow(/already running/);
    });

    it("cleans up stale lock from dead process", async () => {
      const lockPath = path.join(testDir, "run.lock");
      // Use an invalid PID that's unlikely to exist
      const stalePid = 999999999;
      fs.writeFileSync(lockPath, String(stalePid));

      const lock = new PlanLock(testDir);
      await lock.acquire();

      const content = fs.readFileSync(lockPath, "utf-8");
      expect(parseInt(content, 10)).toBe(process.pid);
    });

    it("cleans up lock with invalid PID content", async () => {
      const lockPath = path.join(testDir, "run.lock");
      fs.writeFileSync(lockPath, "not-a-pid");

      const lock = new PlanLock(testDir);
      await lock.acquire();

      const content = fs.readFileSync(lockPath, "utf-8");
      expect(parseInt(content, 10)).toBe(process.pid);
    });
  });

  describe("release", () => {
    it("removes the lock file", async () => {
      const lock = new PlanLock(testDir);
      await lock.acquire();

      const lockPath = path.join(testDir, "run.lock");
      expect(fs.existsSync(lockPath)).toBe(true);

      lock.release();
      expect(fs.existsSync(lockPath)).toBe(false);
    });

    it("does not throw when lock file does not exist", () => {
      const lock = new PlanLock(testDir);
      expect(() => lock.release()).not.toThrow();
    });
  });

  describe("getLockInfo", () => {
    it("returns null when no lock exists", () => {
      const lock = new PlanLock(testDir);
      expect(lock.getLockInfo()).toBeNull();
    });

    it("returns PID when lock exists", async () => {
      const lock = new PlanLock(testDir);
      await lock.acquire();

      const info = lock.getLockInfo();
      expect(info).not.toBeNull();
      expect(info?.pid).toBe(process.pid);
    });

    it("returns null for invalid PID content", () => {
      const lockPath = path.join(testDir, "run.lock");
      fs.writeFileSync(lockPath, "not-a-pid");

      const lock = new PlanLock(testDir);
      expect(lock.getLockInfo()).toBeNull();
    });
  });

  describe("isStale", () => {
    it("returns false when no lock exists", () => {
      const lock = new PlanLock(testDir);
      expect(lock.isStale()).toBe(false);
    });

    it("returns false when lock is held by running process", async () => {
      const lock = new PlanLock(testDir);
      await lock.acquire();

      expect(lock.isStale()).toBe(false);
    });

    it("returns true when lock is held by dead process", () => {
      const lockPath = path.join(testDir, "run.lock");
      // Use an invalid PID that's unlikely to exist
      fs.writeFileSync(lockPath, "999999999");

      const lock = new PlanLock(testDir);
      expect(lock.isStale()).toBe(true);
    });
  });
});

describe("processExists", () => {
  it("returns true for current process", () => {
    expect(processExists(process.pid)).toBe(true);
  });

  it("returns false for invalid PID", () => {
    // Use a very large PID that's unlikely to exist
    expect(processExists(999999999)).toBe(false);
  });

});
