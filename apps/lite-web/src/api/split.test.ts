import { describe, expect, it } from "vitest";
import { effectiveOwner, shareOfWork, splitTodo, type Owner, type Substep } from "./split";

function sub(id: string, owner: Owner, studentOwner: Owner | null = null): Substep {
  return {
    id,
    title: id,
    owner,
    reason: "因为这样快",
    studentOwner,
    studentReason: studentOwner ? "我想自己来" : "",
    status: "todo",
    confirmedAt: studentOwner ? "2026-09-01T00:00:00Z" : null,
    ordinal: 0,
  };
}

describe("effectiveOwner", () => {
  // 她改过就听她的。这是整张卡的意义：她的修改要真的生效。
  it("prefers her change over what 印记 proposed", () => {
    expect(effectiveOwner(sub("a", "yinji", "student"))).toBe("student");
  });

  it("falls back to 印记's proposal when she has not changed it", () => {
    expect(effectiveOwner(sub("a", "yinji"))).toBe("yinji");
  });
});

describe("shareOfWork", () => {
  // 🚨 这个数字要在她**还能改的时候**看得见。它按生效后的归属算，否则她刚把
  // 一格拿回来，数字却还说印记领着——那这行字就在骗她。
  it("counts by the owner after her changes", () => {
    const subs = [sub("a", "yinji"), sub("b", "yinji", "student"), sub("c", "student")];
    expect(shareOfWork(subs)).toEqual({ yinji: 1, total: 3 });
  });

  it("counts 一起做 as not 印记's alone", () => {
    expect(shareOfWork([sub("a", "both"), sub("b", "yinji")])).toEqual({ yinji: 1, total: 2 });
  });

  it("handles an empty card", () => {
    expect(shareOfWork([])).toEqual({ yinji: 0, total: 0 });
  });
});

describe("splitTodo", () => {
  it("says when there is no card yet", () => {
    expect(splitTodo([])).toBe("无分工");
  });

  // 方案由印记整份提出、她整份确认，所以"还有几格没定"这种中间状态不存在。
  it("has nothing left to ask once a plan exists", () => {
    expect(splitTodo([sub("a", "yinji"), sub("b", "yinji", "student")])).toBe("");
  });
});
