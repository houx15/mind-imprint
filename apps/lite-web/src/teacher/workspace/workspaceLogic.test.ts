import { describe, expect, it } from "vitest";
import { applyPatch, clampChoices, isCurrentTurn, trimTurns, TURNS_WINDOW, MAX_CHOICES } from "./workspaceLogic";

describe("applyPatch", () => {
  // 一轮在飞的时候老师改了截止时间：patch 不能把她的修改抹掉。
  it("keeps a field the teacher edited while the turn was in flight", () => {
    const snapshot = { title: "旧标题", dueInput: "2026-09-20T18:00" };
    const current = { title: "旧标题", dueInput: "2026-09-21T09:00" }; // 老师手改过
    const { next, kept } = applyPatch(current, snapshot, { title: "新标题", dueInput: "2026-09-22T18:00" });
    expect(next.title).toBe("新标题");
    expect(next.dueInput).toBe("2026-09-21T09:00");
    expect(kept).toEqual(["dueInput"]);
  });

  it("applies every field when the teacher changed nothing", () => {
    const snapshot = { title: "旧", dueInput: "" };
    const { next, kept } = applyPatch(snapshot, snapshot, { title: "新" });
    expect(next.title).toBe("新");
    expect(kept).toEqual([]);
  });

  it("is identity for an empty patch", () => {
    const cur = { title: "甲", dueInput: "" };
    expect(applyPatch(cur, cur, {}).next).toEqual(cur);
  });

  // 花名册重新拉一次会造出一份新数组，但学生没有变——不能被误判成老师改过。
  it("treats a rebuilt-but-equal array as unedited", () => {
    const snapshot = { userIds: ["a", "b", "c"] };
    const current = { userIds: [...snapshot.userIds] }; // same students, new array identity
    const { next, kept } = applyPatch(current, snapshot, { userIds: ["a", "b"] });
    expect(next.userIds).toEqual(["a", "b"]);
    expect(kept).toEqual([]);
  });

  // 老师在这一轮里真的勾掉了一个学生：这份修改必须留住，不能被 AI 的名单覆盖。
  it("treats a genuinely changed array as edited and keeps it", () => {
    const snapshot = { userIds: ["a", "b", "c"] };
    const current = { userIds: ["a", "b"] }; // she unchecked "c"
    const { next, kept } = applyPatch(current, snapshot, { userIds: ["a", "b", "c", "d"] });
    expect(next.userIds).toEqual(["a", "b"]);
    expect(kept).toEqual(["userIds"]);
  });
});

describe("trimTurns", () => {
  it("keeps the most recent TURNS_WINDOW turns", () => {
    const turns = Array.from({ length: 20 }, (_, i) => ({ role: "teacher" as const, text: String(i) }));
    const got = trimTurns(turns);
    expect(got).toHaveLength(TURNS_WINDOW);
    expect(got[got.length - 1]!.text).toBe("19");
  });

  it("leaves a short thread alone", () => {
    const turns = [{ role: "teacher" as const, text: "a" }];
    expect(trimTurns(turns)).toEqual(turns);
  });
});

describe("isCurrentTurn", () => {
  it("is current when neither the generation nor the class has moved", () => {
    expect(isCurrentTurn({ gen: 1, classId: "A" }, { gen: 1, classId: "A" })).toBe(true);
  });

  it("is stale once the generation has moved, even if the class matches", () => {
    // 老师从 A 切到 B 又切回 A：class 一样，但这不是同一轮对话了。
    expect(isCurrentTurn({ gen: 1, classId: "A" }, { gen: 3, classId: "A" })).toBe(false);
  });

  it("is stale when only the class has moved", () => {
    expect(isCurrentTurn({ gen: 1, classId: "A" }, { gen: 1, classId: "B" })).toBe(false);
  });
});

describe("clampChoices", () => {
  it("drops blank labels and caps at MAX_CHOICES", () => {
    const got = clampChoices([
      { id: "1", label: "论证结构" }, { id: "2", label: "  " }, { id: "3", label: "证据使用" },
      { id: "4", label: "语言表达" }, { id: "5", label: "篇幅" }, { id: "6", label: "体裁" },
    ]);
    expect(got).toHaveLength(MAX_CHOICES);
    expect(got.every((c) => c.label.trim() !== "")).toBe(true);
  });
});
