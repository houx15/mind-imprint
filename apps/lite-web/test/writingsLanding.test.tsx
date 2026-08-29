import { render, screen, waitFor, cleanup, fireEvent, within } from "@testing-library/react";
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
  const row = {
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
  // lastActivityAt is what "last touched" actually means now (atom.
  // last_activity_at); updatedAt only moves on rename/stage/target-words. A
  // fixture that doesn't care about the distinction gets them equal.
  if (row.lastActivityAt === undefined) row.lastActivityAt = row.updatedAt;
  return row;
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

  it("says nothing when everything is finished", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: { writings: [writing({ id: "c", status: "finished", finishedAt: "2026-08-23T00:00:00Z" })] },
    };
    render(<WritingsLanding />);
    await screen.findByRole("heading", { level: 1 });
    expect(screen.queryByText(/还没写完/)).toBeNull();
  });

  // The notice opens the SHELF, not a writing — same fix ReadingsLanding
  // already carries: a count's only honest offer is the list behind it.
  it("opens the shelf from the notice instead of picking a writing for her", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: {
        writings: [
          writing({ id: "older", title: "先写的那篇", lastActivityAt: "2026-08-24T00:00:00Z" }),
          writing({ id: "newer", title: "后写的那篇", lastActivityAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<WritingsLanding />);
    fireEvent.click(await screen.findByText("你有 2 篇还没写完"));

    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(window.location.pathname).toBe("/writings");
  });

  it("opens 我的写作 with unfinished above finished, and routes both", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: {
        writings: [
          writing({ id: "done-1", title: "写完的那篇", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
          writing({ id: "open-1", title: "没写完的那篇", lastActivityAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<WritingsLanding />);
    fireEvent.click(await screen.findByRole("button", { name: /我的写作/ }));

    const dialog = await screen.findByRole("dialog");
    // The old fixed sections' 11px label is gone — chips replaced them.
    expect(dialog).not.toHaveTextContent("还没写完 · ");
    expect(dialog).toHaveTextContent("全部");
    expect(dialog).toHaveTextContent("还没写完");
    expect(dialog).toHaveTextContent("已完成");
    // Unfinished offers 继续, finished offers 看报告.
    expect(dialog).toHaveTextContent("继续");
    expect(dialog).toHaveTextContent("看报告");

    // Order: the unfinished row precedes the finished one in the DOM.
    const text = dialog.textContent ?? "";
    expect(text.indexOf("没写完的那篇")).toBeLessThan(text.indexOf("写完的那篇"));

    fireEvent.click(screen.getByText("写完的那篇"));
    expect(window.location.pathname).toBe("/writings/done-1");
  });

  it("filters to 还没写完 via the chip, and shows the dot only on unfinished rows", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: {
        writings: [
          writing({ id: "done-1", title: "写完的那篇", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
          writing({ id: "open-1", title: "没写完的那篇", lastActivityAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<WritingsLanding />);
    fireEvent.click(await screen.findByRole("button", { name: /我的写作/ }));
    const dialog = await screen.findByRole("dialog");

    // The chip is the button with aria-pressed; an unfinished ROW also
    // matches /还没写完/ by name because its dot's aria-label folds into the
    // row's own accessible name, so the chip has to be picked out from the
    // candidates rather than matched by name alone.
    const chip = within(dialog)
      .getAllByRole("button", { name: /还没写完/ })
      .find((el) => el.hasAttribute("aria-pressed"))!;
    fireEvent.click(chip);
    expect(dialog).toHaveTextContent("没写完的那篇");
    expect(screen.queryByText("写完的那篇")).toBeNull();
    expect(screen.getAllByLabelText("还没写完")).toHaveLength(1);
  });

  it("opens the notice onto 还没写完 and 我的写作 onto everything", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: {
        writings: [
          writing({ id: "done-1", title: "写完的那篇", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
          writing({ id: "open-1", title: "没写完的那篇", lastActivityAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<WritingsLanding />);

    fireEvent.click(await screen.findByText("你有 1 篇还没写完"));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("没写完的那篇");
    expect(screen.queryByText("写完的那篇")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    fireEvent.click(screen.getByRole("button", { name: /我的写作/ }));
    expect(await screen.findByText("写完的那篇")).toBeTruthy();
    expect(screen.getByText("没写完的那篇")).toBeTruthy();
  });

  it("sorts the shelf by when each writing last mattered, not by creation", async () => {
    routes[key("GET", "/api/v1/writings")] = {
      body: {
        writings: [
          // Created LAST and finished long ago; opened first and drafted all week.
          writing({ id: "b", title: "上周写完的", status: "finished", finishedAt: "2026-08-19T00:00:00Z" }),
          writing({ id: "a", title: "这周一直在写的", lastActivityAt: "2026-08-27T00:00:00Z" }),
        ],
      },
    };
    render(<WritingsLanding />);
    fireEvent.click(await screen.findByRole("button", { name: /我的写作/ }));

    const text = (await screen.findByRole("dialog")).textContent ?? "";
    expect(text.indexOf("这周一直在写的")).toBeLessThan(text.indexOf("上周写完的"));
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
