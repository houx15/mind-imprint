import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";

import { ApiError } from "@/api/client";
import { getCourseDefinition } from "@/api/courseDefinition";

// The runtime renderer is stubbed: we test RuntimeCoursePlayer's WIRING (fetch
// the definition, build the four adapters, inject idFactory/clock, surface
// completion as onFinish via CoursePlayer's own `onComplete` callback —
// P1-03), not the renderer's internals (covered in the course-renderer
// package). The stub captures the props it was mounted with and exposes two
// buttons: one that flips the session to `completed` directly (proving that
// alone must NOT fire onFinish anymore), and one that fires `onComplete`
// (the only thing that should).
let lastPlayerProps: any = null;
vi.mock("@mind-imprint/course-renderer", () => ({
  // Passthrough so RuntimeCoursePlayer's <InteractionLoaderProvider> (Slice 3)
  // wrapper renders its children in these host-wiring tests.
  InteractionLoaderProvider: ({ children }: { children: any }) => children,
  CoursePlayer: (props: any) => {
    lastPlayerProps = props;
    return (
      <div data-testid="runtime-player">
        <span>Opening</span>
        <button type="button" onClick={() => void props.adapters.sessionAdapter.setStatus("sid", "completed")}>
          drive-to-completed
        </button>
        <button type="button" onClick={() => props.onComplete?.()}>
          dismiss-closing
        </button>
      </div>
    );
  },
}));

// Session adapter stubbed to a recording fake so RuntimeCoursePlayer's setStatus
// WRAPPER (the onFinish-on-completed seam) is what's under test.
const baseSetStatus = vi.fn().mockResolvedValue(undefined);
// P2-07 lifecycle-flush wiring is what's under test in several specs below —
// this fake must resolve like the real adapter's flush() would.
const baseFlush = vi.fn().mockResolvedValue(undefined);
vi.mock("@/course/apiSessionAdapter", () => ({
  makeApiSessionAdapter: vi.fn(() => ({
    load: vi.fn(),
    create: vi.fn(),
    appendEvent: vi.fn(),
    saveSliceState: vi.fn(),
    saveScene: vi.fn(),
    setStatus: baseSetStatus,
    setCurrent: vi.fn().mockResolvedValue(undefined),
    flush: baseFlush,
    getSaveStatus: vi.fn(() => "idle"),
  })),
}));

// Legacy player stubbed so the 404-routing test can assert which player mounts
// without dragging in the legacy player's own data fetches.
vi.mock("@/shell/courses/CoursePlayer", () => ({
  CoursePlayer: (props: any) => <div data-testid="legacy-player">legacy:{props.courseId}</div>,
}));

vi.mock("@/api/courseDefinition", () => ({
  getCourseDefinition: vi.fn(),
  createCourseSession: vi.fn(),
  saveCourseSession: vi.fn(),
}));
const getDefMock = vi.mocked(getCourseDefinition);

// Asset-url signing: mocked so tests can assert the collect→fetch→resolve
// wiring without a real course-contract document or a network call.
const fetchCourseAssetUrls = vi.fn();
vi.mock("@/api/courseAssetUrls", () => ({
  fetchCourseAssetUrls: (...a: unknown[]) => fetchCourseAssetUrls(...a),
}));

const collectAssetPaths = vi.fn();
vi.mock("@mind-imprint/course-contract", () => ({
  collectAssetPaths: (...a: unknown[]) => collectAssetPaths(...a),
}));

import { RuntimeCoursePlayer } from "@/shell/courses/RuntimeCoursePlayer";
import { CoursesContainer } from "@/shell/courses/CoursesContainer";

const SLUG = "evidence-comparability";
const golden = {
  schemaVersion: "2.0",
  course: { id: "evidence-comparability", title: "Can These Two Claims Be Compared?" },
};
// P2-08/D5 — getCourseDefinition now resolves { definition, hash }; every
// test below mocks it with this fixed hash unless it's specifically about the
// hash wiring itself.
const GOLDEN_HASH = "def-hash-1";

beforeEach(() => {
  vi.clearAllMocks();
  lastPlayerProps = null;
  baseSetStatus.mockResolvedValue(undefined);
  baseFlush.mockResolvedValue(undefined);
  collectAssetPaths.mockReturnValue([]);
  fetchCourseAssetUrls.mockResolvedValue({ assetUrls: {}, expiresAt: "2999-01-01T00:00:00Z" });
});

describe("RuntimeCoursePlayer", () => {
  it("fetches the definition, mounts the runtime player with the four adapters + injected idFactory/clock, and wires onComplete/signalResolver", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    const onFinish = vi.fn();
    const onExit = vi.fn();

    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={onExit} onFinish={onFinish} />);

    // Renders the Opening (the runtime player) once the definition resolves.
    await screen.findByTestId("runtime-player");
    expect(screen.getByText("Opening")).toBeInTheDocument();
    expect(getDefMock).toHaveBeenCalledWith(SLUG);

    // Wiring: the fetched document + the four adapters + host boundary factories.
    expect(lastPlayerProps.document).toEqual(golden);
    // P2-08/D5: the definition's content hash is threaded through unwrapped
    // (never nested inside `document`) so CoursePlayer can compare it against
    // a resumed session's own recorded hash.
    expect(lastPlayerProps.definitionHash).toBe(GOLDEN_HASH);
    expect(lastPlayerProps.studentId).toBe("student-42");
    expect(typeof lastPlayerProps.idFactory).toBe("function");
    expect(typeof lastPlayerProps.clock).toBe("function");
    expect(Object.keys(lastPlayerProps.adapters).sort()).toEqual(
      ["assetResolver", "closingGenerator", "openingGenerator", "sessionAdapter"].sort(),
    );
    // clock returns an ISO string; idFactory a string.
    expect(typeof lastPlayerProps.clock()).toBe("string");
    expect(new Date(lastPlayerProps.clock()).toString()).not.toBe("Invalid Date");

    // P2-05: a typed signal-resolution seam is wired, not a fabricated value.
    expect(typeof lastPlayerProps.signalResolver).toBe("function");
    expect(lastPlayerProps.signalResolver(["recent-course-topics"])).toEqual({});
  });

  // P1-03: the ONLY authoritative completion signal is CoursePlayer's own
  // `onComplete` (the learner dismissing the Closing) — not the session
  // reaching `completed`, which the player can reach before Closing is even
  // shown.
  it("does NOT fire onFinish just because the session status flips to completed", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    const onFinish = vi.fn();

    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={onFinish} />);
    await screen.findByTestId("runtime-player");

    fireEvent.click(screen.getByText("drive-to-completed"));
    await waitFor(() => expect(baseSetStatus).toHaveBeenCalledWith("sid", "completed"));
    expect(onFinish).not.toHaveBeenCalled();
  });

  it("fires onFinish when CoursePlayer's onComplete fires (learner dismisses the Closing)", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    const onFinish = vi.fn();

    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={onFinish} />);
    await screen.findByTestId("runtime-player");

    fireEvent.click(screen.getByText("dismiss-closing"));
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
  });

  it("loads the definition, collects paths, fetches asset urls, and resolves via the map", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    collectAssetPaths.mockReturnValue(["assets/a.png"]);
    fetchCourseAssetUrls.mockResolvedValue({
      assetUrls: { "assets/a.png": "https://cdn/a?auth_key=x" },
      expiresAt: "2999-01-01T00:00:00Z",
    });

    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={vi.fn()} />);

    await screen.findByTestId("runtime-player");
    expect(collectAssetPaths).toHaveBeenCalledWith(golden);
    expect(fetchCourseAssetUrls).toHaveBeenCalledWith(SLUG, ["assets/a.png"]);
    expect(lastPlayerProps.adapters.assetResolver.resolve("assets/a.png")).toBe("https://cdn/a?auth_key=x");
  });

  it("retries the asset-url refresh after a transient failure instead of giving up", async () => {
    vi.useFakeTimers();
    try {
      getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
      collectAssetPaths.mockReturnValue(["assets/a.png"]);
      // Initial sign uses a near expiry so the refresh arms at the 60s floor;
      // the first refresh FAILS; the fix must re-arm so a later retry succeeds.
      const soon = new Date(Date.now() + 60_000).toISOString();
      fetchCourseAssetUrls
        .mockResolvedValueOnce({ assetUrls: { "assets/a.png": "u1" }, expiresAt: soon })
        .mockRejectedValueOnce(new Error("transient"))
        .mockResolvedValueOnce({ assetUrls: { "assets/a.png": "u2" }, expiresAt: "2999-01-01T00:00:00Z" });

      render(<RuntimeCoursePlayer slug={SLUG} studentId="s" onExit={vi.fn()} onFinish={vi.fn()} />);

      await act(async () => { await vi.advanceTimersByTimeAsync(1); }); // flush initial load
      expect(fetchCourseAssetUrls).toHaveBeenCalledTimes(1);

      await act(async () => { await vi.advanceTimersByTimeAsync(60_000); }); // first refresh → fails
      const afterFailure = fetchCourseAssetUrls.mock.calls.length;
      expect(afterFailure).toBeGreaterThanOrEqual(2); // the refresh fired (and rejected)

      // The point: a failed refresh must NOT give up. Advancing again drives at
      // least one more refresh — without the retry, the count would stay put.
      await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
      expect(fetchCourseAssetUrls.mock.calls.length).toBeGreaterThan(afterFailure);
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not page-scroll the course region (P1-06 one-screen; renderer shell owns overflow)", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    collectAssetPaths.mockReturnValue([]);
    render(<RuntimeCoursePlayer slug={SLUG} onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByTestId("runtime-player");
    const region = screen.getByTestId("course-region");
    expect(region.style.overflow).toBe("hidden");
    expect(region.style.overflowY).not.toBe("auto");
  });

  it("skips the asset-urls fetch when the course references no assets", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    collectAssetPaths.mockReturnValue([]);

    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={vi.fn()} />);

    await screen.findByTestId("runtime-player");
    expect(collectAssetPaths).toHaveBeenCalledWith(golden);
    expect(fetchCourseAssetUrls).not.toHaveBeenCalled();
    // Unresolved (unmapped) paths pass through unchanged.
    expect(lastPlayerProps.adapters.assetResolver.resolve("assets/a.png")).toBe("assets/a.png");
  });

  // P2-07: the session adapter's `flush` must survive being wired into the
  // renderer's adapters object and fire on the lifecycle exits a student
  // actually takes — not just a clean unmount.
  it("flushes the session adapter on pagehide", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByTestId("runtime-player");

    expect(baseFlush).not.toHaveBeenCalled();
    await act(async () => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(baseFlush).toHaveBeenCalledTimes(1);
  });

  it("flushes the session adapter when the tab is backgrounded (visibilitychange → hidden)", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByTestId("runtime-player");

    const visibilitySpy = vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    try {
      await act(async () => {
        document.dispatchEvent(new Event("visibilitychange"));
      });
      expect(baseFlush).toHaveBeenCalledTimes(1);
    } finally {
      visibilitySpy.mockRestore();
    }
  });

  it("does NOT flush on visibilitychange while still visible", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByTestId("runtime-player");

    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(baseFlush).not.toHaveBeenCalled();
  });

  it("flushes the session adapter on unmount", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    const { unmount } = render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByTestId("runtime-player");

    expect(baseFlush).not.toHaveBeenCalled();
    unmount();
    expect(baseFlush).toHaveBeenCalledTimes(1);
  });
});

describe("CoursesContainer per-course routing", () => {
  it("mounts the runtime player for a course WITH a 2.0 definition", async () => {
    getDefMock.mockResolvedValue({ definition: golden, hash: GOLDEN_HASH });
    render(<CoursesContainer initialCourseId={SLUG} studentId="student-42" />);
    await screen.findByTestId("runtime-player");
    expect(screen.queryByTestId("legacy-player")).toBeNull();
  });

  it("falls back to the legacy player when the definition 404s (no 2.0 definition)", async () => {
    getDefMock.mockRejectedValue(new ApiError("not_found", "该课程没有 2.0 定义", 404));
    render(<CoursesContainer initialCourseId={SLUG} studentId="student-42" />);
    const legacy = await screen.findByTestId("legacy-player");
    expect(legacy).toHaveTextContent(`legacy:${SLUG}`);
    expect(screen.queryByTestId("runtime-player")).toBeNull();
  });
});
