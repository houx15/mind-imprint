import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  getPublicReport,
  getReport,
  PublicReportNotFoundError,
  sendHeartbeat,
  shareReport,
  unshareReport,
  type LiteReport,
} from "@lite/api/reports";

/**
 * reports.ts — the lite-web client for the end-of-session report. Pins the
 * three contract points task-6-brief.md asks for: `getReport` returns `null`
 * (not throws) on `{"report": null}`, `getPublicReport` hits the public
 * (no-session) path, and `unshareReport` issues a DELETE. Also covers the
 * discrepancy-prone bits: `moments`/`gains` defaulting when absent, and
 * `getPublicReport` telling a 404 apart from a network failure.
 */

type Route = { status?: number; body?: unknown };
let routes: Record<string, Route>;
let calls: { method: string; url: string; body: unknown; credentials: unknown }[];

function key(method: string, url: string) {
  return `${method} ${url}`;
}

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

function stubFetch() {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      const raw = init?.body;
      const body = typeof raw === "string" ? JSON.parse(raw) : undefined;
      calls.push({ method, url, body, credentials: init?.credentials });
      const route = routes[key(method, url)];
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
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
  moments: [{ quote: "碳排放全球第一", where: "写论证的时候" }],
  keep: { label: "我的收获", text: "来源要溯源", source: "student" },
  gains: ["用 CRAAP 检查了来源"],
  lensNotes: [{ lens: "信源辨识卡 CRAAP / CRRAAB", quote: "这份报告由国家能源局发布。", finding: "来源可核实。" }],
  notes: [],
};

beforeEach(() => {
  routes = {};
  stubFetch();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("getReport", () => {
  it("calls /api/v1/readings/{id}/report and returns the report", async () => {
    routes[key("GET", "/api/v1/readings/atom-1/report")] = { body: { report: REPORT } };
    const result = await getReport("reading", "atom-1");
    expect(result).toEqual(REPORT);
    expect(calls[0]).toMatchObject({ method: "GET", url: "/api/v1/readings/atom-1/report" });
  });

  it("returns null when the server sends {report: null} — not finished yet, not an error", async () => {
    routes[key("GET", "/api/v1/readings/atom-1/report")] = { body: { report: null } };
    const result = await getReport("reading", "atom-1");
    expect(result).toBeNull();
  });

  it("maps kind='writing' onto the /writings path segment", async () => {
    routes[key("GET", "/api/v1/writings/atom-2/report")] = {
      body: { report: { ...REPORT, kind: "writing", keep: null } },
    };
    const result = await getReport("writing", "atom-2");
    expect(result?.kind).toBe("writing");
    expect(calls[0]!.url).toBe("/api/v1/writings/atom-2/report");
  });

  it("defaults absent moments/gains/lensNotes (omitempty on the wire) to empty arrays", async () => {
    const { moments, gains, lensNotes, ...rest } = REPORT;
    routes[key("GET", "/api/v1/readings/atom-1/report")] = { body: { report: rest } };
    const result = await getReport("reading", "atom-1");
    expect(result?.moments).toEqual([]);
    expect(result?.gains).toEqual([]);
    expect(result?.lensNotes).toEqual([]);
  });
});

describe("shareReport / unshareReport", () => {
  it("POSTs to {kind}/{id}/report/share and returns the token+url", async () => {
    routes[key("POST", "/api/v1/readings/atom-1/report/share")] = {
      body: { token: "abc123", url: "https://mind-lite.example/s/abc123" },
    };
    const result = await shareReport("reading", "atom-1");
    expect(result).toEqual({ token: "abc123", url: "https://mind-lite.example/s/abc123" });
    expect(calls[0]).toMatchObject({ method: "POST", url: "/api/v1/readings/atom-1/report/share" });
  });

  it("issues a DELETE to the same share path", async () => {
    routes[key("DELETE", "/api/v1/readings/atom-1/report/share")] = { status: 204 };
    await unshareReport("reading", "atom-1");
    expect(calls[0]).toMatchObject({ method: "DELETE", url: "/api/v1/readings/atom-1/report/share" });
  });

  it("writing twin maps onto /writings", async () => {
    routes[key("DELETE", "/api/v1/writings/atom-2/report/share")] = { status: 204 };
    await unshareReport("writing", "atom-2");
    expect(calls[0]!.url).toBe("/api/v1/writings/atom-2/report/share");
  });
});

describe("getPublicReport", () => {
  it("calls /api/v1/public/reports/{token} with no credentials", async () => {
    routes[key("GET", "/api/v1/public/reports/tok-1")] = { body: { report: REPORT } };
    const result = await getPublicReport("tok-1");
    expect(result).toEqual(REPORT);
    expect(calls[0]).toMatchObject({ method: "GET", url: "/api/v1/public/reports/tok-1" });
    // Must not ask the browser to attach a session cookie it does not have.
    expect(calls[0]!.credentials).toBe("omit");
  });

  it("raises PublicReportNotFoundError, distinctly, on a 404", async () => {
    routes[key("GET", "/api/v1/public/reports/gone")] = {
      status: 404,
      body: { error: { code: "not_found", message: "资源不存在" } },
    };
    await expect(getPublicReport("gone")).rejects.toBeInstanceOf(PublicReportNotFoundError);
  });

  it("raises a different error type on a network failure than on a 404", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch");
      }),
    );
    let caught: unknown;
    try {
      await getPublicReport("tok-1");
    } catch (err) {
      caught = err;
    }
    expect(caught).not.toBeInstanceOf(PublicReportNotFoundError);
    expect(caught).toBeInstanceOf(Error);
  });
});

describe("sendHeartbeat", () => {
  it("POSTs {seconds} to {kind}/{id}/heartbeat", async () => {
    routes[key("POST", "/api/v1/readings/atom-1/heartbeat")] = { status: 204 };
    await sendHeartbeat("reading", "atom-1", 60);
    expect(calls[0]).toMatchObject({
      method: "POST",
      url: "/api/v1/readings/atom-1/heartbeat",
      body: { seconds: 60 },
    });
  });
});
