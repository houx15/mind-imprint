import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import toulmin from "../cards/toulmin.json";
import argumentMap from "../cards/argument-map.json";
import factOpinionValue from "../cards/fact-opinion-value.json";

// Item D (studio batch 5) · these three writing cards previously had no
// sentence_frames (only pee/concession did). Frames are BLANK-fill skeletons
// the student adapts — never finished sentences about her own thesis (铁律①:
// 印记 never writes the deliverable) — so every frame must contain at least
// one "____" blank and must parse through the shared CardSpec contract.
describe("sentence_frames on the writing deck's remaining cards", () => {
  const cards: [string, unknown][] = [
    ["toulmin", toulmin],
    ["argument-map", argumentMap],
    ["fact-opinion-value", factOpinionValue],
  ];

  it("all three parse against CardSpec", () => {
    for (const [name, raw] of cards) {
      const r = CardSpec.safeParse(raw);
      if (!r.success) throw new Error(`${name}: ${JSON.stringify(r.error.issues, null, 2)}`);
    }
  });

  it("each card has at least one step with 2-4 blank-fill frames", () => {
    for (const [name, raw] of cards) {
      const spec = CardSpec.parse(raw);
      const stepsWithFrames = spec.steps.filter((s) => (s.sentence_frames?.length ?? 0) > 0);
      expect(stepsWithFrames.length, `${name} should have at least one step with sentence_frames`).toBeGreaterThan(0);
      for (const step of stepsWithFrames) {
        const frames = step.sentence_frames!;
        expect(frames.length, `${name}/${step.key} frame count`).toBeGreaterThanOrEqual(2);
        expect(frames.length, `${name}/${step.key} frame count`).toBeLessThanOrEqual(4);
        for (const f of frames) {
          // a blank-fill skeleton, never a finished sentence about a specific thesis
          expect(f, `${name}/${step.key}: "${f}" is missing a ____ blank`).toContain("____");
        }
      }
    }
  });
});
