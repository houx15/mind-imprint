import { describe, it, expect } from "vitest";
import { Verb, AgentOutput } from "../src/agentOutput";

describe("agent output (C3)", () => {
  it("verb set is the closed list", () => {
    for (const v of ["surface_card","post_intervention","check_gate","plan","replan",
                     "advance","route","invite_commit","reply","propose","order_review"]) {
      expect(Verb.safeParse(v).success).toBe(true);
    }
    expect(Verb.safeParse("write_essay").success).toBe(false);
  });
  it("order_review verb is accepted (S5 whole-draft review work-order)", () => {
    expect(Verb.parse("order_review")).toBe("order_review");
  });
  it("question/diagnostic carry anchor + criterion + body", () => {
    expect(AgentOutput.safeParse({ type: "question", anchor: { kind: "graph_node", id: "n1" },
      criterion: "D4", body: "Who holds the opposing view?" }).success).toBe(true);
  });
  it("reference must quote a student artifact with provenance", () => {
    expect(AgentOutput.safeParse({ type: "reference", anchor: { kind: "artifact", id: "a1" },
      quote: "my earlier claim", provenance: "artifact:a1" }).success).toBe(true);
    expect(AgentOutput.safeParse({ type: "reference", anchor: { kind: "artifact", id: "a1" },
      quote: "x" }).success).toBe(false); // provenance required
  });
  it("accepts a reply with a body", () => {
    expect(AgentOutput.safeParse({ type: "reply", body: "让我们想想。" }).success).toBe(true);
  });
  it("rejects a reply with an empty body", () => {
    expect(AgentOutput.safeParse({ type: "reply", body: "" }).success).toBe(false);
  });
});
