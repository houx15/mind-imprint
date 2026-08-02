import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// WA · the Write room's 整稿体检 (check-my-draft). Mock the thin api modules
// WritingBlock calls directly.
vi.mock("@/workspace/api/workspace", () => ({
  getOutline: vi.fn(async () => []),
  putOutline: vi.fn(async () => []),
  getSnippets: vi.fn(async () => []),
  putSnippets: vi.fn(async () => []),
  getLibrary: vi.fn(async () => ({ collections: [], references: [] })),
  getDraft: vi.fn(async () => "我的草稿第一段。中国在可再生能源上的贡献是实质性的。"),
  coach: vi.fn(async () => ({ reply: "", proposal: null, linkOffer: null, dimSuggestion: null })),
  getCoachHistory: vi.fn(async () => []),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "ci1", reply: "" })),
  dismissProposal: vi.fn(async () => {}),
}));
vi.mock("@/api/writing", () => ({
  putBuffer: vi.fn(async () => {}),
  runDraftReview: vi.fn(),
}));
vi.mock("@/api/exploration", () => ({ getExploration: vi.fn(async () => ({ leads: [], danglingSourceIds: [] })) }));
vi.mock("@/workspace/export", () => ({ exportDraftDocx: vi.fn(async () => new Blob()) }));
vi.mock("@/api/projects", () => ({
  finishWriting: vi.fn(async () => ({ writingFinished: true })),
  reopenWriting: vi.fn(async () => ({ writingFinished: false })),
}));

import { runDraftReview, putBuffer } from "@/api/writing";
import { ApiError } from "@/api/client";
import { coach, getLibrary, getOutline, getSnippets, putSnippets, reflectProjectCard } from "@/workspace/api/workspace";
import { finishWriting, reopenWriting } from "@/api/projects";
import { getExploration } from "@/api/exploration";
import { exportDraftDocx } from "@/workspace/export";
import { WritingBlock, paragraphAtCaret } from "@/workspace/blocks/WritingBlock";

const mockFinishWriting = vi.mocked(finishWriting);
const mockReopenWriting = vi.mocked(reopenWriting);

// WC · M1 — the caret-fallback paragraph math. Real separator offsets, no drift.
describe("paragraphAtCaret (WC · M1)", () => {
  const src = "P0 第一段。\n\nP1 第二段。\n\n\n\nP2 第三段。"; // gaps of 2 and 4 newlines
  it("picks the block the caret sits in (start of a paragraph)", () => {
    const p1Start = src.indexOf("P1");
    expect(paragraphAtCaret(src, p1Start)).toBe("P1 第二段。");
  });
  it("does not drift across a 4-newline gap", () => {
    const p2Start = src.indexOf("P2");
    expect(paragraphAtCaret(src, p2Start)).toBe("P2 第三段。");
    expect(paragraphAtCaret(src, src.length)).toBe("P2 第三段。");
  });
  it("empty text yields empty (button no-ops)", () => {
    expect(paragraphAtCaret("", 0)).toBe("");
  });
});

const mockReview = vi.mocked(runDraftReview);
const mockPutBuffer = vi.mocked(putBuffer);
const mockExport = vi.mocked(exportDraftDocx);
const mockCoach = vi.mocked(coach);
const PROPOSAL = { objective: "论证中国是否让地球更可持续", reason: "r", activities: "a", resources: "res" };

beforeEach(() => {
  vi.clearAllMocks();
  mockReview.mockResolvedValue({
    items: [
      { criterion_code: "AO2", criterion_name: "分析与论证", band: "中段", evidence: "给出了一个反例", missing: "反例没有接回主张", fix: "把反例接回你的核心主张" },
    ],
    wordCount: 120,
    inBand: true,
  });
});

async function openDraftTab() {
  render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
  await userEvent.click(screen.getByRole("button", { name: "正文" }));
  const ta = (await screen.findByPlaceholderText(/在这里写你的草稿/)) as HTMLTextAreaElement;
  // getDraft resolves async — wait for the persisted draft to populate.
  await waitFor(() => expect(ta.value.length).toBeGreaterThan(0));
  return ta;
}

describe("WritingBlock · 整稿体检 (WA)", () => {
  it("runs the review and renders advice read-only, never editing the draft", async () => {
    const ta = await openDraftTab();
    const before = ta.value;
    expect(before).toContain("我的草稿第一段");

    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));

    await waitFor(() => expect(mockReview).toHaveBeenCalledWith("p1", before, "board"));
    expect(await screen.findByText(/把反例接回你的核心主张/)).toBeInTheDocument();
    expect(screen.getByText(/分析与论证/)).toBeInTheDocument();
    // 克制: the review is advice, it must not rewrite the draft.
    expect(ta.value).toBe(before);
  });

  it("re-runs with the chosen voice", async () => {
    await openDraftTab();
    await userEvent.selectOptions(screen.getByLabelText("体检视角"), "sceptic");
    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));
    await waitFor(() => expect(mockReview).toHaveBeenCalledWith("p1", expect.any(String), "sceptic"));
  });

  it("exports the body via 导出成品 (WB)", async () => {
    const ta = await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: /导出成品/ }));
    await waitFor(() => expect(mockExport).toHaveBeenCalledWith(ta.value, expect.objectContaining({ title: "T" })));
  });

  it("完成写作 opens a confirm modal that locks the draft then routes to Review (WB · #20)", async () => {
    const onOpenRoom = vi.fn();
    const refresh = vi.fn(async () => {});
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={refresh} onOpenRoom={onOpenRoom} />);
    await userEvent.click(screen.getByRole("button", { name: "完成写作" }));
    // it's a guarded moment — the modal shows, it doesn't lock/route immediately
    expect(mockFinishWriting).not.toHaveBeenCalled();
    expect(onOpenRoom).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /完成写作，去回顾/ }));
    await waitFor(() => expect(mockFinishWriting).toHaveBeenCalledWith("p1"));
    expect(refresh).toHaveBeenCalled();
    expect(onOpenRoom).toHaveBeenCalledWith("reflection");
  });

  it("writingFinished shows read-only draft + 重新打开写作 (WB · #20)", async () => {
    const refresh = vi.fn(async () => {});
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={true} refreshWorkspace={refresh} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: "正文" }));
    const ta = (await screen.findByPlaceholderText(/在这里写你的草稿/)) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value.length).toBeGreaterThan(0));
    expect(ta).toHaveAttribute("readonly");
    // reversible (铁律②): 重新打开写作 clears the milestone
    await userEvent.click(screen.getByRole("button", { name: "重新打开写作" }));
    await waitFor(() => expect(mockReopenWriting).toHaveBeenCalledWith("p1"));
    expect(refresh).toHaveBeenCalled();
  });

  it("archived project renders the draft read-only, no 体检 (WB · #5 lock)", async () => {
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="done" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: "正文" }));
    const ta = (await screen.findByPlaceholderText(/在这里写你的草稿/)) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value.length).toBeGreaterThan(0));
    expect(ta).toHaveAttribute("readonly");
    expect(screen.queryByRole("button", { name: "让印记体检整稿" })).toBeNull();
    expect(screen.getByText(/正文只读/)).toBeInTheDocument();
    // L2 · the writing coach rail is sealed too (no new turns/cards post-archive)
    expect(screen.queryByPlaceholderText("问问这段逻辑、这个结构……")).toBeNull();
    expect(screen.getByText(/过程已封存/)).toBeInTheDocument();
  });

  it("retries once on a transient failure, then surfaces the network-flavored message if both fail (item #10)", async () => {
    // #10 · one client-side retry guards a transient hiccup — both attempts
    // must fail before the error shows.
    mockReview.mockRejectedValueOnce(new Error("boom")).mockRejectedValueOnce(new Error("boom again"));
    await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));
    expect(await screen.findByText(/体检没跑完.*网络/)).toBeInTheDocument();
    expect(mockReview).toHaveBeenCalledTimes(2);
    // autosave still fired before the review attempt
    expect(mockPutBuffer).toHaveBeenCalled();
  });

  it("a transient failure that succeeds on retry never shows an error (item #10)", async () => {
    mockReview.mockRejectedValueOnce(new Error("boom"));
    const ta = await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));
    expect(await screen.findByText(/分析与论证/)).toBeInTheDocument();
    expect(screen.queryByText(/体检没跑完/)).toBeNull();
    expect(mockReview).toHaveBeenCalledTimes(2);
    expect(ta.value.length).toBeGreaterThan(0); // never touched the draft
  });

  it("a review_rejected server verdict shows its own message and does NOT retry (item #10)", async () => {
    mockReview.mockRejectedValueOnce(new ApiError("review_rejected", "这次体检没通过内部校验，请再试一次", 0));
    await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));
    expect(await screen.findByText("这次体检没通过内部校验，请再试一次。")).toBeInTheDocument();
    // a content decision — never retried client-side (the server already retried once itself).
    expect(mockReview).toHaveBeenCalledTimes(1);
  });

  it("summons a writing card from the deck into a modal (WC · card-hang, #3)", async () => {
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    // #8 · the writing-card shelf is open by default in the always-present rail
    // — still on the default 大纲 panel, whose deck is question-card/perspective-matrix/argument-map.
    const argumentMap = await screen.findByRole("button", { name: /论证地图卡/ });
    await userEvent.click(argumentMap);
    // StudioCardSheet mounts in the centered modal (its 工具卡 label + the card name)
    expect(await screen.findByText("工具卡")).toBeInTheDocument();
  });

  it("materials sidebar inserts a fragment into the draft at the caret (#9)", async () => {
    vi.mocked(getLibrary).mockResolvedValueOnce({
      collections: [],
      references: [
        {
          id: "rx", title: "来源X", classification: "", author: "", credentials: "", year: "", url: "",
          tags: [], collectionId: null, credibility: null, evaluation: "", readingNote: "我的笔记X",
          decision: null, pending: false, searchHints: [], materialId: "m", notes: [], takeaway: null,
        },
      ] as never,
    });
    const ta = await openDraftTab();
    const before = ta.value;
    // expand the material in the sidebar, then place it into the draft
    fireEvent.click(await screen.findByText("来源X"));
    fireEvent.click((await screen.findAllByRole("button", { name: /插入正文/ }))[0]!);
    await waitFor(() => expect(ta.value).toContain("我的笔记X"));
    expect(ta.value).toContain(before.slice(0, 6)); // original draft preserved
  });

  it("mind map grows from the keyboard: Enter adds a sibling, Tab adds a child (#11)", async () => {
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    // the outline tab is default; switch its inner view to the mind map
    await userEvent.click(screen.getByRole("button", { name: "思维导图" }));
    const nodeInputs = () => screen.getAllByPlaceholderText(/回车加同级/);
    // empty outline seeds one blank node
    await waitFor(() => expect(nodeInputs()).toHaveLength(1));
    nodeInputs()[0]!.focus();
    fireEvent.keyDown(nodeInputs()[0]!, { key: "Enter" });
    await waitFor(() => expect(nodeInputs()).toHaveLength(2));
    fireEvent.keyDown(nodeInputs()[0]!, { key: "Tab" });
    await waitFor(() => expect(nodeInputs()).toHaveLength(3));
  });

  it("floating 问印记 chip on a selection scopes the coach turn to it (WC · #7)", async () => {
    const ta = await openDraftTab();
    // simulate highlighting the first sentence, then releasing the mouse
    ta.setSelectionRange(0, 8);
    fireEvent.mouseUp(ta, { clientX: 20, clientY: 20 });
    const chip = await screen.findByRole("button", { name: /问印记/ });
    await userEvent.click(chip);
    // the pinned part appears in the rail + the composer switches to part-mode
    expect(await screen.findByText("就这一段")).toBeInTheDocument();
    const composer = screen.getByPlaceholderText("就这一段，你想问什么？");
    await userEvent.type(composer, "这段够有力吗{Enter}");
    await waitFor(() => {
      const [, scope, turn] = mockCoach.mock.calls.at(-1)!;
      expect(scope).toBe("writing");
      expect(turn).toContain("就这一段想");
      expect(turn).toContain("我的草稿第一段");
      expect(turn).toContain("这段够有力吗");
    });
  });

  it("分节 mode: generate sections from the outline and write under a heading (#5)", async () => {
    vi.mocked(getOutline).mockResolvedValue([{ id: "o1", text: "背景与主张", depth: 0, position: 0 }] as never);
    await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: "分节" }));
    await userEvent.click(await screen.findByRole("button", { name: /从大纲生成章节/ }));
    // the outline heading becomes an editable section heading
    expect(await screen.findByDisplayValue("背景与主张")).toBeInTheDocument();
    // write in the generated section's body → the draft serializes with the heading
    const bodies = screen.getAllByPlaceholderText("在这一节写……");
    await userEvent.type(bodies[bodies.length - 1]!, "这是正文。");
    // Q6 · the autosave debounce is ~1.2s (was 800ms) — give waitFor enough
    // room past the default 1s timeout.
    await waitFor(
      () => {
        const last = vi.mocked(putBuffer).mock.calls.at(-1)?.[1] ?? "";
        expect(last).toContain("# 背景与主张");
        expect(last).toContain("这是正文。");
      },
      { timeout: 3000 },
    );
  });

  // #6 · replaces the old assumption that the live outline auto-appears as a
  // 片段 board section — it must NOT, until the student explicitly imports it.
  it("片段 board: the live outline is not auto-rendered as a section (#6)", async () => {
    vi.mocked(getOutline).mockResolvedValueOnce([{ id: "o1", text: "背景与主张", depth: 0, position: 0 }] as never);
    vi.mocked(getSnippets).mockResolvedValueOnce([{ id: "s1", text: "我的片段", position: 0, section: null }] as never);
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    await screen.findByText("我的片段");
    expect(screen.queryByText("背景与主张")).toBeNull();
  });

  // Batch5 follow-up Item C · the board used to also auto-render a section for
  // every open 探索 线索 — that's now removed entirely; only imported outline
  // sections + 未归类 remain, even when the exploration graph has open leads.
  it("片段 board: no longer auto-creates a 线索 section from the exploration graph (Item C)", async () => {
    vi.mocked(getExploration).mockResolvedValueOnce({
      leads: [
        { id: "l1", text: "碳排放反例线索", status: "open", origin: "manual", sourceReferenceId: null, connectedReferenceId: null, position: 0, parentLeadId: null },
      ],
      danglingSourceIds: [],
    } as never);
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    await screen.findByText("未归类"); // the baseline section always renders
    expect(screen.queryByText("碳排放反例线索")).toBeNull();
    expect(screen.queryByText("线索")).toBeNull(); // no 线索-tagged section header anywhere
  });

  it("片段 board: importing the outline turns headings into foldable sections, and 归到 persists the section (#5/#6)", async () => {
    // Queue TWO resolutions — OutlinePane fetches once on the default 大纲 tab's
    // mount, and the materials sidebar fetches again lazily when its own 大纲
    // source is opened — so both calls see the same heading (not a leaked
    // permanent override that would bleed into later tests).
    const OUTLINE_ROW = [{ id: "o1", text: "背景与主张", depth: 0, position: 0 }] as never;
    vi.mocked(getOutline).mockResolvedValueOnce(OUTLINE_ROW).mockResolvedValueOnce(OUTLINE_ROW);
    vi.mocked(getSnippets).mockResolvedValueOnce([{ id: "s1", text: "我的片段", position: 0, section: null }] as never);
    // A distinct projectId — the imported-section set is a per-project client
    // memory (importedSectionsMemo) that outlives this render, so a shared
    // "p1" would leak "背景与主张" into every later test using that id.
    render(<WritingBlock projectId="p-outline-import" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    // the main 片段 tab (the sidebar also has a 片段 source tab) — the workspace tab comes first in DOM.
    // Switching off 大纲 first also unmounts OutlinePane's own list/思维导图 toggle
    // (which is ALSO labeled "大纲"), so the sidebar's "大纲" source tab becomes
    // the only remaining match.
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    // only 2 "大纲" matches now (the main tab + the sidebar's source tab) — the
    // sidebar's is last, since it renders after the main tab bar in the DOM.
    const outlineButtons = screen.getAllByRole("button", { name: "大纲" });
    await userEvent.click(outlineButtons[outlineButtons.length - 1]!);
    await userEvent.click(await screen.findByRole("button", { name: "把大纲导入为片段分组" }));
    // the section-toggle button's accessible name includes its label — a more
    // specific match than plain text, since "背景与主张" ALSO appears as an
    // <option> inside every snippet's 归到 <select>.
    expect(await screen.findByRole("button", { name: /背景与主张/ })).toBeInTheDocument();
    const select = await screen.findByLabelText("把片段归到");
    await userEvent.selectOptions(select, "背景与主张");
    await waitFor(
      () => {
        const last = vi.mocked(putSnippets).mock.calls.at(-1);
        expect(last?.[1]).toEqual([{ text: "我的片段", section: "背景与主张" }]);
      },
      { timeout: 2000 },
    );
  });

  it("selection chip can run the chosen voice's 体检 on just that paragraph (#8)", async () => {
    const ta = await openDraftTab();
    await userEvent.selectOptions(screen.getByLabelText("体检视角"), "sceptic");
    ta.setSelectionRange(0, 8);
    fireEvent.mouseUp(ta, { clientX: 20, clientY: 20 });
    await userEvent.click(await screen.findByRole("button", { name: "体检这段" }));
    // the review runs on the selected paragraph (not the whole draft), with the
    // chosen voice — and the panel says it checked just this段.
    await waitFor(() => expect(mockReview).toHaveBeenCalledWith("p1", "我的草稿第一段。", "sceptic"));
    expect(await screen.findByText(/体检了你选中的这一段/)).toBeInTheDocument();
  });

  // #9 · a snippet defaults to a read view; only double-click opens edit mode.
  it("片段 board: a snippet defaults to a read view; double-click edits, ✓ leaves edit mode (item A)", async () => {
    vi.mocked(getSnippets).mockResolvedValueOnce([{ id: "s1", text: "我的片段", position: 0, section: null }] as never);
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    const readView = await screen.findByText("我的片段");
    // read mode: no textarea holds this value yet
    expect(screen.queryByDisplayValue("我的片段")).toBeNull();
    await userEvent.dblClick(readView);
    const ta = await screen.findByDisplayValue("我的片段");
    expect(ta.tagName).toBe("TEXTAREA");
    await userEvent.click(screen.getByTitle("完成编辑"));
    await waitFor(() => expect(screen.queryByDisplayValue("我的片段")).toBeNull());
    expect(await screen.findByText("我的片段")).toBeInTheDocument();
  });

  it("片段 board: 「+ 在此加片段」opens the new blank row in edit mode immediately (item A)", async () => {
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    await userEvent.click(await screen.findByRole("button", { name: "+ 在此加片段" }));
    const ta = await screen.findByPlaceholderText("写下或粘贴一个片段……");
    expect(ta.tagName).toBe("TEXTAREA");
  });

  // #8 · finishing a writing card gets AI feedback FIRST (reflect turn), then
  // offers 收进片段 as an explicit action — never a silent labeled-value dump.
  it("finishing a writing card reflects to the coach first, then offers 收进片段 as a full paragraph (item C)", async () => {
    vi.mocked(reflectProjectCard).mockResolvedValueOnce({ cardInstanceId: "ci1", reply: "这个主张已经很清楚了。" });
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    // toulmin lives in the 片段/正文 decks, not 大纲 (the default panel) — switch first.
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    const toulmin = await screen.findByRole("button", { name: /论证构建卡/ });
    await userEvent.click(toulmin);
    const claimField = await screen.findByLabelText("你要论证的核心判断，用一句话说清。");
    await userEvent.type(claimField, "中国的可持续贡献是实质性的");
    await userEvent.click(screen.getByRole("button", { name: "提交并钉到过程树" }));

    await waitFor(() =>
      expect(reflectProjectCard).toHaveBeenCalledWith("p1", "toulmin", expect.any(Object), expect.any(Array), "writing"),
    );
    // the coach's reply shows in the thread, and the compiled student turn too
    expect(await screen.findByText(/这个主张已经很清楚了/)).toBeInTheDocument();
    expect(screen.getByText(/我刚填完《论证构建卡/)).toBeInTheDocument();

    // 收进片段 is an explicit offer — not auto-added
    expect(screen.getByText(/要不要把《论证构建卡/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "收进片段" }));
    expect(await screen.findByText(/收进了「片段」/)).toBeInTheDocument();

    // the 片段 board shows the compiled paragraph as the student's own words —
    // not a "**label**\ntext" dump. Query by the snippet's read-view button
    // role specifically — the coach rail's student-turn bubble (a plain div,
    // not a button) also contains this text and stays mounted alongside.
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    expect(await screen.findByRole("button", { name: /中国的可持续贡献是实质性的/ })).toBeInTheDocument();
  });

  it("an empty writing card is a no-op: no chat turn, no 收进片段 offer (item C)", async () => {
    vi.mocked(reflectProjectCard).mockResolvedValueOnce({ cardInstanceId: "", reply: "" });
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    const toulmin = await screen.findByRole("button", { name: /论证构建卡/ });
    await userEvent.click(toulmin);
    await userEvent.click(screen.getByRole("button", { name: "提交并钉到过程树" }));
    await waitFor(() => expect(reflectProjectCard).toHaveBeenCalled());
    expect(screen.queryByRole("button", { name: "收进片段" })).toBeNull();
  });

  // #8-second (item A) — the persistent rail shelf: always visible (no ＋ to
  // hide it). Batch5 follow-up: the CARD GROUP now depends on which main panel
  // (大纲/片段/正文) is active, and 正文·检查 (the four examiner voices) only
  // shows under 正文 — checking prose only makes sense once there's prose.
  it("the persistent tool shelf swaps card groups per panel; 正文·检查 only shows under 正文 (item A)", async () => {
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    // still on the default 大纲 tab — its deck, no examiner voices
    expect(await screen.findByRole("button", { name: /提问卡/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /视角对照矩阵/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /论证地图卡/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /论证构建卡/ })).toBeNull(); // toulmin isn't in the 大纲 deck
    expect(screen.queryByText(/正文·检查/)).toBeNull();
    expect(screen.queryByRole("button", { name: "评审团" })).toBeNull();
    // the old hidden-behind-＋ shelf is gone — nothing toggles it anymore
    expect(screen.queryByTitle("写作卡")).toBeNull();

    // 片段 — its own deck, still no examiner voices
    await userEvent.click(screen.getAllByRole("button", { name: "片段" })[0]!);
    expect(await screen.findByRole("button", { name: /PEE 写作卡/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /事实\/观点\/价值判断卡/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /让步段/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /论证构建卡/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /提问卡/ })).toBeNull(); // not in the 片段 deck
    expect(screen.queryByText(/正文·检查/)).toBeNull();

    // 正文 — its deck (overlaps 片段's, minus 事实/观点/价值判断) + 正文·检查 appears
    await userEvent.click(screen.getByRole("button", { name: "正文" }));
    expect(await screen.findByRole("button", { name: /PEE 写作卡/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /让步段/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /论证构建卡/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /论证地图卡/ })).toBeInTheDocument();
    expect(screen.getByText(/正文·检查/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "评审团" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "质疑者" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "门外汉" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "审判者" })).toBeInTheDocument();
  });

  it("正文's 质疑者 examiner-voice button runs a whole-draft 体检 (item A)", async () => {
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} onOpenRoom={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: "正文" }));
    // click 质疑者 in the persistent shelf, no scoped paragraph pinned
    await userEvent.click(await screen.findByRole("button", { name: "质疑者" }));
    await waitFor(() => expect(mockReview).toHaveBeenCalledWith("p1", expect.stringContaining("我的草稿第一段"), "sceptic"));
    expect(await screen.findByText(/印记的整稿体检/)).toBeInTheDocument();
  });

  it("once a paragraph is pinned, each voice grows a 这段 option that scopes just to it (item A)", async () => {
    const ta = await openDraftTab();
    ta.setSelectionRange(0, 8);
    fireEvent.mouseUp(ta, { clientX: 20, clientY: 20 });
    await userEvent.click(await screen.findByRole("button", { name: /问印记/ }));
    const scoped = await screen.findAllByRole("button", { name: "这段" });
    expect(scoped).toHaveLength(4); // one per voice, once a paragraph is pinned
    await userEvent.click(scoped[0]!); // paired with 评审团 (VOICE_ORDER[0] = board)
    await waitFor(() => expect(mockReview).toHaveBeenCalledWith("p1", "我的草稿第一段。", "board"));
    expect(await screen.findByText(/体检了你选中的这一段/)).toBeInTheDocument();
  });

  // #9-second (item B) — the sent bubble shows the referenced paragraph as a
  // styled quote callout, never a literal 【就这一段】 text token.
  it("styles a referenced paragraph as a quoted callout, not a literal 【就这一段】 token (item B)", async () => {
    const ta = await openDraftTab();
    ta.setSelectionRange(0, 8);
    fireEvent.mouseUp(ta, { clientX: 20, clientY: 20 });
    await userEvent.click(await screen.findByRole("button", { name: /问印记/ }));
    const composer = screen.getByPlaceholderText("就这一段，你想问什么？");
    await userEvent.type(composer, "这段够有力吗{Enter}");
    await waitFor(() => expect(mockCoach).toHaveBeenCalled());
    expect(screen.queryByText(/【就这一段】/)).toBeNull();
    expect(await screen.findByText("这段够有力吗")).toBeInTheDocument();
    const quote = document.querySelector("blockquote");
    expect(quote?.textContent).toBe("我的草稿第一段。");
  });

  // Q2 · once a 整稿体检 result exists, the draft and the review panel share a
  // responsive two-column container (grid-cols-1 lg:grid-cols-2) instead of
  // the review stacking below the textarea. No column split before a review
  // exists; closing the review (收起) collapses it back to one column.
  it("整稿体检 renders in a side-by-side container with the draft once a result exists (Q2)", async () => {
    const ta = await openDraftTab();
    function splitAncestorOf(el: HTMLElement): HTMLElement | null {
      let cur: HTMLElement | null = el;
      while (cur) {
        if (cur.className?.includes?.("lg:grid-cols-2")) return cur;
        cur = cur.parentElement;
      }
      return null;
    }
    // before running a review: no split container yet
    expect(splitAncestorOf(ta)).toBeNull();

    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));
    expect(await screen.findByText(/分析与论证/)).toBeInTheDocument();

    const split = splitAncestorOf(ta);
    expect(split).not.toBeNull();
    // the draft (textarea) and the review panel (收起 control) are both
    // inside that same split container, side by side.
    const collapse = screen.getByRole("button", { name: "收起" });
    expect(split!.contains(ta)).toBe(true);
    expect(split!.contains(collapse)).toBe(true);

    // closing the review collapses the layout back to a single column
    await userEvent.click(collapse);
    expect(screen.queryByRole("button", { name: "收起" })).toBeNull();
    expect(splitAncestorOf(ta)).toBeNull();
  });

  // Q6 · autosave debounces after typing stops, never loses local edits on a
  // failed save (shows a retry indicator and keeps retrying instead of
  // reverting the textarea), and confirms success once the retry lands.
  it(
    "autosaves on a debounce; a failed save keeps local edits and shows a retry indicator until it recovers (Q6)",
    async () => {
      mockPutBuffer.mockRejectedValueOnce(new Error("network down"));
      const ta = await openDraftTab();
      const before = ta.value;
      await userEvent.type(ta, "又写了一点。");
      const expected = before + "又写了一点。";

      // the debounce (~1.2s after the last keystroke) fires the first save
      // attempt, which fails.
      await waitFor(() => expect(mockPutBuffer).toHaveBeenCalledWith("p1", expected), { timeout: 2500 });
      expect(await screen.findByText(/未保存.*正在重试/)).toBeInTheDocument();
      // the local edit is never clobbered by the failed save.
      expect(ta.value).toBe(expected);

      // the backoff retry lands and succeeds — no student action needed.
      await waitFor(() => expect(mockPutBuffer).toHaveBeenCalledTimes(2), { timeout: 5000 });
      expect(mockPutBuffer).toHaveBeenLastCalledWith("p1", expected);
      expect(await screen.findByText("已保存")).toBeInTheDocument();
      expect(ta.value).toBe(expected); // still untouched by any save round-trip
    },
    12000,
  );

  // Q6 · switching away from 正文 (unmounting DraftPane) flushes any pending
  // dirty edit rather than silently dropping it.
  it("flushes a pending autosave when the 正文 tab is left (Q6)", async () => {
    const ta = await openDraftTab();
    const before = ta.value;
    fireEvent.change(ta, { target: { value: before + "最后一句还没保存。" } });
    // switch away immediately, before the debounce would have fired on its own.
    // "大纲" is ambiguous (the main tab + the materials sidebar's own source
    // tab) — the main workspace tab comes first in the DOM.
    await userEvent.click(screen.getAllByRole("button", { name: "大纲" })[0]!);
    await waitFor(() => expect(mockPutBuffer).toHaveBeenCalledWith("p1", before + "最后一句还没保存。"));
  });
});
