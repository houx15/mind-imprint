import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { createStore, makeMemoryStorage } from "../store";

// Mock the api module so WorkspaceContainer can hydrate without a live server.
vi.mock("../api", () => ({
  api: {
    getTask: vi.fn().mockResolvedValue({ task: { id: "t-a", title: "任务 A", seed: null, status: "active", created_at: "1", last_active_at: "1" }, messages: [], cards: [] }),
    getEvaluation: vi.fn().mockResolvedValue(null),
    runTurn: vi.fn(async function* () { yield { type: "done", messageId: "m1" }; }),
    activateCard: vi.fn(),
    submitCard: vi.fn(),
    skipCard: vi.fn(),
    runEvaluation: vi.fn(),
  },
}));

import { api } from "../api";

function makeStore() {
  return createStore({ storage: makeMemoryStorage() });
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("WorkspaceContainer", () => {
  it("renders the workspace for the given task and routes back", () => {
    const store = makeStore();
    const task = store.createTask({ title: "任务 A", seed: null });
    store.appendMessage({ task_id: task.id, role: "user", content: "你好来自 A" });
    const onBack = vi.fn();
    render(<WorkspaceContainer store={store} taskId={task.id} onBack={onBack} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    fireEvent.click(screen.getByText("返回所有任务"));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("swaps to a fresh task's content when taskId changes", () => {
    const store = makeStore();
    const a = store.createTask({ title: "任务 A", seed: null });
    const b = store.createTask({ title: "任务 B", seed: null });
    store.appendMessage({ task_id: a.id, role: "user", content: "你好来自 A" });
    store.appendMessage({ task_id: b.id, role: "user", content: "你好来自 B" });
    const { rerender } = render(<WorkspaceContainer store={store} taskId={a.id} onBack={vi.fn()} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    rerender(<WorkspaceContainer store={store} taskId={b.id} onBack={vi.fn()} />);
    expect(screen.getByText("你好来自 B")).toBeTruthy();
    expect(screen.queryByText("你好来自 A")).toBeNull();
  });

  it("renders without error for a minimal task", () => {
    const store = makeStore();
    const task = store.createTask({ title: "兼容任务", seed: null });
    expect(() =>
      render(<WorkspaceContainer store={store} taskId={task.id} onBack={vi.fn()} />)
    ).not.toThrow();
  });

  it("calls api.getTask and api.getEvaluation on mount", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "水合任务", seed: null });
    vi.mocked(api.getTask).mockResolvedValueOnce({
      task, messages: [], cards: [],
    });
    render(<WorkspaceContainer store={store} taskId={task.id} onBack={vi.fn()} />);
    // Give the async effect time to run
    await vi.waitFor(() => expect(api.getTask).toHaveBeenCalledWith(task.id));
    await vi.waitFor(() => expect(api.getEvaluation).toHaveBeenCalledWith(task.id));
  });
});
