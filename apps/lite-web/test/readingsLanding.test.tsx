import { render, screen, waitFor, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ReadingsLanding, asLink, deriveTitle } from "@lite/readings/ReadingsLanding";
import { splitReadings, isFinished, shortDay } from "@lite/readings/ReadingHistoryPanel";
import { RECOMMENDED_READINGS } from "@lite/readings/recommendations";

/**
 * ReadingsLanding — the front door, driven through a stubbed `fetch` so the
 * assertions are about the REAL wiring (which endpoint, which body) rather
 * than about a hand-rolled double. Same harness shape as
 * readingRoomHost.test.tsx.
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
      const body =
        typeof raw === "string" ? JSON.parse(raw) : raw instanceof FormData ? "[FormData]" : undefined;
      calls.push({ method, url, body });
      const route = routes[key(method, url)];
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

function reading(over: Partial<Record<string, unknown>> = {}) {
  const row = {
    id: "r1",
    title: "城市为什么比郊区热？",
    lang: "zh",
    status: "reading",
    hasSource: true,
    createdAt: "2026-08-20T00:00:00Z",
    updatedAt: "2026-08-20T00:00:00Z",
    finishedAt: null,
    ...over,
  } as Record<string, unknown>;
  // lastActivityAt is what "last touched" actually means now (atom.
  // last_activity_at); updatedAt only moves on rename/finish. A fixture that
  // doesn't care about the distinction gets them equal.
  if (row.lastActivityAt === undefined) row.lastActivityAt = row.updatedAt;
  return row;
}

beforeEach(() => {
  routes = { [key("GET", "/api/v1/readings")]: { body: { readings: [] } } };
  stubFetch();
  window.history.replaceState(null, "", "/readings");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the greeting and its ink mark", () => {
  it("greets her with 读 circled, not as a plain heading", async () => {
    render(<ReadingsLanding />);
    const heading = await screen.findByRole("heading", { level: 1 });
    // 读 is its own span so it can be circled — the LINE still has to read
    // as one sentence.
    expect(heading.textContent).toBe("Hi，今天要读点什么");
    expect(heading.querySelector("path.lite-ink-ring")).not.toBeNull();
    // pathLength=1 is what lets the draw-on animation use an exact 1→0 dash.
    expect(heading.querySelector("path.lite-ink-ring")?.getAttribute("pathLength")).toBe("1");
  });

  it("wraps the paste box in the breathing halo", async () => {
    const { container } = render(<ReadingsLanding />);
    await screen.findByRole("heading", { level: 1 });
    expect(container.querySelector(".lite-glow")).not.toBeNull();
  });
});

describe("the paste box", () => {
  it("starts a reading from pasted body text, naming it from the first line", async () => {
    routes[key("POST", "/api/v1/readings")] = { status: 201, body: { id: "new-1" } };
    routes[key("PUT", "/api/v1/readings/new-1/source")] = { body: { title: "", sourceUrl: "", blocks: [] } };
    render(<ReadingsLanding />);

    fireEvent.change(await screen.findByPlaceholderText("贴一个链接，或者把整篇正文粘进来——也可以上传 DOCX / PDF"), {
      target: { value: "夏天的傍晚，城市比郊区热\n\n原因不止一个。" },
    });
    fireEvent.click(screen.getByRole("button", { name: "开始阅读" }));

    await waitFor(() => expect(window.location.pathname).toBe("/readings/new-1"));
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.body).toMatchObject({ title: "夏天的傍晚，城市比郊区热", text: expect.stringContaining("原因不止一个") });
  });

  it("sends a bare link as `url` so the server fetches it, not as a one-line article", async () => {
    routes[key("POST", "/api/v1/readings")] = { status: 201, body: { id: "new-2" } };
    routes[key("PUT", "/api/v1/readings/new-2/source")] = { body: { title: "", sourceUrl: "", blocks: [] } };
    render(<ReadingsLanding />);

    fireEvent.change(await screen.findByLabelText("文章正文或链接"), {
      target: { value: "  https://example.com/a-real-article  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "开始阅读" }));

    await waitFor(() => expect(window.location.pathname).toBe("/readings/new-2"));
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.body).toMatchObject({ url: "https://example.com/a-real-article", text: "" });
  });

  it("keeps the half-created reading on retry instead of minting another one", async () => {
    routes[key("POST", "/api/v1/readings")] = { status: 201, body: { id: "new-3" } };
    routes[key("PUT", "/api/v1/readings/new-3/source")] = {
      status: 500,
      body: { error: { code: "internal_error", message: "服务器开小差了" } },
    };
    render(<ReadingsLanding />);

    fireEvent.change(await screen.findByLabelText("文章正文或链接"), { target: { value: "一段正文。" } });
    fireEvent.click(screen.getByRole("button", { name: "开始阅读" }));
    await screen.findByText("服务器开小差了");

    routes[key("PUT", "/api/v1/readings/new-3/source")] = { body: { title: "", sourceUrl: "", blocks: [] } };
    fireEvent.click(screen.getByRole("button", { name: "开始阅读" }));
    await waitFor(() => expect(window.location.pathname).toBe("/readings/new-3"));

    expect(calls.filter((c) => c.method === "POST" && c.url === "/api/v1/readings")).toHaveLength(1);
  });

  it("uploads a DOCX through the multipart endpoint and keeps the name she typed", async () => {
    routes[key("POST", "/api/v1/readings")] = { status: 201, body: { id: "new-4" } };
    routes[key("POST", "/api/v1/readings/new-4/source/file")] = { body: { title: "文件里的标题", sourceUrl: "", blocks: [] } };
    routes[key("PATCH", "/api/v1/readings/new-4")] = { body: reading({ id: "new-4" }) };
    render(<ReadingsLanding />);

    fireEvent.change(await screen.findByPlaceholderText("给这次阅读起个名字（可留空）"), {
      target: { value: "老师发的材料" },
    });
    const picker = screen.getByLabelText("上传 DOCX / PDF") as HTMLInputElement;
    fireEvent.change(picker, {
      target: { files: [new File(["x"], "material.docx", { type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document" })] },
    });

    await waitFor(() => expect(window.location.pathname).toBe("/readings/new-4"));
    expect(calls.find((c) => c.url === "/api/v1/readings/new-4/source/file")?.body).toBe("[FormData]");
    // The server names the reading after the DOCUMENT — hers has to survive.
    expect(calls.find((c) => c.method === "PATCH")?.body).toMatchObject({ title: "老师发的材料" });
  });

  it("surfaces the server's own upload rejection", async () => {
    routes[key("POST", "/api/v1/readings")] = { status: 201, body: { id: "new-5" } };
    routes[key("POST", "/api/v1/readings/new-5/source/file")] = {
      status: 400,
      body: { error: { code: "unsupported_type", message: "只支持 PDF 或 Word 文档。" } },
    };
    render(<ReadingsLanding />);
    await screen.findByRole("heading", { level: 1 });

    fireEvent.change(screen.getByLabelText("上传 DOCX / PDF"), {
      target: { files: [new File(["x"], "photo.png", { type: "image/png" })] },
    });
    expect(await screen.findByText("只支持 PDF 或 Word 文档。")).toBeInTheDocument();
  });
});

describe("今日推荐", () => {
  it("offers a fixed shelf — no paging, no 'more'", async () => {
    render(<ReadingsLanding />);
    expect(await screen.findByText("不知道读什么？")).toBeInTheDocument();
    for (const rec of RECOMMENDED_READINGS) {
      expect(screen.getByText(rec.title)).toBeInTheDocument();
      expect(screen.getByText(rec.reason)).toBeInTheDocument();
    }
    // 铁律②: this is help, not a feed.
    expect(screen.queryByText(/加载更多|更多推荐|继续读/)).toBeNull();
  });

  it("starts a reading with the recommendation's own body and language", async () => {
    routes[key("POST", "/api/v1/readings")] = { status: 201, body: { id: "rec-1" } };
    routes[key("PUT", "/api/v1/readings/rec-1/source")] = { body: { title: "", sourceUrl: "", blocks: [] } };
    render(<ReadingsLanding />);

    const rec = RECOMMENDED_READINGS.find((r) => r.lang === "en")!;
    fireEvent.click(await screen.findByText(rec.title));

    await waitFor(() => expect(window.location.pathname).toBe("/readings/rec-1"));
    expect(calls.find((c) => c.method === "POST" && c.url === "/api/v1/readings")?.body).toMatchObject({
      title: rec.title,
      lang: "en",
    });
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.body).toMatchObject({ title: rec.title });
    expect((put.body as { text: string }).text.length).toBeGreaterThan(200);
  });

  it("every seeded entry carries a title, a one-line reason and real body text", () => {
    expect(RECOMMENDED_READINGS.length).toBeGreaterThanOrEqual(3);
    expect(RECOMMENDED_READINGS.length).toBeLessThanOrEqual(5);
    for (const rec of RECOMMENDED_READINGS) {
      expect(rec.title.trim()).not.toBe("");
      expect(rec.reason.trim()).not.toBe("");
      if (rec.source.kind === "text") {
        // more than one paragraph — the server anchors cards per block
        expect(rec.source.text.split("\n\n").length).toBeGreaterThan(1);
      } else {
        expect(rec.source.url).toMatch(/^https?:\/\//);
      }
    }
  });
});

describe("the unfinished notice and 我的阅读", () => {
  it("counts only unfinished readings", async () => {
    routes[key("GET", "/api/v1/readings")] = {
      body: {
        readings: [
          reading({ id: "a", updatedAt: "2026-08-24T00:00:00Z" }),
          reading({ id: "b", updatedAt: "2026-08-25T00:00:00Z" }),
          reading({ id: "c", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
        ],
      },
    };
    render(<ReadingsLanding />);
    expect(await screen.findByText("你有 2 篇还没读完")).toBeInTheDocument();
  });

  it("says nothing when everything is finished", async () => {
    routes[key("GET", "/api/v1/readings")] = {
      body: { readings: [reading({ id: "c", status: "finished", finishedAt: "2026-08-23T00:00:00Z" })] },
    };
    render(<ReadingsLanding />);
    await screen.findByRole("heading", { level: 1 });
    expect(screen.queryByText(/还没读完/)).toBeNull();
  });

  // "Most recently touched" means lastActivityAt (atom.last_activity_at), NOT
  // updatedAt — which only rename and finish ever write. The fixture here is
  // deliberately adversarial: `newer` was created first and has the OLDER
  // updatedAt, and is still the one she was last reading.
  it("jumps the notice straight into the most recently touched unfinished reading", async () => {
    routes[key("GET", "/api/v1/readings")] = {
      body: {
        readings: [
          reading({ id: "older", updatedAt: "2026-08-25T00:00:00Z", lastActivityAt: "2026-08-24T00:00:00Z" }),
          reading({ id: "newer", updatedAt: "2026-08-20T00:00:00Z", lastActivityAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<ReadingsLanding />);
    fireEvent.click(await screen.findByText("你有 2 篇还没读完"));
    expect(window.location.pathname).toBe("/readings/newer");
  });

  it("opens 我的阅读 with unfinished above finished, and routes both", async () => {
    routes[key("GET", "/api/v1/readings")] = {
      body: {
        readings: [
          reading({ id: "done-1", title: "读完的那篇", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
          reading({ id: "open-1", title: "没读完的那篇", updatedAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<ReadingsLanding />);
    fireEvent.click(await screen.findByRole("button", { name: /我的阅读/ }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("还没读完");
    expect(dialog).toHaveTextContent("已完成");
    // Unfinished offers 继续, finished offers 看报告.
    expect(dialog).toHaveTextContent("继续");
    expect(dialog).toHaveTextContent("看报告");

    // Order: the unfinished row precedes the finished one in the DOM.
    const text = dialog.textContent ?? "";
    expect(text.indexOf("没读完的那篇")).toBeLessThan(text.indexOf("读完的那篇"));

    fireEvent.click(screen.getByText("读完的那篇"));
    expect(window.location.pathname).toBe("/readings/done-1");
  });
});

describe("pure helpers", () => {
  it("asLink accepts a bare URL and nothing with prose around it", () => {
    expect(asLink("https://example.com/x")).toBe("https://example.com/x");
    expect(asLink("  http://example.com  ")).toBe("http://example.com");
    expect(asLink("看这个 https://example.com/x")).toBeNull();
    expect(asLink("一段正文。")).toBeNull();
  });

  it("deriveTitle takes the first line and truncates long ones", () => {
    expect(deriveTitle("城市为什么比郊区热？\n\n正文…")).toBe("城市为什么比郊区热？");
    expect(deriveTitle("\n\n  第二行才有字  \n更多")).toBe("第二行才有字");
    expect(deriveTitle("https://example.com/x")).toBe("");
    expect(deriveTitle("")).toBe("");
    expect([...deriveTitle("一".repeat(80))]).toHaveLength(25); // 24 + 省略号
  });

  it("isFinished trusts either the status or the stamp", () => {
    expect(isFinished(reading() as never)).toBe(false);
    expect(isFinished(reading({ status: "finished" }) as never)).toBe(true);
    expect(isFinished(reading({ finishedAt: "2026-08-01T00:00:00Z" }) as never)).toBe(true);
  });

  // Unfinished are ordered by lastActivityAt — where she left off — so the
  // updatedAt values here are deliberately in the OPPOSITE order.
  it("splitReadings puts unfinished first, each newest-first", () => {
    const rows = [
      reading({ id: "u-old", updatedAt: "2026-08-24T00:00:00Z", lastActivityAt: "2026-08-20T00:00:00Z" }),
      reading({ id: "f-new", status: "finished", finishedAt: "2026-08-25T00:00:00Z" }),
      reading({ id: "u-new", updatedAt: "2026-08-20T00:00:00Z", lastActivityAt: "2026-08-24T00:00:00Z" }),
      reading({ id: "f-old", status: "finished", finishedAt: "2026-08-10T00:00:00Z" }),
    ];
    const { unfinished, finished } = splitReadings(rows as never);
    expect(unfinished.map((r) => r.id)).toEqual(["u-new", "u-old"]);
    expect(finished.map((r) => r.id)).toEqual(["f-new", "f-old"]);
  });

  it("shortDay degrades to an empty string rather than rendering Invalid Date", () => {
    expect(shortDay(null)).toBe("");
    expect(shortDay("not-a-date")).toBe("");
    expect(shortDay(new Date().toISOString())).toBe("今天");
  });
});
