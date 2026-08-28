import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LiteApp } from "@lite/LiteApp";

/**
 * The public share page — what `/s/:token` actually opens for someone with
 * NO account at all (a parent scanning the QR code on a shared report).
 *
 * Today's whole-app boot sequence calls `getMe()` in an effect and renders
 * `AuthScreen` as soon as it resolves to "no session", BEFORE any route is
 * ever inspected. The first test below is the one that proves this task's
 * whole point: mounting `LiteApp` itself (not `PublicReportPage` directly)
 * on the share path, with `getMe()` mocked to REJECT, must render the
 * report and must never show `AuthScreen` — a test written while signed in
 * would prove nothing here.
 */

type Route = { status?: number; body?: unknown; reject?: boolean };

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
      if (route.reject) throw new TypeError("Failed to fetch");
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

const REPORT = {
  version: 1,
  kind: "reading",
  title: "《中国是否让地球变得更可持续？》读后",
  studentName: "小雨",
  finishedAt: "2026-08-20T10:00:00Z",
  stats: [{ key: "words", label: "读了", value: 1200, unit: "字" }],
  moments: [],
  keep: null,
  gains: [],
};

beforeEach(() => {
  window.history.pushState({}, "", "/s/tok-abc123");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

it("renders a shared report with no session at all", async () => {
  stubFetch((method, url) => {
    // getMe() must not even be reached — but if some future regression
    // called it anyway, make sure it looks like "no session" rather than
    // accidentally succeeding and masking the bug this test exists to catch.
    if (method === "GET" && url.endsWith("/api/v1/auth/me")) return { reject: true };
    if (method === "GET" && url.endsWith("/api/v1/public/reports/tok-abc123")) {
      return { body: { report: REPORT } };
    }
    return undefined;
  });

  render(<LiteApp />);

  await waitFor(() => expect(screen.getByText(REPORT.title)).toBeTruthy());

  // The whole point of this task: no sign-in screen, ever, on this route.
  expect(screen.queryByRole("button", { name: "登录" })).toBeNull();
  expect(screen.queryByLabelText("主导航")).toBeNull();

  // And `getMe()` was never even called — the route check happens before
  // the auth boot effect runs at all, not merely before it renders.
  expect(calls.some((c) => c.url.endsWith("/api/v1/auth/me"))).toBe(false);
});

it("says the link is no longer available on a 404", async () => {
  stubFetch((method, url) => {
    if (method === "GET" && url.endsWith("/api/v1/public/reports/tok-abc123")) {
      return { status: 404, body: { error: { code: "not_found", message: "资源不存在" } } };
    }
    return undefined;
  });

  render(<LiteApp />);

  await waitFor(() =>
    expect(screen.getByText("这份记录不存在，或者已经被收回了。")).toBeTruthy(),
  );
  // A dead link is not an invitation to sign in.
  expect(screen.queryByRole("button", { name: "登录" })).toBeNull();
});

it("says something different on a network failure than on a 404", async () => {
  stubFetch((method, url) => {
    if (method === "GET" && url.endsWith("/api/v1/public/reports/tok-abc123")) {
      return { reject: true };
    }
    return undefined;
  });

  render(<LiteApp />);

  await waitFor(() =>
    expect(screen.getByText("网络好像断开了，请稍后再试一次。")).toBeTruthy(),
  );
  // Distinct wording from the 404 case — telling someone a link is dead when
  // their wifi merely dropped would be a lie.
  expect(screen.queryByText("这份记录不存在，或者已经被收回了。")).toBeNull();
});
