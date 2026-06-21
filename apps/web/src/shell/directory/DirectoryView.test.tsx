import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DirectoryView } from "./DirectoryView";
import { createStore, makeMemoryStorage } from "../../store";

function freshStore() {
  let i = 0;
  return createStore({ storage: makeMemoryStorage(), genId: () => `id${i++}`, now: () => "2026-06-21T10:00:00.000Z" });
}

describe("DirectoryView", () => {
  it("creates a task + seed message and opens it", () => {
    const store = freshStore();
    const onOpenTask = vi.fn();
    render(<DirectoryView store={store} onOpenTask={onOpenTask} now={() => new Date("2026-06-21T12:00:00Z")} />);
    fireEvent.change(screen.getByPlaceholderText(/把你正在纠结的问题/), {
      target: { value: "中国是否让地球更可持续？ https://mp.weixin.qq.com/s/x" },
    });
    fireEvent.click(screen.getByRole("button", { name: /开始/ }));
    const tasks = store.listTasks();
    expect(tasks).toHaveLength(1);
    expect(tasks[0]!.seed).toBe("https://mp.weixin.qq.com/s/x");
    expect(store.listMessages(tasks[0]!.id)).toHaveLength(1);
    expect(onOpenTask).toHaveBeenCalledWith(tasks[0]!.id);
  });
  it("empty input does not create a task", () => {
    const store = freshStore();
    render(<DirectoryView store={store} onOpenTask={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /开始/ }));
    expect(store.listTasks()).toHaveLength(0);
  });
  it("renders an existing task card and opens it on click", () => {
    const store = freshStore();
    const t = store.createTask({ title: "已有任务", seed: null });
    const onOpenTask = vi.fn();
    render(<DirectoryView store={store} onOpenTask={onOpenTask} />);
    fireEvent.click(screen.getByText("已有任务"));
    expect(onOpenTask).toHaveBeenCalledWith(t.id);
  });
});
