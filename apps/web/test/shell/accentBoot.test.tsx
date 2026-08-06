import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AppShell } from "@/shell/AppShell";
import { createSession } from "@/shell/session";

/**
 * End-to-end accent boot (platform shell rebuild, Task 11).
 *
 * Exercises the full seam: `getMe` → `MeUser.avatar_color` → `StudentApp`'s
 * `coerceAccent` → `AccentProvider initialAccent` → `applyAccent` on
 * `document.documentElement`. `AppShell.test.tsx` already covers the boot
 * *routing* (auth/teacher/admin/trial); this file is scoped to the accent
 * seed specifically, for both a known preset and a legacy/unset value.
 */

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      getMe: vi.fn(async () => { throw new Error("401"); }),
      signout: vi.fn(async () => {}),
      // HomePage (the 首页 landing StudentApp renders on boot) fetches both
      // of these on mount.
      listProjects: vi.fn(async () => []),
      listCourses: vi.fn(async () => []),
      setAccent: vi.fn(async () => {}),
    },
  };
});

vi.mock("@/workspace/WorkspaceContainer", () => ({
  WorkspaceContainer: () => <div data-testid="studio-container" />,
}));

const mem = () => { let s = "{}"; return { getItem: () => s, setItem: (_: string, v: string) => { s = v; } }; };

function studentWith(avatarColor: string) {
  return {
    id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
    avatar_color: avatarColor, school: { id: "s1", name: "Demo" }, classes: [],
  };
}

describe("shell accent boot (integration)", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("boots a student with a known preset avatar_color (teal) onto 首页 with the accent applied to the document root", async () => {
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => studentWith("teal")), signout: vi.fn() };
    render(<AppShell session={session} client={client as never} />);

    // Both assertions live inside one waitFor: the greeting text commits in
    // the same render as AccentProvider's mount, but its `applyAccent` call
    // runs in a separately-scheduled passive effect — asserting it outside
    // waitFor's retry loop is a race under load (flaky in the full suite).
    await waitFor(() => {
      expect(screen.getByText(/你好，Phoebe/)).toBeInTheDocument();
      // teal preset's 500 step (ui/accent.tsx ACCENT_PRESETS).
      expect(document.documentElement.style.getPropertyValue("--mk-accent-500")).toBe("#1F9488");
    });
  });

  it("boots a student with a legacy free-hex avatar_color onto the default vermilion accent (coerceAccent rejects non-preset values)", async () => {
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => studentWith("#2A3B7A")), signout: vi.fn() };
    render(<AppShell session={session} client={client as never} />);

    await waitFor(() => {
      expect(screen.getByText(/你好，Phoebe/)).toBeInTheDocument();
      // Default vermilion 500 step — the localStorage/default fallback path.
      expect(document.documentElement.style.getPropertyValue("--mk-accent-500")).toBe("#EA5140");
    });
  });

  it("boots a student with an empty-string avatar_color onto the default vermilion accent", async () => {
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => studentWith("")), signout: vi.fn() };
    render(<AppShell session={session} client={client as never} />);

    await waitFor(() => {
      expect(screen.getByText(/你好，Phoebe/)).toBeInTheDocument();
      expect(document.documentElement.style.getPropertyValue("--mk-accent-500")).toBe("#EA5140");
    });
  });
});
