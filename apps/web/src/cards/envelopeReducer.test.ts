import { describe, it, expect } from "vitest";
import { CardInstance } from "@mind-imprint/contracts";
import { envelopeReducer, newEnvelope } from "./envelopeReducer";

const clock = () => "2026-06-20T10:00:00.000Z";
const start = () => newEnvelope("sift_craap", "t_1");

describe("envelopeReducer", () => {
  it("activate flips proposed -> active without a trace event", () => {
    const env = envelopeReducer(start(), { type: "activate" }, clock);
    expect(env.status).toBe("active");
    expect(env.event_trace).toHaveLength(0);
  });
  it("field_change writes the value and appends a timestamped event", () => {
    const env = envelopeReducer(start(), { type: "field_change", path: "stop", value: "证明中国让地球变绿" }, clock);
    expect(env.field_values.stop).toBe("证明中国让地球变绿");
    expect(env.event_trace).toEqual([{ kind: "field_change", path: "stop", at: clock() }]);
  });
  it("field_change supports nested repeatable paths", () => {
    let env = envelopeReducer(start(), { type: "field_change", path: "sources[0].verdict", value: "存疑" }, clock);
    expect((env.field_values.sources as any)[0].verdict).toBe("存疑");
  });
  it("step_expand and note_open only append events", () => {
    let env = envelopeReducer(start(), { type: "step_expand", step_key: "craap" }, clock);
    env = envelopeReducer(env, { type: "note_open", step_key: "sift" }, clock);
    expect(env.event_trace.map((e) => e.kind)).toEqual(["step_expand", "note_open"]);
    expect(env.field_values).toEqual({});
  });
  it("skip sets status skipped + event", () => {
    const env = envelopeReducer(start(), { type: "skip" }, clock);
    expect(env.status).toBe("skipped");
    expect(env.event_trace.at(-1)).toEqual({ kind: "skip", at: clock() });
  });
  it("submit sets status completed, completed_at, + event; output is schema-valid", () => {
    const env = envelopeReducer(start(), { type: "submit" }, clock);
    expect(env.status).toBe("completed");
    expect(env.completed_at).toBe(clock());
    expect(CardInstance.safeParse(env).success).toBe(true);
  });
});
