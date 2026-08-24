import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DEMO_PROJECT_ID } from "@/tour/types";

// Integration test for the projects group of the guided tour (P3 Task 4):
// playing it must drive the shell into the demo project's studio
// (WorkspaceContainer.initialProjectId === DEMO_PROJECT_ID), propagate a room
// switch (WorkspaceContainer.pendingRoom), and — at the evaluation-report step —
// focus the demo's report (ReportsView.initialProjectId === DEMO_PROJECT_ID).
//
// We drive the tour by SKIPPING segment-to-segment (跳过本节). Every nav-driving
// onEnter the projects group cares about (openDemoProject, setStudioRoom,
// openDemoReport) lives on its segment's FIRST step, and skipping lands on each
// segment's first step, firing its onEnter — while never landing on the one
// advance:"action" step (courses tujian, a non-first step) whose target lives
// inside the mocked CoursesContainer and would otherwise dead-end a 下一步 walk.

const { putOnboarding } = vi.hoisted(() => ({ putOnboarding: vi.fn(async () => {}) }));
const { wsProps, reportsProps } = vi.hoisted(() => ({
  wsProps: { initialProjectId: null as string | null, pendingRoom: null as string | null },
  reportsProps: { initialProjectId: null as string | null },
}));

vi.mock("@/api", async (orig) => {
  const actual = await (orig as any)();
  return { ...actual, api: { ...actual.api, putOnboarding, setAccent: vi.fn(async () => {}), setBackground: vi.fn(async () => {}) } };
});
vi.mock("@/workspace/WorkspaceContainer", () => ({
  WorkspaceContainer: (props: { initialProjectId?: string | null; pendingRoom?: string | null }) => {
    // Latest non-null props win; the mock never consumes them (so they persist).
    if (props.initialProjectId) wsProps.initialProjectId = props.initialProjectId;
    if (props.pendingRoom) wsProps.pendingRoom = props.pendingRoom;
    return <div data-testid="ws" />;
  },
}));
vi.mock("@/shell/report/ReportsView", () => ({
  ReportsView: (props: { initialProjectId?: string | null }) => {
    if (props.initialProjectId) reportsProps.initialProjectId = props.initialProjectId;
    return <div data-testid="reports" />;
  },
}));
vi.mock("@/shell/courses/CoursesContainer", () => ({ CoursesContainer: () => <div /> }));
// Anchor resolution is irrelevant here (children are mocked) — stub it to null
// so no real 2s poll intervals or jsdom scrollIntoView calls fire during skips.
vi.mock("@/tour/anchors", () => ({ resolveAnchor: vi.fn(async () => null) }));

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

// Start the full journey via the Nav footer's 重新开始引导 (unique, unlike the
// welcome modal's 课程/项目 which collide with the nav tab labels).
function startTour() {
  render(<StudentApp session={makeSession()} onLogout={() => {}} />);
  fireEvent.click(screen.getByRole("button", { name: "重新开始引导" }));
}

// Click 跳过本节 until `pred` holds (or the tour ends / a guard trips).
function skipUntil(pred: () => boolean, max = 60): boolean {
  for (let i = 0; i < max; i++) {
    if (pred()) return true;
    const btn = screen.queryByRole("button", { name: "跳过本节" });
    if (!btn) break;
    fireEvent.click(btn);
  }
  return pred();
}

describe("StudentApp projects tour", () => {
  beforeEach(() => {
    putOnboarding.mockClear();
    wsProps.initialProjectId = null;
    wsProps.pendingRoom = null;
    reportsProps.initialProjectId = null;
  });

  it("opens the demo project into the studio and propagates a room switch", () => {
    startTour();
    const reached = skipUntil(() => wsProps.pendingRoom != null);
    expect(reached).toBe(true);
    expect(wsProps.initialProjectId).toBe(DEMO_PROJECT_ID);
    expect(["forming", "plan", "reading", "writing", "reflection"]).toContain(wsProps.pendingRoom);
  });

  it("focuses the demo's evaluation report at the eval-report step", () => {
    startTour();
    const reached = skipUntil(() => reportsProps.initialProjectId != null);
    expect(reached).toBe(true);
    expect(reportsProps.initialProjectId).toBe(DEMO_PROJECT_ID);
  });
});
