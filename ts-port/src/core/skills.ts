/**
 * Skills installer - fetches skills from GitHub during `rafa init`
 *
 * Downloads and extracts skills from the pablasso/skills repository.
 * Only extracts the required skill directories.
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import { createWriteStream } from "node:fs";
import { pipeline } from "node:stream/promises";
import { createGunzip } from "node:zlib";
import { Readable } from "node:stream";
import * as tar from "tar";

const DEFAULT_SKILLS_URL =
  "https://api.github.com/repos/pablasso/skills/tarball/main";

/**
 * Required skills that must be installed
 */
export const REQUIRED_SKILLS = [
  "prd",
  "prd-review",
  "technical-design",
  "technical-design-review",
  "code-review",
] as const;

export type SkillName = (typeof REQUIRED_SKILLS)[number];

/**
 * Error thrown when skills installation fails
 */
export class SkillsInstallError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SkillsInstallError";
  }
}

/**
 * Interface for HTTP fetching to allow testing
 */
export interface HTTPFetcher {
  fetch(url: string): Promise<Response>;
}

/**
 * Default fetcher using global fetch
 */
const defaultFetcher: HTTPFetcher = {
  fetch: (url: string) =>
    fetch(url, {
      headers: {
        "User-Agent": "rafa",
        Accept: "application/vnd.github+json",
      },
    }),
};

/**
 * Skills installer options
 */
export interface SkillsInstallerOptions {
  targetDir: string;
  skillsUrl?: string;
  httpFetcher?: HTTPFetcher;
}

/**
 * Installs skills from GitHub to the target directory
 */
export async function installSkills(
  options: SkillsInstallerOptions
): Promise<void> {
  const { targetDir, skillsUrl = DEFAULT_SKILLS_URL, httpFetcher = defaultFetcher } = options;

  // Create target directory
  await fs.mkdir(targetDir, { recursive: true });

  // Download tarball
  let response: Response;
  try {
    response = await httpFetcher.fetch(skillsUrl);
  } catch (err) {
    throw new SkillsInstallError(
      `Failed to download skills: ${err instanceof Error ? err.message : err}`
    );
  }

  if (!response.ok) {
    throw new SkillsInstallError(
      `Failed to download skills: HTTP ${response.status}`
    );
  }

  if (!response.body) {
    throw new SkillsInstallError("Failed to download skills: No response body");
  }

  // Extract tarball
  try {
    await extractTarball(response.body, targetDir);
  } catch (err) {
    // Clean up partial installation
    await uninstallSkills(targetDir);
    throw new SkillsInstallError(
      `Failed to extract skills: ${err instanceof Error ? err.message : err}`
    );
  }

  // Verify required skills are present
  try {
    await verifySkills(targetDir);
  } catch (err) {
    // Clean up partial installation
    await uninstallSkills(targetDir);
    throw err;
  }
}

/**
 * Extracts a GitHub tarball to the target directory.
 * Only extracts directories matching REQUIRED_SKILLS.
 */
async function extractTarball(
  body: ReadableStream<Uint8Array>,
  targetDir: string
): Promise<void> {
  // Create a temporary file to store the tarball
  const tmpDir = await fs.mkdtemp(path.join(targetDir, ".tmp-"));
  const tmpFile = path.join(tmpDir, "skills.tar.gz");

  try {
    // Convert web stream to Node stream and save to temp file
    const nodeStream = Readable.fromWeb(body as import("stream/web").ReadableStream<Uint8Array>);
    const writeStream = createWriteStream(tmpFile);
    await pipeline(nodeStream, writeStream);

    // Extract using tar with filter to only include required skills
    let rootPrefix = "";

    await tar.x({
      file: tmpFile,
      cwd: targetDir,
      strip: 1, // Strip the root directory (e.g., pablasso-skills-abc123/)
      filter: (entryPath: string) => {
        // Get the root prefix from the first entry if not set
        if (!rootPrefix) {
          const parts = entryPath.split("/");
          if (parts.length > 0) {
            rootPrefix = parts[0];
          }
        }

        // Get path after root prefix
        const pathParts = entryPath.split("/");
        if (pathParts.length < 2) return false;

        const skillName = pathParts[1];

        // Only extract files inside required skill directories
        return REQUIRED_SKILLS.includes(skillName as SkillName);
      },
    });
  } finally {
    // Clean up temp directory
    await fs.rm(tmpDir, { recursive: true, force: true });
  }
}

/**
 * Verifies all required skills are installed with SKILL.md files
 */
async function verifySkills(targetDir: string): Promise<void> {
  for (const skill of REQUIRED_SKILLS) {
    const skillFile = path.join(targetDir, skill, "SKILL.md");
    try {
      await fs.access(skillFile);
    } catch {
      throw new SkillsInstallError(
        `Required skill "${skill}" missing SKILL.md file`
      );
    }
  }
}

/**
 * Checks if all required skills are installed
 */
export async function areSkillsInstalled(targetDir: string): Promise<boolean> {
  for (const skill of REQUIRED_SKILLS) {
    const skillFile = path.join(targetDir, skill, "SKILL.md");
    try {
      await fs.access(skillFile);
    } catch {
      return false;
    }
  }
  return true;
}

/**
 * Uninstalls skills by removing only the required skill directories
 */
export async function uninstallSkills(targetDir: string): Promise<void> {
  for (const skill of REQUIRED_SKILLS) {
    const skillDir = path.join(targetDir, skill);
    // fs.rm with { force: true } already ignores ENOENT errors
    await fs.rm(skillDir, { recursive: true, force: true });
  }
}
