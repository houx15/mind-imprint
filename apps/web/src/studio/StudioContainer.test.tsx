import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
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
};

const fakeApi = {
  listProjects: async () => [{ id: "p1", title: "China sustainability", qualLabel: "0457 个人报告", activeStation: "S4" }],
  getProject: async () => projection,
} as any;

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
      clearMessages: vi.fn(),
    };

    render(<StudioContainer api={api as never} makeConversation={() => conv as any} />);
    await screen.findByText(/信源档案/);
    expect(screen.getByText("待评估")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText(/这条来源在你的论证里起什么作用/), {
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

  it("does not duplicate the coach thread when a refetch's projection already contains this session's live turns", async () => {
    const studentTurn = { kind: "student", body: "它想证明中国是认真在转型的。" };
    const aiTurn = { kind: "ai", body: "连到治理决心", tag: "D5", anchor: "论证图 · 治理决心主张" };

    // A minimal stateful fake mirroring the real conversation controller's
    // shape closely enough that clearMessages() actually mutates what
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
      clearMessages: vi.fn(() => {
        convState = { ...convState, messages: [] };
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
});
