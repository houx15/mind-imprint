import { describe, it, expect } from "vitest";
import { SelectionEval } from "../src/reading";

describe("SelectionEval", () => {
  it("parses a program-verdict payload", () => {
    const ok = SelectionEval.parse({
      verdict: "rethink",
      verdictLabel: "暂不匹配",
      verdictReason: "对象没找对",
      checks: [
        {
          key: "target",
          label: "找对对象",
          status: "miss",
          evidence: "",
          explanation: "x",
        },
      ],
      finding: "f",
      judgment: "j",
      support: "s",
      caveat: "",
      nextStep: "n",
      spanIds: ["s0"],
    });
    expect(ok.verdict).toBe("rethink");
  });
  it("rejects an unknown verdict", () => {
    expect(() => SelectionEval.parse({ verdict: "amazing" } as any)).toThrow();
  });
});
