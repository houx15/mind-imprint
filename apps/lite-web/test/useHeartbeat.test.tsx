import { render, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * useHeartbeat — the client half of the report's minute count. The one line
 * that matters most: NOTHING is posted while the tab is not visible, so a
 * student who leaves the room open overnight does not accrue hours of
 * "focus time" she never spent. Fake timers throughout — assertions are on
 * call COUNTS, never on wall-clock time.
 */

const heartbeat = vi.fn().mockResolvedValue(undefined);
vi.mock("../src/api/reports", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  sendHeartbeat: (...args: unknown[]) => heartbeat(...args),
}));

const { useHeartbeat } = await import("../src/shared/useHeartbeat");

function Probe({ enabled }: { enabled: boolean }) {
  useHeartbeat("reading", "r1", enabled);
  return null;
}

/** Stubs the read-only `document.visibilityState` getter, restorable via the
 *  returned function — jsdom does not let plain assignment touch it. Saves
 *  whatever OWN descriptor `document` had (none, ordinarily — the getter
 *  lives on `Document.prototype`) and puts exactly that back afterward, so
 *  the stub never leaks into a later test. */
function stubVisibility(state: DocumentVisibilityState) {
  const own = Object.getOwnPropertyDescriptor(document, "visibilityState");
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => state,
  });
  return () => {
    if (own) {
      Object.defineProperty(document, "visibilityState", own);
    } else {
      delete (document as unknown as Record<string, unknown>).visibilityState;
    }
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  heartbeat.mockClear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("useHeartbeat", () => {
  it("posts once per interval while the tab is visible", async () => {
    const restore = stubVisibility("visible");
    render(<Probe enabled={true} />);

    await vi.advanceTimersByTimeAsync(60_000);
    expect(heartbeat).toHaveBeenCalledTimes(1);
    expect(heartbeat).toHaveBeenLastCalledWith("reading", "r1", 60);

    await vi.advanceTimersByTimeAsync(60_000);
    expect(heartbeat).toHaveBeenCalledTimes(2);

    await vi.advanceTimersByTimeAsync(120_000);
    expect(heartbeat).toHaveBeenCalledTimes(4);

    restore();
  });

  it("posts nothing while the tab is hidden", async () => {
    const restore = stubVisibility("hidden");
    render(<Probe enabled={true} />);

    await vi.advanceTimersByTimeAsync(240_000);
    expect(heartbeat).not.toHaveBeenCalled();

    restore();
  });

  it("posts nothing when disabled (a finished atom)", async () => {
    const restore = stubVisibility("visible");
    render(<Probe enabled={false} />);

    await vi.advanceTimersByTimeAsync(240_000);
    expect(heartbeat).not.toHaveBeenCalled();

    restore();
  });

  it("stops on unmount", async () => {
    const restore = stubVisibility("visible");
    const { unmount } = render(<Probe enabled={true} />);

    await vi.advanceTimersByTimeAsync(60_000);
    expect(heartbeat).toHaveBeenCalledTimes(1);

    unmount();

    await vi.advanceTimersByTimeAsync(240_000);
    expect(heartbeat).toHaveBeenCalledTimes(1);

    restore();
  });
});
