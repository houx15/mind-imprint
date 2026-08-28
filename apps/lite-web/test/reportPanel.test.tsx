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

type Route = { status?: number; body?: unknown; delay?: boolean };
let routes: Record<string, Route>;

function key(method: string, url: string) {
  return `${method} ${url}`;
}

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

function stubFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      const route = routes[key(method, url)];
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      if (route.delay) return new Promise<Response>(() => {}); // never resolves
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

const REPORT: LiteReport = {
  version: 1,
  kind: "reading",
  title: "中国是否让地球变得更可持续？",
  studentName: "Phoebe",
  finishedAt: "2026-08-29T03:00:00Z",
  stats: [{ key: "focusMinutes", label: "专注时长", value: 12, unit: "分钟" }],
  moments: [],
  keep: null,
  gains: [],
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
    expect(screen.getByText("专注时长")).toBeTruthy();
  });

  it("says something honest while the report is being written", async () => {
    // a never-resolving fetch → a waiting line, not a blank panel and not
    // an error, and not a bare "加载中" either.
    routes[key("GET", `/api/v1/readings/${ATOM_ID}/report`)] = { delay: true };
    render(<ReportPanel kind="reading" atomId={ATOM_ID} />);

    expect(await screen.findByText(/印记正在把这次读的东西整理成一份报告/)).toBeTruthy();
    expect(screen.queryByText("加载中")).toBeNull();
  });

  it("says the writing version of the same honest line for a writing report", async () => {
    routes[key("GET", `/api/v1/writings/${ATOM_ID}/report`)] = { delay: true };
    render(<ReportPanel kind="writing" atomId={ATOM_ID} />);

    expect(await screen.findByText(/印记正在把这次写的东西整理成一份报告/)).toBeTruthy();
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
