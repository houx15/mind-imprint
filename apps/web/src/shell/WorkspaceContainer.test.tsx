import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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

// A chat mock that behaves like the real client (resolves a ChatResult), so the
// on-mount kickoff() can complete cleanly.
const okChat = () => vi.fn().mockResolvedValue({ text: "好的，我们开始吧。", stopReason: "stop" });

describe("WorkspaceContainer", () => {
  it("renders the workspace for the given task and routes back", () => {
    const { store, a } = storeWithTwoTasks();
    const onBack = vi.fn();
    render(<WorkspaceContainer store={store} taskId={a.id} onBack={onBack} chat={okChat()} config={{}} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    fireEvent.click(screen.getByText("返回所有任务"));
    expect(onBack).toHaveBeenCalledOnce();
  });
  it("swaps to a fresh task's content when taskId changes", () => {
    const { store, a, b } = storeWithTwoTasks();
    const { rerender } = render(<WorkspaceContainer store={store} taskId={a.id} onBack={vi.fn()} chat={okChat()} config={{}} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    rerender(<WorkspaceContainer store={store} taskId={b.id} onBack={vi.fn()} chat={okChat()} config={{}} />);
    expect(screen.getByText("你好来自 B")).toBeTruthy();
    expect(screen.queryByText("你好来自 A")).toBeNull();
  });
  it("kicks off the LLM for a task opened with an unanswered opening message", async () => {
    const { store, a } = storeWithTwoTasks();
    const chat = okChat();
    render(<WorkspaceContainer store={store} taskId={a.id} onBack={vi.fn()} chat={chat} config={{}} />);
    // The seeded opening message ("你好来自 A") had no reply — kickoff should call the model.
    await waitFor(() => expect(chat).toHaveBeenCalledOnce());
    await waitFor(() => expect(screen.getByText("好的，我们开始吧。")).toBeTruthy());
  });
});
