import { render, screen, waitFor, fireEvent } from "@testing-library/react";
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
});
