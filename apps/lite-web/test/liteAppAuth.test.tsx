import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LiteApp } from "@lite/LiteApp";

/**
 * LiteApp's front door: sign in, land in the right edition, reach settings.
 *
 * Before this, lite had no auth surface at all — it rendered the reading tab
 * immediately and every request 401'd with no way to sign in, which is why it
 * could never be deployed. These tests hold that door shut.
 */

type Route = { status?: number; body?: unknown };

let calls: { method: string; url: string }[];

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

function stubFetch(handler: (method: string, url: string) => Route | undefined) {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      calls.push({ method, url });
      const route = handler(method, url);
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

function meUser(edition: string) {
  return {
    user: {
      id: "u1",
      email: "s@lite.local",
      display_name: "小雨",
      role: "student",
      avatar_color: "vermilion",
      page_background: "paper",
      onboarded_at: null,
      school: { id: "s1", name: "启明中学", edition },
      classes: [{ id: "c1", name: "初二 3 班", role_in_class: "student" }],
    },
  };
}

/** Answer /auth/me with the given user; everything else 404s, which is what
 * the landing pages get and is fine — none of these tests assert on them. */
function stubSignedIn(edition: string) {
  stubFetch((method, url) => {
    if (method === "GET" && url.endsWith("/api/v1/auth/me")) return { body: meUser(edition) };
    return undefined;
  });
}

beforeEach(() => {
  window.history.pushState({}, "", "/");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe("signing in", () => {
  it("shows the auth screen when there is no session", async () => {
    stubFetch((method, url) => {
      if (method === "GET" && url.endsWith("/api/v1/auth/me")) {
        return { status: 401, body: { error: { code: "unauthorized", message: "请先登录" } } };
      }
      return undefined;
    });

    render(<LiteApp />);

    // Sign-in fields, not the reading rail: an unauthenticated student must
    // never see an app whose every request will fail.
    await waitFor(() => expect(screen.getByRole("button", { name: "登录" })).toBeTruthy());
    expect(screen.queryByLabelText("主导航")).toBeNull();
  });

  it("renders the shell for a lite student with a live session", async () => {
    stubSignedIn("lite");
    render(<LiteApp />);
    await waitFor(() => expect(screen.getByLabelText("主导航")).toBeTruthy());
    expect(screen.getByRole("button", { name: "阅读" })).toBeTruthy();
  });
});

describe("landing on the wrong edition", () => {
  it("tells a full-edition student what happened and offers the way over", async () => {
    vi.stubEnv("VITE_PRO_APP_URL", "https://mind-web.uni-robot.cn");
    stubSignedIn("pro");

    render(<LiteApp />);

    const dialog = await screen.findByRole("dialog");
    // Named, not silent — the student learns which address is actually theirs.
    expect(dialog.textContent).toContain("完整版");
    expect(screen.getByRole("button", { name: /前往完整版/ })).toBeTruthy();
    // And they are NOT dropped into the lite shell in the meantime.
    expect(screen.queryByLabelText("主导航")).toBeNull();
  });

  it("keeps a lite student in the lite app", async () => {
    vi.stubEnv("VITE_PRO_APP_URL", "https://mind-web.uni-robot.cn");
    stubSignedIn("lite");
    render(<LiteApp />);
    await waitFor(() => expect(screen.getByLabelText("主导航")).toBeTruthy());
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  // The graceful-degradation rule, from the lite side: an API too old to
  // report an edition must not eject a student who is already in the right
  // place.
  it("stays put when the API reports no edition at all", async () => {
    stubFetch((method, url) => {
      if (method === "GET" && url.endsWith("/api/v1/auth/me")) {
        const payload = meUser("lite");
        delete (payload.user.school as { edition?: string }).edition;
        return { body: payload };
      }
      return undefined;
    });
    render(<LiteApp />);
    await waitFor(() => expect(screen.getByLabelText("主导航")).toBeTruthy());
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("settings", () => {
  it("reaches the settings page from the rail, with the signed-in student on it", async () => {
    stubSignedIn("lite");
    window.history.pushState({}, "", "/settings");

    render(<LiteApp />);

    await waitFor(() => expect(screen.getByLabelText("主导航")).toBeTruthy());
    // Pro's real SettingsView, showing this student — not a lite copy of it.
    await waitFor(() => expect(screen.getAllByText("小雨").length).toBeGreaterThan(0));
    // School and class arrive joined into one label ("启明中学 · 初二 3 班"),
    // so match the substring rather than the whole node.
    expect(screen.getByText(/启明中学/)).toBeTruthy();
  });

  it("signs the student out and returns them to the auth screen", async () => {
    stubFetch((method, url) => {
      if (method === "GET" && url.endsWith("/api/v1/auth/me")) return { body: meUser("lite") };
      if (method === "POST" && url.endsWith("/api/v1/auth/signout")) return { status: 204 };
      return undefined;
    });
    window.history.pushState({}, "", "/settings");

    render(<LiteApp />);
    const logout = await screen.findByRole("button", { name: /退出登录/ });
    logout.click();

    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/api/v1/auth/signout"))).toBe(true));
    await waitFor(() => expect(screen.queryByLabelText("主导航")).toBeNull());
  });
});
