import { StrictMode, useState } from "react";
import { render, screen, waitFor, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SnippetsStage } from "@lite/writings/SnippetsStage";
import type { Comment, WritingBlockGuide, WritingOutlineItem, WritingSnippet } from "@lite/api/writingRoom";

/**
 * SnippetsStage — 段落, driven directly (not through the whole room) since
 * both B2 and H1 are entirely local to this component's own slot-building
 * and save logic.
 *
 * `Harness` mimics the one bit of WritingRoomHost this component actually
 * depends on: a parent that re-renders it with whatever `onSnippetsChange`
 * hands back, the same controlled-child relationship StagePanel gives it in
 * the real room.
 */

const WID = "w1";

let calls: { method: string; url: string; body: unknown }[];

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

function stubFetch(handler: (method: string, url: string, body: unknown) => { status?: number; body?: unknown } | undefined) {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      const body = init?.body ? JSON.parse(init.body as string) : undefined;
      calls.push({ method, url, body });
      const route = handler(method, url, body);
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function Harness({
  outline,
  initialSnippets,
}: {
  outline: WritingOutlineItem[];
  initialSnippets: WritingSnippet[];
}) {
  const [snippets, setSnippets] = useState(initialSnippets);
  return (
    <SnippetsStage
      writingId={WID}
      outline={outline}
      snippets={snippets}
      onSnippetsChange={setSnippets}
      onGoToStructure={() => {}}
      onGoToDraft={() => {}}
      onSay={async () => {}}
    />
  );
}

describe("B2 — every persisted snippet must stay visible and editable", () => {
  it("shows the paragraph added via 加一段 even though a confirmed outline exists", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "打工能带来的收获", role: "", depth: 0, position: 0 },
      { id: "o2", text: "打工的代价", role: "", depth: 0, position: 1 },
    ];
    stubFetch((method, url) => {
      if (method === "PUT" && url === `/api/v1/writings/${WID}/snippets`) {
        return {
          body: {
            snippets: [{ id: "s3", outlineId: null, outlineHeading: "", position: 2, text: "" }],
          },
        };
      }
      return undefined;
    });

    render(<Harness outline={outline} initialSnippets={[]} />);
    expect(screen.getAllByPlaceholderText("写这一段……")).toHaveLength(2);

    fireEvent.click(screen.getByRole("button", { name: "加一段" }));

    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    // Bug: buildSlots returns ONLY outline-derived slots whenever an outline
    // exists, so the newly-persisted free paragraph (position 2, no
    // outlineId) never gets a slot — the PUT succeeds but nothing new
    // renders.
    await waitFor(() => expect(screen.getAllByPlaceholderText("写这一段……")).toHaveLength(3));
  });

  it("keeps a free paragraph written before the outline existed visible after the outline is confirmed", async () => {
    const outline: WritingOutlineItem[] = [{ id: "o1", text: "打工能带来的收获", role: "", depth: 0, position: 0 }];
    // position 5 — nowhere near any outline point's position, so this can
    // only be shown by NOT being silently swallowed into buildSlots'
    // outline-only branch (a position collision with an outline slot would
    // paper over the bug rather than reproduce it).
    const freeSnippet: WritingSnippet = {
      id: "s0",
      outlineId: null,
      outlineHeading: "",
      position: 5,
      text: "这是提纲确认之前写的自由段落",
      updatedAt: "",
    };
    stubFetch(() => undefined);

    render(<Harness outline={outline} initialSnippets={[freeSnippet]} />);

    // One slot for the outline point, plus one for the pre-existing free
    // paragraph that has no outline link at all.
    expect(await screen.findByDisplayValue("这是提纲确认之前写的自由段落")).toBeTruthy();
  });
});

describe("H1 — snippets link to outline points by id, never by position", () => {
  it("matches a snippet to its outline point by outlineId, not by array position", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "第一部分", role: "", depth: 0, position: 0 },
      { id: "o2", text: "第二部分", role: "", depth: 0, position: 1 },
    ];
    // Deliberately mismatched: this snippet is genuinely linked to o2 (the
    // SECOND outline point) but its own storage `position` is 0 — the same
    // position o1 happens to occupy. Position-matching would wrongly file it
    // under o1.
    const snippet: WritingSnippet = {
      id: "s1",
      outlineId: "o2",
      outlineHeading: "第二部分",
      position: 0,
      text: "这是第二部分的内容",
      updatedAt: "",
    };
    stubFetch(() => undefined);

    render(<Harness outline={outline} initialSnippets={[snippet]} />);

    const textarea = (await screen.findByDisplayValue("这是第二部分的内容")) as HTMLTextAreaElement;
    const block = textarea.closest("div.rounded-mk-md")!;
    expect(block.textContent).toContain("第二部分");
    expect(block.textContent).not.toContain("第一部分");
  });

  it("does not persist a client-guessed outlineId back onto an already-linked snippet", async () => {
    const outline: WritingOutlineItem[] = [{ id: "o1", text: "第一部分", role: "", depth: 0, position: 0 }];
    const snippet: WritingSnippet = {
      id: "s1",
      outlineId: "o1",
      outlineHeading: "第一部分",
      position: 0,
      text: "已有的内容",
      updatedAt: "",
    };
    stubFetch((method, url) => {
      if (method === "PUT" && url === `/api/v1/writings/${WID}/snippets`) {
        return { body: { snippets: [{ ...snippet, text: "改过的内容" }] } };
      }
      return undefined;
    });

    render(<Harness outline={outline} initialSnippets={[snippet]} />);
    const textarea = screen.getByDisplayValue("已有的内容");
    fireEvent.change(textarea, { target: { value: "改过的内容" } });
    fireEvent.blur(textarea);

    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    const put = calls.find((c) => c.method === "PUT")!;
    const posted = (put.body as { snippets: { outlineId: string | null }[] }).snippets[0]!;
    // The server already has the correct link (possibly just repaired by
    // heading text after an outline edit). Resending our own locally-cached
    // outlineId on every ordinary text save risks clobbering that repair
    // with a client-side guess — so an update to an EXISTING snippet must
    // omit outlineId (server semantics: absent = preserve).
    expect(posted.outlineId).toBeNull();
  });
});

describe("free paragraphs must sit where an outline can never grow into them", () => {
  it("mints a free paragraph past any outline position, so a later outline point cannot overwrite it", async () => {
    // The bug this guards, found by the whole-branch re-review. position is
    // writing_snippet's upsert key, and outline positions are array indices
    // reassigned on every outline save. Under "one past the current maximum",
    // a free paragraph minted beside a one-point outline lands at position 1 —
    // exactly where a SECOND outline point goes the next time she adds one.
    // The first save of that new outline slot then upserts onto her free
    // paragraph's row: her text is destroyed and the row is relinked to a
    // heading she never wrote it under. Silent, and only noticed much later.
    const outline: WritingOutlineItem[] = [{ id: "o1", text: "打工能带来的收获", role: "", depth: 0, position: 0 }];
    stubFetch((method, url) => {
      if (method === "PUT" && url === `/api/v1/writings/${WID}/snippets`) {
        return { body: { snippets: [] } };
      }
      return undefined;
    });

    render(<Harness outline={outline} initialSnippets={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "加一段" }));

    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    const put = calls.find((c) => c.method === "PUT")!;
    const posted = (put.body as { snippets: { position: number }[] }).snippets[0];
    expect(posted).toBeTruthy();
    const position = posted!.position;
    expect(
      position,
      `free paragraph minted at position ${position}; an outline grown to ${position + 1} points would upsert onto this row and destroy her text`,
    ).toBeGreaterThanOrEqual(1000);
  });
});

/**
 * Guidance on arrival (Task 11, spec B1).
 *
 * The failure this pins is not a crash — it is a student who never finds the
 * teaching. While the guide was only reachable through 「卡住了？」, the
 * student least likely to press it was exactly the one who needed it most.
 */

const GUIDE: WritingBlockGuide = {
  job: "这一段要让读者相信「便宜」这个说法不成立。",
  methods: [
    {
      name: "正反",
      formalName: "对比论证",
      definition: "一正一反两个例子放在一起。",
      examples: [{ topic: "两个菜市场", text: "东街留了装卸区…" }],
      patterns: [],
    },
  ],
  questions: ["你见过哪条街上的树长不开？"],
};

describe("B1 — the guiding box is there when she arrives", () => {
  it("paints the guide the outline already carries, with no call and no click", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0, guide: GUIDE },
    ];
    stubFetch(() => undefined);

    render(<Harness outline={outline} initialSnippets={[]} />);

    // Present on the FIRST render — not after a click, not after an await.
    expect(screen.getByText("你见过哪条街上的树长不开？")).toBeTruthy();
    expect(screen.getByText(/这一段要让读者相信/)).toBeTruthy();
    // …and nothing was spent to show it: the guide was already stored.
    await waitFor(() => expect(calls.every((c) => !c.url.endsWith("/guide"))).toBe(true));
  });

  it("asks the batch route once when no block has been guided yet", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0 },
      { id: "o2", text: "谁来养", role: "结尾", depth: 0, position: 1 },
    ];
    stubFetch((method, url) => {
      if (method === "POST" && url === `/api/v1/writings/${WID}/guide`) {
        return { body: { guides: { o1: GUIDE } } };
      }
      return undefined;
    });

    render(<Harness outline={outline} initialSnippets={[]} />);

    expect(await screen.findByText("你见过哪条街上的树长不开？")).toBeTruthy();
    // ONE batch call for the whole outline — not one per block, and never a
    // retry loop: a failed or empty batch must not bill again on re-render.
    expect(calls.filter((c) => c.url === `/api/v1/writings/${WID}/guide`)).toHaveLength(1);
  });

  it("keeps 卡住了？ for a block with nothing yet, and regenerates one that has a guide", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0, guide: GUIDE },
    ];
    stubFetch((method, url) => {
      if (method === "POST" && url === `/api/v1/writings/${WID}/outline/o1/guide`) {
        return { body: { ...GUIDE, questions: ["换个角度：这笔钱本来打算花在哪儿？"] } };
      }
      return undefined;
    });

    render(<Harness outline={outline} initialSnippets={[]} />);
    expect(screen.queryByRole("button", { name: /卡住了/ })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /换一组问题/ }));
    expect(await screen.findByText("换个角度：这笔钱本来打算花在哪儿？")).toBeTruthy();
  });

  it("收起 hides the box without throwing the guidance away", () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0, guide: GUIDE },
    ];
    stubFetch(() => undefined);

    render(<Harness outline={outline} initialSnippets={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "收起" }));
    expect(screen.queryByText("你见过哪条街上的树长不开？")).toBeNull();

    // Reopening must not cost a model call for something we already hold.
    fireEvent.click(screen.getByRole("button", { name: "打开引导" }));
    expect(screen.getByText("你见过哪条街上的树长不开？")).toBeTruthy();
    expect(calls.filter((c) => c.method === "POST")).toHaveLength(0);
  });
});

describe("B3 — 深入一层 opens the SAME 印记, never a second character", () => {
  it("opens a drawer headed 印记 · 关于这一块 and returns to the stored thread", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0, guide: GUIDE },
    ];
    stubFetch((method, url) => {
      if (method === "GET" && url === `/api/v1/writings/${WID}/outline/o1/deepen`) {
        return { body: { messages: [{ seq: 3, role: "ai", content: "上次我们聊到养护的钱", createdAt: "" }] } };
      }
      return undefined;
    });

    render(<Harness outline={outline} initialSnippets={[]} />);
    fireEvent.click(screen.getByRole("button", { name: /深入一层/ }));

    const drawer = await screen.findByRole("dialog");
    expect(drawer.textContent).toContain("印记 · 关于这一块");
    // The thing that must never appear: a second name, a second avatar, or an
    // "assistant/helper" label. This drawer is 印记, scoped to one block.
    expect(drawer.textContent).not.toMatch(/助手|小老师|助教|assistant/i);
    // The thread persists per block — reopening returns to the conversation.
    expect(await screen.findByText("上次我们聊到养护的钱")).toBeTruthy();
  });

  it("sends a turn to the block-scoped endpoint and shows the reply", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0, guide: GUIDE },
    ];
    stubFetch((method, url) => {
      if (url !== `/api/v1/writings/${WID}/outline/o1/deepen`) return undefined;
      if (method === "GET") return { body: { messages: [] } };
      return { body: { reply: "那笔钱如果不花在树上，本来会花在哪儿？" } };
    });

    render(<Harness outline={outline} initialSnippets={[]} />);
    fireEvent.click(screen.getByRole("button", { name: /深入一层/ }));
    await screen.findByRole("dialog");

    const composer = screen.getByPlaceholderText("说说你卡在哪儿");
    fireEvent.change(composer, { target: { value: "我不知道怎么算这笔账" } });
    fireEvent.keyDown(composer, { key: "Enter" });

    expect(await screen.findByText("那笔钱如果不花在树上，本来会花在哪儿？")).toBeTruthy();
    const post = calls.find((c) => c.method === "POST" && c.url === `/api/v1/writings/${WID}/outline/o1/deepen`);
    expect(post).toBeTruthy();
    expect((post!.body as { text: string }).text).toBe("我不知道怎么算这笔账");
  });
});

/**
 * B4 at paragraph zoom — 请印记看看这一段.
 *
 * The endpoint (Task 5) and the renderer (Task 10) both existed; the button
 * belonged to neither task brief, so 段落 had no way to reach either.
 */

const PARAGRAPH = "街上的树种得太密，夏天反而不凉快。修剪的钱也没人出。";

const BLOCK_COMMENT: Comment = {
  id: "c1",
  scope: "block",
  snippetId: "s1",
  summary: "理由说清楚了，但没说是谁的钱。",
  points: [{ text: "这句只说了结果，没说原因。", quote: "修剪的钱也没人出。" }],
  createdAt: "",
};

const linkedSnippet: WritingSnippet = {
  id: "s1",
  outlineId: "o1",
  outlineHeading: "种树不便宜",
  position: 0,
  text: PARAGRAPH,
  updatedAt: "",
};

const OUTLINE_O1: WritingOutlineItem[] = [
  { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0, guide: GUIDE },
];

describe("B4 — 请印记看看这一段", () => {
  it("comments on this paragraph and renders it through the shared CommentPanel", async () => {
    stubFetch((method, url) => {
      if (method === "POST" && url === `/api/v1/writings/${WID}/snippets/s1/comment`) {
        return { body: { comment: BLOCK_COMMENT } };
      }
      return undefined;
    });

    render(<Harness outline={OUTLINE_O1} initialSnippets={[linkedSnippet]} />);
    fireEvent.click(screen.getByRole("button", { name: /请印记看看这一段/ }));

    expect(await screen.findByText("理由说清楚了，但没说是谁的钱。")).toBeTruthy();
    expect(screen.getByText("这句只说了结果，没说原因。")).toBeTruthy();
    // The shared renderer, not a second one built for this stage.
    expect(document.querySelectorAll("[data-comment-point]")).toHaveLength(1);
  });

  it("saves the block first, so 印记 judges what is actually on screen", async () => {
    stubFetch((method, url) => {
      if (method === "PUT" && url === `/api/v1/writings/${WID}/snippets`) {
        return { body: { snippets: [{ ...linkedSnippet, text: `${PARAGRAPH}还得有人天天浇。` }] } };
      }
      if (method === "POST" && url === `/api/v1/writings/${WID}/snippets/s1/comment`) {
        return { body: { comment: BLOCK_COMMENT } };
      }
      return undefined;
    });

    render(<Harness outline={OUTLINE_O1} initialSnippets={[linkedSnippet]} />);
    fireEvent.change(screen.getByDisplayValue(PARAGRAPH), {
      target: { value: `${PARAGRAPH}还得有人天天浇。` },
    });
    fireEvent.click(screen.getByRole("button", { name: /请印记看看这一段/ }));

    await screen.findByText("理由说清楚了，但没说是谁的钱。");
    // Order matters: commenting on a stale save would anchor every point to
    // sentences she has since rewritten.
    const put = calls.findIndex((c) => c.method === "PUT" && c.url === `/api/v1/writings/${WID}/snippets`);
    const post = calls.findIndex((c) => c.url === `/api/v1/writings/${WID}/snippets/s1/comment`);
    expect(put).toBeGreaterThanOrEqual(0);
    expect(post).toBeGreaterThan(put);
  });

  it("refuses to spend a call on an empty paragraph", async () => {
    const empty: WritingSnippet = { ...linkedSnippet, text: "" };
    stubFetch(() => undefined);

    render(<Harness outline={OUTLINE_O1} initialSnippets={[empty]} />);
    fireEvent.click(screen.getByRole("button", { name: /请印记看看这一段/ }));

    expect(await screen.findByText(/这一段还没有内容/)).toBeTruthy();
    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/comment"))).toBe(false));
  });

  it("shows a stored comment on arrival, not only right after generating one", async () => {
    stubFetch((method, url) => {
      if (method === "GET" && url === `/api/v1/writings/${WID}/comments`) {
        return {
          body: {
            comments: [
              // A draft-scope comment belongs to 成稿 and must not surface here.
              { ...BLOCK_COMMENT, id: "c0", scope: "draft", snippetId: null, summary: "整篇的评语" },
              BLOCK_COMMENT,
            ],
          },
        };
      }
      return undefined;
    });

    render(<Harness outline={OUTLINE_O1} initialSnippets={[linkedSnippet]} />);

    expect(await screen.findByText("理由说清楚了，但没说是谁的钱。")).toBeTruthy();
    expect(screen.queryByText("整篇的评语")).toBeNull();
  });
});

describe("B4 — tracing a point back to the sentence, or honestly not at all", () => {
  async function renderWithComment(comment: Comment, snippet: WritingSnippet) {
    stubFetch((method, url) => {
      if (method === "GET" && url === `/api/v1/writings/${WID}/comments`) {
        return { body: { comments: [comment] } };
      }
      return undefined;
    });
    render(<Harness outline={OUTLINE_O1} initialSnippets={[snippet]} />);
    await screen.findByText(comment.summary);
    return document.querySelector("[data-comment-point]") as HTMLButtonElement;
  }

  it("selects the quoted sentence in the block's own textarea", async () => {
    const point = await renderWithComment(BLOCK_COMMENT, linkedSnippet);
    const textarea = screen.getByDisplayValue(PARAGRAPH) as HTMLTextAreaElement;

    fireEvent.click(point);

    const at = PARAGRAPH.indexOf("修剪的钱也没人出。");
    expect(textarea.selectionStart).toBe(at);
    expect(textarea.selectionEnd).toBe(at + "修剪的钱也没人出。".length);
    expect(document.activeElement).toBe(textarea);
  });

  it("does NOTHING when the quote is no longer in the text — never an approximate highlight", async () => {
    // She rewrote the sentence after the comment was generated. An
    // approximate match here would land on a neighbouring sentence and teach
    // her something false about her own paragraph — the same reason the
    // server drops points whose quote is not a literal substring.
    const rewritten: WritingSnippet = { ...linkedSnippet, text: "街上的树种得太密，夏天反而不凉快。" };
    const point = await renderWithComment(BLOCK_COMMENT, rewritten);
    const textarea = screen.getByDisplayValue(rewritten.text) as HTMLTextAreaElement;

    fireEvent.click(point);

    expect(textarea.selectionStart).toBe(0);
    expect(textarea.selectionEnd).toBe(0);
    expect(document.activeElement).not.toBe(textarea);
  });
});

/**
 * StrictMode regression — the trap that cost the writing walk a red run.
 *
 * `main.tsx` renders the whole app inside `<React.StrictMode>`, so in dev
 * React mounts every effect, runs its cleanup, and mounts it again. A
 * "fire only once" `useRef` latch combined with a per-invocation
 * `let cancelled = false` cleanup flag disagree under that: pass 1 sets the
 * latch and fires the request, the cleanup marks THAT closure cancelled,
 * pass 2 is skipped because the latch is set — and when the one real
 * request lands, the only closure watching it has been told to drop it.
 * Result: `batching` never clears and the guide box never appears, on a
 * request the server answered perfectly.
 *
 * These tests render through StrictMode on purpose. The ones above do not,
 * which is exactly why they stayed green while the room hung.
 */
describe("guidance must survive React StrictMode's double-invoked effects", () => {
  it("paints the batch guide after mount → cleanup → remount, with exactly one call", async () => {
    const outline: WritingOutlineItem[] = [
      { id: "o1", text: "种树不便宜", role: "中心论点", depth: 0, position: 0 },
      { id: "o2", text: "谁来养", role: "结尾", depth: 0, position: 1 },
    ];
    stubFetch((method, url) => {
      if (method === "POST" && url === `/api/v1/writings/${WID}/guide`) {
        return { body: { guides: { o1: GUIDE } } };
      }
      return undefined;
    });

    render(
      <StrictMode>
        <Harness outline={outline} initialSnippets={[]} />
      </StrictMode>,
    );

    expect(await screen.findByText("你见过哪条街上的树长不开？")).toBeTruthy();
    // Still exactly one metered call — the fix must not buy its correctness
    // by letting StrictMode bill the model twice.
    expect(calls.filter((c) => c.url === `/api/v1/writings/${WID}/guide`)).toHaveLength(1);
    // And the "印记 is thinking" line must be gone: the `finally` that clears
    // it is the first casualty of the cancelled-closure trap.
    await waitFor(() => expect(screen.queryByText(/印记正在把每一块都先想一遍/)).toBeNull());
  });
});
