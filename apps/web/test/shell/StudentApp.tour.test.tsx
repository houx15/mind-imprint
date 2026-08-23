import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import type { TourNavContext } from "@/tour/types";

const { putOnboarding } = vi.hoisted(() => ({ putOnboarding: vi.fn(async () => {}) }));
// Capture the assembled TourNavContext + the props that reach WorkspaceContainer
// so a test can call the nav hooks directly and assert they thread through the
// ProjectsTab → WorkspaceContainer chain (P5 Task 4).
const { navRef, wsProps } = vi.hoisted(() => ({
  navRef: { current: null as TourNavContext | null },
  wsProps: { pendingReadingView: null as string | null, initialProjectId: null as string | null },
}));
vi.mock("@/api", async (orig) => {
  const actual = await (orig as any)();
  return { ...actual, api: { ...actual.api, putOnboarding, setAccent: vi.fn(async () => {}), setBackground: vi.fn(async () => {}) } };
});
// Wrap the REAL TourProvider so the welcome-modal/journey tests keep working
// (real play()/step rendering) while still capturing the assembled `nav`.
vi.mock("@/tour/TourProvider", async (orig) => {
  const actual: any = await (orig as any)();
  return {
    ...actual,
    TourProvider: (props: any) => {
      navRef.current = props.nav;
      return <actual.TourProvider {...props} />;
    },
  };
});
vi.mock("@/workspace/WorkspaceContainer", () => ({
  WorkspaceContainer: (props: { pendingReadingView?: string | null; initialProjectId?: string | null }) => {
    // Latest non-null props win; the mock never consumes them (so they persist).
    if (props.pendingReadingView) wsProps.pendingReadingView = props.pendingReadingView;
    if (props.initialProjectId) wsProps.initialProjectId = props.initialProjectId;
    return <div data-testid="ws" />;
  },
}));
vi.mock("@/shell/courses/CoursesContainer", () => ({ CoursesContainer: () => <div /> }));

import { StudentApp } from "@/shell/StudentApp";
import { createSession } from "@/shell/session";

function makeSession(onboardedAt: string | null) {
  const store: Record<string, string> = {};
  const s = createSession({ storage: { getItem: (k: string) => store[k] ?? null, setItem: (k: string, v: string) => { store[k] = v; } } as any });
  s.setUser({
    id: "u1", email: "p@x.cn", display_name: "Phoebe", role: "student",
    avatar_color: "vermilion", page_background: "paper", onboarded_at: onboardedAt,
    school: { id: "s1", name: "X" }, classes: [{ id: "c1", name: "1", role_in_class: "student" }],
  });
  return s;
}

describe("StudentApp onboarding", () => {
  beforeEach(() => {
    putOnboarding.mockClear();
    navRef.current = null;
    wsProps.pendingReadingView = null;
    wsProps.initialProjectId = null;
  });

  it("shows the welcome modal for a never-onboarded user", () => {
    render(<StudentApp session={makeSession(null)} onLogout={() => {}} />);
    expect(screen.getByText(/欢迎来到思维印记/)).toBeInTheDocument();
  });

  it("does NOT show the welcome modal once onboarded", () => {
    render(<StudentApp session={makeSession("2026-08-01T00:00:00Z")} onLogout={() => {}} />);
    expect(screen.queryByText(/欢迎来到思维印记/)).not.toBeInTheDocument();
  });

  it("stamps onboarding when the user picks 稍后再说", () => {
    render(<StudentApp session={makeSession(null)} onLogout={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "稍后再说" }));
    expect(putOnboarding).toHaveBeenCalled();
    expect(screen.queryByText(/欢迎来到思维印记/)).not.toBeInTheDocument();
  });

  it("picking 课程 starts the tour on the courses group", () => {
    render(<StudentApp session={makeSession(null)} onLogout={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "课程" }));
    expect(screen.getByText("先聊聊课程")).toBeInTheDocument();
  });

  it("picking 项目 starts the tour on the projects group", () => {
    render(<StudentApp session={makeSession(null)} onLogout={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "项目" }));
    expect(screen.getByText("再聊聊项目")).toBeInTheDocument();
  });
});

// P5 Task 4: the new reading-room nav hooks must reach WorkspaceContainer
// through the same ProjectsTab prop chain `pendingRoom` uses.
describe("StudentApp · reading-view tour hooks", () => {
  beforeEach(() => {
    navRef.current = null;
    wsProps.pendingReadingView = null;
    wsProps.initialProjectId = null;
  });

  it("setReadingView threads pendingReadingView through to WorkspaceContainer", () => {
    render(<StudentApp session={makeSession("2026-08-01T00:00:00Z")} onLogout={() => {}} />);
    expect(navRef.current).not.toBeNull();
    // Move onto the 项目 tab so WorkspaceContainer mounts, then force the view.
    act(() => navRef.current!.setTab("projects"));
    act(() => navRef.current!.setReadingView("graph"));
    expect(wsProps.pendingReadingView).toBe("graph");
  });

  // openDemoReadingRoom now opens the REAL immersive Reading Room (P6, Task 5)
  // instead of the old P5-T4 setReadingView("list") stub — see
  // StudentApp.demoReading.test.tsx for its success/fallback coverage.
});
