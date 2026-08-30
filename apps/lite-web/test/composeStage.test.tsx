import { render, screen, cleanup, fireEvent, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ComposeStage } from "@lite/writings/ComposeStage";
import type { WritingDraft, WritingSnippet } from "@lite/api/writingRoom";

/**
 * ComposeStage — 成稿 as a page.
 *
 * The test that earns its keep here is the re-assembly guard. Everything else
 * on this screen is recoverable; re-pulling from 段落 over a draft she has
 * edited HERE is not — it is the same silent-data-loss shape as the snippet
 * position collision this room already shipped once, and she would only notice
 * an afternoon later.
 */

const WID = "w1";

/** `hold` keeps the request in flight until `release()`, which is the only way
 *  to observe an in-flight state (the autosave marker) deterministically. */
type Route = { status?: number; body?: unknown; hold?: boolean };
let routes: Record<string, Route>;
let calls: { method: string; url: string; body: unknown }[];
let held: (() => void)[];

function release() {
  const pending = held;
  held = [];
  pending.forEach((r) => r());
}

function key(method: string, url: string) {
  return `${method} ${url}`;
}

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

function stubFetch() {
  calls = [];
  held = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      const body = init?.body ? JSON.parse(init.body as string) : undefined;
      calls.push({ method, url, body });
      const route = routes[key(method, url)];
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      const response = jsonResponse(route.status ?? 200, route.body ?? {});
      if (route.hold) await new Promise<void>((resolve) => held.push(resolve));
      return response;
    }),
  );
}

const base = (path: string) => `/api/v1/writings/${WID}${path}`;

const SNIPPET: WritingSnippet = {
  id: "s1",
  outlineId: "o1",
  outlineHeading: "夏天路上晒得受不了",
  position: 0,
  text: "去年七月我从校门口走到公交站，只有两百米，衣服全湿了。",
  updatedAt: "2026-08-28T00:00:00Z",
};

const COMMENT = {
  id: "cm1",
  scope: "draft",
  snippetId: null,
  summary: "整体清楚，但让步段还没有真正撞上反例。",
  points: [{ text: "这个例子说的是过密，不是数量多。", quote: "两排树掘得密密麻麻" }],
  createdAt: "2026-08-28T00:00:00Z",
};

function draftOf(body: string): WritingDraft {
  return { body, updatedAt: body ? "2026-08-28T00:00:00Z" : null };
}

function renderStage(over: { draft?: WritingDraft; snippets?: WritingSnippet[] } = {}) {
  const onDraftChange = vi.fn();
  const onFinished = vi.fn();
  const onRenamed = vi.fn();
  const utils = render(
    <ComposeStage
      writingId={WID}
      draft={over.draft ?? draftOf("")}
      snippets={over.snippets ?? [SNIPPET]}
      onDraftChange={onDraftChange}
      onFinished={onFinished}
      onRenamed={onRenamed}
    />,
  );
  return { ...utils, onDraftChange, onFinished, onRenamed };
}

const page = () => screen.getByRole("textbox") as HTMLTextAreaElement;
const composeCalls = () => calls.filter((c) => c.method === "POST" && c.url === base("/compose"));

beforeEach(() => {
  routes = {
    [key("GET", base("/comments"))]: { body: { comments: [] } },
    [key("POST", base("/compose"))]: { body: { body: "拼好的正文", updatedAt: "2026-08-28T00:01:00Z" } },
    [key("PUT", base("/draft"))]: { body: { body: "存下来的正文", updatedAt: "2026-08-28T00:02:00Z" } },
  };
  stubFetch();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the draft is there when she arrives", () => {
  it("shows an existing draft without her pressing anything, and does not re-pull over it", async () => {
    renderStage({ draft: draftOf("我自己写的正文。") });

    expect(page().value).toBe("我自己写的正文。");
    await waitFor(() => expect(calls.some((c) => c.url === base("/comments"))).toBe(true));
    expect(composeCalls()).toHaveLength(0);
  });

  it("assembles her paragraphs on arrival when the page is still blank", async () => {
    renderStage({ draft: draftOf("") });

    expect(await screen.findByDisplayValue("拼好的正文")).toBeTruthy();
    expect(composeCalls()).toHaveLength(1);
  });

  it("does not paint the assembled text over what she typed while it was in flight", async () => {
    // The arrival assemble is one non-LLM round trip on a blank page, but the
    // textarea is live throughout it. Before the guard, those keystrokes were
    // discarded on resolution AND the next autosave pushed the clobbered text
    // to the server — the same silent loss the re-assembly dialog exists for.
    routes[key("POST", base("/compose"))] = {
      hold: true,
      body: { body: "拼好的正文", updatedAt: "2026-08-28T00:01:00Z" },
    };
    renderStage({ draft: draftOf("") });

    await waitFor(() => expect(composeCalls()).toHaveLength(1));
    fireEvent.change(page(), { target: { value: "我趁它还在转的时候写的第一句。" } });

    release();
    // aria-busy clears in the same `finally` that used to write the assembled
    // text, so waiting on it is waiting on the resolution itself.
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "从段落重新拼一次" }).getAttribute("aria-busy"),
      ).toBeNull(),
    );

    expect(page().value).toBe("我趁它还在转的时候写的第一句。");
    // And nothing was ever saved on top of it.
    expect(
      calls.filter((c) => c.method === "PUT" && c.url === base("/draft")).map((c) => c.body),
    ).not.toContainEqual({ body: "拼好的正文" });
  });

  it("does not call compose when there are no paragraphs to assemble", async () => {
    renderStage({ draft: draftOf(""), snippets: [] });

    await waitFor(() => expect(calls.some((c) => c.url === base("/comments"))).toBe(true));
    expect(composeCalls()).toHaveLength(0);
  });
});

describe("the re-assembly guard", () => {
  it("refuses to re-assemble over an edited draft without confirming, and names what it would overwrite", async () => {
    renderStage({ draft: draftOf("") });
    await screen.findByDisplayValue("拼好的正文");

    fireEvent.change(page(), { target: { value: "拼好的正文，还有我在这一页上加的一整段。" } });
    fireEvent.click(screen.getByRole("button", { name: "从段落重新拼一次" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toMatch(/会覆盖/);
    // Not a vague "are you sure": it says what is on the page right now.
    expect(within(dialog).getByText(/现在这篇成稿有/)).toBeTruthy();
    // And it has NOT pulled yet.
    expect(composeCalls()).toHaveLength(1);
    expect(page().value).toBe("拼好的正文，还有我在这一页上加的一整段。");
  });

  it("keeps her text when she backs out of the dialog", async () => {
    renderStage({ draft: draftOf("") });
    await screen.findByDisplayValue("拼好的正文");

    fireEvent.change(page(), { target: { value: "我改过的正文。" } });
    fireEvent.click(screen.getByRole("button", { name: "从段落重新拼一次" }));
    fireEvent.click(await screen.findByRole("button", { name: "先不拼" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(composeCalls()).toHaveLength(1);
    expect(page().value).toBe("我改过的正文。");
  });

  it("re-assembles once she says so explicitly", async () => {
    renderStage({ draft: draftOf("") });
    await screen.findByDisplayValue("拼好的正文");

    fireEvent.change(page(), { target: { value: "我改过的正文。" } });
    fireEvent.click(screen.getByRole("button", { name: "从段落重新拼一次" }));
    fireEvent.click(await screen.findByRole("button", { name: "覆盖，重新拼" }));

    await waitFor(() => expect(composeCalls()).toHaveLength(2));
    expect(page().value).toBe("拼好的正文");
  });

  it("re-assembles freely while the page is still exactly what was assembled", async () => {
    renderStage({ draft: draftOf("") });
    await screen.findByDisplayValue("拼好的正文");

    fireEvent.click(screen.getByRole("button", { name: "从段落重新拼一次" }));

    await waitFor(() => expect(composeCalls()).toHaveLength(2));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("asks even about a draft it did not assemble itself — it cannot tell whose words those are", async () => {
    renderStage({ draft: draftOf("上次留在这里的正文。") });

    fireEvent.click(screen.getByRole("button", { name: "从段落重新拼一次" }));

    expect((await screen.findByRole("dialog")).textContent).toMatch(/会覆盖/);
    expect(composeCalls()).toHaveLength(0);
  });
});

describe("印记's comment, in the rail", () => {
  it("restores the last stored critique instead of losing it on navigation", async () => {
    routes[key("GET", base("/comments"))] = { body: { comments: [COMMENT] } };
    renderStage({ draft: draftOf("两排树掘得密密麻麻，看着很壮观。") });

    expect(await screen.findByText(COMMENT.summary)).toBeTruthy();
  });

  it("highlights the sentence a point is about when she clicks it", async () => {
    routes[key("GET", base("/comments"))] = { body: { comments: [COMMENT] } };
    const { container } = renderStage({ draft: draftOf("前面一句。两排树掘得密密麻麻，看着很壮观。") });
    await screen.findByText(COMMENT.summary);

    expect(container.querySelector("mark")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /这个例子说的是过密/ }));

    const mark = await waitFor(() => container.querySelector("mark")!);
    expect(mark.textContent).toBe("两排树掘得密密麻麻");
  });

  it("never writes a comment into her draft (铁律①)", async () => {
    routes[key("POST", base("/review"))] = { body: { comment: COMMENT } };
    renderStage({ draft: draftOf("两排树掘得密密麻麻。") });

    fireEvent.click(screen.getByRole("button", { name: "请印记看看" }));

    expect(await screen.findByText(COMMENT.summary)).toBeTruthy();
    expect(page().value).toBe("两排树掘得密密麻麻。");
  });
});

describe("the page itself", () => {
  it("saves on blur", async () => {
    renderStage({ draft: draftOf("原来的正文。") });

    fireEvent.change(page(), { target: { value: "改过的正文。" } });
    fireEvent.blur(page());

    await waitFor(() => {
      const put = calls.find((c) => c.method === "PUT" && c.url === base("/draft"));
      expect(put?.body).toEqual({ body: "改过的正文。" });
    });
  });

  it("marks the save with a fixed marker, so it cannot reflow the page under her hands", async () => {
    routes[key("PUT", base("/draft"))] = { hold: true, body: { body: "改过的正文。", updatedAt: null } };
    renderStage({ draft: draftOf("原来的正文。") });

    fireEvent.change(page(), { target: { value: "改过的正文。" } });
    fireEvent.blur(page());

    // An in-flow 保存中… shifts the paragraph under her cursor mid-sentence,
    // which is precisely what it used to do.
    const marker = await screen.findByText("保存中…");
    expect(marker.className).toMatch(/\bfixed\b/);
    release();
    await waitFor(() => expect(screen.queryByText("保存中…")).toBeNull());
  });

  it("does not finish a piece whose last edits failed to save", async () => {
    routes[key("PUT", base("/draft"))] = {
      status: 502,
      body: { error: { code: "save_failed", message: "保存失败，请重试。" } },
    };
    routes[key("POST", base("/finish"))] = { body: {} };
    const { onFinished } = renderStage({ draft: draftOf("原来的正文。") });

    fireEvent.change(page(), { target: { value: "最后改的一句，很重要。" } });
    fireEvent.click(screen.getByRole("button", { name: "完成这篇" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("保存失败，请重试。");
    expect(calls.some((c) => c.url === base("/finish"))).toBe(false);
    expect(onFinished).not.toHaveBeenCalled();
  });

  it("hands her a blank page with something a teacher would say on it", () => {
    renderStage({ draft: draftOf(""), snippets: [] });

    const placeholder = page().placeholder;
    expect(placeholder).not.toMatch(/拼出来的初稿会出现在这里/);
    expect(placeholder.length).toBeGreaterThan(0);
  });

  it("shows her paragraphs beside the page without a second editable copy", async () => {
    renderStage({ draft: draftOf("正文。") });

    expect(await screen.findByText(SNIPPET.text)).toBeTruthy();
    // One textbox on this screen: the page. The rail is read-only, so 段落 and
    // 成稿 can never disagree about which copy is current.
    expect(screen.getAllByRole("textbox")).toHaveLength(1);
  });
});

/**
 * 给这篇起个名字 — the last moment before the title becomes public.
 *
 * The title starts life as her raw 「我想写：…」 sentence, and at 完成这篇 it
 * becomes the report's hero, the exported poster's headline, and what a
 * stranger reads through the share link. This is a real branching flow with
 * four exits, and every one of them is invisible in a screenshot — exactly
 * what a logic test is for.
 */
describe("naming the piece at 完成这篇", () => {
  const RENAME_URL = `/api/v1/writings/${WID}`;
  const finishClick = () => fireEvent.click(screen.getByRole("button", { name: "完成这篇" }));
  const dialog = () => screen.findByRole("dialog");

  beforeEach(() => {
    routes[key("POST", base("/finish"))] = { body: { id: WID, title: "转弯中的国家", stage: "finished" } };
    routes[key("POST", base("/title-ideas"))] = {
      body: { needsName: true, ideas: ["转弯中的国家", "总量第一，人均第五十"] },
    };
    routes[key("PATCH", RENAME_URL)] = { body: { id: WID, title: "转弯中的国家", stage: "draft" } };
  });

  it("asks for a name before finishing, and finishes with the one she picks", async () => {
    const { onFinished, onRenamed } = renderStage({ draft: draftOf("我写完的正文。") });

    finishClick();
    await dialog();

    // The suggestions are offers: tapping one fills the box, nothing is saved
    // until she presses the button.
    fireEvent.click(screen.getByRole("button", { name: "总量第一，人均第五十" }));
    expect(calls.some((c) => c.method === "PATCH")).toBe(false);

    fireEvent.click(screen.getByRole("button", { name: "就叫这个，完成" }));

    await waitFor(() => expect(onFinished).toHaveBeenCalled());
    const rename = calls.find((c) => c.method === "PATCH" && c.url === RENAME_URL);
    expect(rename?.body).toEqual({ title: "总量第一，人均第五十" });
    // The header's EditableTitle must see it immediately, not at the next load.
    expect(onRenamed).toHaveBeenCalled();
  });

  it("she can type her own name over the suggestions", async () => {
    const { onFinished } = renderStage({ draft: draftOf("我写完的正文。") });

    finishClick();
    const box = within(await dialog()).getByLabelText("这篇文章叫") as HTMLInputElement;
    // Pre-filled with the first suggestion, so the fast path still produces a
    // real title — but it is an editable box, not a picker.
    expect(box.value).toBe("转弯中的国家");

    fireEvent.change(box, { target: { value: "看方向盘，不是看车道" } });
    fireEvent.click(screen.getByRole("button", { name: "就叫这个，完成" }));

    await waitFor(() => expect(onFinished).toHaveBeenCalled());
    expect(calls.find((c) => c.method === "PATCH")?.body).toEqual({ title: "看方向盘，不是看车道" });
  });

  // 铁律②: this is a question, not a gate.
  it("用原来的 finishes without renaming anything", async () => {
    const { onFinished, onRenamed } = renderStage({ draft: draftOf("我写完的正文。") });

    finishClick();
    await dialog();
    fireEvent.click(screen.getByRole("button", { name: "用原来的" }));

    await waitFor(() => expect(onFinished).toHaveBeenCalled());
    expect(calls.some((c) => c.method === "PATCH")).toBe(false);
    expect(onRenamed).not.toHaveBeenCalled();
  });

  it("never asks a student who already named her piece", async () => {
    routes[key("POST", base("/title-ideas"))] = { body: { needsName: false, ideas: [] } };
    const { onFinished } = renderStage({ draft: draftOf("我写完的正文。") });

    finishClick();

    await waitFor(() => expect(onFinished).toHaveBeenCalled());
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  // A failure asking for names must never cost her the finish — a missing
  // title prompt is a small loss, a 完成这篇 that refuses to work is a real one.
  it("finishes anyway when the suggestion call fails", async () => {
    routes[key("POST", base("/title-ideas"))] = {
      status: 503,
      body: { error: { code: "model_unavailable", message: "AI 暂时不可用" } },
    };
    const { onFinished } = renderStage({ draft: draftOf("我写完的正文。") });

    finishClick();

    await waitFor(() => expect(onFinished).toHaveBeenCalled());
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  // The opposite ruling to the one above, and deliberately so: this is HER
  // chosen name being dropped. Finishing under the placeholder anyway is the
  // exact outcome this whole flow exists to prevent.
  it("does not finish when saving the name fails", async () => {
    routes[key("PATCH", RENAME_URL)] = {
      status: 500,
      body: { error: { code: "save_failed", message: "名字没存上，请重试。" } },
    };
    const { onFinished } = renderStage({ draft: draftOf("我写完的正文。") });

    finishClick();
    await dialog();
    fireEvent.click(screen.getByRole("button", { name: "就叫这个，完成" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("名字没存上，请重试。");
    expect(onFinished).not.toHaveBeenCalled();
    expect(calls.some((c) => c.url === base("/finish"))).toBe(false);
  });

  // needsName true with an empty ideas list is a real state, not a bug: she
  // pressed 完成这篇 with nothing written, so there was nothing to name from.
  it("still offers the box when there is nothing to suggest", async () => {
    routes[key("POST", base("/title-ideas"))] = { body: { needsName: true, ideas: [] } };
    renderStage({ draft: draftOf("我写完的正文。") });

    finishClick();
    const box = within(await dialog()).getByLabelText("这篇文章叫") as HTMLInputElement;
    expect(box.value).toBe("");
  });
});
