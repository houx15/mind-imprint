import { render, screen, cleanup, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ReportPanel } from "@lite/reports/ReportPanel";
import type { LiteReport } from "@lite/api/reports";

/**
 * ReportPanel — the fetch-and-render wrapper `FinishedReadingPanel` /
 * `FinishedWritingPanel` mount where the 「报告还在路上」 placeholder used to
 * sit. Pins the three states task-8-brief.md calls out: the report arriving
 * and replacing the placeholder, the honest waiting copy while the first
 * open generates it, and a quiet nothing when the server has nothing to say
 * (yet, or ever).
 */

afterEach(cleanup);

const ATOM_ID = "atom-1";

/** `handler` lets one route answer DIFFERENTLY on successive calls, which is
 *  what the server's two-phase report needs: phase 1 returns
 *  `prosePending: true`, the follow-up returns the enriched report. */
type Route = { status?: number; body?: unknown; delay?: boolean; handler?: () => unknown };
let routes: Record<string, Route>;

function key(method: string, url: string) {
  return `${method} ${url}`;
}

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

/** The route-table fetch, as a plain function — so a test can wrap it (see
 *  the phase-2-hangs case) instead of rebuilding the whole table. */
function stubbedFetch() {
  return async (url: string, init?: RequestInit): Promise<Response> => {
    const method = (init?.method ?? "GET").toUpperCase();
    const route = routes[key(method, url)];
    if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
    if (route.delay) return new Promise<Response>(() => {}); // never resolves
    if (route.handler) return jsonResponse(route.status ?? 200, route.handler());
    return jsonResponse(route.status ?? 200, route.body ?? {});
  };
}

function stubFetch() {
  vi.stubGlobal("fetch", vi.fn(stubbedFetch()));
}

const REPORT: LiteReport = {
  version: 1,
  kind: "reading",
  title: "中国是否让地球变得更可持续？",
  studentName: "Phoebe",
  finishedAt: "2026-08-29T03:00:00Z",
  ordinal: 8,
  stats: [{ key: "focusMinutes", label: "专注时长", value: 12, unit: "分钟" }],
  moments: [],
  keep: null,
  gains: [],
  lensNotes: [],
  notes: [],
  piece: "",
  prosePending: false,
};

beforeEach(() => {
  routes = {};
  stubFetch();
});

describe("ReportPanel", () => {
  it("replaces the placeholder once the report arrives", async () => {
    routes[key("GET", `/api/v1/readings/${ATOM_ID}/report`)] = { body: { report: REPORT } };
    render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    expect(await screen.findByText("中国是否让地球变得更可持续？")).toBeTruthy();
    expect(screen.getByText("12")).toBeTruthy();
    expect(screen.getByText("阅读时长")).toBeTruthy();
  });

  it("shows a live loading state, announced, while the report is on its way", async () => {
    // 🚨 2026-09-03. This used to assert a static sentence
    // （「印记正在把这次读的东西整理成一份报告，稍等一下。」）, and the
    // colleague trial reported that same screen twice: 「报告没有loading状态」
    // and 「it never finishes」. A line of grey text that does not move is
    // indistinguishable from a dead page, and back then the wait could
    // genuinely run to two and a half minutes.
    //
    // What is pinned now is the part she can actually perceive: a
    // `role="status"` region (so it is announced, not just drawn) carrying
    // the moving dots. Not the exact wording — that is copy, and copy is the
    // owner's to change without breaking a test.
    routes[key("GET", `/api/v1/readings/${ATOM_ID}/report`)] = { delay: true };
    const { container } = render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    const status = await screen.findByRole("status");
    expect(status).toBeTruthy();
    expect(container.querySelectorAll(".mk-think-dot").length).toBeGreaterThan(0);
    // 「加载中」 was rejected before this panel existed and stays rejected.
    expect(screen.queryByText("加载中")).toBeNull();
  });

  it("🚨 renders her whole report while the prose call is still running", async () => {
    // THE bug this split exists for. The prose is an `assess`-class call
    // (`reasoning: "max"`, 180s budget) that used to run inline inside this
    // GET, so 「印记正在把这次读的东西整理成一份报告，稍等一下。」 was the
    // entire screen for up to two and a half minutes.
    //
    // The second fetch here NEVER resolves, which is the whole point: even
    // with the prose call hanging forever, she is looking at her real report
    // — title, stats, everything deterministic — and not at a placeholder.
    let calls = 0;
    routes[key("GET", `/api/v1/readings/${ATOM_ID}/report`)] = {
      handler: () => {
        calls += 1;
        return { report: { ...REPORT, prosePending: true } };
      },
    };
    // Phase 2 hangs: swap the route to a never-resolving one once phase 1
    // has been served.
    const original = stubbedFetch();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        if (calls >= 1) return new Promise<Response>(() => {});
        return original(url, init);
      }),
    );

    render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    expect(await screen.findByText("中国是否让地球变得更可持续？")).toBeTruthy();
    expect(screen.getByText("阅读时长")).toBeTruthy();
    // And it says so, rather than letting a report missing those two
    // sections read as a finished report that simply has none.
    expect(screen.getByText(/处理中/)).toBeTruthy();
  });

  it("fills the prose in on the follow-up fetch, with nothing for her to do", async () => {
    let calls = 0;
    routes[key("GET", `/api/v1/readings/${ATOM_ID}/report`)] = {
      handler: () => {
        calls += 1;
        return calls === 1
          ? { report: { ...REPORT, prosePending: true } }
          : { report: { ...REPORT, prosePending: false, gains: ["学会了先看来源"] } };
      },
    };
    render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    expect(await screen.findByText("学会了先看来源")).toBeTruthy();
    await waitFor(() => expect(calls).toBe(2));
    // Once the prose is in, the 处理中 line goes away on its own.
    expect(screen.queryByText(/处理中/)).toBeNull();
  });

  it("does not re-fetch a report whose prose is already there", async () => {
    let calls = 0;
    routes[key("GET", `/api/v1/readings/${ATOM_ID}/report`)] = {
      handler: () => {
        calls += 1;
        return { report: { ...REPORT, prosePending: false } };
      },
    };
    render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    expect(await screen.findByText("中国是否让地球变得更可持续？")).toBeTruthy();
    // Every reopen of a finished report must stay free — one request, and no
    // 处理中 line either.
    await waitFor(() => expect(calls).toBe(1));
    expect(screen.queryByText(/处理中/)).toBeNull();
  });

  it("renders nothing loud when the server says null", async () => {
    routes[key("GET", `/api/v1/readings/${ATOM_ID}/report`)] = { body: { report: null } };
    const { container } = render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    await waitFor(() => expect(container.textContent).toBe(""));
  });

  it("degrades quietly, never a stack or a retry button, when the fetch fails", async () => {
    // no route stubbed → the fetch resolves 404
    const { container } = render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    await waitFor(() => expect(container.textContent).toBe(""));
    expect(screen.queryByRole("button", { name: /重试|重新/ })).toBeNull();
  });
});
