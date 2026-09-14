import { describe, expect, it } from "vitest";
import { treeCopy } from "./treeCopy";

// Why this test exists: the tree renders the same component for the student
// and (readOnly) for her teacher. Two invariants nobody can keep by eye across
// later edits — the student's wording must not drift, and no second-person
// line may leak onto the teacher screen.
describe("treeCopy", () => {
  it("keeps the student's strings exactly as they were", () => {
    const s = treeCopy(false);
    expect(s.header).toBe("我的兴趣树 · INTEREST TREE");
    expect(s.intro).toBe("你的树刚开始长。每读完一篇、写完一篇、做完一个项目，它就会多一个词。");
    expect(s.evidenceLabel).toBe("你自己写的");
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
