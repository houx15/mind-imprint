import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

const { putOnboarding } = vi.hoisted(() => ({ putOnboarding: vi.fn(async () => {}) }));
vi.mock("@/api", async (orig) => {
  const actual = await (orig as any)();
  return { ...actual, api: { ...actual.api, putOnboarding, setAccent: vi.fn(async () => {}), setBackground: vi.fn(async () => {}) } };
});
vi.mock("@/workspace/WorkspaceContainer", () => ({ WorkspaceContainer: () => <div /> }));
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
  beforeEach(() => putOnboarding.mockClear());

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
});
