import { PROTOCOL_NAME, PROTOCOL_VERSION, parseFrameMessage } from "../../src/blocks/html/protocol";

const ctx = { sessionToken: "tok-123", expectedVersion: "1.0" as const };

function msg(overrides: Record<string, unknown> = {}) {
  return {
    protocol: PROTOCOL_NAME,
    version: PROTOCOL_VERSION,
    sessionToken: "tok-123",
    type: "completed",
    payload: { score: 1 },
    ...overrides,
  };
}

describe("parseFrameMessage", () => {
  it("exposes the frozen protocol identity", () => {
    expect(PROTOCOL_NAME).toBe("mind-course-interaction");
    expect(PROTOCOL_VERSION).toBe("1.0");
  });

  it("accepts a well-formed completed message with the right token/version", () => {
    const r = parseFrameMessage(msg(), ctx);
    expect(r).toEqual({ ok: true, type: "completed", payload: { score: 1 } });
  });

  it("accepts each known type (ready/progress/completed/error)", () => {
    for (const type of ["ready", "progress", "completed", "error"]) {
      const r = parseFrameMessage(msg({ type }), ctx);
      expect(r.ok).toBe(true);
      if (r.ok) expect(r.type).toBe(type);
    }
  });

  it("rejects a wrong session token with reason 'token'", () => {
    expect(parseFrameMessage(msg({ sessionToken: "nope" }), ctx)).toEqual({ ok: false, reason: "token" });
  });

  it("rejects a wrong protocol version with reason 'version'", () => {
    expect(parseFrameMessage(msg({ version: "2.0" }), ctx)).toEqual({ ok: false, reason: "version" });
  });

  it("rejects an unknown type with reason 'type'", () => {
    expect(parseFrameMessage(msg({ type: "explode" }), ctx)).toEqual({ ok: false, reason: "type" });
  });

  it("rejects a non-object with reason 'shape'", () => {
    expect(parseFrameMessage(null, ctx)).toEqual({ ok: false, reason: "shape" });
    expect(parseFrameMessage("completed", ctx)).toEqual({ ok: false, reason: "shape" });
    expect(parseFrameMessage(42, ctx)).toEqual({ ok: false, reason: "shape" });
  });

  it("rejects a message with a missing/mismatched protocol name with reason 'shape'", () => {
    expect(parseFrameMessage(msg({ protocol: undefined }), ctx)).toEqual({ ok: false, reason: "shape" });
    expect(parseFrameMessage(msg({ protocol: "other-app" }), ctx)).toEqual({ ok: false, reason: "shape" });
  });

  it("carries payload through as unknown (may be absent)", () => {
    const r = parseFrameMessage(msg({ type: "ready", payload: undefined }), ctx);
    expect(r).toEqual({ ok: true, type: "ready", payload: undefined });
  });
});
