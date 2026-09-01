import { describe, expect, it } from "vitest";
import { currentReframe, draftReframe, reframeSentence, type Reframe } from "./reframe";

function r(id: string, opts: Partial<Reframe> = {}): Reframe {
  return {
    id,
    who: "打饭的人",
    needs: "知道还剩什么",
    why: "白跑一趟",
    hmw: "我们可以怎样提前告诉他",
    supersedes: null,
    confirmedAt: "2026-09-01T00:00:00Z",
    createdAt: "2026-09-01T00:00:00Z",
    ...opts,
  };
}

describe("currentReframe", () => {
  // 被替掉的那一版不再是"现在的问题"，但它还留在列表里——她要看得见自己
  // 原来以为问题是什么。
  it("skips a version that a later one superseded", () => {
    const all = [r("v1"), r("v2", { supersedes: "v1" })];
    expect(currentReframe(all)?.id).toBe("v2");
  });

  // 还没确认的不算数。填了一半的问题陈述读起来像想清楚了，其实没有。
  it("ignores an unconfirmed draft", () => {
    const all = [r("v1"), r("v2", { supersedes: "v1", confirmedAt: null })];
    expect(currentReframe(all)?.id).toBe("v1");
  });

  it("returns null before anything is confirmed", () => {
    expect(currentReframe([r("v1", { confirmedAt: null })])).toBeNull();
    expect(currentReframe([])).toBeNull();
  });

  // 改过三轮之后，只剩最后那一版是"现在的"。
  it("follows a chain of rewrites to the last one", () => {
    const all = [r("v1"), r("v2", { supersedes: "v1" }), r("v3", { supersedes: "v2" })];
    expect(currentReframe(all)?.id).toBe("v3");
  });
});

describe("draftReframe", () => {
  it("finds the one still being written", () => {
    const all = [r("v1"), r("v2", { supersedes: "v1", confirmedAt: null })];
    expect(draftReframe(all)?.id).toBe("v2");
  });

  it("is null when everything is settled", () => {
    expect(draftReframe([r("v1")])).toBeNull();
  });
});

describe("reframeSentence", () => {
  it("reads as one sentence", () => {
    expect(reframeSentence({ who: "阿姨", needs: "少剩点", why: "倒掉可惜" })).toBe(
      "阿姨 需要 少剩点，因为 倒掉可惜。",
    );
  });

  // 写到一半也要能读——她是一句一句填的，中间每一步都会看见这句话。
  it("shows blanks for the parts not written yet", () => {
    expect(reframeSentence({ who: "阿姨", needs: "", why: "" })).toBe("阿姨 需要 ……，因为 ……。");
  });

  it("is empty before she has written anything", () => {
    expect(reframeSentence({ who: "", needs: "", why: "" })).toBe("");
  });
});
