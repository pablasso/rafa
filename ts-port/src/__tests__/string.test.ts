import { describe, it, expect } from "vitest";
import { toKebabCase, generateTaskId } from "../utils/string.js";

describe("toKebabCase", () => {
  it("converts simple string to lowercase", () => {
    expect(toKebabCase("HelloWorld")).toBe("helloworld");
  });

  it("replaces spaces with hyphens", () => {
    expect(toKebabCase("hello world")).toBe("hello-world");
  });

  it("replaces underscores with hyphens", () => {
    expect(toKebabCase("hello_world")).toBe("hello-world");
  });

  it("removes non-alphanumeric characters", () => {
    expect(toKebabCase("hello@world!test")).toBe("helloworldtest");
  });

  it("collapses multiple hyphens", () => {
    expect(toKebabCase("hello   world")).toBe("hello-world");
    expect(toKebabCase("hello---world")).toBe("hello-world");
    expect(toKebabCase("hello _ - world")).toBe("hello-world");
  });

  it("trims leading and trailing hyphens", () => {
    expect(toKebabCase("-hello world-")).toBe("hello-world");
    expect(toKebabCase("  hello world  ")).toBe("hello-world");
  });

  it("handles mixed case with spaces", () => {
    expect(toKebabCase("My Feature Name")).toBe("my-feature-name");
  });

  it("handles already kebab-case strings", () => {
    expect(toKebabCase("already-kebab-case")).toBe("already-kebab-case");
  });

  it("handles numbers", () => {
    expect(toKebabCase("feature 123")).toBe("feature-123");
  });

  it("handles empty string", () => {
    expect(toKebabCase("")).toBe("");
  });
});

describe("generateTaskId", () => {
  it("generates t01 for index 0", () => {
    expect(generateTaskId(0)).toBe("t01");
  });

  it("generates t02 for index 1", () => {
    expect(generateTaskId(1)).toBe("t02");
  });

  it("generates t10 for index 9", () => {
    expect(generateTaskId(9)).toBe("t10");
  });

  it("generates t99 for index 98", () => {
    expect(generateTaskId(98)).toBe("t99");
  });

  it("generates t100 for index 99", () => {
    expect(generateTaskId(99)).toBe("t100");
  });
});
