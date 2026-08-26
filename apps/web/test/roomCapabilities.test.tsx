import { describe, expect, it } from "vitest";
import { PRO_CAPABILITIES, DEMO_CAPABILITIES, LITE_READING_CAPABILITIES } from "@/rooms/capabilities";

describe("RoomCapabilities", () => {
  it("pro keeps the whole project lifecycle", () => {
    expect(PRO_CAPABILITIES.mode).toBe("pro");
    expect(PRO_CAPABILITIES.evidenceMap).toBe(true);
    expect(PRO_CAPABILITIES.proposalImpact).toBe(true);
  });
  it("lite reading drops every project-lifecycle surface", () => {
    expect(LITE_READING_CAPABILITIES.mode).toBe("lite");
    for (const k of ["plan", "evidenceMap", "explorationLeads", "proposalImpact", "essayTrack"] as const) {
      expect(LITE_READING_CAPABILITIES[k]).toBe(false);
    }
  });
  it("lite reading does not turn on exemplars — that belongs to a future lite writing preset", () => {
    // exemplars denotes the English-writing 示范 paragraphs, a writing-room
    // feature (P3). It is forward-declared on RoomCapabilities but must stay
    // false in the READING preset — its home is a future
    // LITE_WRITING_CAPABILITIES. Asserted explicitly (not folded into the
    // loop above) so a future edit flipping it back to true fails loudly.
    expect(LITE_READING_CAPABILITIES.exemplars).toBe(false);
  });
  it("demo is read-only pro, not a third lifecycle", () => {
    expect(DEMO_CAPABILITIES.mode).toBe("demo");
    expect(DEMO_CAPABILITIES.evidenceMap).toBe(PRO_CAPABILITIES.evidenceMap);
  });
});
