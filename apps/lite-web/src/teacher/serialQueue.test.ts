import { describe, expect, it } from "vitest";
import { createSerialQueue } from "./serialQueue";

// The editor's saves and hide toggles share this lane. Two chains that waited
// on each other deadlocked (follow-ups T2 fix round 1); these pin the lane.

const tick = (ms = 0) => new Promise<void>((r) => setTimeout(r, ms));

describe("createSerialQueue", () => {
  it("runs tasks one at a time, in enqueue order", async () => {
    const q = createSerialQueue();
    const log: string[] = [];
    let running = 0;
    let maxRunning = 0;
    const task = (name: string, ms: number) => async () => {
      running++;
      maxRunning = Math.max(maxRunning, running);
      log.push(`start ${name}`);
      await tick(ms);
      log.push(`end ${name}`);
      running--;
      return name;
    };
    const results = await Promise.all([q.enqueue(task("a", 20)), q.enqueue(task("b", 0)), q.enqueue(task("c", 5))]);
    expect(results).toEqual(["a", "b", "c"]);
    expect(log).toEqual(["start a", "end a", "start b", "end b", "start c", "end c"]);
    expect(maxRunning).toBe(1);
  });

  it("keeps going after a task rejects, and hands the rejection to its caller", async () => {
    const q = createSerialQueue();
    const failed = q.enqueue(async () => {
      throw new Error("PATCH 500");
    });
    const next = q.enqueue(async () => "saved");
    await expect(failed).rejects.toThrow("PATCH 500");
    await expect(next).resolves.toBe("saved");
  });

  it("does not deadlock when a running task enqueues another", async () => {
    const q = createSerialQueue();
    const log: string[] = [];
    let inner: Promise<string> | null = null;
    const outer = q.enqueue(async () => {
      log.push("outer");
      // Enqueued from inside, not awaited: it runs after this task settles.
      inner = q.enqueue(async () => {
        log.push("inner");
        return "inner done";
      });
      return "outer done";
    });
    await expect(outer).resolves.toBe("outer done");
    expect(inner).not.toBeNull();
    await expect(inner!).resolves.toBe("inner done");
    expect(log).toEqual(["outer", "inner"]);
  });

  it("idle settles after everything enqueued so far, including a rejection", async () => {
    const q = createSerialQueue();
    const log: string[] = [];
    void q.enqueue(async () => {
      await tick(10);
      log.push("a");
    });
    q.enqueue(async () => {
      log.push("b");
      throw new Error("x");
    }).catch(() => undefined);
    await q.idle();
    expect(log).toEqual(["a", "b"]);
  });
});
