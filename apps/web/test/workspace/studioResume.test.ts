import { describe, it, expect } from "vitest";
import { roomForResume, openToolToRoom } from "@/workspace/studioResume";

const base = { openTool: "plan", widthTier: "half", reference: [], updatedAtTurn: 0 } as const;

describe("roomForResume", () => {
  it("proposal_forming resumes into 提案 (forming)", () => {
    expect(roomForResume({ ...base, stage: "proposal_forming" } as any)).toBe("forming");
  });
  it("topic_discussion resumes into 提案 (forming)", () => {
    expect(roomForResume({ ...base, stage: "topic_discussion" } as any)).toBe("forming");
  });
  it("plan_generation and later resume into 管理 (plan board)", () => {
    expect(roomForResume({ ...base, stage: "plan_generation" } as any)).toBe("plan");
    expect(roomForResume({ ...base, stage: "body_writing", openTool: "writing" } as any)).toBe("writing");
  });
  it("reading/writing/reflection openTools win regardless of stage", () => {
    expect(roomForResume({ ...base, stage: "proposal_forming", openTool: "reading" } as any)).toBe("reading");
  });
  it("explicit forming openTool (P2b) maps to 提案 and wins regardless of stage", () => {
    expect(openToolToRoom("forming")).toBe("forming");
    expect(roomForResume({ ...base, stage: "plan_generation", openTool: "forming" } as any)).toBe("forming");
  });
});
