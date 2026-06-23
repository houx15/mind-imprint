import { describe, it, expect } from "vitest";
import { pickCardBody } from "./customRenderers";
import { CardRenderer } from "./CardRenderer";
import { BeliefSpectrumRenderer } from "./renderers/BeliefSpectrumRenderer";

describe("pickCardBody", () => {
  it("falls back to the schema CardRenderer for an unregistered card", () => {
    expect(pickCardBody("sift_craap")).toBe(CardRenderer);
    expect(pickCardBody("anything")).toBe(CardRenderer);
  });
  it("returns the custom renderer for belief-spectrum", () => {
    expect(pickCardBody("belief-spectrum")).toBe(BeliefSpectrumRenderer);
  });
});
