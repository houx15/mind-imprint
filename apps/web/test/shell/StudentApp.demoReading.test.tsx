import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, act } from "@testing-library/react";
import type { TourNavContext } from "@/tour/types";
import { DEMO_PROJECT_ID, DEMO_READING_MATERIAL_ID, DEMO_READING_REFERENCE_ID } from "@/tour/types";

// Unit test of `StudentApp`'s `tourNav.openDemoReadingRoom` (P6, Task 5) —
// success opens the real immersive ReadingRoom via WorkspaceContainer's
// `pendingDemoReading` deep-link; a GET failure falls back to forcing the
// 列表 view instead of dead-ending. We capture the `nav` object StudentApp
// hands to `TourProvider` and call `openDemoReadingRoom()` on it directly,
// bypassing the tour engine entirely (no segment in the journey drives this
// path yet — that's a later task) — mirrors how `StudentApp.tour.test.tsx`
// isolates nav-driven behaviour from the step-by-step walk.

const { putOnboarding } = vi.hoisted(() => ({ putOnboarding: vi.fn(async () => {}) }));
const { wsProps } = vi.hoisted(() => ({
  wsProps: {
    pendingRoom: null as string | null,
    pendingReadingView: null as string | null,
    pendingDemoReading: null as { source: { id: string }; referenceId: string } | null,
  },
}));
const { getMaterialSource } = vi.hoisted(() => ({ getMaterialSource: vi.fn() }));

vi.mock("@/api", async (orig) => {
  const actual = await (orig as any)();
  return { ...actual, api: { ...actual.api, putOnboarding, setAccent: vi.fn(async () => {}), setBackground: vi.fn(async () => {}) } };
});
vi.mock("@/workspace/api/workspace", () => ({
  getMaterialSource: (...args: unknown[]) => getMaterialSource(...args),
}));
vi.mock("@/workspace/WorkspaceContainer", () => ({
  WorkspaceContainer: (props: {
    pendingRoom?: string | null;
    pendingReadingView?: string | null;
    pendingDemoReading?: { source: { id: string }; referenceId: string } | null;
  }) => {
    if (props.pendingRoom) wsProps.pendingRoom = props.pendingRoom;
    if (props.pendingReadingView) wsProps.pendingReadingView = props.pendingReadingView;
    if (props.pendingDemoReading) wsProps.pendingDemoReading = props.pendingDemoReading;
    return <div data-testid="ws" />;
  },
}));
vi.mock("@/shell/report/ReportsView", () => ({ ReportsView: () => <div data-testid="reports" /> }));
vi.mock("@/shell/courses/CoursesContainer", () => ({ CoursesContainer: () => <div /> }));

const { capturedNav } = vi.hoisted(() => ({ capturedNav: { current: null as TourNavContext | null } }));
vi.mock("@/tour/TourProvider", async (orig) => {
  const actual = await (orig as any)();
  return {
    ...actual,
    TourProvider: (props: { nav: TourNavContext; children: unknown }) => {
      capturedNav.current = props.nav;
      return actual.TourProvider(props);
    },
  };
});

import { StudentApp } from "@/shell/StudentApp";
import { createSession } from "@/shell/session";

function makeSession() {
  const store: Record<string, string> = {};
  const s = createSession({ storage: { getItem: (k: string) => store[k] ?? null, setItem: (k: string, v: string) => { store[k] = v; } } as any });
  s.setUser({
    id: "u1", email: "p@x.cn", display_name: "Phoebe", role: "student",
    avatar_color: "vermilion", page_background: "paper", onboarded_at: "2026-08-01T00:00:00Z",
    school: { id: "s1", name: "X" }, classes: [{ id: "c1", name: "1", role_in_class: "student" }],
  });
  return s;
}

describe("StudentApp · openDemoReadingRoom", () => {
  beforeEach(() => {
    putOnboarding.mockClear();
    getMaterialSource.mockReset();
    wsProps.pendingRoom = null;
    wsProps.pendingReadingView = null;
    wsProps.pendingDemoReading = null;
    capturedNav.current = null;
  });

  it("switches to the reading room and opens the demo material into the real immersive room on success", async () => {
    const fakeSource = { id: DEMO_READING_MATERIAL_ID };
    getMaterialSource.mockResolvedValueOnce(fakeSource);
    render(<StudentApp session={makeSession()} onLogout={() => {}} />);

    expect(capturedNav.current).not.toBeNull();
    await act(async () => {
      // The real journey is already on the 项目 tab (with the demo project
      // open) by the time openDemoReadingRoom's segment is reached — switch
      // there first so the mocked WorkspaceContainer actually mounts.
      capturedNav.current!.setTab("projects");
      capturedNav.current!.openDemoReadingRoom();
      // let the mocked GET's microtask resolve.
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(getMaterialSource).toHaveBeenCalledWith(DEMO_PROJECT_ID, DEMO_READING_MATERIAL_ID);
    expect(wsProps.pendingRoom).toBe("reading");
    await waitFor(() => expect(wsProps.pendingDemoReading).not.toBeNull());
    expect(wsProps.pendingDemoReading?.source.id).toBe(DEMO_READING_MATERIAL_ID);
    expect(wsProps.pendingDemoReading?.referenceId).toBe(DEMO_READING_REFERENCE_ID);
    // No dead-end fallback fired on the success path.
    expect(wsProps.pendingReadingView).toBeNull();
  });

  it("falls back to forcing the 列表 view (no dead-end, no throw) when the GET fails", async () => {
    getMaterialSource.mockRejectedValueOnce(new Error("network down"));
    render(<StudentApp session={makeSession()} onLogout={() => {}} />);

    expect(capturedNav.current).not.toBeNull();
    // Calling straight through must never throw / leave an unhandled
    // rejection — the whole point of the fallback.
    await act(async () => {
      capturedNav.current!.setTab("projects");
      capturedNav.current!.openDemoReadingRoom();
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(wsProps.pendingRoom).toBe("reading");
    await waitFor(() => expect(wsProps.pendingReadingView).toBe("list"));
    // The immersive open never fired on the failure path.
    expect(wsProps.pendingDemoReading).toBeNull();
  });
});
