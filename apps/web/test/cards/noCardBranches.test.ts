// @vitest-environment node
import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

describe("schema-driven guarantee", () => {
  it("CardRenderer contains no card-id-specific branches", () => {
    const src = readFileSync(fileURLToPath(new URL("../../src/cards/CardRenderer.tsx", import.meta.url)), "utf8");
    expect(src).not.toMatch(/sift_craap|concession|card\.id\s*===|card_id\s*===/);
  });
});
