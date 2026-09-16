import { describe, expect, it } from "vitest";
import {
  beginTurn,
  initialThread,
  resetThread,
  setComposer,
  settleFailure,
  settleSuccess,
  shouldRestore,
  type ThreadState,
} from "./threadLogic";
import { MAX_CHOICES, TURNS_WINDOW, type Choice } from "./workspaceLogic";

type Key = "title" | "dueInput";

function started(state: ThreadState<Key>, text: string, scope = "class-a") {
  const r = beginTurn(state, { text }, scope);
  if (!r) throw new Error("beginTurn refused");
  return r;
}

const reply = (kept: Key[] = []) => ({ reply: "请确认截止时间", choices: [] as Choice[], cards: [], kept });

describe("a successful turn", () => {
  it("appends her turn, then the reply, and records kept fields", () => {
    const a = started(initialThread<Key>(), "布置一篇阅读");
    expect(a.state.busy).toBe(true);
    expect(a.state.turns).toEqual([{ role: "teacher", text: "布置一篇阅读" }]);

    const b = settleSuccess(a.state, a.pending, "class-a", reply(["dueInput"]));
    expect(b.busy).toBe(false);
    expect(b.turns.map((t) => t.role)).toEqual(["teacher", "ai"]);
    expect(b.kept).toEqual(["dueInput"]);
  });

  it("sends only the last TURNS_WINDOW turns, including the new one", () => {
    let s = initialThread<Key>();
    for (let i = 0; i < 6; i++) {
      const r = started(s, `第${i}句`);
      s = settleSuccess(r.state, r.pending, "class-a", reply());
    }
    const r = started(s, "最后一句");
    expect(r.state.turns).toHaveLength(13);
    expect(r.wireTurns).toHaveLength(TURNS_WINDOW);
    expect(r.wireTurns[TURNS_WINDOW - 1]).toEqual({ role: "teacher", text: "最后一句" });
  });

  it("clamps choices", () => {
    const a = started(initialThread<Key>(), "选文章");
    const choices = Array.from({ length: 6 }, (_, i) => ({ id: `c${i}`, label: i === 0 ? " " : `选项${i}` }));
    const b = settleSuccess(a.state, a.pending, "class-a", { ...reply(), choices });
    expect(b.choices).toHaveLength(MAX_CHOICES);
    expect(b.choices[0]!.id).toBe("c1");
  });

  it("refuses a second turn while one is in flight", () => {
    const a = started(initialThread<Key>(), "一");
    expect(beginTurn(a.state, { text: "二" }, "class-a")).toBeNull();
  });

  it("clears the previous kept note when a new turn starts", () => {
    const a = started(initialThread<Key>(), "一");
    const b = settleSuccess(a.state, a.pending, "class-a", reply(["title"]));
    expect(started(b, "二").state.kept).toEqual([]);
  });
});

describe("a failed turn", () => {
  it("rolls back her bubble, sets the error and exposes the failed input", () => {
    const a = started(initialThread<Key>(), "布置一篇阅读");
    const b = settleFailure(a.state, a.pending, "class-a", { text: "布置一篇阅读" }, "对话失败：超时");
    expect(b.turns).toEqual([]);
    expect(b.busy).toBe(false);
    expect(b.error).toBe("对话失败：超时");
    expect(b.failed).toEqual({ text: "布置一篇阅读" });
  });

  it("puts the sentence back into an empty composer", () => {
    const a = started(initialThread<Key>(), "布置一篇阅读");
    const b = settleFailure(a.state, a.pending, "class-a", { text: "布置一篇阅读" }, "对话失败");
    expect(b.composer).toBe("布置一篇阅读");
  });

  it("does not overwrite what she typed while the turn was in flight", () => {
    const a = started(initialThread<Key>(), "布置一篇阅读");
    const typing = setComposer(a.state, "截止时间改到周五");
    const b = settleFailure(typing, a.pending, "class-a", { text: "布置一篇阅读" }, "对话失败");
    expect(b.composer).toBe("截止时间改到周五");
    expect(b.failed).toEqual({ text: "布置一篇阅读" });
  });

  it("does not write a tapped choice's label into the composer", () => {
    const r = beginTurn(initialThread<Key>(), { choiceId: "c1", label: "用这篇", slug: "a" }, "class-a")!;
    const b = settleFailure(r.state, r.pending, "class-a", { choiceId: "c1", label: "用这篇", slug: "a" }, "对话失败");
    expect(b.composer).toBe("");
    expect(b.turns).toEqual([]);
    expect(b.failed).toEqual({ choiceId: "c1", label: "用这篇", slug: "a" });
  });

  it("retrying the restored sentence clears it from the composer", () => {
    const a = started(initialThread<Key>(), "布置一篇阅读");
    const failed = settleFailure(a.state, a.pending, "class-a", { text: "布置一篇阅读" }, "对话失败");
    const retry = started(failed, "布置一篇阅读");
    expect(retry.state.composer).toBe("");
    expect(retry.state.failed).toBeNull();
    expect(retry.state.error).toBeNull();
    expect(retry.state.turns).toEqual([{ role: "teacher", text: "布置一篇阅读" }]);
  });

  it("retrying keeps a composer she has edited", () => {
    const a = started(initialThread<Key>(), "布置一篇阅读");
    const failed = settleFailure(a.state, a.pending, "class-a", { text: "布置一篇阅读" }, "对话失败");
    const edited = setComposer(failed, "布置一篇阅读，周五截止");
    expect(started(edited, "布置一篇阅读").state.composer).toBe("布置一篇阅读，周五截止");
  });
});

describe("shouldRestore", () => {
  it("restores only into an empty or whitespace-only composer", () => {
    expect(shouldRestore("")).toBe(true);
    expect(shouldRestore("  \n")).toBe(true);
    expect(shouldRestore("新的话")).toBe(false);
  });
});

describe("stale responses", () => {
  it("drops a success from a reset generation", () => {
    const a = started(initialThread<Key>(), "一");
    const reset = resetThread(a.state);
    const b = settleSuccess(reset, a.pending, "class-a", reply(["title"]));
    expect(b).toBe(reset);
  });

  it("drops a failure from a reset generation, leaving the new turn busy", () => {
    const a = started(initialThread<Key>(), "一");
    const next = started(resetThread(a.state), "二");
    const b = settleFailure(next.state, a.pending, "class-a", { text: "一" }, "对话失败");
    expect(b).toBe(next.state);
    expect(b.busy).toBe(true);
    expect(b.turns).toEqual([{ role: "teacher", text: "二" }]);
  });

  // 班级在没有 reset() 的情况下变了：旧回复不能落地，busy 也不能一直是 true。
  it("resets the thread when the scope changed (A to B) without a reset", () => {
    const a = started(initialThread<Key>(), "一", "class-a");
    for (const b of [
      settleSuccess(a.state, a.pending, "class-b", reply(["title"])),
      settleFailure(a.state, a.pending, "class-b", { text: "一" }, "对话失败"),
    ]) {
      expect(b.busy).toBe(false);
      expect(b.turns).toEqual([]);
      expect(b.error).toBeNull();
      expect(b.failed).toBeNull();
      expect(b.kept).toEqual([]);
      expect(b.gen).toBe(a.state.gen + 1);
      expect(beginTurn(b, { text: "二" }, "class-b")).not.toBeNull();
    }
  });

  it("drops a response after A to B to A (same scope, newer generation)", () => {
    const a = started(initialThread<Key>(), "一", "class-a");
    const back = resetThread(resetThread(a.state));
    expect(settleSuccess(back, a.pending, "class-a", reply())).toBe(back);
  });
});

describe("resetThread", () => {
  it("clears the conversation, bumps the generation and keeps the composer", () => {
    const a = started(initialThread<Key>(), "一");
    const b = settleFailure(a.state, a.pending, "class-a", { text: "一" }, "对话失败");
    const typed = setComposer(b, "另一句");
    const r = resetThread(typed);
    expect(r.gen).toBe(typed.gen + 1);
    expect(r).toMatchObject({ turns: [], busy: false, error: null, choices: [], cards: [], kept: [], failed: null });
    expect(r.composer).toBe("另一句");
  });

  it("a turn in flight at reset time does not block the next one", () => {
    const a = started(initialThread<Key>(), "一");
    expect(beginTurn(resetThread(a.state), { text: "二" }, "class-b")).not.toBeNull();
  });
});
