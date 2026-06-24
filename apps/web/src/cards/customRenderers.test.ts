import { describe, it, expect } from "vitest";
import { pickCardBody } from "./customRenderers";
import { CardRenderer } from "./CardRenderer";
import { BeliefSpectrumRenderer } from "./renderers/BeliefSpectrumRenderer";
import { SiftCraapRenderer } from "./renderers/SiftCraapRenderer";

describe("pickCardBody", () => {
  it("falls back to the schema CardRenderer for an unregistered card", () => {
    expect(pickCardBody("anything")).toBe(CardRenderer);
    expect(pickCardBody("unknown_card")).toBe(CardRenderer);
  });
  it("returns the custom renderer for belief-spectrum", () => {
    expect(pickCardBody("belief-spectrum")).toBe(BeliefSpectrumRenderer);
  });
  it("returns the custom renderer for sift_craap", () => {
    expect(pickCardBody("sift_craap")).toBe(SiftCraapRenderer);
  });
});
