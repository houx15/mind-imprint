import { describe, it, expect } from "vitest";
import { CardInstance } from "../src/envelope";

const base = {
  id: "ci_1", card_id: "sift_craap", task_id: "t_1", parent_node_id: null,
  status: "proposed", field_values: {}, event_trace: [], rubric_tags: [],
  created_at: "2026-06-20T10:00:00.000Z", completed_at: null,
};

describe("CardInstance", () => {
  it("accepts a minimal proposed envelope", () => {
    expect(CardInstance.safeParse(base).success).toBe(true);
  });
  it("rejects an unknown status", () => {
    expect(CardInstance.safeParse({ ...base, status: "open" }).success).toBe(false);
  });
  it("rejects a missing card_id", () => {
    const { card_id, ...rest } = base;
    expect(CardInstance.safeParse(rest).success).toBe(false);
  });
  it("accepts a valid field_change trace event", () => {
    const env = { ...base, event_trace: [{ kind: "field_change", path: "sift.stop", at: base.created_at }] };
    expect(CardInstance.safeParse(env).success).toBe(true);
  });
  it("rejects a trace event with an unknown kind", () => {
    const env = { ...base, event_trace: [{ kind: "wiggle", at: base.created_at }] };
    expect(CardInstance.safeParse(env).success).toBe(false);
  });
});
