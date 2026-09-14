import { render, screen, waitFor, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ReadingsLanding, asLink, deriveTitle } from "@lite/readings/ReadingsLanding";
import { splitReadings, isFinished, shortDay } from "@lite/readings/ReadingHistoryPanel";

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

/** 一份最小的书架，形状照抄 Go 的 libraryShelfDTO。 */
function shelf(over: Record<string, unknown> = {}) {
  return {
    tier: 2,
    fields: [{ id: "science", zh: "科学与自然", field: "science" }],
    articles: [
      {
        slug: "nasa-osiris-rex",
        title: "NASA probe delivers asteroid samples",
        zhTitle: "贝努小行星的样本回到地球",
        reason: "七年任务、四十亿英里，以及着陆点中途更改的原因。",
        field: "science",
        tags: [{ id: "astronomy", zh: "天文与宇宙学", field: "science" }],
        coverUrl: "https://cdn.example/cover.webp?auth_key=x",
        levels: [
          { tier: 1, name: "入门", lexile: 430, words: 471, minutes: 4 },
          { tier: 2, name: "基础", lexile: 710, words: 650, minutes: 5 },
          { tier: 3, name: "进阶", lexile: 970, words: 783, minutes: 6 },
          { tier: 4, name: "高阶", lexile: 1130, words: 914, minutes: 7 },
          { tier: 5, name: "原文", lexile: 0, words: 973, minutes: 7 },
        ],
        finished: false,
      },
    ],
    recommended: [{ slug: "nasa-osiris-rex", tier: 3, why: ["天文与宇宙学"] }],
    ...over,
  };
}

beforeEach(() => {
  routes = {
    [key("GET", "/api/v1/readings")]: { body: { readings: [] } },
    [key("GET", "/api/v1/library")]: { body: shelf() },
  };
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

    fireEvent.change(await screen.findByPlaceholderText("贴一个链接，或者把整篇正文粘进来——也可以上传 PDF / DOCX / TXT"), {
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
    await screen.findByText(/服务器开小差了/);

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
    const picker = screen.getByLabelText("上传 PDF / DOCX / TXT") as HTMLInputElement;
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
      body: { error: { code: "unsupported_type", message: "只收这几种文件：.pdf / .docx / .txt / .md。" } },
    };
    render(<ReadingsLanding />);
    await screen.findByRole("heading", { level: 1 });

    fireEvent.change(screen.getByLabelText("上传 PDF / DOCX / TXT"), {
      target: { files: [new File(["x"], "photo.png", { type: "image/png" })] },
    });
    // 服务端说的那句话要原样到她眼前；外面裹了「后台错误：」，所以用包含匹配。
    // 🚨 用包含匹配的函数，不用正则字面量：这句话里有斜杠（.pdf / .docx），
    // 写进 /.../ 会把正则提前收尾。
    // 祖先元素的 textContent 也包含这句话，所以 findByText 会命中好几个 ——
    // 取最里面那一个。
    expect(
      (
        await screen.findAllByText((_, el) =>
          (el?.textContent ?? "").includes("只收这几种文件：.pdf / .docx / .txt / .md。"),
        )
      ).at(-1),
    ).toBeInTheDocument();
  });
});

describe("不知道读什么？ —— 分级阅读库的推荐位", () => {
  it("shows what the server recommended, at the level it recommended", async () => {
    render(<ReadingsLanding />);
    expect(await screen.findByText("不知道读什么？")).toBeInTheDocument();
    expect(screen.getByText("贝努小行星的样本回到地球")).toBeInTheDocument();
    expect(screen.getByText("七年任务、四十亿英里，以及着陆点中途更改的原因。")).toBeInTheDocument();
    // 推荐的是第 3 档，所以卡片上说的是进阶那一档的字数与时长，不是默认的基础档。
    expect(screen.getByText(/进阶 · 970L · 783 词/)).toBeInTheDocument();
  });

  // 「因为你关心 X」只在服务端真的给了理由时出现。树是空的时候 why 是空数组，
  // 那一行必须不在 —— 把补位的推荐说成按她的兴趣挑的，是在骗人。
  it("names the interest only when the server gave one", async () => {
    render(<ReadingsLanding />);
    expect(await screen.findByText("因为你关心天文与宇宙学")).toBeInTheDocument();

    cleanup();
    routes[key("GET", "/api/v1/library")] = {
      body: shelf({ recommended: [{ slug: "nasa-osiris-rex", tier: 2, why: [] }] }),
    };
    render(<ReadingsLanding />);
    await screen.findByText("贝努小行星的样本回到地球");
    expect(screen.queryByText(/因为你关心/)).toBeNull();
  });

  it("starts the reading through the library endpoint at the chosen tier", async () => {
    routes[key("POST", "/api/v1/library/nasa-osiris-rex/levels/3")] = {
      status: 201,
      body: { id: "lib-1" },
    };
    render(<ReadingsLanding />);
    fireEvent.click(await screen.findByText("读这一篇"));
    await waitFor(() => expect(window.location.pathname).toBe("/readings/lib-1"));
    // 旧书架是「建一篇 + PUT 正文」两步；库里的一篇由服务端一个事务开出来。
    expect(calls.some((c) => c.method === "POST" && c.url === "/api/v1/readings")).toBe(false);
  });

  it("keeps the page usable when the shelf itself fails", async () => {
    routes[key("GET", "/api/v1/library")] = { status: 500, body: { error: { code: "boom", message: "书架挂了" } } };
    render(<ReadingsLanding />);
    // 落地页的主入口照常在，报错不占「开始阅读」那一格。
    expect(await screen.findByPlaceholderText(/贴一个链接/)).toBeInTheDocument();
    expect(screen.queryByText("不知道读什么？")).toBeNull();
    expect(screen.queryByText("书架挂了")).toBeNull();
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

  // The notice used to JUMP into the most recently touched unfinished reading.
  // It is a count, and a count's only honest offer is the list behind it:
  //
  //   > when click "还有8篇没读完" I was directly navigated to a paper, but it
  //   > should pop the reading history list so I can select.
  //
  // The pathname assertion is the load-bearing half — an edit that restores
  // the shortcut still satisfies the drawer assertions and fails there.
  it("opens the shelf from the notice instead of picking a reading for her", async () => {
    routes[key("GET", "/api/v1/readings")] = {
      body: {
        readings: [
          reading({ id: "older", title: "先读的那篇", lastActivityAt: "2026-08-24T00:00:00Z" }),
          reading({ id: "newer", title: "后读的那篇", lastActivityAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<ReadingsLanding />);
    fireEvent.click(await screen.findByText("你有 2 篇还没读完"));

    // The shelf opens; nothing was chosen for her. What the shelf then holds
    // is the 我的阅读 test's job, not this one's.
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(window.location.pathname).toBe("/readings");
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

  // The two doors into the drawer want different chips, and which door she
  // came through has to beat whatever she last picked — so this asserts the
  // filter after a round trip through the OTHER entry, not just on first open.
  it("opens the notice onto 还没读完 and 我的阅读 onto everything", async () => {
    routes[key("GET", "/api/v1/readings")] = {
      body: {
        readings: [
          reading({ id: "done-1", title: "读完的那篇", status: "finished", finishedAt: "2026-08-23T00:00:00Z" }),
          reading({ id: "open-1", title: "没读完的那篇", lastActivityAt: "2026-08-25T00:00:00Z" }),
        ],
      },
    };
    render(<ReadingsLanding />);

    fireEvent.click(await screen.findByText("你有 1 篇还没读完"));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("没读完的那篇");
    // Pre-filtered: the finished one is counted on its chip but not listed.
    expect(screen.queryByText("读完的那篇")).toBeNull();
    // The dot is the at-a-glance signal, and only unfinished rows carry it.
    expect(screen.getAllByLabelText("还没读完")).toHaveLength(1);

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    fireEvent.click(screen.getByRole("button", { name: /我的阅读/ }));
    expect(await screen.findByText("读完的那篇")).toBeTruthy();
    expect(screen.getByText("没读完的那篇")).toBeTruthy();
  });

  it("sorts the shelf by when each reading last mattered, not by creation", async () => {
    routes[key("GET", "/api/v1/readings")] = {
      body: {
        readings: [
          // Created LAST and finished long ago; opened first and read all week.
          reading({ id: "b", title: "上周读完的", status: "finished", finishedAt: "2026-08-19T00:00:00Z" }),
          reading({ id: "a", title: "这周一直在读的", lastActivityAt: "2026-08-27T00:00:00Z" }),
        ],
      },
    };
    render(<ReadingsLanding />);
    fireEvent.click(await screen.findByRole("button", { name: /我的阅读/ }));

    const text = (await screen.findByRole("dialog")).textContent ?? "";
    expect(text.indexOf("这周一直在读的")).toBeLessThan(text.indexOf("上周读完的"));
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
