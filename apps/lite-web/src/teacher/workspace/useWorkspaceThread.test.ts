import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useWorkspaceThread, type ThreadReply } from "./useWorkspaceThread";

// Hook-level state-machine tests (no DOM assertions). They pin the wiring
// that the pure tests in threadLogic.test.ts cannot see:
// - the patch is applied against the LIVE artifact, not the snapshot
//   (`applyPatch(snapshot, snapshot, …)` was a real silent bug here once);
// - a stale response is stopped before `setArtifact`.

interface Card {
  classId: string;
  title: string;
  dueInput: string;
}

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

function setup() {
  const pending = deferred<ThreadReply<Card>>();
  const applied: Card[] = [];
  const hook = renderHook(
    ({ artifact }: { artifact: Card }) =>
      useWorkspaceThread<Card>({
        artifact,
        setArtifact: (next) => applied.push(next),
        scopeOf: (c) => c.classId,
        post: () => pending.promise,
        describeError: () => "对话失败",
      }),
    { initialProps: { artifact: { classId: "class-a", title: "", dueInput: "2026-09-20T18:00" } } },
  );
  return { hook, pending, applied };
}

const patchReply: ThreadReply<Card> = {
  reply: "已填写",
  patch: { title: "气候阅读", dueInput: "2026-09-22T18:00" },
  choices: [],
  cards: [],
};

describe("useWorkspaceThread", () => {
  it("keeps a field she edited while the turn was in flight", async () => {
    const { hook, pending, applied } = setup();
    act(() => {
      expect(hook.result.current.run({ text: "布置一篇阅读" })).toBe(true);
    });
    expect(hook.result.current.busy).toBe(true);

    // She edits the due time while the model is thinking.
    hook.rerender({ artifact: { classId: "class-a", title: "", dueInput: "2026-09-21T09:00" } });

    await act(async () => {
      pending.resolve(patchReply);
      await pending.promise;
    });

    expect(applied).toHaveLength(1);
    expect(applied[0]).toEqual({ classId: "class-a", title: "气候阅读", dueInput: "2026-09-21T09:00" });
    expect(hook.result.current.kept).toEqual(["dueInput"]);
    expect(hook.result.current.busy).toBe(false);
    expect(hook.result.current.turns.map((t) => t.role)).toEqual(["teacher", "ai"]);
  });

  it("does not patch the artifact when the scope changed before the reply", async () => {
    const { hook, pending, applied } = setup();
    act(() => {
      hook.result.current.run({ text: "布置一篇阅读" });
    });

    // The class changed without reset() (e.g. the page replaced it).
    hook.rerender({ artifact: { classId: "class-b", title: "", dueInput: "2026-09-20T18:00" } });

    await act(async () => {
      pending.resolve(patchReply);
      await pending.promise;
    });

    expect(applied).toEqual([]);
    expect(hook.result.current.busy).toBe(false);
    expect(hook.result.current.turns).toEqual([]);
  });

  it("refuses a second turn while one is in flight", () => {
    const { hook } = setup();
    act(() => {
      hook.result.current.setComposer("第二句");
      hook.result.current.run({ text: "第一句" });
    });
    let accepted = true;
    act(() => {
      accepted = hook.result.current.run({ text: "第二句" });
    });
    expect(accepted).toBe(false);
    expect(hook.result.current.composer).toBe("第二句");
  });
});
