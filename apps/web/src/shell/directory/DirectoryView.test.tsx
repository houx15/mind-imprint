import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { createStore } from "../../store/createStore";
import { DirectoryView } from "./DirectoryView";

vi.mock("../../api", () => ({
  api: {
    listTasks: vi.fn(async () => []),
    createTask: vi.fn(async (i: any) => ({ id: "t-new", title: i.title, seed: i.seed, status: "active", created_at: "1", last_active_at: "1" })),
  },
}));

describe("DirectoryView", () => {
  beforeEach(() => { vi.clearAllMocks(); });
  it("creates a task via the API and opens it with the opening text", async () => {
    const store = createStore({});
    const onOpenTask = vi.fn();
    render(<DirectoryView store={store} onOpenTask={onOpenTask} />);
    fireEvent.change(screen.getByPlaceholderText(/把你正在纠结的问题/), { target: { value: "中国是否让地球更可持续？https://x" } });
    fireEvent.click(screen.getByText("开始"));
    await waitFor(() => expect(onOpenTask).toHaveBeenCalledWith("t-new", "中国是否让地球更可持续？https://x"));
  });
  it("empty input does not call the API or open a task", async () => {
    const store = createStore({});
    const onOpenTask = vi.fn();
    const { api } = await import("../../api");
    render(<DirectoryView store={store} onOpenTask={onOpenTask} />);
    fireEvent.click(screen.getByText("开始"));
    await new Promise((r) => setTimeout(r, 50));
    expect(onOpenTask).not.toHaveBeenCalled();
    expect(api.createTask).not.toHaveBeenCalled();
  });
});
