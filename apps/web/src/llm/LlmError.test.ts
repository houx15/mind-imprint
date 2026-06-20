import { describe, it, expect } from "vitest";
import { LlmError } from "./LlmError";

describe("LlmError", () => {
  it("is an Error named LlmError carrying status/provider/body", () => {
    const e = new LlmError("boom", { status: 401, provider: "openai", body: { x: 1 } });
    expect(e).toBeInstanceOf(Error);
    expect(e.name).toBe("LlmError");
    expect(e.message).toBe("boom");
    expect(e.status).toBe(401);
    expect(e.provider).toBe("openai");
    expect(e.body).toEqual({ x: 1 });
  });
  it("defaults optional fields to undefined", () => {
    const e = new LlmError("x");
    expect(e.status).toBeUndefined();
    expect(e.provider).toBeUndefined();
  });
});
