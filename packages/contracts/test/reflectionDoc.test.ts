import { describe, it, expect } from "vitest";
import { ReflectionDoc } from "../src/reflectionDoc";

describe("ReflectionDoc", () => {
  it("parses answers + done", () => {
    const ok = ReflectionDoc.parse({ answers: ["a", "b", "c", "d", "e"], done: true });
    expect(ok.answers).toHaveLength(5);
  });
  it("rejects a non-boolean done", () => {
    expect(() => ReflectionDoc.parse({ answers: [], done: "yes" } as any)).toThrow();
  });
});
