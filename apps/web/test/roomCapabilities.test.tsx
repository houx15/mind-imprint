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
  it("demo is read-only pro, not a third lifecycle", () => {
    expect(DEMO_CAPABILITIES.mode).toBe("demo");
    expect(DEMO_CAPABILITIES.evidenceMap).toBe(PRO_CAPABILITIES.evidenceMap);
  });
});
