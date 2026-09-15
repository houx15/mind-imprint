import { describe, expect, it } from "vitest";
import { treeCopy } from "./treeCopy";

// Why this test exists: the tree renders the same component for the student
// and (readOnly) for her teacher. Two invariants nobody can keep by eye across
// later edits — the student keeps personal labels, and no second-person
// line may leak onto the teacher screen.
describe("treeCopy", () => {
  it("keeps personal labels in the student view", () => {
    const s = treeCopy(false);
    expect(s.header).toBe("我的兴趣树 · INTEREST TREE");
    expect(s.evidenceLabel).toBe("你的原话");
    expect(s.loadingTitle).toBe("正在读取你的兴趣树");
  });

  it("has no 你 / 我的 anywhere in the read-only copy", () => {
    const t = treeCopy(true);
    for (const v of Object.values(t)) {
      if (v === null) continue;
      expect(v).not.toMatch(/你|我的/);
    }
    expect(t.header).toBe("兴趣树 · INTEREST TREE");
    expect(t.intro).toBeNull();
    expect(t.evidenceLabel).toBe("学生原话");
  });
});
