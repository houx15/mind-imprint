import { describe, it, expect } from "vitest";
import {
  LeadStatus,
  LeadOrigin,
  ExplorationLead,
  ExplorationView,
  GuideDirection,
  ExplorationGuide,
} from "../src/exploration";

const lead = {
  id: "l1",
  text: "核实人均口径",
  status: "open" as const,
  origin: "takeaway" as const,
  sourceReferenceId: "r1",
  connectedReferenceId: null,
  position: 0,
};

describe("LeadStatus", () => {
  it("accepts the three statuses", () => {
    expect(LeadStatus.parse("open")).toBe("open");
    expect(LeadStatus.parse("connected")).toBe("connected");
    expect(LeadStatus.parse("pruned")).toBe("pruned");
  });
  it("rejects an empty-string status", () => {
    expect(() => LeadStatus.parse("")).toThrow();
  });
});

describe("LeadOrigin", () => {
  it("accepts the three origins", () => {
    expect(LeadOrigin.parse("takeaway")).toBe("takeaway");
    expect(LeadOrigin.parse("manual")).toBe("manual");
    expect(LeadOrigin.parse("guide")).toBe("guide");
  });
});

describe("ExplorationLead", () => {
  it("parses a valid lead", () => {
    const parsed = ExplorationLead.parse(lead);
    expect(parsed.status).toBe("open");
    expect(parsed.origin).toBe("takeaway");
  });
  it("accepts sourceReferenceId: null (manual/guide-origin leads with no parent source)", () => {
    const parsed = ExplorationLead.parse({ ...lead, sourceReferenceId: null, origin: "manual" });
    expect(parsed.sourceReferenceId).toBeNull();
  });
  it("accepts connectedReferenceId: null (still-open lead)", () => {
    const parsed = ExplorationLead.parse({ ...lead, connectedReferenceId: null });
    expect(parsed.connectedReferenceId).toBeNull();
  });
  it("accepts connectedReferenceId once connected", () => {
    const parsed = ExplorationLead.parse({ ...lead, status: "connected", connectedReferenceId: "r2" });
    expect(parsed.connectedReferenceId).toBe("r2");
  });
  // Enum-empty-string guard: an S2 bug let a naive full-replace PUT persist status: ""
  // and that slipped past validation, breaking the whole library's array parse. LeadStatus
  // is a closed 3-value enum with no "" member — prove ExplorationLead rejects it outright
  // rather than silently degrading.
  it("rejects an empty-string status — must throw, not silently pass", () => {
    expect(() => ExplorationLead.parse({ ...lead, status: "" })).toThrow();
  });
  it("rejects an unknown status", () => {
    expect(() => ExplorationLead.parse({ ...lead, status: "closed" })).toThrow();
  });
  it("rejects an unknown origin", () => {
    expect(() => ExplorationLead.parse({ ...lead, origin: "ai" })).toThrow();
  });
});

describe("ExplorationView", () => {
  it("parses with empty arrays", () => {
    const parsed = ExplorationView.parse({ leads: [], danglingSourceIds: [] });
    expect(parsed.leads).toEqual([]);
    expect(parsed.danglingSourceIds).toEqual([]);
  });
  it("parses with populated leads + dangling sources", () => {
    const parsed = ExplorationView.parse({ leads: [lead], danglingSourceIds: ["r3"] });
    expect(parsed.leads).toHaveLength(1);
    expect(parsed.danglingSourceIds).toEqual(["r3"]);
  });
  // Same guard, at the array level: one bad row must not silently slip through
  // an ExplorationView parse of the whole leads array.
  it("rejects a view whose leads array contains an empty-string status", () => {
    expect(() =>
      ExplorationView.parse({ leads: [{ ...lead, status: "" }], danglingSourceIds: [] })
    ).toThrow();
  });
});

describe("GuideDirection / ExplorationGuide", () => {
  it("parses a single direction", () => {
    const parsed = GuideDirection.parse({
      direction: "对比印度的碳排放增长曲线",
      why: "同为发展中大国，构成有力的类比反例",
    });
    expect(parsed.direction).toBe("对比印度的碳排放增长曲线");
  });
  it("parses a guide with directions:[{direction,why}]", () => {
    const parsed = ExplorationGuide.parse({
      directions: [
        { direction: "对比印度的碳排放增长曲线", why: "同为发展中大国，构成有力的类比反例" },
        { direction: "追踪 IPCC 对人均排放口径的定义", why: "让步段需要口径一致才站得住" },
      ],
    });
    expect(parsed.directions).toHaveLength(2);
    expect(parsed.directions[1]!.why).toBe("让步段需要口径一致才站得住");
  });
  it("parses a guide with an empty directions array", () => {
    const parsed = ExplorationGuide.parse({ directions: [] });
    expect(parsed.directions).toEqual([]);
  });
});
