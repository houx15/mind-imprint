// teacher/serialQueue.ts — one lane for every write the parent report editor
// makes (section saves, hide toggles, redraft, the export snapshot GET).
//
// Why one lane: with two chains (saves, toggles) that each waited on the
// other, a save queued during a toggle could end up waiting on the toggle
// while the toggle waited on that save — a deadlock (follow-ups T2 fix round 1).
// With a single FIFO, each task starts only after the previous one settled,
// and responses are applied strictly in the order the requests were made.

export interface SerialQueue {
  /** Runs `task` after every task enqueued before it has settled (resolved or
   * rejected). The returned promise settles with the task's own result.
   * 🚨 A task must not AWAIT a task it enqueues itself: that one runs after it. */
  enqueue<T>(task: () => Promise<T>): Promise<T>;
  /** Settles once every task enqueued so far has settled. */
  idle(): Promise<void>;
}

export function createSerialQueue(): SerialQueue {
  let tail: Promise<void> = Promise.resolve();
  return {
    enqueue<T>(task: () => Promise<T>): Promise<T> {
      const run = tail.then(() => task());
      tail = run.then(
        () => undefined,
        () => undefined,
      );
      return run;
    },
    idle() {
      return tail;
    },
  };
}
