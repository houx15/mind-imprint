import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { createStore, makeMemoryStorage } from "../store";

function storeWithTwoTasks() {
  const store = createStore({ storage: makeMemoryStorage() });
  const a = store.createTask({ title: "任务 A", seed: null });
  const b = store.createTask({ title: "任务 B", seed: null });
  store.appendMessage({ task_id: a.id, role: "user", content: "你好来自 A" });
  store.appendMessage({ task_id: b.id, role: "user", content: "你好来自 B" });
  return { store, a, b };
}

describe("WorkspaceContainer", () => {
  it("renders the workspace for the given task and routes back", () => {
    const { store, a } = storeWithTwoTasks();
    const onBack = vi.fn();
    render(<WorkspaceContainer store={store} taskId={a.id} onBack={onBack} chat={vi.fn()} config={{}} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    fireEvent.click(screen.getByText("返回所有任务"));
    expect(onBack).toHaveBeenCalledOnce();
  });
  it("swaps to a fresh task's content when taskId changes", () => {
    const { store, a, b } = storeWithTwoTasks();
    const { rerender } = render(<WorkspaceContainer store={store} taskId={a.id} onBack={vi.fn()} chat={vi.fn()} config={{}} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    rerender(<WorkspaceContainer store={store} taskId={b.id} onBack={vi.fn()} chat={vi.fn()} config={{}} />);
    expect(screen.getByText("你好来自 B")).toBeTruthy();
    expect(screen.queryByText("你好来自 A")).toBeNull();
  });
});
