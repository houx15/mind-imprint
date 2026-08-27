import { render, screen, waitFor, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WritingsLanding } from "@lite/writings/WritingsLanding";
import { WRITING_TOPICS } from "@lite/writings/topics";
import { splitWritings, shortDay } from "@lite/writings/WritingHistoryPanel";

/**
 * WritingsLanding — driven through a stubbed `fetch`, same harness shape as
 * readingsLanding.test.tsx: assertions are about the REAL wiring (which
 * endpoint, which body), not a hand-rolled double.
 */

type Route = { status?: number; body?: unknown };
let routes: Record<string, Route>;
let calls: { method: string; url: string; body: unknown }[];

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
      calls.push({ method, url, body });
      const route = routes[key(method, url)];
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

function writing(over: Partial<Record<string, unknown>> = {}) {
  return {
    id: "w1",
    title: "该不该把上学时间往后推？",
    lang: "zh",
    stage: "ideate",
    targetWords: null,
    status: "active",
    createdAt: "2026-08-20T00:00:00Z",
    updatedAt: "2026-08-20T00:00:00Z",
    finishedAt: null,
    ...over,
  } as Record<string, unknown>;
}

beforeEach(() => {
  routes = { [key("GET", "/api/v1/writings")]: { body: { writings: [] } } };
  stubFetch();
  window.history.replaceState(null, "", "/writings");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the greeting and its ink mark", () => {
  it("greets her with 写 circled, not as a plain heading", async () => {
    render(<WritingsLanding />);
    const heading = await screen.findByRole("heading", { level: 1 });
    expect(heading.textContent).toBe("Hi，今天想写点什么");
    expect(heading.querySelector("path.lite-ink-ring")).not.toBeNull();
  });
});

describe("the one box", () => {
  it("starts a writing from typed text and routes into it", async () => {
    routes[key("POST", "/api/v1/writings")] = { status: 201, body: { id: "new-1" } };
    render(<WritingsLanding />);

    fireEvent.change(await screen.findByPlaceholderText("说说你想写点什么，直接开始"), {
      target: { value: "我想论证一下短视频有没有让人变笨。" },
    });
    fireEvent.click(screen.getByRole("button", { name: "开始写作" }));

    await waitFor(() => expect(window.location.pathname).toBe("/writings/new-1"));
    const post = calls.find((c) => c.method === "POST" && c.url === "/api/v1/writings")!;
    expect(post.body).toEqual({ idea: "我想论证一下短视频有没有让人变笨。", lang: "zh" });
  });

  it("sends on Enter, not Shift+Enter", async () => {
    routes[key("POST", "/api/v1/writings")] = { status: 201, body: { id: "new-2" } };
    render(<WritingsLanding />);

    const box = await screen.findByPlaceholderText("说说你想写点什么，直接开始");
    fireEvent.change(box, { target: { value: "一句想法。" } });
    fireEvent.keyDown(box, { key: "Enter", shiftKey: true });
    expect(window.location.pathname).toBe("/writings");

    fireEvent.keyDown(box, { key: "Enter" });
    await waitFor(() => expect(window.location.pathname).toBe("/writings/new-2"));
  });

  it("does not start on an empty box", async () => {
    render(<WritingsLanding />);
    expect(await screen.findByRole("button", { name: "开始写作" })).toBeDisabled();
  });
});

describe("不知道写什么？", () => {
  it("offers a fixed shelf and seeds the idea + language from the topic", async () => {
    routes[key("POST", "/api/v1/writings")] = { status: 201, body: { id: "topic-1" } };
    render(<WritingsLanding />);

    expect(await screen.findByText("不知道写什么？")).toBeInTheDocument();
    const enTopic = WRITING_TOPICS.find((t) => t.lang === "en")!;
    fireEvent.click(screen.getByText(enTopic.title));

    await waitFor(() => expect(window.location.pathname).toBe("/writings/topic-1"));
    expect(calls.find((c) => c.method === "POST")?.body).toEqual({ idea: enTopic.idea, lang: "en" });
  });

  it("every seeded topic carries a real opening sentence, not just a title", () => {
    expect(WRITING_TOPICS.length).toBeGreaterThanOrEqual(3);
    for (const t of WRITING_TOPICS) {
      expect(t.idea.trim().length).toBeGreaterThan(t.title.length / 2);
    }
  });
});

describe("the unfinished notice and 我的写作", () => {
  it("counts only unfinished writings", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: {
        writings: [
          writing({ id: "a" }),
          writing({ id: "b" }),
          writing({ id: "c", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
        ],
      },
    };
    render(<WritingsLanding />);
    expect(await screen.findByText("你有 2 篇还没写完")).toBeInTheDocument();
  });

  it("opens 我的写作 with unfinished above finished, and routes both", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: {
        writings: [
          writing({ id: "done-1", title: "写完的那篇", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
          writing({ id: "open-1", title: "没写完的那篇", updatedAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<WritingsLanding />);
    fireEvent.click(await screen.findByRole("button", { name: /我的写作/ }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("还没写完");
    expect(dialog).toHaveTextContent("已完成");
    expect(dialog).toHaveTextContent("继续");
    expect(dialog).toHaveTextContent("看报告");

    const text = dialog.textContent ?? "";
    expect(text.indexOf("没写完的那篇")).toBeLessThan(text.indexOf("写完的那篇"));

    fireEvent.click(screen.getByText("写完的那篇"));
    expect(window.location.pathname).toBe("/writings/done-1");
  });
});

describe("pure helpers", () => {
  it("splitWritings puts unfinished first, each newest-first by updatedAt", () => {
    const rows = [
      writing({ id: "u-old", updatedAt: "2026-08-20T00:00:00Z" }),
      writing({ id: "f-new", status: "finished", finishedAt: "2026-08-25T00:00:00Z" }),
      writing({ id: "u-new", updatedAt: "2026-08-24T00:00:00Z" }),
      writing({ id: "f-old", status: "finished", finishedAt: "2026-08-10T00:00:00Z" }),
    ];
    const { unfinished, finished } = splitWritings(rows as never);
    expect(unfinished.map((w) => w.id)).toEqual(["u-new", "u-old"]);
    expect(finished.map((w) => w.id)).toEqual(["f-new", "f-old"]);
  });

  it("shortDay degrades to an empty string rather than rendering Invalid Date", () => {
    expect(shortDay(null)).toBe("");
    expect(shortDay("not-a-date")).toBe("");
    expect(shortDay(new Date().toISOString())).toBe("今天");
  });
});
