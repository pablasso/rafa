import { describe, it, expect } from "vitest";
import { generateId } from "../utils/id.js";

describe("generateId", () => {
  it("generates a 6-character ID", () => {
    const id = generateId();
    expect(id).toHaveLength(6);
  });

  it("generates only alphanumeric characters", () => {
    const id = generateId();
    expect(id).toMatch(/^[a-zA-Z0-9]+$/);
  });

  it("generates unique IDs", () => {
    const ids = new Set<string>();
    for (let i = 0; i < 100; i++) {
      ids.add(generateId());
    }
    // With 62^6 possibilities, 100 IDs should all be unique
    expect(ids.size).toBe(100);
  });
});
