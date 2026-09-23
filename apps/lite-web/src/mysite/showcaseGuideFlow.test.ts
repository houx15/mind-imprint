import { describe, expect, it } from "vitest";
import type { ShowcaseState } from "../api/showcase";
import { destinationEditor, initialShowcaseGuide } from "./showcaseGuideFlow";

const state = (overrides: Partial<ShowcaseState> = {}) => ({
  draft: {} as ShowcaseState["draft"], revision: 0, published: false, hasLegacySite: false, ...overrides,
});

describe("showcase guide state", () => {
  it("starts a never-saved page with the welcome, then resumes a saved build step", () => {
    expect(initialShowcaseGuide(state())).toEqual({ phase: "welcome", stage: "design" });
    expect(initialShowcaseGuide(state({ revision: 2, draft: { guideStage: "profile" } as ShowcaseState["draft"] })))
      .toEqual({ phase: "build", stage: "profile" });
  });

  it("asks for a modification on completed, published, and legacy pages", () => {
    for (const changes of [
      { draft: { guideCompleted: true, guideStage: "finish" } as ShowcaseState["draft"] },
      { published: true },
      { hasLegacySite: true },
      { revision: 1 },
    ]) expect(initialShowcaseGuide(state(changes))).toEqual({ phase: "revise", stage: "revise" });
  });

  it("maps specific requests to the correct editor subtab", () => {
    expect(destinationEditor("hero-image")).toEqual({ stage: "hero", heroTab: "image" });
    expect(destinationEditor("profile-avatar")).toEqual({ stage: "profile", profileTab: "avatar" });
    expect(destinationEditor("works")).toEqual({ stage: "works" });
  });
});
