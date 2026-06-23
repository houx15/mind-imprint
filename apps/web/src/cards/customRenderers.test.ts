import { describe, it, expect } from "vitest";
import { pickCardBody } from "./customRenderers";
import { CardRenderer } from "./CardRenderer";

describe("pickCardBody", () => {
  it("falls back to the schema CardRenderer for an unregistered card", () => {
    expect(pickCardBody("sift_craap")).toBe(CardRenderer);
    expect(pickCardBody("anything")).toBe(CardRenderer);
  });
});
