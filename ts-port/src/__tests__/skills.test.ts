/**
 * Tests for skills installer
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import {
  installSkills,
  uninstallSkills,
  areSkillsInstalled,
  REQUIRED_SKILLS,
  SkillsInstallError,
  type HTTPFetcher,
} from "../core/skills.js";

describe("skills installer", () => {
  let testDir: string;

  beforeEach(async () => {
    testDir = await fs.mkdtemp(path.join(process.cwd(), ".test-skills-"));
  });

  afterEach(async () => {
    await fs.rm(testDir, { recursive: true, force: true });
  });

  describe("REQUIRED_SKILLS", () => {
    it("should contain expected skills", () => {
      expect(REQUIRED_SKILLS).toContain("prd");
      expect(REQUIRED_SKILLS).toContain("prd-review");
      expect(REQUIRED_SKILLS).toContain("technical-design");
      expect(REQUIRED_SKILLS).toContain("technical-design-review");
      expect(REQUIRED_SKILLS).toContain("code-review");
    });
  });

  describe("areSkillsInstalled", () => {
    it("should return false for empty directory", async () => {
      expect(await areSkillsInstalled(testDir)).toBe(false);
    });

    it("should return false if some skills are missing", async () => {
      // Create one skill
      const skillDir = path.join(testDir, "prd");
      await fs.mkdir(skillDir, { recursive: true });
      await fs.writeFile(path.join(skillDir, "SKILL.md"), "# PRD Skill");

      expect(await areSkillsInstalled(testDir)).toBe(false);
    });

    it("should return true if all skills are installed", async () => {
      // Create all required skills
      for (const skill of REQUIRED_SKILLS) {
        const skillDir = path.join(testDir, skill);
        await fs.mkdir(skillDir, { recursive: true });
        await fs.writeFile(path.join(skillDir, "SKILL.md"), `# ${skill} Skill`);
      }

      expect(await areSkillsInstalled(testDir)).toBe(true);
    });

    it("should return false if SKILL.md is missing", async () => {
      // Create all skill directories but missing SKILL.md
      for (const skill of REQUIRED_SKILLS) {
        const skillDir = path.join(testDir, skill);
        await fs.mkdir(skillDir, { recursive: true });
      }

      expect(await areSkillsInstalled(testDir)).toBe(false);
    });
  });

  describe("uninstallSkills", () => {
    it("should remove all skill directories", async () => {
      // Create all required skills
      for (const skill of REQUIRED_SKILLS) {
        const skillDir = path.join(testDir, skill);
        await fs.mkdir(skillDir, { recursive: true });
        await fs.writeFile(path.join(skillDir, "SKILL.md"), `# ${skill} Skill`);
      }

      await uninstallSkills(testDir);

      // Verify all removed
      for (const skill of REQUIRED_SKILLS) {
        const skillDir = path.join(testDir, skill);
        await expect(fs.stat(skillDir)).rejects.toThrow();
      }
    });

    it("should not throw if directories do not exist", async () => {
      await expect(uninstallSkills(testDir)).resolves.not.toThrow();
    });

    it("should preserve other directories", async () => {
      // Create a non-skill directory
      const otherDir = path.join(testDir, "other");
      await fs.mkdir(otherDir, { recursive: true });
      await fs.writeFile(path.join(otherDir, "test.txt"), "test");

      await uninstallSkills(testDir);

      // Verify other directory is preserved
      const stat = await fs.stat(otherDir);
      expect(stat.isDirectory()).toBe(true);
    });
  });

  describe("installSkills", () => {
    it("should throw SkillsInstallError on HTTP error", async () => {
      const mockFetcher: HTTPFetcher = {
        fetch: async () => {
          return new Response(null, { status: 404 });
        },
      };

      await expect(
        installSkills({
          targetDir: testDir,
          httpFetcher: mockFetcher,
        }),
      ).rejects.toThrow(SkillsInstallError);
    });

    it("should throw SkillsInstallError on network error", async () => {
      const mockFetcher: HTTPFetcher = {
        fetch: async () => {
          throw new Error("Network error");
        },
      };

      await expect(
        installSkills({
          targetDir: testDir,
          httpFetcher: mockFetcher,
        }),
      ).rejects.toThrow(SkillsInstallError);
    });

    it("should create target directory if missing", async () => {
      const nestedDir = path.join(testDir, "nested", "skills");
      const mockFetcher: HTTPFetcher = {
        fetch: async () => {
          return new Response(null, { status: 404 });
        },
      };

      // This will fail on HTTP but should create the directory first
      try {
        await installSkills({
          targetDir: nestedDir,
          httpFetcher: mockFetcher,
        });
      } catch {
        // Expected to fail
      }

      // The directory should have been created
      const stat = await fs.stat(nestedDir);
      expect(stat.isDirectory()).toBe(true);
    });
  });
});
