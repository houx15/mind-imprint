import { render, screen, waitFor, fireEvent, within, act } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { StudioContainer } from "./StudioContainer";
import * as apiModule from "../api";

const projection = {
  project: { title: "China sustainability", qualLabel: "0457 个人报告" },
  stations: [
    { code: "S0", name: "任务解码", view: "评估", state: "done" },
    { code: "S4", name: "论证构建", view: "结构", state: "current", gate: { total: 7, passed: 2 } },
  ],
  activeStation: "S4",
  coach: { anchor: "论证图 · 治理决心主张", messages: [], equipment: [] },
  onboarding: { restatePrompt: "r", rubricRows: [], planSteps: [] },
  structure: [] as unknown[],
};

const fakeApi = {
  listProjects: async () => [{ id: "p1", title: "China sustainability", qualLabel: "0457 个人报告", activeStation: "S4" }],
  getProject: async () => projection,
} as any;

// Several tests below trigger a click handler that never returns its
// internal promise chain to the caller (e.g. onSubmitCard's
// `conv.submitCard(...).then(refetchProject).catch(setSyncError)` is fire-
// and-forget — fireEvent.click doesn't await it, React doesn't await it,
// nothing does). The only way to observe it settle is to wait for it —
// and `waitFor`'s default budget is a REAL 1000ms wall-clock timeout that
// races against however long the JS engine takes to actually get around to
// draining a several-hops-deep microtask chain. In isolation that's instant;
// under full-suite CPU contention (many worker processes competing for the
// same cores) the process can go unscheduled for long enough that the
// chain hasn't settled by the time `waitFor` gives up — a false failure,
// not a logic race (the outcome is always the same; only the wall-clock
// time to observe it varies).
//
// `flush` sidesteps that by never imposing its own budget: a macrotask
// (setTimeout) only runs once the JS engine has fully drained the
// microtask queue — including any microtasks newly queued while draining —
// so awaiting one deterministically waits for an arbitrarily deep .then/
// .catch chain to finish, however long the engine takes to get there,
// bounded only by vitest's own generous per-test timeout (not this file's
// assertions). Wrapped in `act` so the resulting setState calls are
// flushed/batched the same way React expects.
async function flush() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

describe("StudioContainer", () => {
  it("renders the Studio from live projection data", async () => {
    render(<StudioContainer api={fakeApi} />);
    // "论证构建" renders twice by design: once in the station rail item, once
    // as the ViewFrame header for the active station — use getAllByText.
    await waitFor(() => expect(screen.getAllByText("论证构建").length).toBeGreaterThan(0));
    // The anchor renders in more than one place (CoachRail header + DispositionCard).
    expect(screen.getAllByText(/论证图 · 治理决心主张/).length).toBeGreaterThan(0);
  });

  it("shows an error state when loading fails", async () => {
    const failing = { listProjects: async () => { throw new Error("boom"); }, getProject: async () => projection } as any;
    render(<StudioContainer api={failing} />);
    await waitFor(() => expect(screen.getByText(/加载失败|出错|重试/)).toBeInTheDocument());
  });

  it("sends a composer message and renders the live coach reply", async () => {
    // NOTE: getSnapshot must return a STABLE reference across calls — the
    // real createStudioConversation's getSnapshot closes over `let state`
    // reassigned only in `set()`, so it satisfies useSyncExternalStore's
    // contract. A fresh object per call here would trip React's infinite
    // re-render guard, which is exactly the deviation this fixture must not
    // reintroduce (see StudioContainer.tsx's useSyncExternalStore usage).
    const snapshot = {
      messages: [{ kind: "ai", body: "连到治理决心", tag: "D5", anchor: "论证图 · 治理决心主张" }],
      sending: false,
      error: null,
      disposableInterventionId: "iid",
    };
    const conv = {
      getSnapshot: () => snapshot,
      subscribe: () => () => {},
      send: vi.fn(async () => {}),
      dispose: vi.fn(async () => {}),
    };
    render(<StudioContainer api={fakeApi} makeConversation={() => conv as any} />);
    // "论证构建" renders twice (rail item + ViewFrame header) — see the first
    // test's comment above; use getAllByText for the same reason here.
    await waitFor(() => expect(screen.getAllByText("论证构建").length).toBeGreaterThan(0));
    // the live coach reply from the controller is rendered — it appears
    // both in the thread and mirrored in the DispositionCard body, so use
    // getAllByText for the same reason as above.
    await waitFor(() => expect(screen.getAllByText("连到治理决心").length).toBeGreaterThan(0));
  });

  it("wires the composer's send action through to conv.send", async () => {
    const snapshot = { messages: [], sending: false, error: null, disposableInterventionId: null };
    const conv = {
      getSnapshot: () => snapshot,
      subscribe: () => () => {},
      send: vi.fn(async () => {}),
      dispose: vi.fn(async () => {}),
    };
    render(<StudioContainer api={fakeApi} makeConversation={() => conv as any} />);
    await waitFor(() => expect(screen.getAllByText("论证构建").length).toBeGreaterThan(0));

    fireEvent.change(screen.getByPlaceholderText(/发给印记/), { target: { value: "我加了一条证据" } });
    fireEvent.click(screen.getByLabelText(/发送|send/i));

    expect(conv.send).toHaveBeenCalledWith("我加了一条证据");
  });

  it("wires the card from the conversation to the rail", async () => {
    // getSnapshot must return a STABLE reference across calls (see the
    // "sends a composer message" test's note above) — a fresh object
    // literal per call trips useSyncExternalStore's tearing check and
    // triggers React's infinite-update guard.
    const snapshot = { messages: [], sending: false, error: null, disposableInterventionId: null, card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "proposed" } };
    const conv = { getSnapshot: () => snapshot, subscribe: () => () => {}, send: vi.fn(), dispose: vi.fn(), openCard: vi.fn(), submitCard: vi.fn(), skipCard: vi.fn() };
    render(<StudioContainer api={fakeApi} makeConversation={() => conv as any} />);
    await waitFor(() => expect(screen.getAllByText("论证构建").length).toBeGreaterThan(0));
    // the craap proposal is visible in the rail (name + purpose text both
    // match the regex, so use getAllByText per this file's convention above)
    await waitFor(() => expect(screen.getAllByText(/CRAAP|来源|核查/).length).toBeGreaterThan(0));
  });

  // CRITICAL (whole-branch review): before FIX-D, losing the client's only
  // reference to an open card was merely a lost card. Now that FIX-D
  // suppresses EVERY surface_card candidate project-wide while any
  // card_instance is proposed/active, a page reload that fails to rehydrate
  // the open card would brick the workspace forever — no card would ever be
  // able to surface again. This test mounts StudioContainer FRESH (the real
  // createStudioConversation, no SSE snapshot at all) against a projection
  // whose server-side `activeCard` is already "active", and proves the card
  // is actually RENDERED in the coach rail from that alone.
  it("rehydrates an active card from the projection on a fresh mount — no SSE snapshot at all (CRITICAL)", async () => {
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => ({
        ...projection,
        stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
        activeStation: "S3",
        materials: [],
        activeCard: { cardInstanceId: "ci1", cardId: "craap", status: "active", anchors: [], materialId: "" },
      }),
    };
    // The REAL conversation controller (no makeConversation override) — this
    // is the actual production wiring, not a stub standing in for it.
    render(<StudioContainer api={api as never} />);
    // "工具卡 · CRAAP" alone would be a vacuous assertion — CraapPlaceholder
    // (the NO-card fallback for this same view) carries the identical label.
    // "锁定，进下一条" is StudioAnnotateCard's own submit control, rendered
    // only for an actually-open active card — proof this is the real card,
    // not the placeholder.
    await screen.findByText("锁定，进下一条");
  });

  it("shows an honest empty state when the student has no projects", async () => {
    const api = { listProjects: vi.fn(async () => []), getProject: vi.fn() };
    render(<StudioContainer api={api as any} />);
    await waitFor(() => expect(screen.getByText("还没有项目")).toBeTruthy());
    expect(screen.queryByText("加载失败，请重试")).toBeNull();
    expect(api.getProject).not.toHaveBeenCalled();
  });

  it("shows the error state when the request fails", async () => {
    const api = { listProjects: vi.fn(async () => { throw new Error("boom"); }), getProject: vi.fn() };
    render(<StudioContainer api={api as any} />);
    await waitFor(() => expect(screen.getByText("加载失败，请重试")).toBeTruthy());
    expect(screen.queryByText("还没有项目")).toBeNull();
  });

  it("never signs in on its own — auth is the shell's job", async () => {
    const signin = vi.spyOn(apiModule.api, "signin");
    const api = { listProjects: vi.fn(async () => { throw new Error("401"); }), getProject: vi.fn() };
    render(<StudioContainer api={api as any} />);
    await waitFor(() => expect(screen.getByText("加载失败，请重试")).toBeTruthy());
    expect(signin).not.toHaveBeenCalled();
  });

  it("projects the server's materials into the 素材 view — no fixture", async () => {
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => ({
        ...projection,
        // this file's base `projection` only carries S0/S4 — add S3 (素材)
        // so ViewFrame's station lookup resolves and renders the dossier.
        stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
        activeStation: "S3",
        materials: [{
          id: "m1", title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
          origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
          locked: false, role: "", tier: "二手 · 需追源", takeaway: "结论被放大了。", anchors: [],
        }],
      }),
    };
    render(<StudioContainer api={api as never} />);
    // The material's tier + takeaway are both set, so the title legitimately
    // renders twice once Slice 6b's 检索日志 ledger (Task 8) mounts below the
    // source list — scope the query to the source-list card itself (its
    // sibling, the ledger, is a separate, deliberately duplicate render) so
    // this doesn't just pass because the title showed up *somewhere*.
    await screen.findByText(/信源档案 · 已收集 1 篇/);
    expect(within(screen.getByTestId("dossier-source-list")).getByText("《卫星图看中国变绿》")).toBeInTheDocument();
  });

  it("posts a new source and shows it in the dossier", async () => {
    const newMaterial = {
      id: "m2",
      title: "《IPCC AR6 综合报告》",
      sourceUrl: "https://ipcc.ch/report",
      kind: "article",
      origin: "fetched",
      blocks: [{ id: "b1", text: "……" }],
      locked: false,
      role: "",
      tier: "机构报告",
      takeaway: "报告本身的口径。",
      anchors: [],
    };
    const baseProjection = {
      ...projection,
      stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
      activeStation: "S3",
      materials: [] as unknown[],
    };
    let getProjectCalls = 0;
    const addMaterial = vi.fn(async () => newMaterial);
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        return getProjectCalls === 1 ? baseProjection : { ...baseProjection, materials: [newMaterial] };
      },
      addMaterial,
      logSourceOpen: vi.fn(async () => {}),
    };

    render(<StudioContainer api={api as never} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    fireEvent.click(screen.getByText("添加信源"));
    fireEvent.change(screen.getByPlaceholderText(/粘贴链接/), { target: { value: "https://ipcc.ch/report" } });
    fireEvent.change(screen.getByPlaceholderText(/一句话说说/), { target: { value: "报告本身的口径。" } });
    fireEvent.click(screen.getByLabelText("机构报告"));
    fireEvent.click(screen.getByText("加入信源档案"));

    await waitFor(() =>
      expect(addMaterial).toHaveBeenCalledWith("p1", { url: "https://ipcc.ch/report", takeaway: "报告本身的口径。", tier: "机构报告" }),
    );
    await waitFor(() =>
      expect(within(screen.getByTestId("dossier-source-list")).getByText(newMaterial.title)).toBeInTheDocument(),
    );
  });

  it("posts the reading time when the student leaves a source", async () => {
    const material = {
      id: "m1",
      title: "《卫星图看中国变绿》",
      sourceUrl: "https://x.test/a",
      kind: "article",
      origin: "fetched",
      blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false,
      role: "",
      tier: "二手 · 需追源",
      takeaway: "结论被放大了。",
      anchors: [],
    };
    const logSourceOpen = vi.fn(async () => {});
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => ({
        ...projection,
        stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
        activeStation: "S3",
        materials: [material],
      }),
      addMaterial: vi.fn(),
      logSourceOpen,
    };

    render(<StudioContainer api={api as never} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    // Fake timers only wrap the synchronous open→advance→close sequence —
    // reportOpenElapsed fires onOpenLogged synchronously from the click
    // handler, so there's no async gap here that would fight
    // testing-library's own (real-timer) polling.
    vi.useFakeTimers();
    fireEvent.click(within(screen.getByTestId("dossier-source-list")).getByText(material.title));
    vi.advanceTimersByTime(30_000);
    fireEvent.click(screen.getByText("返回信源列表"));
    vi.useRealTimers();

    expect(logSourceOpen).toHaveBeenCalledWith("p1", "m1", 30);
  });

  it("refetches the projection after a card submit resolves — the lock and the persisted anchors must render without a page reload", async () => {
    const materialId = "m1";
    const anchor = {
      id: "a1", material_id: materialId, block_id: "b1", start: 0, end: 4,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "只是一个博主",
    };
    const riskNoteAnchor = {
      id: "risk_note", material_id: materialId, block_id: "", start: 0, end: 0, quote: "",
      dimension: "risk_note", author: "student", question: "这条来源在你的论证里起什么作用？有什么风险 / 局限？",
      answer: "触发关注的入口——需要横向核实。",
    };
    const unlockedMaterial = {
      id: materialId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [],
    };
    const lockedMaterial = {
      ...unlockedMaterial, locked: true, role: "触发关注的入口——需要横向核实。",
      anchors: [anchor, riskNoteAnchor],
    };

    let getProjectCalls = 0;
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        const materials = getProjectCalls === 1 ? [unlockedMaterial] : [lockedMaterial];
        return {
          ...projection,
          stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
          activeStation: "S3",
          materials,
        };
      },
    };

    const snapshot = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "active", anchors: [anchor] },
    };
    const submitCard = vi.fn(async () => {});
    const conv = {
      getSnapshot: () => snapshot,
      subscribe: () => () => {},
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      submitCard,
      skipCard: vi.fn(),
      dropFirst: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();
    expect(screen.getByText("待评估")).toBeInTheDocument();

    // This field only exists once StudioContainer's second effect (creating
    // the live conversation, gated on projectId) has committed — a separate,
    // later render than the one `findByText` above resolved on. Under
    // full-suite CPU contention that second commit can lag behind a plain
    // synchronous `getByPlaceholderText`, so use the awaited `find*` query
    // instead of assuming it already landed.
    fireEvent.change(await screen.findByPlaceholderText(/这条来源在你的论证里起什么作用/), {
      target: { value: "触发关注的入口——需要横向核实。" },
    });
    fireEvent.click(screen.getByText("锁定，进下一条"));

    expect(submitCard).toHaveBeenCalled();
    // The stale-until-reload bug: without a refetch, getProject is never
    // called a second time and the chip/role never update.
    await waitFor(() => expect(getProjectCalls).toBe(2));
    await waitFor(() => expect(screen.getByText("✓ 已锁定")).toBeInTheDocument());
    expect(screen.getByText(/作用与风险：触发关注的入口——需要横向核实。/)).toBeInTheDocument();
  });

  it("keeps a message that enters the buffer AFTER the refetch's GET is issued but BEFORE it resolves (bug A)", async () => {
    const midFlightTurn = { kind: "student", body: "那反例呢？" };
    let convState: any = { messages: [], sending: false, error: null, disposableInterventionId: null, card: null };
    const listeners = new Set<() => void>();
    const emit = () => listeners.forEach((l) => l());
    const conv = {
      getSnapshot: () => convState,
      subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      submitCard: vi.fn(async () => {}),
      skipCard: vi.fn(async () => {}),
      dropFirst: vi.fn((n: number) => {
        convState = { ...convState, messages: convState.messages.slice(n) };
        emit();
      }),
    };

    const newMaterial = {
      id: "m2", title: "《IPCC AR6 综合报告》", sourceUrl: "https://ipcc.ch/report", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "……" }],
      locked: false, role: "", tier: "机构报告", takeaway: "报告本身的口径。", anchors: [],
    };
    const baseProjection = {
      ...projection,
      stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
      activeStation: "S3",
      materials: [] as unknown[],
    };

    let getProjectCalls = 0;
    let resolveSecondGet!: (v: unknown) => void;
    const addMaterial = vi.fn(async () => newMaterial);
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        if (getProjectCalls === 1) return baseProjection;
        // Held open until the test resolves it — models the real network
        // round trip an add-source ingestion GET takes.
        return new Promise((resolve) => { resolveSecondGet = resolve; });
      },
      addMaterial,
      logSourceOpen: vi.fn(async () => {}),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    fireEvent.click(screen.getByText("添加信源"));
    fireEvent.change(screen.getByPlaceholderText(/粘贴链接/), { target: { value: "https://ipcc.ch/report" } });
    fireEvent.change(screen.getByPlaceholderText(/一句话说说/), { target: { value: "报告本身的口径。" } });
    fireEvent.click(screen.getByLabelText("机构报告"));
    fireEvent.click(screen.getByText("加入信源档案"));

    // The GET is now in flight (issued, unresolved).
    await waitFor(() => expect(getProjectCalls).toBe(2));

    // A live turn enters the buffer WHILE the GET is in flight — exactly
    // what happens if the student keeps chatting during ingestion.
    act(() => {
      convState = { ...convState, messages: [midFlightTurn] };
      emit();
    });
    await waitFor(() => expect(screen.getAllByText(midFlightTurn.body).length).toBeGreaterThan(0));

    // Now let the GET resolve.
    resolveSecondGet({ ...baseProjection, materials: [newMaterial] });

    await waitFor(() =>
      expect(within(screen.getByTestId("dossier-source-list")).getByText(newMaterial.title)).toBeInTheDocument(),
    );
    // The message that arrived mid-flight must still be on screen — exactly
    // once, not wiped by the reconciliation.
    expect(screen.getAllByText(midFlightTurn.body)).toHaveLength(1);
  });

  it("keeps the article's highlighted anchors visible across a card submit → refetch transition (bug B)", async () => {
    const materialId = "m1";
    const anchor = {
      id: "a1", material_id: materialId, block_id: "b1", start: 0, end: 4,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "只是一个博主",
    };
    const riskNoteAnchor = {
      id: "risk_note", material_id: materialId, block_id: "", start: 0, end: 0, quote: "",
      dimension: "risk_note", author: "student", question: "这条来源在你的论证里起什么作用？有什么风险 / 局限？",
      answer: "触发关注的入口——需要横向核实。",
    };
    const unlockedMaterial = {
      id: materialId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [],
    };
    const lockedMaterial = {
      ...unlockedMaterial, locked: true, role: "触发关注的入口——需要横向核实。",
      anchors: [anchor, riskNoteAnchor],
    };

    let getProjectCalls = 0;
    let resolveSecondGet!: (v: unknown) => void;
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        if (getProjectCalls === 1) {
          return {
            ...projection,
            stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
            activeStation: "S3",
            materials: [unlockedMaterial],
          };
        }
        return new Promise((resolve) => { resolveSecondGet = resolve; });
      },
      // The test opens the source (to render its highlighted spans) and
      // never navigates back before the component unmounts at test-end —
      // SourceDossier's unmount cleanup reports the reading-time sample, so
      // this needs a fake, same as every other test that opens a source.
      logSourceOpen: vi.fn(async () => {}),
    };

    let convState: any = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "active", anchors: [anchor] },
    };
    const listeners = new Set<() => void>();
    const emit = () => listeners.forEach((l) => l());
    const conv = {
      getSnapshot: () => convState,
      subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      // Mirrors the real controller: card is nulled as soon as submit's SSE
      // "done" frame lands — well before the refetch below resolves.
      submitCard: vi.fn(async () => {
        convState = { ...convState, card: null };
        emit();
      }),
      skipCard: vi.fn(async () => {}),
      dropFirst: vi.fn((n: number) => {
        convState = { ...convState, messages: convState.messages.slice(n) };
        emit();
      }),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    // Open the source so its blocks (and the card's live anchor highlight)
    // actually render.
    fireEvent.click(within(screen.getByTestId("dossier-source-list")).getByText(unlockedMaterial.title));
    // The highlight comes from convSnapshot.card, which only exists once
    // StudioContainer's SECOND effect (creating the live conversation, gated
    // on projectId) has committed — a separate, later render than the one
    // `findByText(/信源档案/)` above already resolved on. Under full-suite
    // CPU contention that second commit can lag behind this synchronous
    // check; wait for it explicitly instead of assuming it already landed
    // (this is the shared root cause behind this file's flakiness — see the
    // `findByPlaceholderText` note below for the same race).
    await waitFor(() => expect(document.querySelectorAll("mark").length).toBeGreaterThan(0));

    // Same race as the `mark` wait above — this field only exists once the
    // live card (from that second, later effect/commit) is mounted, so use
    // the awaited `find*` query rather than assuming `getBy*` already sees it.
    fireEvent.change(await screen.findByPlaceholderText(/这条来源在你的论证里起什么作用/), {
      target: { value: "触发关注的入口——需要横向核实。" },
    });
    fireEvent.click(screen.getByText("锁定，进下一条"));

    await waitFor(() => expect(conv.submitCard).toHaveBeenCalled());
    await waitFor(() => expect(getProjectCalls).toBe(2));

    // The card is gone (submitCard's own "done" cleared it) but the refetch
    // that would restore the persisted anchors hasn't landed yet — the
    // highlight must not disappear in this gap.
    expect(document.querySelectorAll("mark").length).toBeGreaterThan(0);

    resolveSecondGet({
      ...projection,
      stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
      activeStation: "S3",
      materials: [lockedMaterial],
    });

    // The open article view (not the list) is still on screen — assert on
    // the persisted role text it shows once the refetch lands.
    await waitFor(() =>
      expect(screen.getByText(/作用与风险：触发关注的入口——需要横向核实。/)).toBeInTheDocument(),
    );
    expect(document.querySelectorAll("mark").length).toBeGreaterThan(0);
  });

  it("catches a refetch failure after a successful card submit — no unhandled rejection, no false success shown (bug C)", async () => {
    const materialId = "m1";
    const anchor = {
      id: "a1", material_id: materialId, block_id: "b1", start: 0, end: 4,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "只是一个博主",
    };
    const unlockedMaterial = {
      id: materialId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [],
    };

    let getProjectCalls = 0;
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        if (getProjectCalls === 1) {
          return {
            ...projection,
            stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
            activeStation: "S3",
            materials: [unlockedMaterial],
          };
        }
        throw new Error("network blip");
      },
    };

    const snapshot = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "active", anchors: [anchor] },
    };
    const submitCard = vi.fn(async () => {});
    const conv = {
      getSnapshot: () => snapshot,
      subscribe: () => () => {},
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      submitCard,
      skipCard: vi.fn(),
      dropFirst: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    // This field only exists once StudioContainer's second effect (creating
    // the live conversation, gated on projectId) has committed — a separate,
    // later render than the one `findByText` above resolved on. Under
    // full-suite CPU contention that second commit can lag behind a plain
    // synchronous `getByPlaceholderText`, so use the awaited `find*` query
    // instead of assuming it already landed.
    fireEvent.change(await screen.findByPlaceholderText(/这条来源在你的论证里起什么作用/), {
      target: { value: "触发关注的入口——需要横向核实。" },
    });
    fireEvent.click(screen.getByText("锁定，进下一条"));

    expect(submitCard).toHaveBeenCalled();
    // submitCard resolves → refetchProject's GET rejects → its catch clears
    // pendingAnchors and rethrows → the outer .catch sets syncError. None of
    // that chain is awaited anywhere in the production code (fire-and-
    // forget from the click handler), so flush it deterministically rather
    // than polling with waitFor's real-time budget — see `flush`'s comment.
    await flush();
    expect(getProjectCalls).toBe(2);

    // The refetch that would confirm the lock failed — the dossier must
    // still show the pre-refetch (unlocked) state, not a falsely-locked one.
    expect(screen.getByText("待评估")).toBeInTheDocument();
    // The failure must be surfaced to the student, not swallowed.
    expect(screen.getByText(/同步|请刷新/)).toBeInTheDocument();
  });

  it("does not report a post-add refetch failure as an add failure, and still resets the form (bug D)", async () => {
    const newMaterial = {
      id: "m2", title: "《IPCC AR6 综合报告》", sourceUrl: "https://ipcc.ch/report", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "……" }],
      locked: false, role: "", tier: "机构报告", takeaway: "报告本身的口径。", anchors: [],
    };
    const baseProjection = {
      ...projection,
      stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
      activeStation: "S3",
      materials: [] as unknown[],
    };
    let getProjectCalls = 0;
    const addMaterial = vi.fn(async () => newMaterial);
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        if (getProjectCalls === 1) return baseProjection;
        throw new Error("refetch blew up");
      },
      addMaterial,
      logSourceOpen: vi.fn(async () => {}),
    };

    render(<StudioContainer api={api as never} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    fireEvent.click(screen.getByText("添加信源"));
    fireEvent.change(screen.getByPlaceholderText(/粘贴链接/), { target: { value: "https://ipcc.ch/report" } });
    fireEvent.change(screen.getByPlaceholderText(/一句话说说/), { target: { value: "报告本身的口径。" } });
    fireEvent.click(screen.getByLabelText("机构报告"));
    fireEvent.click(screen.getByText("加入信源档案"));

    // onAddSource awaits addMaterial, then internally awaits+catches
    // refetchProject's rejection (setting syncError, not rethrowing) —
    // AddSourceForm's own handleSubmit awaits that whole chain before
    // reset()/setSubmitting(false). Same unawaited-from-fireEvent's-
    // perspective shape as bug C — flush deterministically instead of
    // racing waitFor's real-time budget against however deep this chain is.
    await flush();
    expect(addMaterial).toHaveBeenCalled();
    expect(getProjectCalls).toBe(2);

    // The add itself succeeded — must NOT show the generic add-failure copy.
    expect(screen.queryByText("添加信源失败，请重试")).toBeNull();
    // The form must reset (collapse back to the "添加信源" button) rather
    // than stay primed to re-submit — a second submit here would duplicate
    // the source the student already successfully added.
    expect(screen.getByText("添加信源")).toBeInTheDocument();
  });

  it("does not duplicate the coach thread when a refetch's projection already contains this session's live turns", async () => {
    const studentTurn = { kind: "student", body: "它想证明中国是认真在转型的。" };
    const aiTurn = { kind: "ai", body: "连到治理决心", tag: "D5", anchor: "论证图 · 治理决心主张" };

    // A minimal stateful fake mirroring the real conversation controller's
    // shape closely enough that dropFirst() actually mutates what
    // getSnapshot returns (a static fixture can't exercise the reconciliation).
    let convState = { messages: [studentTurn, aiTurn], sending: false, error: null, disposableInterventionId: null, card: null };
    const listeners = new Set<() => void>();
    const conv = {
      getSnapshot: () => convState,
      subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      submitCard: vi.fn(async () => {}),
      skipCard: vi.fn(async () => {}),
      dropFirst: vi.fn((n: number) => {
        convState = { ...convState, messages: convState.messages.slice(n) };
        listeners.forEach((l) => l());
      }),
    };

    const newMaterial = {
      id: "m2", title: "《IPCC AR6 综合报告》", sourceUrl: "https://ipcc.ch/report", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "……" }],
      locked: false, role: "", tier: "机构报告", takeaway: "报告本身的口径。", anchors: [],
    };
    const baseProjection = {
      ...projection,
      stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
      activeStation: "S3",
      materials: [] as unknown[],
    };
    // The refetched projection already carries THIS session's two live
    // turns as persisted history — the server-side rebuild projectCoach
    // does exactly this once a chat_message/intervention row exists.
    const projectionWithPersistedTurns = {
      ...baseProjection,
      coach: { ...baseProjection.coach, messages: [studentTurn, aiTurn] },
      materials: [newMaterial],
    };
    let getProjectCalls = 0;
    const addMaterial = vi.fn(async () => newMaterial);
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        return getProjectCalls === 1 ? baseProjection : projectionWithPersistedTurns;
      },
      addMaterial,
      logSourceOpen: vi.fn(async () => {}),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();
    // Pre-refetch: the live turn renders exactly once, straight from the
    // conversation controller.
    expect(screen.getAllByText(aiTurn.body)).toHaveLength(1);

    fireEvent.click(screen.getByText("添加信源"));
    fireEvent.change(screen.getByPlaceholderText(/粘贴链接/), { target: { value: "https://ipcc.ch/report" } });
    fireEvent.change(screen.getByPlaceholderText(/一句话说说/), { target: { value: "报告本身的口径。" } });
    fireEvent.click(screen.getByLabelText("机构报告"));
    fireEvent.click(screen.getByText("加入信源档案"));

    await waitFor(() => expect(getProjectCalls).toBe(2));
    // The bug: CoachRail is keyed by array index with no dedupe, so without
    // reconciliation the turn now renders twice — once from the refetched
    // projection's history, once still sitting in the conversation buffer.
    await waitFor(() => expect(screen.getAllByText(aiTurn.body)).toHaveLength(1));
    expect(screen.getAllByText(studentTurn.body)).toHaveLength(1);
  });

  // --- Fix-wave-3 findings [1]+[2]: overlapping refetches need a
  // request-generation guard, or (a) an older, later-resolving response can
  // stomp a newer one's state, and (b) its dropFirst(n) applies n — captured
  // against a buffer a DIFFERENT refetch may since have shifted — to
  // whatever buffer happens to exist when it lands. ---

  it("discards a stale refetch when a newer one has since been issued — the newer projection wins even if it resolves FIRST (bug 1+2)", async () => {
    const materialId = "m1";
    const unlockedMaterial = {
      id: materialId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [],
    };
    const lockedMaterial = { ...unlockedMaterial, locked: true, role: "触发关注的入口——需要横向核实。", anchors: [] };
    const newMaterial = {
      id: "m2", title: "《IPCC AR6 综合报告》", sourceUrl: "https://ipcc.ch/report", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "……" }],
      locked: false, role: "", tier: "机构报告", takeaway: "报告本身的口径。", anchors: [],
    };
    const baseProjection = {
      ...projection,
      stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
      activeStation: "S3",
    };

    let getProjectCalls = 0;
    const resolvers: Array<(v: unknown) => void> = [];
    const addMaterial = vi.fn(async () => newMaterial);
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        if (getProjectCalls === 1) return { ...baseProjection, materials: [unlockedMaterial] };
        // Both the add-source refetch (GET_A, gen 1) and the card-lock
        // refetch (GET_B, gen 2) are held open here, resolved by the test in
        // a chosen order below.
        return new Promise((resolve) => { resolvers.push(resolve); });
      },
      addMaterial,
      logSourceOpen: vi.fn(async () => {}),
    };

    // FIX 2 (whole-branch review CRITICAL): a zero-anchor card can no longer
    // be locked (StudioAnnotateCard.tsx's `hasAnchors` guard) — this test is
    // about the overlapping-refetch race, not card completion, so it needs
    // one real (non-vacuous) anchor to legitimately reach the lock button.
    const craapAnchor = {
      id: "a0", material_id: materialId, block_id: "b1", start: 0, end: 4,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "",
    };
    const snapshot = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "active", anchors: [craapAnchor] },
    };
    const conv = {
      getSnapshot: () => snapshot,
      subscribe: () => () => {},
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      submitCard: vi.fn(async () => {}),
      skipCard: vi.fn(),
      dropFirst: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();
    expect(screen.getByText("待评估")).toBeInTheDocument();

    // GET_A: issued by adding a source.
    fireEvent.click(screen.getByText("添加信源"));
    fireEvent.change(screen.getByPlaceholderText(/粘贴链接/), { target: { value: "https://ipcc.ch/report" } });
    fireEvent.change(screen.getByPlaceholderText(/一句话说说/), { target: { value: "报告本身的口径。" } });
    fireEvent.click(screen.getByLabelText("机构报告"));
    fireEvent.click(screen.getByText("加入信源档案"));
    await waitFor(() => expect(getProjectCalls).toBe(2));

    // GET_B: issued by locking the card, WHILE GET_A is still in flight.
    // This field only exists once StudioContainer's second effect (creating
    // the live conversation, gated on projectId) has committed — a separate,
    // later render than the one `findByText` above resolved on. Under
    // full-suite CPU contention that second commit can lag behind a plain
    // synchronous `getByPlaceholderText`, so use the awaited `find*` query
    // instead of assuming it already landed.
    // Fill the one real anchor (FIX 2 needs at least one) — it is the only
    // textbox on the page with NO placeholder (every AddSourceForm field and
    // the card's own risk-note field all carry one), so filter for that
    // instead of assuming a document-order index the add-source form above
    // also contributes textboxes to.
    await screen.findAllByRole("textbox");
    const anchorBox = screen.getAllByRole("textbox").find((el) => !el.getAttribute("placeholder"));
    if (!anchorBox) throw new Error("expected the card's own (placeholder-less) anchor textbox to be present");
    fireEvent.change(anchorBox, { target: { value: "NASA地球观测团队发布" } });
    fireEvent.change(await screen.findByPlaceholderText(/这条来源在你的论证里起什么作用/), {
      target: { value: "触发关注的入口——需要横向核实。" },
    });
    fireEvent.click(screen.getByText("锁定，进下一条"));
    await waitFor(() => expect(getProjectCalls).toBe(3));

    // Resolve the NEWER one (GET_B) FIRST, with the locked projection. (Only
    // m1 here — newMaterial is unlocked by construction and would add its
    // own 待评估 badge, muddying the assertion below that no 待评估 badge
    // remains; the add-source refetch path itself is covered separately.)
    resolvers[1]!({ ...baseProjection, materials: [lockedMaterial] });
    await waitFor(() => expect(screen.getByText("✓ 已锁定")).toBeInTheDocument());

    // Now resolve the OLDER one (GET_A) — a stale snapshot from before the
    // lock (or the add) reached the server. It must be discarded outright,
    // not allowed to flip the just-applied lock back to 待评估.
    resolvers[0]!({ ...baseProjection, materials: [unlockedMaterial] });
    await Promise.resolve();
    await Promise.resolve();
    expect(screen.getByText("✓ 已锁定")).toBeInTheDocument();
    expect(screen.queryByText("待评估")).toBeNull();
  });

  it("keeps a turn sent between two overlapping refetches' resolutions on screen exactly once (bug 1+2)", async () => {
    const midFlightTurn = { kind: "student", body: "它想证明中国是认真在转型的。" };
    // FIX 2 (whole-branch review CRITICAL): a zero-anchor card can no longer
    // be locked (StudioAnnotateCard.tsx's `hasAnchors` guard) — this test is
    // about the overlapping-refetch race, not card completion, so it needs
    // one real (non-vacuous) anchor to legitimately reach the lock button.
    const craapAnchor = {
      id: "a0", material_id: "m1", block_id: "b1", start: 0, end: 4,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "",
    };
    let convState: any = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "active", anchors: [craapAnchor] },
    };
    const listeners = new Set<() => void>();
    const emit = () => listeners.forEach((l) => l());
    const conv = {
      getSnapshot: () => convState,
      subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      // Mirrors the real controller: card is nulled as soon as submit's own
      // "done" frame lands.
      submitCard: vi.fn(async () => {
        convState = { ...convState, card: null };
        emit();
      }),
      skipCard: vi.fn(async () => {}),
      dropFirst: vi.fn((n: number) => {
        convState = { ...convState, messages: convState.messages.slice(n) };
        emit();
      }),
    };

    const newMaterial = {
      id: "m2", title: "《IPCC AR6 综合报告》", sourceUrl: "https://ipcc.ch/report", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "……" }],
      locked: false, role: "", tier: "机构报告", takeaway: "报告本身的口径。", anchors: [],
    };
    const baseProjection = {
      ...projection,
      stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
      activeStation: "S3",
      materials: [] as unknown[],
    };

    let getProjectCalls = 0;
    const resolvers: Array<(v: unknown) => void> = [];
    const addMaterial = vi.fn(async () => newMaterial);
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        if (getProjectCalls === 1) return baseProjection;
        return new Promise((resolve) => { resolvers.push(resolve); });
      },
      addMaterial,
      logSourceOpen: vi.fn(async () => {}),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    // GET_A: add-source.
    fireEvent.click(screen.getByText("添加信源"));
    fireEvent.change(screen.getByPlaceholderText(/粘贴链接/), { target: { value: "https://ipcc.ch/report" } });
    fireEvent.change(screen.getByPlaceholderText(/一句话说说/), { target: { value: "报告本身的口径。" } });
    fireEvent.click(screen.getByLabelText("机构报告"));
    fireEvent.click(screen.getByText("加入信源档案"));
    await waitFor(() => expect(getProjectCalls).toBe(2));

    // GET_B: card lock, issued while GET_A is still in flight. Both are
    // captured with priorMessageCount === 0 (no turns sent yet).
    // This field only exists once StudioContainer's second effect (creating
    // the live conversation, gated on projectId) has committed — a separate,
    // later render than the one `findByText` above resolved on. Under
    // full-suite CPU contention that second commit can lag behind a plain
    // synchronous `getByPlaceholderText`, so use the awaited `find*` query
    // instead of assuming it already landed.
    // Fill the one real anchor (FIX 2 needs at least one) — it is the only
    // textbox on the page with NO placeholder (every AddSourceForm field and
    // the card's own risk-note field all carry one), so filter for that
    // instead of assuming a document-order index the add-source form above
    // also contributes textboxes to.
    await screen.findAllByRole("textbox");
    const anchorBox = screen.getAllByRole("textbox").find((el) => !el.getAttribute("placeholder"));
    if (!anchorBox) throw new Error("expected the card's own (placeholder-less) anchor textbox to be present");
    fireEvent.change(anchorBox, { target: { value: "NASA地球观测团队发布" } });
    fireEvent.change(await screen.findByPlaceholderText(/这条来源在你的论证里起什么作用/), {
      target: { value: "触发关注的入口——需要横向核实。" },
    });
    fireEvent.click(screen.getByText("锁定，进下一条"));
    await waitFor(() => expect(getProjectCalls).toBe(3));

    // Resolve the OLDER one (GET_A) first — it must be discarded outright:
    // no dropFirst call at all (a stale dropFirst is exactly what deletes a
    // turn that later arrives on top of a buffer it was never measured
    // against).
    resolvers[0]!({ ...baseProjection, materials: [] });
    await Promise.resolve();
    await Promise.resolve();
    expect(conv.dropFirst).not.toHaveBeenCalled();

    // A live turn enters the buffer WHILE the newer refetch (GET_B) is still
    // in flight.
    act(() => {
      convState = { ...convState, messages: [midFlightTurn] };
      emit();
    });
    await waitFor(() => expect(screen.getAllByText(midFlightTurn.body).length).toBeGreaterThan(0));

    // Now let GET_B resolve — its own projection was fetched before the
    // turn was sent, so it does not carry the turn in coach.messages.
    resolvers[1]!({ ...baseProjection, materials: [newMaterial] });
    await waitFor(() =>
      expect(within(screen.getByTestId("dossier-source-list")).getByText(newMaterial.title)).toBeInTheDocument(),
    );

    // dropFirst must have been applied exactly once — for GET_B, keyed to
    // the buffer as it stood when GET_B was issued (0) — never for the
    // discarded GET_A.
    expect(conv.dropFirst).toHaveBeenCalledTimes(1);
    expect(conv.dropFirst).toHaveBeenCalledWith(0);
    expect(screen.getAllByText(midFlightTurn.body)).toHaveLength(1);
  });

  // --- Fix-wave-3 finding [3]: a skip that never reached the server (the
  // skip API call itself failed) must not be reported with the
  // refetch-failure copy — the card is still open, nothing is "out of
  // sync". ---

  it("reports a genuinely failed skip as a skip failure, not a sync problem (bug 3)", async () => {
    const materialId = "m1";
    const anchor = {
      id: "a1", material_id: materialId, block_id: "b1", start: 0, end: 4,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "只是一个博主",
    };
    const unlockedMaterial = {
      id: materialId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [],
    };

    let getProjectCalls = 0;
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        return {
          ...projection,
          stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
          activeStation: "S3",
          materials: [unlockedMaterial],
        };
      },
    };

    const snapshot = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "active", anchors: [anchor] },
    };
    // Mirrors the real conversation controller: skipCard rejects when the
    // skip API call itself fails (it has no internal try/catch, unlike
    // submitCard).
    const skipCard = vi.fn(async () => { throw new Error("skip endpoint 500"); });
    const conv = {
      getSnapshot: () => snapshot,
      subscribe: () => () => {},
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      submitCard: vi.fn(),
      skipCard,
      dropFirst: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    // The skip button only exists once StudioContainer's second effect
    // (creating the live conversation, gated on projectId) has committed —
    // a separate, later render than the one `findByText` above resolved on.
    // Wait for it explicitly rather than assuming it already landed.
    fireEvent.click(await screen.findByText("跳过这张卡"));

    expect(skipCard).toHaveBeenCalled();
    await waitFor(() => expect(screen.getByText(/跳过失败/)).toBeInTheDocument());
    // The skip never reached the point of needing a refetch at all.
    expect(getProjectCalls).toBe(1);
    // And it must not be misreported with the (wrong) sync-failure copy.
    expect(screen.queryByText(/同步|请刷新/)).toBeNull();
  });

  // --- Fix-wave-3 finding [4]: pendingAnchors is only ever cleared on
  // refetchProject's success path — a refetch failure after a submit leaves
  // it set for the rest of the session, permanently shadowing whatever the
  // (eventually correct) persisted anchors would show. ---

  it("clears pendingAnchors after a failed post-submit refetch — it must not shadow persisted anchors forever (bug 4)", async () => {
    const materialId = "m1";
    const anchor = {
      id: "a1", material_id: materialId, block_id: "b1", start: 0, end: 4,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "只是一个博主",
    };
    const unlockedMaterial = {
      id: materialId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [],
    };

    let getProjectCalls = 0;
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        if (getProjectCalls === 1) {
          return {
            ...projection,
            stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
            activeStation: "S3",
            materials: [unlockedMaterial],
          };
        }
        throw new Error("network blip");
      },
      logSourceOpen: vi.fn(async () => {}),
    };

    let convState: any = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "active", anchors: [anchor] },
    };
    const listeners = new Set<() => void>();
    const emit = () => listeners.forEach((l) => l());
    const conv = {
      getSnapshot: () => convState,
      subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(),
      // Mirrors the real controller: card is nulled as soon as submit's own
      // "done" frame lands, well before the refetch below settles.
      submitCard: vi.fn(async () => {
        convState = { ...convState, card: null };
        emit();
      }),
      skipCard: vi.fn(),
      dropFirst: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // `信源档案` comes from StudioContainer's FIRST effect (loading the
    // project). The live conversation (conv/card, and anything sourced from
    // it — the coach thread, the card's fields, its anchor highlights) only
    // exists once the SECOND effect (gated on projectId, creating the
    // conversation) has ALSO committed — a separate, later render. Under
    // full-suite CPU contention that second commit can lag behind the first
    // enough that a synchronous assertion right after `findByText` above
    // sees stale (pre-conv) DOM — this is this file's actual shared flake
    // cause, not a product race (the outcome never depends on interleaving,
    // only how long it takes to observe). `flush` waits out both effects
    // deterministically, with no timeout of its own to lose a race against.
    await flush();

    fireEvent.click(within(screen.getByTestId("dossier-source-list")).getByText(unlockedMaterial.title));
    // The card's live anchor highlights the span before any submit happens.
    // This depends on convSnapshot.card, which only exists once
    // StudioContainer's second effect (creating the live conversation) has
    // committed — a separate, later render than the one `findByText` above
    // resolved on. Wait for it explicitly rather than assuming it already
    // landed (see the `findByPlaceholderText` note below for the same race).
    await waitFor(() => expect(document.querySelectorAll("mark").length).toBe(1));

    // This field only exists once StudioContainer's second effect (creating
    // the live conversation, gated on projectId) has committed — a separate,
    // later render than the one `findByText` above resolved on. Under
    // full-suite CPU contention that second commit can lag behind a plain
    // synchronous `getByPlaceholderText`, so use the awaited `find*` query
    // instead of assuming it already landed.
    fireEvent.change(await screen.findByPlaceholderText(/这条来源在你的论证里起什么作用/), {
      target: { value: "触发关注的入口——需要横向核实。" },
    });
    fireEvent.click(screen.getByText("锁定，进下一条"));

    // Same unawaited submitCard→refetchProject→catch chain as bug C —
    // flush deterministically instead of racing waitFor's real-time budget.
    await flush();
    expect(getProjectCalls).toBe(2);
    // The refetch failed — confirms the failure path actually ran.
    expect(screen.getByText(/同步|请刷新/)).toBeInTheDocument();

    // The card is gone (submitCard nulled it) and the failed refetch never
    // delivered persisted anchors (the material fetched at load time has
    // none) — pendingAnchors must be cleared here too, or this stale,
    // pre-submission overlay would go on masking the (empty) truth forever.
    expect(document.querySelectorAll("mark").length).toBe(0);
  });

  // --- Task 11 (SIFT): the compare card is the first primitive whose
  // anchors span TWO materials. It has NO refetch path of its own — it must
  // ride the SAME onSubmitCard → refetchProject wiring every other card
  // already uses (see StudioContainer.tsx's onSubmitCard). This is the test
  // Slice 6b paid for: locking a card must be visible ON SCREEN, not just a
  // network call that fired. ---

  it("clears the dossier's 需横向阅读 chip after a SIFT (compare) card submit resolves — no new refetch path, the existing one (Task 11)", async () => {
    const blogId = "m1";
    const nasaId = "m2";
    const blog = {
      id: blogId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      // locked: true — CRAAP already finished on this source (Task 11's fix:
      // the chip is derived from `locked && !lateralRead`, a fact about the
      // SOURCE, not from whether a card happens to be open on it).
      locked: true, role: "", tier: "", takeaway: "", anchors: [], lateralRead: false, isLateralInstrument: false, siftSkipped: false,
    };
    const nasa = {
      id: nasaId, title: "Chen et al. (2019), Nature Sustainability", sourceUrl: "https://doi.org/x",
      kind: "paper", origin: "fetched", blocks: [{ id: "b1", text: "……" }],
      // lateralRead: true so this row never carries the chip itself — this
      // test is only about the blog source's chip transition.
      locked: true, role: "", tier: "", takeaway: "", anchors: [], lateralRead: true, isLateralInstrument: false, siftSkipped: false,
    };
    // What the server would honestly persist once the cross_check mints:
    // lateralRead flips true. (The chip's disappearance below doesn't
    // actually depend on this value — see the comment at the final
    // assertion — but a fixture with lateralRead still false would be
    // dishonest about what the real endpoint does.)
    const blogAfterSift = { ...blog, lateralRead: true, locked: true, role: "触发关注的入口——已横向核实。" };

    let getProjectCalls = 0;
    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => {
        getProjectCalls += 1;
        const materials = getProjectCalls === 1 ? [blog, nasa] : [blogAfterSift, nasa];
        return {
          ...projection,
          stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
          activeStation: "S3",
          materials,
        };
      },
    };

    const siftSpec = CARD_REGISTRY["sift"]!;
    let convState: any = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      // Proposed, not yet active: ViewFrame only swaps the 素材 pane over to
      // Compare once a compare card is ACTIVE (Task 11's own trap #1 — see
      // ViewFrame.test.tsx) — while it's merely proposed (a bubble in the
      // rail awaiting the student's own "打开" click, per the product's
      // no-forced-open rule), the dossier list is still what's on screen.
      // This anchor is otherwise inert for the chip itself — Task 11's fix
      // derives the chip from `blog.locked && !blog.lateralRead`, not from
      // any live card's presence.
      card: {
        cardInstanceId: "ci1", cardId: "sift", spec: siftSpec, status: "proposed",
        anchors: [{ id: "a-stop", material_id: blogId, block_id: "", start: 0, end: 0, quote: "", dimension: "stop", author: "ai", question: "", answer: "" }],
        // The card's own (checked) material — FIX-A's
        // card_instance--evaluates-->material edge, carried end to end.
        // Without it StudioCompareCard can no longer fall back to guessing
        // materials[0] (whole-branch review finding [5]), so this fixture
        // must supply the real field to stay a real payload shape.
        materialId: blogId,
      },
    };
    const listeners = new Set<() => void>();
    const emit = () => listeners.forEach((l) => l());
    const conv = {
      getSnapshot: () => convState,
      subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(() => {
        convState = { ...convState, card: { ...convState.card, status: "active" } };
        emit();
      }),
      // Mirrors the real controller: submit clears the live card client-side
      // the moment its own "done" frame lands — well before any refetch
      // settles (fix-wave bug [B]/[4], already fixed in StudioContainer via
      // pendingAnchors — this test is what exercises that path for `compare`).
      submitCard: vi.fn(async () => {
        convState = { ...convState, card: null };
        emit();
      }),
      skipCard: vi.fn(),
      dropFirst: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    // See the flush() doc comment above — the live conversation only exists
    // once StudioContainer's second effect has also committed.
    await flush();

    // GET1: the dossier shows the chip on the blog source because it's
    // locked (CRAAP already evaluated it) and lateralRead is still false —
    // this holds independent of the SIFT card merely being proposed here.
    expect(within(screen.getByTestId("dossier-source-list")).getByText(/需横向阅读/)).toBeInTheDocument();

    // Open the card (coach rail now renders the interactive SIFT form) and
    // complete it exactly the way a student would — same script as
    // StudioCompareCard.test.tsx's own submit test, which already proves
    // this form builds its anchors off the DECLARED lateral_dimension, not
    // array position (Task 11's trap #2).
    fireEvent.click(await screen.findByRole("button", { name: /打开|开始/ }));

    fireEvent.change(await screen.findByPlaceholderText(/先写下来/), { target: { value: "第一反应：有点意外" } });
    fireEvent.change(screen.getByPlaceholderText(/机构、个人/), { target: { value: "自媒体博主，无机构背景" } });
    fireEvent.click(within(screen.getByTestId("lateral-material-picker")).getByRole("button", { name: nasa.title }));
    fireEvent.change(screen.getByPlaceholderText(/怎么说同一件事/), { target: { value: "NASA 数据显示排放仍在上升" } });
    fireEvent.click(screen.getByRole("button", { name: "印证" }));
    fireEvent.change(screen.getByPlaceholderText(/原始的出处是哪里/), { target: { value: "Nature Sustainability 论文" } });
    fireEvent.click(screen.getByRole("button", { name: "原始证据" }));
    fireEvent.change(screen.getByPlaceholderText(/和一开始比/), { target: { value: "从二手转述降级为需要追源的说法" } });
    fireEvent.click(screen.getByRole("button", { name: "锁定这张卡" }));

    expect(conv.submitCard).toHaveBeenCalled();
    // The stale-until-reload bug this test exists to catch: without a
    // refetch after submit, getProject is never called a second time.
    await waitFor(() => expect(getProjectCalls).toBe(2));

    // GET2 (the post-submit refetch): the chip must be GONE from the
    // dossier — not just "the network call fired", but actually off screen.
    // (Mechanically this holds even before considering blogAfterSift's own
    // lateralRead value: submit already cleared the live card, and the
    // pending-anchors overlay that was standing in for it during the round
    // trip is cleared only once THIS refetch resolves — so without the
    // refetch, that overlay — and the chip it drives — would never clear.)
    await waitFor(() =>
      expect(within(screen.getByTestId("dossier-source-list")).queryByText(/需横向阅读/)).not.toBeInTheDocument(),
    );
  });

  // --- FIX-C finding [2]: `lateralMaterialId` is LIFTED to StudioContainer
  // and reset by an effect keyed on the active card instance id (see that
  // effect, just above `activeCardInstanceId`, in StudioContainer.tsx).
  // Nothing exercised it: a FIX-B reviewer gutted the effect into a no-op
  // and all other tests in this file still passed green — proof the reset
  // was previously unverified. Without it, the lateral source she picked
  // for one SIFT card would silently bleed into the NEXT SIFT card (a
  // different instance, on a different material) — pre-filling a source
  // she never chose for THIS card, and risking a cross_check that cites the
  // wrong independent source. ---

  it("clears the student's lateral-source pick when a NEW SIFT card instance becomes active (fix-C finding [2])", async () => {
    const blogId = "m1";
    const nasaId = "m2";
    const article2Id = "m3";
    const blog = {
      id: blogId, title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [], lateralRead: false, isLateralInstrument: false, siftSkipped: false,
    };
    const nasa = {
      id: nasaId, title: "Chen et al. (2019), Nature Sustainability", sourceUrl: "https://doi.org/x",
      kind: "paper", origin: "fetched", blocks: [{ id: "b1", text: "……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [], lateralRead: false, isLateralInstrument: false, siftSkipped: false,
    };
    const article2 = {
      id: article2Id, title: "《全球气候观察》专栏", sourceUrl: "https://x.test/c", kind: "article",
      origin: "fetched", blocks: [{ id: "b1", text: "另一段完全独立的材料……" }],
      locked: false, role: "", tier: "", takeaway: "", anchors: [], lateralRead: false, isLateralInstrument: false, siftSkipped: false,
    };

    const api = {
      listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
      getProject: async () => ({
        ...projection,
        stations: [...projection.stations, { code: "S3", name: "信源评估", view: "素材", state: "current" }],
        activeStation: "S3",
        materials: [blog, nasa, article2],
      }),
    };

    const siftSpec = CARD_REGISTRY["sift"]!;
    let convState: any = {
      messages: [], sending: false, error: null, disposableInterventionId: null,
      // Card A starts ACTIVE directly (the proposed→open click is already
      // covered by the Task 11 test above) — what matters here is her
      // lateral pick on THIS instance.
      card: { cardInstanceId: "ci1", cardId: "sift", spec: siftSpec, status: "active", anchors: [], materialId: blogId },
    };
    const listeners = new Set<() => void>();
    const emit = () => listeners.forEach((l) => l());
    const conv = {
      getSnapshot: () => convState,
      subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
      send: vi.fn(),
      dispose: vi.fn(),
      openCard: vi.fn(() => {
        convState = { ...convState, card: { ...convState.card, status: "active" } };
        emit();
      }),
      // Mirrors the real controller: submit clears the live card client-side
      // the moment its own "done" frame lands.
      submitCard: vi.fn(async () => {
        convState = { ...convState, card: null };
        emit();
      }),
      skipCard: vi.fn(),
      dropFirst: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    await flush();

    // Pick NASA as card A's lateral source — the exact interaction
    // StudioCompareCard's picker exposes (same script as the Task 11 test).
    fireEvent.click(within(screen.getByTestId("lateral-material-picker")).getByRole("button", { name: nasa.title }));
    // The center pane reads the SAME lifted pick live (finding [3]) — the
    // empty-state invitation must be gone once a real pick exists.
    await waitFor(() => expect(screen.queryByText("去找一个独立的来源")).not.toBeInTheDocument());

    // Complete and submit card A for real — a genuine card transition
    // driven through the UI, not a hand-rolled state poke.
    fireEvent.change(await screen.findByPlaceholderText(/先写下来/), { target: { value: "第一反应：有点意外" } });
    fireEvent.change(screen.getByPlaceholderText(/机构、个人/), { target: { value: "自媒体博主，无机构背景" } });
    fireEvent.change(screen.getByPlaceholderText(/怎么说同一件事/), { target: { value: "NASA 数据显示排放仍在上升" } });
    fireEvent.click(screen.getByRole("button", { name: "印证" }));
    fireEvent.change(screen.getByPlaceholderText(/原始的出处是哪里/), { target: { value: "Nature Sustainability 论文" } });
    fireEvent.click(screen.getByRole("button", { name: "原始证据" }));
    fireEvent.change(screen.getByPlaceholderText(/和一开始比/), { target: { value: "从二手转述降级为需要追源的说法" } });
    fireEvent.click(screen.getByRole("button", { name: "锁定这张卡" }));

    expect(conv.submitCard).toHaveBeenCalled();
    await flush();
    // Card A is gone — the dossier list (not Compare) is back on screen.
    await waitFor(() => expect(screen.getByTestId("dossier-source-list")).toBeInTheDocument());

    // A NEW SIFT card instance (ci2, a DIFFERENT material) is proposed, then
    // opened by the student — the exact "打开" interaction every proposed
    // card requires (no-forced-open).
    act(() => {
      convState = {
        ...convState,
        card: { cardInstanceId: "ci2", cardId: "sift", spec: siftSpec, status: "proposed", anchors: [], materialId: article2Id },
      };
      emit();
    });
    fireEvent.click(await screen.findByRole("button", { name: /打开|开始/ }));

    // The right pane for card B must be back to its empty invitation — not
    // pre-filled with card A's stale nasa pick. This is the RENDERED DOM
    // assertion the reset must actually drive, not a check on props.
    await waitFor(() => expect(screen.getByText("去找一个独立的来源")).toBeInTheDocument());
  });
});
